// check_json_exit_test.go -- CLI integration tests for v0.15 SP-SP-021:
// exit-code parity between `specter check` and `specter check --json`
// (spec-check 1.7.0, C-14, AC-28 through AC-33).
//
// These tests cross the CLI boundary on purpose. C-14 is a statement about
// a process exit code, and the checker package is pure: it returns
// diagnostics and cannot observe an exit status. Only a subprocess run can
// assert the criterion, so every case here goes through runCLISplit.
//
// runCLISplit is a local variant of runCLI. runCLI merges stdout and stderr,
// which is enough to assert an exit code but not enough to assert C-14's
// "stdout MUST carry that document alone". The coverage precedent
// (coverage_json_exit_test.go, spec-coverage AC-34) had to decode the first
// JSON value out of a merged stream and ignore whatever followed. AC-33
// requires the stronger claim, so the streams are kept apart here.
//
// @spec spec-check
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Subprocess harness
// ---------------------------------------------------------------------------

// runCLISplit re-invokes the test binary as the CLI in the given directory and
// returns stdout and stderr separately, plus the process exit code.
func runCLISplit(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SPECTER_TEST=1")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	_ = cmd.Run()
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return outBuf.String(), errBuf.String(), code
}

// ---------------------------------------------------------------------------
// Fixture builders
//
// The builders take every field the criteria vary as an independent
// parameter: spec id, status, tier, the constraint set (including whether a
// constraint carries an `enforcement` override), and the acceptance criteria
// with their own ids, references, and priorities. Nothing is derived from
// anything else, so a case that keys on the wrong field cannot pass by
// accident.
//
// `enforcement` matters to these fixtures. It overrides the tier default for
// any diagnostic about that constraint, so an orphan constraint that is
// supposed to route by tier (AC-29 row_orphan, AC-30, AC-32) must leave it
// unset.
// ---------------------------------------------------------------------------

type constraintFixture struct {
	id          string
	desc        string
	ctype       string
	enforcement string // empty means "no override, route by tier"
}

type acFixture struct {
	id       string
	desc     string
	refs     []string
	priority string
}

func checkParitySpecYAML(id, status string, tier int, constraints []constraintFixture, acs []acFixture) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec:\n  id: %s\n  version: \"1.0.0\"\n  status: %s\n  tier: %d\n\n", id, status, tier)
	fmt.Fprintf(&b, "  context:\n    system: %s control system\n    feature: %s exit-code parity fixture\n\n", id, id)
	fmt.Fprintf(&b, "  objective:\n    summary: Fixture workspace for spec-check C-14 exit-code parity.\n\n")

	b.WriteString("  constraints:\n")
	for _, c := range constraints {
		fmt.Fprintf(&b, "    - id: %s\n      description: \"%s\"\n      type: %s\n", c.id, c.desc, c.ctype)
		if c.enforcement != "" {
			fmt.Fprintf(&b, "      enforcement: %s\n", c.enforcement)
		}
	}

	b.WriteString("\n  acceptance_criteria:\n")
	for _, ac := range acs {
		quoted := make([]string, 0, len(ac.refs))
		for _, r := range ac.refs {
			quoted = append(quoted, "\""+r+"\"")
		}
		fmt.Fprintf(&b, "    - id: %s\n      description: \"%s\"\n      references_constraints: [%s]\n      priority: %s\n",
			ac.id, ac.desc, strings.Join(quoted, ", "), ac.priority)
	}
	return b.String()
}

// writeCheckParityWorkspace writes one spec into dir/specs/ and returns dir.
func writeCheckParityWorkspace(t *testing.T, specID, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, specID+".spec.yaml", yaml)
	return dir
}

// tier2OrphanWorkspace builds the workspace AC-30 and AC-31 share. The spec
// mandates that AC-31 runs `--strict` over "the tier 2 orphan of AC-30", so
// the two criteria must see byte-identical input; only the flags differ.
func tier2OrphanWorkspace(t *testing.T) string {
	t.Helper()
	yaml := checkParitySpecYAML("notify-dispatch", "approved", 2,
		[]constraintFixture{
			{id: "C-01", desc: "MUST deliver each notification once", ctype: "technical", enforcement: "error"},
			{id: "C-02", desc: "MUST record a delivery receipt", ctype: "business", enforcement: "error"},
			// C-03 is the orphan. No enforcement override, so tier 2 routes
			// it to warning.
			{id: "C-03", desc: "MUST retry a failed delivery three times", ctype: "technical"},
		},
		[]acFixture{
			{id: "AC-01", desc: "A queued notification is delivered exactly once", refs: []string{"C-01"}, priority: "critical"},
			{id: "AC-07", desc: "A delivered notification writes one receipt row", refs: []string{"C-02"}, priority: "medium"},
		})
	return writeCheckParityWorkspace(t, "notify-dispatch", yaml)
}

