// settings_annotation_test.go — pure-function tests for the v0.15 1D-a
// `settings.annotation` manifest surface (C-32, C-33; AC-52, AC-53, AC-55).
//
// Settings.Annotation does not exist in the tree these tests were written
// against. They read it through reflect rather than through a direct field
// reference on purpose: a direct reference is a compile error, and a compile
// error in this package takes down every other manifest test with it, which
// reads like a kill and is the opposite of one. Reflection keeps each failure
// a runtime assertion naming the missing field. Once C-32 lands the same
// assertions exercise the real struct.
//
// @spec spec-manifest
package manifest

import (
	"reflect"
	"strings"
	"testing"
)

// annotationManifest builds a manifest whose only variable is the settings
// body. Everything else is byte-identical across cases, so a case that keys
// on the wrong thing cannot pass by accident.
func annotationManifest(settingsBody string) string {
	return "system:\n  name: annotation-test\n" + settingsBody
}

// annotationField returns Settings.Annotation as a reflect.Value, failing at
// runtime when C-32's field is missing or is not a pointer. The pointer shape
// is what lets nil mean "no annotation key in the manifest".
func annotationField(t *testing.T, m *Manifest) reflect.Value {
	t.Helper()
	if m == nil {
		t.Fatal("ParseManifest returned a nil Manifest")
	}
	v := reflect.ValueOf(m.Settings).FieldByName("Annotation")
	if !v.IsValid() {
		t.Fatalf("manifest.Settings has no field Annotation: C-32 requires Settings.Annotation to record a declared settings.annotation block")
	}
	if v.Kind() != reflect.Pointer {
		t.Fatalf("Settings.Annotation has kind %s, expected a pointer so that nil distinguishes an absent block (C-32)", v.Kind())
	}
	return v
}

// annotationPermissive reads Permissive off a non-nil Settings.Annotation.
func annotationPermissive(t *testing.T, v reflect.Value) bool {
	t.Helper()
	if v.IsNil() {
		t.Fatal("Settings.Annotation is nil, cannot read Permissive")
	}
	p := v.Elem().FieldByName("Permissive")
	if !p.IsValid() {
		t.Fatalf("Settings.Annotation has no field Permissive: C-32 requires one sub-key, permissive")
	}
	if p.Kind() != reflect.Bool {
		t.Fatalf("Settings.Annotation.Permissive has kind %s, expected bool (C-32)", p.Kind())
	}
	return p.Bool()
}

// @ac AC-52
// Every declared form of the block parses and is distinguishable from an
// absent one. The bare `annotation:` form is the discriminating case:
// gopkg.in/yaml.v3 leaves the decoded pointer nil there while allocating it
// for `annotation: {}`, so declaredness read from the pointer alone fails
// that row and passes the rest.
func TestParseManifest_AnnotationBlock_DeclarednessSeparateFromValue(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		wantDeclared      bool
		wantPermissive    bool
		permissiveChecked bool
	}{
		{
			name:              "permissive true",
			settings:          "settings:\n  annotation:\n    permissive: true\n",
			wantDeclared:      true,
			wantPermissive:    true,
			permissiveChecked: true,
		},
		{
			name:              "permissive false",
			settings:          "settings:\n  annotation:\n    permissive: false\n",
			wantDeclared:      true,
			wantPermissive:    false,
			permissiveChecked: true,
		},
		{
			name:              "empty map form",
			settings:          "settings:\n  annotation: {}\n",
			wantDeclared:      true,
			wantPermissive:    false,
			permissiveChecked: true,
		},
		{
			name:              "bare key form",
			settings:          "settings:\n  annotation:\n",
			wantDeclared:      true,
			wantPermissive:    false,
			permissiveChecked: true,
		},
		{
			name:         "absent, settings block carries another key",
			settings:     "settings:\n  strict: false\n",
			wantDeclared: false,
		},
		{
			name:         "absent, no settings block at all",
			settings:     "",
			wantDeclared: false,
		},
	}

	for _, tc := range cases {
		t.Run("spec-manifest/AC-52 "+tc.name, func(t *testing.T) {
			m, err := ParseManifest(annotationManifest(tc.settings))
			if err != nil {
				t.Fatalf("expected a clean parse, got error: %v", err)
			}
			v := annotationField(t, m)
			declared := !v.IsNil()
			if declared != tc.wantDeclared {
				t.Fatalf("Settings.Annotation declared = %v, want %v", declared, tc.wantDeclared)
			}
			if !tc.permissiveChecked {
				return
			}
			if got := annotationPermissive(t, v); got != tc.wantPermissive {
				t.Errorf("Settings.Annotation.Permissive = %v, want %v", got, tc.wantPermissive)
			}
		})
	}
}

