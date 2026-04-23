// @spec spec-ingest
package ingest

import "testing"

// @ac AC-03
func TestParseGoTest_PassAction_ReturnsPassedResult(t *testing.T) {
	t.Run("spec-ingest/AC-03 pass action returns passed result", func(t *testing.T) {
		input := []byte(`{"Action":"run","Package":"github.com/acme/auth","Test":"TestAuthService/engine-transaction/AC-03"}
{"Action":"pass","Package":"github.com/acme/auth","Test":"TestAuthService/engine-transaction/AC-03"}
`)

		results, err := ParseGoTest(input)
		if err != nil {
			t.Fatalf("ParseGoTest returned error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.SpecID != "engine-transaction" {
			t.Errorf("SpecID = %q, want engine-transaction", r.SpecID)
		}
		if r.ACID != "AC-03" {
			t.Errorf("ACID = %q, want AC-03", r.ACID)
		}
		if r.Status != StatusPassed {
			t.Errorf("Status = %q, want passed", r.Status)
		}
	})
}

// @ac AC-04
func TestParseGoTest_FailAction_ReturnsFailedStatus(t *testing.T) {
	t.Run("spec-ingest/AC-04 fail action returns failed status", func(t *testing.T) {
		input := []byte(`{"Action":"fail","Package":"p","Test":"TestX/spec-a/AC-02"}`)
		results, _ := ParseGoTest(input)
		if len(results) != 1 || results[0].Status != StatusFailed {
			t.Fatalf("expected failed, got %+v", results)
		}
	})
}

// @ac AC-04
func TestParseGoTest_SkipAction_ReturnsSkippedStatus(t *testing.T) {
	t.Run("spec-ingest/AC-04 skip action returns skipped status", func(t *testing.T) {
		input := []byte(`{"Action":"skip","Package":"p","Test":"TestX/spec-a/AC-09"}`)
		results, _ := ParseGoTest(input)
		if len(results) != 1 || results[0].Status != StatusSkipped {
			t.Fatalf("expected skipped, got %+v", results)
		}
	})
}

// @ac AC-04
// Output-action lines carrying // @spec / // @ac annotations should
// establish SpecID/ACID for the current test, enabling Go tests that
// don't embed the IDs in their subtest name.
func TestParseGoTest_OutputAnnotation_SetsSpecAndAC(t *testing.T) {
	t.Run("spec-ingest/AC-04 output annotation sets spec and AC", func(t *testing.T) {
		input := []byte(`{"Action":"run","Package":"p","Test":"TestAuthHappy"}
{"Action":"output","Package":"p","Test":"TestAuthHappy","Output":"// @spec auth-service\n"}
{"Action":"output","Package":"p","Test":"TestAuthHappy","Output":"// @ac AC-11\n"}
{"Action":"pass","Package":"p","Test":"TestAuthHappy"}
`)
		results, _ := ParseGoTest(input)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].SpecID != "auth-service" || results[0].ACID != "AC-11" {
			t.Errorf("got SpecID=%q ACID=%q", results[0].SpecID, results[0].ACID)
		}
	})
}

// @ac AC-05
func TestParseGoTest_NoAnnotation_Dropped(t *testing.T) {
	t.Run("spec-ingest/AC-05 no annotation dropped", func(t *testing.T) {
		input := []byte(`{"Action":"pass","Package":"p","Test":"TestUnrelated"}`)
		results, err := ParseGoTest(input)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results (no annotation), got %d", len(results))
		}
	})
}
