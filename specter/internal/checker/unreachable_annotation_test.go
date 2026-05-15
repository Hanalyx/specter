// Pure-function tests for unreachable_annotation detection (C-10).
//
// Each test asserts the diagnostic contract for one AC (AC-13..AC-18)
// from spec-check 1.3.0. The implementation lives in
// unreachable_annotation.go; until commit 3 replaces the stub, every
// assertion below is expected to fail — that's the contract the SDD
// cycle enforces.
//
// @spec spec-check
package checker

import (
	"strings"
	"testing"
)

// @ac AC-13
func TestCheckUnreachableAnnotations_BasicDetection(t *testing.T) {
	t.Run("spec-check/AC-13 source-only @ac without runner-visible token emits unreachable_annotation", func(t *testing.T) {
		// Go test file: declares @spec and @ac in source comments,
		// but the subtest name does NOT carry foo-spec/AC-01 AND the
		// body has no fmt.Println / t.Log emitting the pair.
		testFiles := map[string]string{
			"foo_test.go": `package foo
// @spec foo-spec
// @ac AC-01
func TestFoo(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// no runner-visible foo-spec/AC-01 here
	})
}
`,
		}

		diags := CheckUnreachableAnnotations(testFiles, "threshold")

		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				found = true
				if !strings.Contains(d.Message, "foo_test.go") {
					t.Errorf("expected file path in message, got: %s", d.Message)
				}
				if !strings.Contains(d.Message, "AC-01") {
					t.Errorf("expected AC-01 in message, got: %s", d.Message)
				}
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation diagnostic, got: %+v", diags)
		}
	})
}

// @ac AC-14
func TestCheckUnreachableAnnotations_GoReachability(t *testing.T) {
	specs := map[string]string{
		// Reachable: subtest name carries the pair (Convention A).
		"reachable_via_subtest_test.go": `package foo
// @spec foo-spec
// @ac AC-01
func TestFoo(t *testing.T) {
	t.Run("foo-spec/AC-01 valid input", func(t *testing.T) {})
}
`,
		// Reachable: body emits the pair via fmt.Println (Convention B).
		"reachable_via_print_test.go": `package foo
// @spec foo-spec
// @ac AC-02
func TestBar(t *testing.T) {
	fmt.Println("// @spec foo-spec")
	fmt.Println("// @ac AC-02")
}
`,
		// Unreachable: neither subtest carries the pair nor body prints.
		"unreachable_test.go": `package foo
// @spec foo-spec
// @ac AC-03
func TestBaz(t *testing.T) {
	t.Run("does something", func(t *testing.T) {})
}
`,
	}

	t.Run("spec-check/AC-14 Go reachable cases produce no diagnostic", func(t *testing.T) {
		reachable := map[string]string{
			"reachable_via_subtest_test.go": specs["reachable_via_subtest_test.go"],
			"reachable_via_print_test.go":   specs["reachable_via_print_test.go"],
		}
		diags := CheckUnreachableAnnotations(reachable, "threshold")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected no unreachable_annotation for reachable Go tests, got: %+v", d)
			}
		}
	})

	t.Run("spec-check/AC-14 Go unreachable case emits unreachable_annotation", func(t *testing.T) {
		unreachable := map[string]string{
			"unreachable_test.go": specs["unreachable_test.go"],
		}
		diags := CheckUnreachableAnnotations(unreachable, "threshold")
		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" && strings.Contains(d.Message, "AC-03") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation for AC-03, got: %+v", diags)
		}
	})
}