// @ac AC-53
// An unknown sub-key under settings.annotation gets the C-26 error shape:
// the offending key, the closest valid sub-key within Levenshtein 3, and the
// valid sub-keys at that level. This is the generic shape AC-55 requires
// `scope` NOT to receive, so the pair separates the special case from the
// general one.
func TestParseManifest_AnnotationUnknownSubKey_UsesGenericUnknownKeyShape(t *testing.T) {
	t.Run("spec-manifest/AC-53 typo'd sub-key names the key and suggests permissive", func(t *testing.T) {
		m, err := ParseManifest(annotationManifest("settings:\n  annotation:\n    permisive: true\n"))
		if err == nil {
			t.Fatal("expected an error for unknown settings.annotation sub-key 'permisive', got nil")
		}
		if m != nil {
			t.Errorf("expected a nil Manifest alongside the error, got %+v", m)
		}
		msg := err.Error()
		if !strings.Contains(msg, "permisive") {
			t.Errorf("expected the error to name the offending key 'permisive', got: %s", msg)
		}
		// One s away from the only valid sub-key, so the suggestion must fire.
		if !strings.Contains(msg, "permissive") {
			t.Errorf("expected the error to suggest 'permissive', got: %s", msg)
		}
	})

	t.Run("spec-manifest/AC-53 unrelated sub-key names the key and lists valid sub-keys", func(t *testing.T) {
		m, err := ParseManifest(annotationManifest("settings:\n  annotation:\n    bogus: 1\n"))
		if err == nil {
			t.Fatal("expected an error for unknown settings.annotation sub-key 'bogus', got nil")
		}
		if m != nil {
			t.Errorf("expected a nil Manifest alongside the error, got %+v", m)
		}
		msg := err.Error()
		if !strings.Contains(msg, "bogus") {
			t.Errorf("expected the error to name the offending key 'bogus', got: %s", msg)
		}
		// 'bogus' is far from 'permissive', so this occurrence is the valid
		// sub-key listing rather than a did-you-mean suggestion.
		if !strings.Contains(msg, "permissive") {
			t.Errorf("expected the error to list the valid sub-keys at that level, got: %s", msg)
		}
	})
}

// @ac AC-55
// settings.annotation.scope is rejected with the SSRB-104 section 7.7 staging
// message, not with the generic unknown-key error of C-26. Both values get
// the same message. Routing scope through the generic path fails here,
// because the AC-53 message carries none of the five substrings.
func TestParseManifest_AnnotationScope_RejectedWithStagingMessage(t *testing.T) {
	want := []string{
		"settings.annotation.scope",
		"SSRB-104",
		"not implemented in v0.15.0",
		"test-only",
		"remove the key",
	}

	messages := map[string]string{}
	for _, value := range []string{"test", "all"} {
		t.Run("spec-manifest/AC-55 scope "+value+" is rejected with the staging message", func(t *testing.T) {
			m, err := ParseManifest(annotationManifest("settings:\n  annotation:\n    scope: " + value + "\n"))
			if err == nil {
				t.Fatalf("expected an error for settings.annotation.scope: %s, got nil", value)
			}
			if m != nil {
				t.Errorf("expected a nil Manifest alongside the error, got %+v", m)
			}
			msg := err.Error()
			messages[value] = msg
			for _, sub := range want {
				if !strings.Contains(msg, sub) {
					t.Errorf("expected the staging message to contain %q, got: %s", sub, msg)
				}
			}
		})
	}

	t.Run("spec-manifest/AC-55 both values produce the same message", func(t *testing.T) {
		if messages["test"] == "" || messages["all"] == "" {
			t.Skip("both scope cases must produce a message before they can be compared")
		}
		if messages["test"] != messages["all"] {
			t.Errorf("expected identical messages for scope: test and scope: all\n  test: %s\n   all: %s",
				messages["test"], messages["all"])
		}
	})
}
