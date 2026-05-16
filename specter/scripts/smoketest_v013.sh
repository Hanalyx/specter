#!/usr/bin/env bash
# v0.13.0 feature smoke test.
#
# Exercises each new v0.13 CLI surface end-to-end against either a
# synthetic fixture workspace or the Specter repo itself. Captures
# stdout/stderr/exit code, asserts the headline behavior, and writes
# a Markdown summary to docs/release-testing/v0.13.0-smoke.md.

set -uo pipefail

SPECTER_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPECTER_BIN="${SPECTER_REPO_ROOT}/bin/specter"
RESULTS_DIR="${SPECTER_REPO_ROOT}/docs/release-testing"
RESULTS_FILE="${RESULTS_DIR}/v0.13.0-smoke.md"
WORKDIR="$(mktemp -d -t specter-smoke-XXXXXX)"
mkdir -p "${RESULTS_DIR}"

pass=0
fail=0
declare -a LINES

note() {
    LINES+=("$@")
}

check() {
    local label="$1"; shift
    local expectation="$1"; shift
    if "$@"; then
        pass=$((pass + 1))
        note "- **PASS** ${label} — ${expectation}"
        echo "  PASS ${label}"
    else
        fail=$((fail + 1))
        note "- **FAIL** ${label} — ${expectation}"
        echo "  FAIL ${label}"
    fi
}

echo "=== v0.13.0 smoke ==="
echo "binary: ${SPECTER_BIN}"
echo "workdir: ${WORKDIR}"
echo

#-----------------------------------------------------------------------
# D1 — doctor --fix preserves description-block prose mentions of
# trust_level when stripping the real key.
#-----------------------------------------------------------------------
echo "[D1] doctor --fix line-targeted deletion"
mkdir -p "${WORKDIR}/d1/specs"
# The prose mention of trust_level lives in context.description (a
# real schema field that accepts a block scalar) so the rewrite
# predicate matches the actual trust_level additionalProperties
# error rather than a spurious one elsewhere.
cat > "${WORKDIR}/d1/specs/legacy.spec.yaml" <<'YAML'
spec:
  id: legacy-spec
  version: "1.0.0"
  status: draft
  tier: 3
  trust_level: high
  context:
    system: test
    feature: test
    description: |
      The trust_level: high field was removed in v0.6.5.
      Use tier: instead of trust_level: medium.
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "MUST something"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      priority: high
YAML

(cd "${WORKDIR}/d1" && "${SPECTER_BIN}" doctor --fix --yes 2>&1 > /dev/null) || true
check "D1.1" "real trust_level key removed" \
    bash -c "! grep -E '^  trust_level: high\$' ${WORKDIR}/d1/specs/legacy.spec.yaml >/dev/null"
check "D1.2" "description prose 'trust_level: high' line preserved" \
    grep -q "The trust_level: high field was removed" "${WORKDIR}/d1/specs/legacy.spec.yaml"
check "D1.3" "description prose 'trust_level: medium' line preserved" \
    grep -q "Use tier: instead of trust_level: medium" "${WORKDIR}/d1/specs/legacy.spec.yaml"

#-----------------------------------------------------------------------
# D2 — coverage stderr surfaces unrecognized status value.
#-----------------------------------------------------------------------
echo "[D2] coverage invalid-status diagnostic"
mkdir -p "${WORKDIR}/d2/specs"
cat > "${WORKDIR}/d2/specs/my.spec.yaml" <<'YAML'
spec:
  id: my-spec
  version: "1.0.0"
  status: draft
  tier: 2
  context: { system: t, feature: t }
  objective: { summary: t }
  constraints:
    - id: C-01
      description: "MUST"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      priority: high
YAML
cat > "${WORKDIR}/d2/foo_test.go" <<'GO'
// @spec my-spec
// @ac AC-01
package main
GO
cat > "${WORKDIR}/d2/.specter-results.json" <<'JSON'
{"results":[{"spec_id":"my-spec","ac_id":"AC-01","status":"pass","test_name":"x"}]}
JSON
cat > "${WORKDIR}/d2/specter.yaml" <<'YAML'
system:
  name: t
settings:
  specs_dir: specs
YAML

d2_out=$(cd "${WORKDIR}/d2" && "${SPECTER_BIN}" coverage --strict 2>&1 || true)
check "D2.1" "warning names the offending value '\"pass\"'" \
    bash -c "echo '${d2_out}' | grep -q 'status=\"pass\"'"
check "D2.2" "warning names the documented enum" \
    bash -c "echo '${d2_out}' | grep -q 'passed|failed|skipped|errored'"

# --json mode must surface invalid_status_warnings
d2_json=$(cd "${WORKDIR}/d2" && "${SPECTER_BIN}" coverage --strict --json 2>/dev/null || true)
check "D2.3" "--json output includes invalid_status_warnings field" \
    bash -c "echo '${d2_json}' | grep -q invalid_status_warnings"