// ---------------------------------------------------------------------------
// JSON document helpers
//
// C-14 mandates no field vocabulary, and AC-33 states its requirement in
// terms of diagnostics rather than of a schema. So nothing below reads a
// field name. Objects are located by the values they carry: a diagnostic kind
// the spec names (`duplicate_ac_id`, `orphan_constraint`, `unknown_spec_ref`,
// `malformed_ac_id`), a constraint or AC id the fixture puts in the
// workspace, and a severity word the spec names (`error`, `warning`, `info`).
//
// Severity is asserted on the same object that carries the kind, never on the
// document at large. "A warning appears somewhere in the output" is not the
// claim; "this diagnostic is a warning" is.
// ---------------------------------------------------------------------------

var severityWords = []string{"error", "warning", "info"}

// jsonWalkObjects returns every JSON object in the document, depth first.
func jsonWalkObjects(v any) []map[string]any {
	var out []map[string]any
	switch t := v.(type) {
	case map[string]any:
		out = append(out, t)
		for _, child := range t {
			out = append(out, jsonWalkObjects(child)...)
		}
	case []any:
		for _, child := range t {
			out = append(out, jsonWalkObjects(child)...)
		}
	}
	return out
}

// jsonDirectStrings returns the string-valued fields of one object. Field
// names are discarded; only the values are used.
func jsonDirectStrings(obj map[string]any) []string {
	var out []string
	for _, v := range obj {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// jsonObjectHasValue reports whether the object carries the exact string
// value, compared case insensitively.
func jsonObjectHasValue(obj map[string]any, want string) bool {
	for _, s := range jsonDirectStrings(obj) {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// jsonObjectMentions reports whether any string value of the object contains
// the substring. Used for identifiers that arrive embedded in a message.
func jsonObjectMentions(obj map[string]any, sub string) bool {
	for _, s := range jsonDirectStrings(obj) {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// kindTokenPattern is the shape every diagnostic kind the spec names takes:
// lowercase words joined by underscores (duplicate_ac_id, orphan_constraint,
// unknown_spec_ref, malformed_ac_id, tier_conflict, unreachable_annotation).
var kindTokenPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)+$`)

// jsonDiagnosticObjects returns every object that carries both a severity
// word and a kind-shaped token as its own values. AC-33 says each diagnostic
// carries its kind and its severity, so that pair identifies the document's
// diagnostic population without naming a field or an array. Requiring both
// keeps a summary or root object out of the count: a summary carries no
// underscore-joined kind token.
func jsonDiagnosticObjects(doc any) []map[string]any {
	var out []map[string]any
	for _, obj := range jsonWalkObjects(doc) {
		hasSeverity := false
		for _, sev := range severityWords {
			if jsonObjectHasValue(obj, sev) {
				hasSeverity = true
				break
			}
		}
		if !hasSeverity {
			continue
		}
		for _, s := range jsonDirectStrings(obj) {
			if kindTokenPattern.MatchString(s) {
				out = append(out, obj)
				break
			}
		}
	}
	return out
}

// jsonFindDiagnostic returns the object that carries the given diagnostic
// kind as one of its values and mentions every locator. Returns nil when no
// such object exists.
func jsonFindDiagnostic(doc any, kind string, locators ...string) map[string]any {
	for _, obj := range jsonWalkObjects(doc) {
		if !jsonObjectHasValue(obj, kind) {
			continue
		}
		matched := true
		for _, loc := range locators {
			if !jsonObjectMentions(obj, loc) {
				matched = false
				break
			}
		}
		if matched {
			return obj
		}
	}
	return nil
}

// decodeSoleJSONDocument decodes stdout as exactly one JSON document and
// fails if anything but whitespace follows it. This is the AC-33 claim that
// stdout carries the document alone; the other criteria use it because a
// document that shares stdout with prose is not usable by the caller C-14
// describes either.
func decodeSoleJSONDocument(t *testing.T, stdout string) any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout must parse as a JSON document, got error %v; stdout:\n%s", err, stdout)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		t.Errorf("stdout must carry the JSON document alone; decoding past it returned %v (want io.EOF); stdout:\n%s", err, stdout)
	}
	return doc
}

// ---------------------------------------------------------------------------
// AC-28 -- duplicate AC id, exit 1 in both formats
// ---------------------------------------------------------------------------

// The workspace's only defect is a spec declaring AC-01 twice. Per C-13 that
// is a `duplicate_ac_id` diagnostic at error severity regardless of tier, so
// the run reports one error and the check contract puts the exit code at 1.
// C-14 makes `--json` report the same 1.
//
// Not vacuous: the criterion asserts a non-zero code, and the JSON arm also
// has to name the duplicated id, so a binary that exits 1 without running the
// checker fails here.
//
// @ac AC-28
func TestCheckJSONExit_DuplicateACID_BothFormatsExitOne(t *testing.T) {
	t.Run("spec-check/AC-28 duplicate ac id exits 1 under check and under check --json", func(t *testing.T) {
		// Declaration order AC-01, AC-02, AC-01 is what the criterion
		// specifies. Every AC references C-01, so the spec has no orphan and
		// the duplicate is the only defect.
		yaml := checkParitySpecYAML("billing-dup", "approved", 1,
			[]constraintFixture{
				{id: "C-01", desc: "MUST charge an invoice once", ctype: "business", enforcement: "error"},
			},
			[]acFixture{
				{id: "AC-01", desc: "A paid invoice is not charged again", refs: []string{"C-01"}, priority: "critical"},
				{id: "AC-02", desc: "A voided invoice is not charged", refs: []string{"C-01"}, priority: "high"},
				{id: "AC-01", desc: "A refunded invoice reopens the balance", refs: []string{"C-01"}, priority: "medium"},
			})
		dir := writeCheckParityWorkspace(t, "billing-dup", yaml)

		textOut, textErr, textCode := runCLISplit(t, dir, "check")
		if textCode != 1 {
			t.Fatalf("text mode: expected exit 1 for a duplicate AC id, got %d;\nstdout:\n%s\nstderr:\n%s", textCode, textOut, textErr)
		}

		jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, "check", "--json")
		if jsonCode != textCode {
			t.Errorf("exit-code parity broken: `check` exited %d, `check --json` exited %d;\njson stdout:\n%s\njson stderr:\n%s",
				textCode, jsonCode, jsonOut, jsonErr)
		}
		if jsonCode != 1 {
			t.Errorf("--json: expected exit 1 for a duplicate AC id, got %d", jsonCode)
		}

		// The exit code has to come with the reason. The document must report
		// the duplicate, naming the duplicated id.
		doc := decodeSoleJSONDocument(t, jsonOut)
		diag := jsonFindDiagnostic(doc, "duplicate_ac_id", "AC-01")
		if diag == nil {
			t.Fatalf("--json document must report a duplicate_ac_id diagnostic naming AC-01; document:\n%s", jsonOut)
		}
		if !jsonObjectHasValue(diag, "error") {
			t.Errorf("the duplicate_ac_id diagnostic must carry error severity, got object %v", diag)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-29 -- parity holds for every diagnostic kind
// ---------------------------------------------------------------------------

// Four workspaces, each run twice. The three error rows carry different
// diagnostic kinds, different tiers, different spec ids, and different AC
// sets, so no row can pass on a code path that keys on the wrong field. The
// clean row is the parity case at exit 0; the three error rows in the same
// table are its control, because they prove this harness sees a non-zero code
// when one is produced.
//
// @ac AC-29
func TestCheckJSONExit_EveryDiagnosticKind_ParityHolds(t *testing.T) {
	// row_orphan: tier 1 orphan constraint C-03, no enforcement override, so
	// tier routing makes it an error.
	orphanDir := func(t *testing.T) string {
		yaml := checkParitySpecYAML("ledger-post", "approved", 1,
			[]constraintFixture{
				{id: "C-01", desc: "MUST post every entry to one account", ctype: "business", enforcement: "error"},
				{id: "C-02", desc: "MUST balance debits against credits", ctype: "business", enforcement: "error"},
				{id: "C-03", desc: "MUST reject an entry dated in the future", ctype: "technical"},
			},
			[]acFixture{
				{id: "AC-01", desc: "An entry posts to the named account", refs: []string{"C-01"}, priority: "critical"},
				{id: "AC-02", desc: "An unbalanced entry is rejected", refs: []string{"C-02"}, priority: "high"},
			})
		return writeCheckParityWorkspace(t, "ledger-post", yaml)
	}

	// row_unknown_spec_ref: a clean tier 3 spec plus a test file naming a
	// spec that does not exist. Different tier and different AC ids from
	// every other row.
	unknownRefDir := func(t *testing.T) string {
		yaml := checkParitySpecYAML("inventory-count", "approved", 3,
			[]constraintFixture{
				{id: "C-01", desc: "MUST count each SKU once per cycle", ctype: "technical", enforcement: "error"},
				{id: "C-02", desc: "MUST record who counted a SKU", ctype: "business", enforcement: "error"},
			},
			[]acFixture{
				{id: "AC-01", desc: "A cycle count visits each SKU once", refs: []string{"C-01"}, priority: "high"},
				{id: "AC-04", desc: "A count records the counting operator", refs: []string{"C-02"}, priority: "medium"},
			})
		dir := writeCheckParityWorkspace(t, "inventory-count", yaml)
		body := "// @spec bogus-spec\n// @ac AC-01\nfunc TestCount(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "count_test.go"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	// row_malformed_ac_id: a clean tier 2 spec plus a test file whose @ac is
	// single digit. The @spec resolves, so the cascade rule does not
	// suppress the @ac check.
	malformedDir := func(t *testing.T) string {
		yaml := checkParitySpecYAML("shipping-label", "approved", 2,
			[]constraintFixture{
				{id: "C-01", desc: "MUST print one label per parcel", ctype: "technical", enforcement: "error"},
			},
			[]acFixture{
				{id: "AC-01", desc: "One parcel yields one label", refs: []string{"C-01"}, priority: "high"},
				{id: "AC-02", desc: "A voided parcel prints no label", refs: []string{"C-01"}, priority: "medium"},
				{id: "AC-03", desc: "A reprint is recorded", refs: []string{"C-01"}, priority: "low"},
			})
		dir := writeCheckParityWorkspace(t, "shipping-label", yaml)
		body := "// @spec shipping-label\n// @ac AC-1\nfunc TestLabel(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "label_test.go"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	// row_clean: no defect of any kind. Every constraint is referenced, the
	// status is approved, and there is no test file to scan.
	cleanDir := func(t *testing.T) string {
		yaml := checkParitySpecYAML("catalog-search", "approved", 2,
			[]constraintFixture{
				{id: "C-01", desc: "MUST return results ranked by relevance", ctype: "technical", enforcement: "error"},
				{id: "C-02", desc: "MUST exclude delisted products", ctype: "business", enforcement: "error"},
			},
			[]acFixture{
				{id: "AC-01", desc: "Results come back ranked", refs: []string{"C-01"}, priority: "high"},
				{id: "AC-02", desc: "A delisted product is absent from results", refs: []string{"C-02"}, priority: "critical"},
			})
		return writeCheckParityWorkspace(t, "catalog-search", yaml)
	}

	rows := []struct {
		name     string
		build    func(*testing.T) string
		args     []string
		wantCode int
		kind     string   // diagnostic the JSON document must report, empty for the clean row
		locators []string // values that bind the diagnostic to this fixture
		severity string
	}{
		{
			name:     "spec-check/AC-29 row_orphan tier 1 orphan constraint parity",
			build:    orphanDir,
			args:     []string{"check"},
			wantCode: 1,
			kind:     "orphan_constraint",
			locators: []string{"C-03"},
			severity: "error",
		},
		{
			name:     "spec-check/AC-29 row_unknown_spec_ref parity under --test",
			build:    unknownRefDir,
			args:     []string{"check", "--test"},
			wantCode: 1,
			kind:     "unknown_spec_ref",
			locators: []string{"bogus-spec"},
			severity: "error",
		},
		{
			name:     "spec-check/AC-29 row_malformed_ac_id parity under --test",
			build:    malformedDir,
			args:     []string{"check", "--test"},
			wantCode: 1,
			kind:     "malformed_ac_id",
			locators: []string{"AC-1"},
			severity: "error",
		},
		{
			name:     "spec-check/AC-29 row_clean parity at exit 0",
			build:    cleanDir,
			args:     []string{"check"},
			wantCode: 0,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			dir := row.build(t)

			textOut, textErr, textCode := runCLISplit(t, dir, row.args...)
			if textCode != row.wantCode {
				t.Fatalf("text mode: expected exit %d, got %d;\nstdout:\n%s\nstderr:\n%s", row.wantCode, textCode, textOut, textErr)
			}

			jsonArgs := append(append([]string{}, row.args...), "--json")
			jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, jsonArgs...)
			if jsonCode != textCode {
				t.Errorf("exit-code parity broken: %v exited %d, %v exited %d;\njson stdout:\n%s\njson stderr:\n%s",
					row.args, textCode, jsonArgs, jsonCode, jsonOut, jsonErr)
			}
			if jsonCode != row.wantCode {
				t.Errorf("--json: expected exit %d, got %d", row.wantCode, jsonCode)
			}

			doc := decodeSoleJSONDocument(t, jsonOut)

			if row.kind == "" {
				// Fixture validation for the clean row: the workspace must
				// really be clean, otherwise its 0/0 parity would prove
				// nothing about a run that produced no error.
				if diags := jsonDiagnosticObjects(doc); len(diags) != 0 {
					t.Errorf("clean workspace must report no diagnostics, got %d: %v", len(diags), diags)
				}
				return
			}

			diag := jsonFindDiagnostic(doc, row.kind, row.locators...)
			if diag == nil {
				t.Fatalf("--json document must report a %s diagnostic mentioning %v; document:\n%s", row.kind, row.locators, jsonOut)
			}
			if !jsonObjectHasValue(diag, row.severity) {
				t.Errorf("the %s diagnostic must carry %s severity, got object %v", row.kind, row.severity, diag)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-30 -- warning-only run exits 0 in both formats
// ---------------------------------------------------------------------------

// A tier 2 orphan routes to warning, and warnings do not fail the check. The
// criterion asserts exit 0, which a binary that always exits 0 satisfies for
// free, so the test carries its own control: the JSON document must report
// the orphan at warning severity, and text mode must name it too. That proves
// the checker ran and produced a diagnostic, and that exit 0 is a verdict
// rather than an omission.
//
// AC-31 is the second control at the system level. It runs the same workspace
// with --strict and requires 1 from both formats, so a fix that hard-codes 0
// for `--json` fails there.
//
// @ac AC-30
func TestCheckJSONExit_WarningOnly_BothFormatsExitZero(t *testing.T) {
	t.Run("spec-check/AC-30 warning-only run exits 0 under check and under check --json", func(t *testing.T) {
		dir := tier2OrphanWorkspace(t)

		textOut, textErr, textCode := runCLISplit(t, dir, "check")
		if textCode != 0 {
			t.Fatalf("text mode: expected exit 0 for a warning-only run, got %d;\nstdout:\n%s\nstderr:\n%s", textCode, textOut, textErr)
		}
		// Control: the run really did produce the diagnostic in text mode.
		combinedText := textOut + textErr
		if !strings.Contains(combinedText, "orphan_constraint") || !strings.Contains(combinedText, "C-03") {
			t.Fatalf("text mode must report the orphan constraint C-03, so exit 0 is not vacuous;\nstdout:\n%s\nstderr:\n%s", textOut, textErr)
		}

		jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, "check", "--json")
		if jsonCode != textCode {
			t.Errorf("exit-code parity broken: `check` exited %d, `check --json` exited %d;\njson stdout:\n%s\njson stderr:\n%s",
				textCode, jsonCode, jsonOut, jsonErr)
		}
		if jsonCode != 0 {
			t.Errorf("--json: expected exit 0 for a warning-only run, got %d", jsonCode)
		}

		doc := decodeSoleJSONDocument(t, jsonOut)
		diag := jsonFindDiagnostic(doc, "orphan_constraint", "C-03")
		if diag == nil {
			t.Fatalf("--json document must report the orphan constraint C-03; document:\n%s", jsonOut)
		}
		if !jsonObjectHasValue(diag, "warning") {
			t.Errorf("the tier 2 orphan_constraint diagnostic must carry warning severity, got object %v", diag)
		}
		if jsonObjectHasValue(diag, "error") {
			t.Errorf("the tier 2 orphan_constraint diagnostic must not be an error without --strict, got object %v", diag)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-31 -- --strict upgrades the warning, both formats exit 1
// ---------------------------------------------------------------------------

// Same workspace as AC-30. --strict upgrades the tier 2 orphan to error, and
// the exit code follows severity after the upgrade, so both formats report 1.
// The baseline arm below runs the same workspace without --strict and
// requires 0 from text mode, which binds the 1 to the upgrade rather than to
// anything else in the fixture.
//
// @ac AC-31
func TestCheckJSONExit_StrictUpgrade_BothFormatsExitOne(t *testing.T) {
	t.Run("spec-check/AC-31 --strict upgrade exits 1 under check --strict and under check --strict --json", func(t *testing.T) {
		dir := tier2OrphanWorkspace(t)

		// Baseline: without --strict this workspace exits 0. Any 1 seen below
		// is therefore produced by the upgrade.
		_, _, baselineCode := runCLISplit(t, dir, "check")
		if baselineCode != 0 {
			t.Fatalf("baseline: the tier 2 orphan workspace must exit 0 without --strict, got %d", baselineCode)
		}

		textOut, textErr, textCode := runCLISplit(t, dir, "check", "--strict")
		if textCode != 1 {
			t.Fatalf("text mode: expected exit 1 under --strict, got %d;\nstdout:\n%s\nstderr:\n%s", textCode, textOut, textErr)
		}

		jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, "check", "--strict", "--json")
		if jsonCode != textCode {
			t.Errorf("exit-code parity broken: `check --strict` exited %d, `check --strict --json` exited %d;\njson stdout:\n%s\njson stderr:\n%s",
				textCode, jsonCode, jsonOut, jsonErr)
		}
		if jsonCode != 1 {
			t.Errorf("--json: expected exit 1 under --strict, got %d", jsonCode)
		}

		// The upgrade must be visible in the document too, bound to the same
		// diagnostic that was a warning in AC-30.
		doc := decodeSoleJSONDocument(t, jsonOut)
		diag := jsonFindDiagnostic(doc, "orphan_constraint", "C-03")
		if diag == nil {
			t.Fatalf("--json document must report the orphan constraint C-03; document:\n%s", jsonOut)
		}
		if !jsonObjectHasValue(diag, "error") {
			t.Errorf("--strict must report the tier 2 orphan_constraint diagnostic at error severity, got object %v", diag)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-32 -- info-only run exits 0 in both formats
// ---------------------------------------------------------------------------

// A tier 3 orphan routes to info. Info changes no verdict, so both formats
// exit 0. Same vacuity problem as AC-30 and the same control: the document
// must carry the diagnostic at info severity, and text mode must name it, so
// exit 0 is asserted about a run that produced output rather than about a run
// that did nothing.
//
// @ac AC-32
func TestCheckJSONExit_InfoOnly_BothFormatsExitZero(t *testing.T) {
	t.Run("spec-check/AC-32 info-only run exits 0 under check and under check --json", func(t *testing.T) {
		// Tier 3, a different spec id, a different constraint set, and three
		// ACs. Only the orphan and the tier are shared with AC-30.
		yaml := checkParitySpecYAML("audit-trail", "approved", 3,
			[]constraintFixture{
				{id: "C-01", desc: "MUST append one record per privileged action", ctype: "security", enforcement: "error"},
				{id: "C-02", desc: "MUST keep records immutable", ctype: "security", enforcement: "error"},
				// C-03 is the orphan. No enforcement override, so tier 3
				// routes it to info.
				{id: "C-03", desc: "MUST retain records for seven years", ctype: "business"},
			},
			[]acFixture{
				{id: "AC-01", desc: "A privileged action appends one record", refs: []string{"C-01"}, priority: "critical"},
				{id: "AC-02", desc: "An existing record cannot be edited", refs: []string{"C-02"}, priority: "high"},
				{id: "AC-03", desc: "An existing record cannot be deleted", refs: []string{"C-02"}, priority: "high"},
			})
		dir := writeCheckParityWorkspace(t, "audit-trail", yaml)

		textOut, textErr, textCode := runCLISplit(t, dir, "check")
		if textCode != 0 {
			t.Fatalf("text mode: expected exit 0 for an info-only run, got %d;\nstdout:\n%s\nstderr:\n%s", textCode, textOut, textErr)
		}
		combinedText := textOut + textErr
		if !strings.Contains(combinedText, "orphan_constraint") || !strings.Contains(combinedText, "C-03") {
			t.Fatalf("text mode must report the orphan constraint C-03, so exit 0 is not vacuous;\nstdout:\n%s\nstderr:\n%s", textOut, textErr)
		}

		jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, "check", "--json")
		if jsonCode != textCode {
			t.Errorf("exit-code parity broken: `check` exited %d, `check --json` exited %d;\njson stdout:\n%s\njson stderr:\n%s",
				textCode, jsonCode, jsonOut, jsonErr)
		}
		if jsonCode != 0 {
			t.Errorf("--json: expected exit 0 for an info-only run, got %d", jsonCode)
		}

		doc := decodeSoleJSONDocument(t, jsonOut)
		diag := jsonFindDiagnostic(doc, "orphan_constraint", "C-03")
		if diag == nil {
			t.Fatalf("--json document must report the orphan constraint C-03; document:\n%s", jsonOut)
		}
		if !jsonObjectHasValue(diag, "info") {
			t.Errorf("the tier 3 orphan_constraint diagnostic must carry info severity, got object %v", diag)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-33 -- a non-zero exit still delivers the whole document
// ---------------------------------------------------------------------------

// The caller reads the exit code and then reads the reason from the same
// output. So on a failing run stdout must hold one complete JSON document and
// nothing else, and that document must carry the diagnostics text mode
// reports, each with its kind and its severity.
//
// The fixture produces exactly one diagnostic, which is what makes "matches
// text mode" assertable without pinning an output format: the document must
// hold one diagnostic, it must be the duplicate_ac_id text mode names, and it
// must carry the same severity.
//
// @ac AC-33
func TestCheckJSONExit_NonZeroExitCarriesWholeDocument(t *testing.T) {
	t.Run("spec-check/AC-33 --json writes one complete document to stdout on a non-zero exit", func(t *testing.T) {
		// Declaration order AC-01, AC-02, AC-01, per the criterion. A
		// different spec id and different constraint from AC-28, so the two
		// criteria do not share a fixture.
		yaml := checkParitySpecYAML("order-intake", "approved", 1,
			[]constraintFixture{
				{id: "C-01", desc: "MUST accept one order per idempotency key", ctype: "technical", enforcement: "error"},
			},
			[]acFixture{
				{id: "AC-01", desc: "A repeated key returns the first order", refs: []string{"C-01"}, priority: "critical"},
				{id: "AC-02", desc: "A new key creates an order", refs: []string{"C-01"}, priority: "high"},
				{id: "AC-01", desc: "A repeated key writes no second row", refs: []string{"C-01"}, priority: "critical"},
			})
		dir := writeCheckParityWorkspace(t, "order-intake", yaml)

		// What text mode reports for this run, for the comparison below.
		textOut, textErr, textCode := runCLISplit(t, dir, "check")
		if textCode != 1 {
			t.Fatalf("text mode: expected exit 1 for a duplicate AC id, got %d;\nstdout:\n%s\nstderr:\n%s", textCode, textOut, textErr)
		}
		combinedText := textOut + textErr
		if !strings.Contains(combinedText, "duplicate_ac_id") {
			t.Fatalf("text mode must report duplicate_ac_id for this workspace;\nstdout:\n%s\nstderr:\n%s", textOut, textErr)
		}

		jsonOut, jsonErr, jsonCode := runCLISplit(t, dir, "check", "--json")
		if jsonCode != 1 {
			t.Fatalf("--json: expected exit 1, got %d;\nstdout:\n%s\nstderr:\n%s", jsonCode, jsonOut, jsonErr)
		}

		// stdout parses, and holds the document alone. decodeSoleJSONDocument
		// fails if anything but whitespace follows the first value, which is
		// where a human-readable notice written to stdout would land.
		doc := decodeSoleJSONDocument(t, jsonOut)

		// Every diagnostic text mode reports is in the document. This run
		// reports one.
		diags := jsonDiagnosticObjects(doc)
		if len(diags) != 1 {
			t.Fatalf("expected the document to carry the 1 diagnostic this run reports, got %d: %v\ndocument:\n%s", len(diags), diags, jsonOut)
		}

		// It carries its kind and its severity, and it names the duplicated
		// id so the caller can act on it.
		diag := diags[0]
		if !jsonObjectHasValue(diag, "duplicate_ac_id") {
			t.Errorf("the diagnostic must carry its kind, duplicate_ac_id, got object %v", diag)
		}
		if !jsonObjectHasValue(diag, "error") {
			t.Errorf("the diagnostic must carry its severity, error, got object %v", diag)
		}
		if !jsonObjectMentions(diag, "AC-01") {
			t.Errorf("the diagnostic must name the duplicated id AC-01, got object %v", diag)
		}
	})
}