// Regression guard: free-floating @ac above a non-test func must NOT
// be attributed to the next test function (where it would emit a
// false-positive unreachable_annotation). Surfaced by the v0.13 cycle
// review (commit 3a, finding #10). Owned by AC-14 (Go reachability
// correctness).
//
// @ac AC-14
func TestCheckUnreachableAnnotations_FreeFloatingAcAboveNonTest(t *testing.T) {
	t.Run("spec-check/AC-14 @ac above helper() routes to _unknown, not false-positive unreachable", func(t *testing.T) {
		testFiles := map[string]string{
			"mixed_test.go": `package foo
// @spec foo-spec
// @ac AC-01
func helper() {}

// @ac AC-02
func TestFoo(t *testing.T) {
	t.Run("foo-spec/AC-02 valid", func(t *testing.T) {})
}
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")

		// AC-01 belongs to helper() (not a test) — must be _unknown,
		// NOT a false-positive unreachable_annotation against TestFoo.
		// AC-02 belongs to TestFoo and is reachable via the subtest.
		var ac01Unknown, ac01FalsePos, ac02FalsePos bool
		for _, d := range diags {
			if strings.Contains(d.Message, "AC-01") && d.Kind == "unreachable_annotation_unknown" {
				ac01Unknown = true
			}
			if strings.Contains(d.Message, "AC-01") && d.Kind == "unreachable_annotation" {
				ac01FalsePos = true
			}
			if strings.Contains(d.Message, "AC-02") && d.Kind == "unreachable_annotation" {
				ac02FalsePos = true
			}
		}
		if !ac01Unknown {
			t.Errorf("expected AC-01 (above helper()) to emit unreachable_annotation_unknown, got: %+v", diags)
		}
		if ac01FalsePos {
			t.Errorf("AC-01 above helper() must NOT emit unreachable_annotation (false positive), got: %+v", diags)
		}
		if ac02FalsePos {
			t.Errorf("AC-02 is reachable via t.Run(\"foo-spec/AC-02 ...\"), got false-positive unreachable: %+v", diags)
		}
	})
}

// Regression guard: a phantom `it("title", ...)` inside a `/* ... */`
// docblock must not be registered as a real test entry. Surfaced by
// the v0.13 cycle review (commit 3b, finding B1). Owned by AC-16.
//
// @ac AC-16
func TestCheckUnreachableAnnotations_TS_BlockCommentItIsIgnored(t *testing.T) {
	t.Run("spec-check/AC-16 it() inside /* */ block does not register as a real test entry", func(t *testing.T) {
		testFiles := map[string]string{
			"docblocked.test.ts": "/*\n * Example usage:\n *   it(\"old example\", () => {});\n */\n// @spec foo-spec\n// @ac AC-01\nit('[foo-spec/AC-01] real test', () => {});\n",
		}
		diags := CheckUnreachableAnnotations(testFiles, "zero-tolerance")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected NO unreachable_annotation (the real it() carries the pair in its title), got false-positive: %+v", d)
			}
		}
	})
}

// Regression guard: an `// @ac` comment inside a nested `it()` body
// must be attributed to the it(), not its enclosing describe().
// Surfaced by the v0.13 cycle review (commit 3b, finding B2). Owned
// by AC-16.
//
// @ac AC-16
func TestCheckUnreachableAnnotations_TS_NestedAttribution(t *testing.T) {
	t.Run("spec-check/AC-16 nested it() inside describe(): @ac attributes to innermost", func(t *testing.T) {
		testFiles := map[string]string{
			"nested.test.ts": "// @spec foo-spec\ndescribe('payment suite', () => {\n  // @ac AC-01\n  it('[foo-spec/AC-01] valid amount', () => {});\n});\n",
		}
		diags := CheckUnreachableAnnotations(testFiles, "zero-tolerance")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected NO unreachable_annotation (the it() title carries the pair); attribution should pick innermost it() not enclosing describe(). got: %+v", d)
			}
		}
	})
}

// Regression guard: Python's common logger patterns must be recognized
// as Convention B emitters. A test using LOG.info / self.logger.info /
// logging.getLogger(__name__).info / my_log.info must not produce a
// false-positive unreachable_annotation. Surfaced by v0.13 commit 3c
// agent review (finding M1). Owned by AC-15.
//
// @ac AC-15
func TestCheckUnreachableAnnotations_PythonLoggerPatterns(t *testing.T) {
	t.Run("spec-check/AC-15 Python common logger patterns are recognized as reachable", func(t *testing.T) {
		// One test function per common logger idiom. Each emits @spec
		// and @ac as the logger's string-literal arg.
		testFiles := map[string]string{
			"test_loggers.py": `# @spec foo-spec
# @ac AC-01
def test_uppercase_log(client):
    LOG.info('// @spec foo-spec')
    LOG.info('// @ac AC-01')

# @ac AC-02
def test_class_logger(client):
    self.logger.info('// @spec foo-spec')
    self.logger.info('// @ac AC-02')

# @ac AC-03
def test_local_logger(client):
    my_log.info('// @spec foo-spec')
    my_log.info('// @ac AC-03')

# @ac AC-04
def test_inline_getlogger(client):
    logging.getLogger(__name__).info('// @spec foo-spec')
    logging.getLogger(__name__).info('// @ac AC-04')
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "zero-tolerance")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected no unreachable_annotation for known Python logger patterns, got: %+v", d)
			}
		}
	})
}

// @ac AC-15
func TestCheckUnreachableAnnotations_PythonReachability(t *testing.T) {
	t.Run("spec-check/AC-15 Python Convention B (runtime print) is reachable", func(t *testing.T) {
		testFiles := map[string]string{
			"test_foo.py": `# @spec foo-spec
# @ac AC-01
def test_valid_input(client):
    print('// @spec foo-spec')
    print('// @ac AC-01')
    assert True
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected no unreachable_annotation for Python with runtime prints, got: %+v", d)
			}
		}
	})

	t.Run("spec-check/AC-15 Python source-only @ac (no runtime print) is unreachable", func(t *testing.T) {
		testFiles := map[string]string{
			"test_bar.py": `# @spec foo-spec
# @ac AC-02
def test_other_input(client):
    assert True
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" && strings.Contains(d.Message, "AC-02") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation for AC-02 (Python source-only), got: %+v", diags)
		}
	})
}

// @ac AC-16
func TestCheckUnreachableAnnotations_TSJestReachability(t *testing.T) {
	t.Run("spec-check/AC-16 TS reachable via it title (Convention A)", func(t *testing.T) {
		testFiles := map[string]string{
			"foo.test.ts": `// @spec foo-spec
// @ac AC-01
it('[foo-spec/AC-01] valid input returns 200', () => {
    expect(true).toBe(true);
});
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected no unreachable_annotation for TS with title-embedded pair, got: %+v", d)
			}
		}
	})

	t.Run("spec-check/AC-16 TS reachable via console.log (Convention B)", func(t *testing.T) {
		testFiles := map[string]string{
			"bar.test.ts": `// @spec foo-spec
// @ac AC-02
it('valid input returns 200', () => {
    console.log('// @spec foo-spec');
    console.log('// @ac AC-02');
    expect(true).toBe(true);
});
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected no unreachable_annotation for TS with console.log emit, got: %+v", d)
			}
		}
	})

	t.Run("spec-check/AC-16 TS unreachable (no title pair, no console.log)", func(t *testing.T) {
		testFiles := map[string]string{
			"baz.test.ts": `// @spec foo-spec
// @ac AC-03
it('does the thing', () => {
    expect(true).toBe(true);
});
`,
		}
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" && strings.Contains(d.Message, "AC-03") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation for AC-03 (TS source-only), got: %+v", diags)
		}
	})
}

// @ac AC-17
func TestCheckUnreachableAnnotations_UnknownReachability(t *testing.T) {
	t.Run("spec-check/AC-17 unsupported test shape emits unreachable_annotation_unknown (warning, not error)", func(t *testing.T) {
		// A test shape the language-aware parser does not recognize:
		// dynamically-registered tests via a custom runner (no
		// recognizable testing.T.Run / fmt.Println / it() / etc.)
		testFiles := map[string]string{
			"dynamic_test.go": `package foo
// @spec foo-spec
// @ac AC-04
func init() {
    customRunner.Register("foo-spec/AC-04", func() {
        // body runs via reflection; no t.Run, no println
    })
}
`,
		}

		// Even under zero-tolerance, _unknown is warning, never error.
		diags := CheckUnreachableAnnotations(testFiles, "zero-tolerance")

		var foundUnknown bool
		var foundFalsePositive bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation_unknown" {
				foundUnknown = true
				if d.Severity != "warning" {
					t.Errorf("expected severity=warning for unreachable_annotation_unknown, got: %s", d.Severity)
				}
			}
			if d.Kind == "unreachable_annotation" {
				foundFalsePositive = true
			}
		}
		if !foundUnknown {
			t.Errorf("expected unreachable_annotation_unknown for custom test shape, got: %+v", diags)
		}
		if foundFalsePositive {
			t.Errorf("expected NO unreachable_annotation (false positive) for unsupported shape, got: %+v", diags)
		}
	})
}

// @ac AC-18
func TestCheckUnreachableAnnotations_StrictnessRouting(t *testing.T) {
	// Same unreachable test fixture across all three strictness modes.
	testFiles := map[string]string{
		"foo_test.go": `package foo
// @spec foo-spec
// @ac AC-01
func TestFoo(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {})
}
`,
	}

	t.Run("spec-check/AC-18 zero-tolerance routes unreachable_annotation as error", func(t *testing.T) {
		diags := CheckUnreachableAnnotations(testFiles, "zero-tolerance")
		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				found = true
				if d.Severity != "error" {
					t.Errorf("expected severity=error under zero-tolerance, got: %s", d.Severity)
				}
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation diagnostic under zero-tolerance, got: %+v", diags)
		}
	})

	t.Run("spec-check/AC-18 threshold routes unreachable_annotation as warning", func(t *testing.T) {
		diags := CheckUnreachableAnnotations(testFiles, "threshold")
		var found bool
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				found = true
				if d.Severity != "warning" {
					t.Errorf("expected severity=warning under threshold, got: %s", d.Severity)
				}
			}
		}
		if !found {
			t.Errorf("expected unreachable_annotation diagnostic under threshold, got: %+v", diags)
		}
	})

	t.Run("spec-check/AC-18 annotation mode suppresses unreachable_annotation entirely", func(t *testing.T) {
		diags := CheckUnreachableAnnotations(testFiles, "annotation")
		for _, d := range diags {
			if d.Kind == "unreachable_annotation" {
				t.Errorf("expected unreachable_annotation suppressed under annotation mode, got: %+v", d)
			}
		}
	})
}