#-----------------------------------------------------------------------
# D3 — coverage --strict --json exit code matches text mode for
# zero-tolerance violations.
#-----------------------------------------------------------------------
echo "[D3] coverage --strict --json exit-code parity"
mkdir -p "${WORKDIR}/d3/specs"
cat > "${WORKDIR}/d3/specs/my.spec.yaml" <<'YAML'
spec:
  id: my-spec
  version: "1.0.0"
  status: draft
  tier: 2
  context: { system: t, feature: t }
  objective: { summary: t }
  constraints:
    - id: C-01
      description: "MUST"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "t"
      references_constraints: ["C-01"]
      priority: high
    - id: AC-02
      description: "t"
      references_constraints: ["C-01"]
      priority: high
YAML
cat > "${WORKDIR}/d3/foo_test.go" <<'GO'
// @spec my-spec
// @ac AC-01
// @ac AC-02
package main
GO
cat > "${WORKDIR}/d3/.specter-results.json" <<'JSON'
{"results":[
  {"spec_id":"my-spec","ac_id":"AC-01","status":"failed","test_name":"x"},
  {"spec_id":"my-spec","ac_id":"AC-02","status":"passed","test_name":"x"}
]}
JSON
cat > "${WORKDIR}/d3/specter.yaml" <<'YAML'
system:
  name: t
settings:
  specs_dir: specs
  strictness: zero-tolerance
YAML

(cd "${WORKDIR}/d3" && "${SPECTER_BIN}" coverage --strict --json > /dev/null 2>&1)
d3_json_exit=$?
(cd "${WORKDIR}/d3" && "${SPECTER_BIN}" coverage --strict > /dev/null 2>&1)
d3_text_exit=$?
check "D3.1" "--json exits non-zero on zero-tolerance violation (matches text mode)" \
    test "${d3_json_exit}" != "0"
check "D3.2" "--json exit code matches text-mode exit code (${d3_json_exit} vs ${d3_text_exit})" \
    test "${d3_json_exit}" = "${d3_text_exit}"

#-----------------------------------------------------------------------
# C2 — diff coverage compares two CoverageReport JSON snapshots.
# Use --strict mode + flipped results so the two snapshots actually
# differ (without --strict, results-file changes don't affect
# coverage_pct so the snapshots would be byte-identical).
#-----------------------------------------------------------------------
echo "[C2] diff coverage"
snap_a="${WORKDIR}/d3/snap-a.json"
snap_b="${WORKDIR}/d3/snap-b.json"
(cd "${WORKDIR}/d3" && "${SPECTER_BIN}" coverage --strict --json > "${snap_a}" 2>/dev/null) || true
# Flip AC-01 to passed in results — under --strict this changes coverage.
cat > "${WORKDIR}/d3/.specter-results.json" <<'JSON'
{"results":[
  {"spec_id":"my-spec","ac_id":"AC-01","status":"passed","test_name":"x"},
  {"spec_id":"my-spec","ac_id":"AC-02","status":"passed","test_name":"x"}
]}
JSON
(cd "${WORKDIR}/d3" && "${SPECTER_BIN}" coverage --strict --json > "${snap_b}" 2>/dev/null) || true

c2_out=$("${SPECTER_BIN}" diff coverage "${snap_a}" "${snap_b}" 2>&1 || true)
check "C2.1" "diff coverage names my-spec in delta output" \
    bash -c "echo '${c2_out}' | grep -qi 'my-spec'"

#-----------------------------------------------------------------------
# C4 — resolve dependents against the Specter repo itself.
#-----------------------------------------------------------------------
echo "[C4] resolve dependents"
c4_out=$(cd "${SPECTER_REPO_ROOT}" && "${SPECTER_BIN}" resolve dependents spec-parse 2>&1 || true)
check "C4.1" "resolve dependents lists at least one dependent of spec-parse" \
    bash -c "echo '${c4_out}' | grep -E 'spec-(coverage|check|resolve|sync|doctor)' -q"

#-----------------------------------------------------------------------
# C5+C6 — reverse emits summary + handoff in non-JSON mode, suppresses
# in --json.
#-----------------------------------------------------------------------
echo "[C5+C6] reverse summary + handoff"
mkdir -p "${WORKDIR}/c5"
cat > "${WORKDIR}/c5/user.ts" <<'TS'
import { z } from 'zod';
export const UserSchema = z.object({
  email: z.string().email(),
});
TS
c5_text=$(cd "${WORKDIR}/c5" && "${SPECTER_BIN}" reverse --dry-run 2>&1 || true)
check "C5.1" "reverse non-JSON emits 'Found N constraints' summary line" \
    bash -c "echo '${c5_text}' | grep -qE 'Found [0-9]+ constraints?'"
check "C6.1" "reverse non-JSON emits 'specter explain' handoff line" \
    bash -c "echo '${c5_text}' | grep -q 'specter explain'"

c5_json=$(cd "${WORKDIR}/c5" && "${SPECTER_BIN}" reverse --dry-run --json 2>&1 || true)
check "C6.2" "reverse --json suppresses prose summary line" \
    bash -c "! echo '${c5_json}' | grep -qE 'Found [0-9]+ constraints? across'"

#-----------------------------------------------------------------------
# B — sync --strictness flag override.
#-----------------------------------------------------------------------
echo "[B] sync --strictness override"
b_help=$("${SPECTER_BIN}" sync --help 2>&1)
check "B.1" "sync --strictness flag registered in CLI" \
    bash -c "echo '${b_help}' | grep -q -- '--strictness'"

#-----------------------------------------------------------------------
# F3 — unreachable_annotation fires on a test that lacks the runner-
# visible token; @reachable manual suppresses it.
#-----------------------------------------------------------------------
echo "[F3] unreachable_annotation diagnostic"
mkdir -p "${WORKDIR}/f3/specs"
cat > "${WORKDIR}/f3/specs/x.spec.yaml" <<'YAML'
spec:
  id: x-spec
  version: "1.0.0"
  status: draft
  tier: 2
  context: { system: t, feature: t }
  objective: { summary: t }
  constraints:
    - id: C-01
      description: "MUST"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "t"
      references_constraints: ["C-01"]
      priority: high
YAML
# Test file with @ac AC-01 but no spec-id/AC-NN in subtest name AND no
# print/log of "@ac" — unreachable per F3.
cat > "${WORKDIR}/f3/foo_test.go" <<'GO'
package foo

import "testing"

// @spec x-spec
// @ac AC-01
func TestFoo(t *testing.T) {
    // no runner-visible spec-id/AC-NN token
}
GO
cat > "${WORKDIR}/f3/specter.yaml" <<'YAML'
system:
  name: t
settings:
  specs_dir: specs
YAML

f3_out=$(cd "${WORKDIR}/f3" && "${SPECTER_BIN}" check --test 2>&1 || true)
check "F3.1" "check --test reports unreachable_annotation diagnostic" \
    bash -c "echo '${f3_out}' | grep -qi unreachable"

# Add // @reachable manual at top of file — diagnostic suppressed
sed -i '1i // @reachable manual' "${WORKDIR}/f3/foo_test.go"
f3_off=$(cd "${WORKDIR}/f3" && "${SPECTER_BIN}" check --test 2>&1 || true)
check "A4.1" "@reachable manual off-switch suppresses the unreachable_annotation diagnostic" \
    bash -c "! echo '${f3_off}' | grep -qi unreachable"

#-----------------------------------------------------------------------
# Emit the report
#-----------------------------------------------------------------------
{
    echo "# v0.13.0 Feature Smoke Test"
    echo
    echo "End-to-end CLI exercise of each new v0.13 feature against synthetic"
    echo "fixture workspaces (with one cross-check against the Specter repo for"
    echo "the \`resolve dependents\` query)."
    echo
    echo "- binary: \`${SPECTER_BIN}\`"
    echo "- host commit: \`$(cd "${SPECTER_REPO_ROOT}" && git rev-parse --short HEAD)\`"
    echo "- date: $(date -u +%Y-%m-%d)"
    echo
    echo "## Results"
    echo
    printf '%s\n' "${LINES[@]}"
    echo
    echo "## Tally"
    echo
    echo "- **PASS**: ${pass}"
    echo "- **FAIL**: ${fail}"
    echo
    if [[ "${fail}" -gt 0 ]]; then
        echo "## Findings"
        echo
        if [[ "${f3_out:-}" != *unreachable* ]]; then
            echo "### F3 — \`unreachable_annotation\` is not wired into any CLI command"
            echo
            echo "The function \`checker.CheckUnreachableAnnotations\` exists in"
            echo "\`internal/checker/\` and is fully unit-tested, but has zero callers"
            echo "in \`cmd/specter/\`, \`internal/sync/\`, or \`internal/coverage/\`."
            echo "End users running \`specter check --test\`, \`specter coverage\`, or"
            echo "\`specter sync\` will never see \`unreachable_annotation\` diagnostics."
            echo
            echo "spec-check 1.3.0 promises the diagnostic (C-10, AC-13..AC-18); the"
            echo "unit tests verify the pure function; the CLI plumbing was never"
            echo "added. This makes the headline v0.13 feature dead code from the"
            echo "user's perspective."
            echo
            echo "**A4 caveat:** the \`@reachable manual\` off-switch test (A4.1)"
            echo "PASSes only trivially — there are no diagnostics to suppress in"
            echo "the first place. Treat A4.1 as inconclusive until F3 is wired."
            echo
            echo "**Likely fix surface:** invoke \`CheckUnreachableAnnotations\` from"
            echo "\`coverage --strict\` (or \`check --test\`), route the resulting"
            echo "\`CheckDiagnostic\` entries through the existing diagnostic-printing"
            echo "code path, and gate the severity by the active strictness mode"
            echo "per C-10."
        fi
    fi
} > "${RESULTS_FILE}"

echo
echo "wrote ${RESULTS_FILE}"
echo "pass=${pass} fail=${fail}"

exit $((fail > 0 ? 1 : 0))
