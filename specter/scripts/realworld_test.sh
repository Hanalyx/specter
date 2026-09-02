#!/usr/bin/env bash
# Real-world pre-release test for Specter.
#
# Clones 12 open-source repositories at HEAD (shallow), runs the reverse
# compiler against each, and then runs the pipeline ON REVERSE'S OWN OUTPUT.
#
# Stage 1, extraction:
#   - FilesProcessed / SpecsGenerated / AssertionsFound
#   - Diagnostic count, exit code, wall time
#
# Stage 2, the round trip. Can Specter consume what Specter produced?
#   - parse    do the generated specs parse
#   - resolve  do they form a resolvable graph (id collisions surface here)
#   - check    orphan constraints, structural conflicts, duplicate AC ids
#
# The round trip is the half the original harness could not see. It ran
# `reverse --dry-run` and counted crashes, so a run that exits 0 and emits
# valid JSON scored clean even when its output was unusable. bugs/SP-SP-060
# was found the first time the pipeline was pointed at reverse output, on
# Specter's own source.
#
# `coverage` and `sync` are deliberately NOT run. A public repo carries no
# @spec/@ac annotations, so coverage is 0% by construction and any number from
# it would measure the repo's annotation habits, not Specter.
#
# One `reverse --json -o` per repo serves both stages: the JSON report on
# stdout, the generated files on disk. That needed SP-SP-061 fixed, which
# separated `--json` (report format) from `--dry-run` (whether files are
# written). Before that the harness scanned every repo twice.
#
# One reading caution. `check` refuses to run structural checks while `resolve`
# has errors, so a repo that trips SP-SP-060 reports zero structural conflicts
# because none were looked for, not because none exist.
#
# Mirrors the v0.2.x evidence-base test (docs/IMPROVEMENT_ROADMAP.md).
#
# Usage:
#   scripts/realworld_test.sh [WORKDIR]
#
# WORKDIR defaults to a temp directory; clones land there.
# Results are written to docs/release-testing/<version>-realworld.md, named by
# the binary's own version so a re-run does not overwrite an earlier baseline.

set -euo pipefail

SPECTER_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPECTER_BIN="${SPECTER_REPO_ROOT}/bin/specter"
WORKDIR="${1:-$(mktemp -d -t specter-rwtest-XXXXXX)}"
RESULTS_DIR="${SPECTER_REPO_ROOT}/docs/release-testing"
PER_REPO_LOG="${WORKDIR}/per-repo"

mkdir -p "${WORKDIR}" "${RESULTS_DIR}" "${PER_REPO_LOG}"

if [[ ! -x "${SPECTER_BIN}" ]]; then
    echo "error: specter binary not found at ${SPECTER_BIN}. Run 'make build' first" >&2
    exit 1
fi

SPECTER_VERSION="$(${SPECTER_BIN} --version 2>/dev/null | head -1 || echo unknown)"
HOST_GIT_COMMIT="$(cd "${SPECTER_REPO_ROOT}" && git rev-parse --short HEAD)"

# Name the report by the version under test. The original hardcoded
# v0.13.0-realworld.md, so re-running would have overwritten the only
# baseline that existed and destroyed the comparison the test is for.
VERSION_SLUG="$(printf '%s' "${SPECTER_VERSION}" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)"
[[ -n "${VERSION_SLUG}" ]] || VERSION_SLUG="unknown"
RESULTS_FILE="${RESULTS_DIR}/v${VERSION_SLUG}-realworld.md"

echo "specter: ${SPECTER_VERSION}"
echo "host commit: ${HOST_GIT_COMMIT}"
echo "workdir: ${WORKDIR}"
echo "results: ${RESULTS_FILE}"
echo

# Repo list: name | language | clone URL
# Ordered small-to-large so harness bugs surface before the big clones.
REPOS=(
    "go-chi/chi|Go|https://github.com/go-chi/chi.git"
    "go-playground/validator|Go|https://github.com/go-playground/validator.git"
    "fastapi/full-stack-fastapi-template|Python|https://github.com/fastapi/full-stack-fastapi-template.git"
    "t3-oss/create-t3-app|TS/Next.js|https://github.com/t3-oss/create-t3-app.git"
    "TanStack/router|TypeScript|https://github.com/TanStack/router.git"
    "gofiber/fiber|Go|https://github.com/gofiber/fiber.git"
    "trpc/trpc|TypeScript|https://github.com/trpc/trpc.git"
    "payloadcms/payload|TS/Next.js|https://github.com/payloadcms/payload.git"
    "refinedev/refine|React/TS|https://github.com/refinedev/refine.git"
    "pydantic/pydantic|Python|https://github.com/pydantic/pydantic.git"
    "calcom/cal.com|TS/Next.js|https://github.com/calcom/cal.com.git"
    "django/django|Python|https://github.com/django/django.git"
)

declare -a ROW_DATA
declare -a ROUND_TRIP

# grep -c on a file with zero matches prints "0" and exits 1. Under `set -e`
# and command substitution that turns into a two-line result. Always one line.
count_kind() {
    local n
    n=$(grep -c "\[$1\]" "$2" 2>/dev/null) || n=0
    printf '%s' "${n:-0}"
}

run_one() {
    local entry="$1"
    local name lang url
    IFS='|' read -r name lang url <<< "${entry}"

    local slug="${name//\//_}"
    local clone_dir="${WORKDIR}/${slug}"
    local log_file="${PER_REPO_LOG}/${slug}.log"
    local json_file="${PER_REPO_LOG}/${slug}.json"

    echo "=== ${name} (${lang}) ==="

    if [[ ! -d "${clone_dir}/.git" ]]; then
        if ! git clone --depth=1 --quiet "${url}" "${clone_dir}" 2>"${log_file}.clone"; then
            echo "  CLONE FAILED, see ${log_file}.clone"
            ROW_DATA+=("${name}|${lang}|CLONE FAILED|-|-|-|-|-|-|-")
            ROUND_TRIP+=("${name}|-|CLONE FAILED|-|-|-|-|-|-|-")
            return
        fi
    fi

    local commit_sha
    commit_sha="$(cd "${clone_dir}" && git rev-parse --short HEAD)"

    local start_ts end_ts elapsed
    start_ts=$(date +%s)
    local exit_code=0
    # Writes the specs stage 2 needs AND emits the stage-1 JSON, in one scan.
    rm -rf "${WORKDIR}/generated/${slug}"; mkdir -p "${WORKDIR}/generated/${slug}/specs"
    (cd "${clone_dir}" && "${SPECTER_BIN}" reverse --json . \
        -o "${WORKDIR}/generated/${slug}/specs" > "${json_file}" 2> "${log_file}") || exit_code=$?
    end_ts=$(date +%s)
    elapsed=$((end_ts - start_ts))

    if [[ ${exit_code} -ne 0 ]] || ! python3 -c "import json; json.load(open('${json_file}'))" 2>/dev/null; then
        echo "  CRASH or non-JSON output, exit ${exit_code}; see ${log_file}"
        ROW_DATA+=("${name}|${lang}|${commit_sha}|CRASH (exit ${exit_code})|-|-|-|-|-|${elapsed}s")
        ROUND_TRIP+=("${name}|-|CRASH|-|-|-|-|-|-|-")
        return
    fi

    # Extract metrics from JSON
    local metrics
    metrics=$(python3 <<PYEOF
import json
with open("${json_file}") as f:
    d = json.load(f)
s = d.get("summary") or {}
diags = d.get("diagnostics") or []
errors = sum(1 for x in diags if (x.get("severity") or "").lower() == "error")
print(f"{s.get('FilesProcessed', 0)}|{s.get('SpecsGenerated', 0)}|{s.get('AssertionsFound', 0)}|{errors}|{s.get('ConstraintsFound', 0)}|{s.get('GapsDetected', 0)}")
PYEOF
)

    IFS='|' read -r files specs assertions errors constraints gaps <<< "${metrics}"
    echo "  files=${files} specs=${specs} constraints=${constraints} assertions=${assertions}" \
         "gaps=${gaps} errors=${errors} time=${elapsed}s"

    # Assertions-per-file, to two decimals. A repo the extractor understands
    # nothing about scores 0 errors and 0 crashes, identically to one it
    # handles perfectly. t3-oss/create-t3-app yielded 0 assertions from 183
    # files in the v0.12.1 run and was indistinguishable from a clean result.
    local ratio="0.00"
    if [[ "${files}" -gt 0 ]]; then
        ratio=$(python3 -c "print(f'{${assertions}/${files}:.2f}')")
    fi
    local extraction="ok"
    if [[ "${assertions}" -eq 0 && "${files}" -gt 0 ]]; then
        extraction="ZERO"
        echo "  extraction: ZERO assertions from ${files} files, not a clean result"
    fi

    # --- Stage 2: the round trip. Can Specter consume its own output? ---
    #
    # This is a second `reverse` over the same tree. It costs a full re-scan and
    # it cannot be avoided: `--json` returns before the write loop, so
    # `reverse --json -o DIR` reports specs generated and writes nothing
    # (`bugs/SP-SP-061`). Stage 1 needs the JSON, stage 2 needs the files.
    local out_dir="${WORKDIR}/generated/${slug}"
    local parse_rc="-" resolve_rc="-" check_rc="-"
    local dup_ids="-" orphans="-" structconf="-" dupacs="-"

    # The per-spec GENERATED and SKIPPED lines are on stderr under --json, so
    # the stage-1 log doubles as the generation log.
    cp "${log_file}" "${PER_REPO_LOG}/${slug}.gen.log" 2>/dev/null || true

    local gen_count
    gen_count=$(find "${out_dir}/specs" -name '*.spec.yaml' 2>/dev/null | wc -l | tr -d ' ')
    if [[ "${gen_count}" -gt 0 ]]; then
        # A manifest with specs_dir lets parse/resolve/check discover the tree
        # with no arguments, so no generated filename is ever word-split.
        printf 'schema_version: 1\nsystem:\n  name: rw\nsettings:\n  specs_dir: specs\n' \
            > "${out_dir}/specter.yaml"

        (cd "${out_dir}" && "${SPECTER_BIN}" parse \
            > "${PER_REPO_LOG}/${slug}.parse.log" 2>&1) && parse_rc=0 || parse_rc=$?
        (cd "${out_dir}" && "${SPECTER_BIN}" resolve \
            > "${PER_REPO_LOG}/${slug}.resolve.log" 2>&1) && resolve_rc=0 || resolve_rc=$?
        (cd "${out_dir}" && "${SPECTER_BIN}" check \
            > "${PER_REPO_LOG}/${slug}.check.log" 2>&1) && check_rc=0 || check_rc=$?

        # grep -c prints 0 and exits 1 when the file has no match, so a bare
        # `|| echo 0` captures "0\n0" and injects a newline into the row.
        dup_ids=$(count_kind "duplicate_id" "${PER_REPO_LOG}/${slug}.resolve.log")
        orphans=$(count_kind "orphan_constraint" "${PER_REPO_LOG}/${slug}.check.log")
        structconf=$(count_kind "structural_conflict" "${PER_REPO_LOG}/${slug}.check.log")
        dupacs=$(count_kind "duplicate_ac_id" "${PER_REPO_LOG}/${slug}.check.log")
        echo "  round trip: parse=${parse_rc} resolve=${resolve_rc} check=${check_rc}" \
             "dup_ids=${dup_ids} orphans=${orphans} struct_conflicts=${structconf}"
    else
        echo "  round trip: reverse wrote no spec files; pipeline not run"
    fi

    ROW_DATA+=("${name}|${lang}|${commit_sha}|${files}|${specs}|${constraints}|${assertions}|${gaps}|${errors}|${elapsed}s")
    ROUND_TRIP+=("${name}|${ratio}|${extraction}|${parse_rc}|${resolve_rc}|${check_rc}|${dup_ids}|${orphans}|${structconf}|${dupacs}")
}

for entry in "${REPOS[@]}"; do
    run_one "${entry}" || echo "  (continuing past error)"
done

# Aggregate totals (only over rows with numeric data)
TOTAL_FILES=0
TOTAL_SPECS=0
TOTAL_ASSERTIONS=0
TOTAL_CONSTRAINTS=0
TOTAL_GAPS=0
TOTAL_ERRORS=0
CRASH_COUNT=0
ZERO_EXTRACTION=0
RESOLVE_FAIL=0
CHECK_FAIL=0
PIPELINE_CLEAN=0
for row in "${ROUND_TRIP[@]}"; do
    IFS='|' read -r _ _ extraction prc rrc crc _ _ _ _ <<< "${row}"
    [[ "${extraction}" == "ZERO" ]] && ZERO_EXTRACTION=$((ZERO_EXTRACTION + 1))
    [[ "${rrc}" != "0" && "${rrc}" != "-" ]] && RESOLVE_FAIL=$((RESOLVE_FAIL + 1))
    [[ "${crc}" != "0" && "${crc}" != "-" ]] && CHECK_FAIL=$((CHECK_FAIL + 1))
    [[ "${prc}" == "0" && "${rrc}" == "0" && "${crc}" == "0" ]] && PIPELINE_CLEAN=$((PIPELINE_CLEAN + 1))
done
for row in "${ROW_DATA[@]}"; do
    IFS='|' read -r _ _ _ files specs constraints assertions gaps errors _ <<< "${row}"
    if [[ "${files}" == CRASH* ]] || [[ "${files}" == "-" ]]; then
        CRASH_COUNT=$((CRASH_COUNT + 1))
        continue
    fi
    TOTAL_FILES=$((TOTAL_FILES + files))
    TOTAL_SPECS=$((TOTAL_SPECS + specs))
    TOTAL_CONSTRAINTS=$((TOTAL_CONSTRAINTS + constraints))
    TOTAL_ASSERTIONS=$((TOTAL_ASSERTIONS + assertions))
    TOTAL_GAPS=$((TOTAL_GAPS + gaps))
    TOTAL_ERRORS=$((TOTAL_ERRORS + errors))
done

# Emit the Markdown report
{
    echo "# v${VERSION_SLUG} Real-World Test Results"
    echo
    echo "Pre-release validation against the 12 open-source repositories from"
    echo "the v0.2.x evidence base. Two stages: \`specter reverse\` over each"
    echo "repo, then the pipeline over what \`reverse\` produced."
    echo
    echo "- specter: \`${SPECTER_VERSION}\`"
    echo "- host commit: \`${HOST_GIT_COMMIT}\`"
    echo "- workdir: \`${WORKDIR}\`"
    echo "- date: $(date -u +%Y-%m-%d)"
    echo
    echo "## Per-repo results"
    echo
    echo "| Repo | Language | HEAD | Files | Specs | Constraints | Assertions | Gaps | Errors | Time |"
    echo "|------|----------|------|-------|-------|-------------|------------|------|--------|------|"
    for row in "${ROW_DATA[@]}"; do
        IFS='|' read -r name lang sha files specs constraints assertions gaps errors time <<< "${row}"
        echo "| ${name} | ${lang} | \`${sha}\` | ${files} | ${specs} | ${constraints} | ${assertions} | ${gaps} | ${errors} | ${time} |"
    done
    echo "| **TOTAL** | | | **${TOTAL_FILES}** | **${TOTAL_SPECS}** | **${TOTAL_CONSTRAINTS}** | **${TOTAL_ASSERTIONS}** | **${TOTAL_GAPS}** | **${TOTAL_ERRORS}** | |"
    echo
    echo "**The gaps column is not a measure of test coverage.** A constraint"
    echo "counts as covered only when an assertion's text contains the"
    echo "constraint's field name, and real test names rarely do, so most of"
    echo "this column is false. It is recorded to characterize the extractor,"
    echo "not the repositories. See \`bugs/SP-SP-064\`, which is deferred: the"
    echo "reverse compiler is a bootstrap tool and this number is not a claim"
    echo "it is being asked to support."
    echo
    echo "## Round trip: can Specter consume its own output"
    echo
    echo "The pipeline run against the specs \`reverse\` generated. A non-zero"
    echo "rc here means the reverse compiler produced a workspace the rest of the"
    echo "toolchain refuses. \`coverage\` and \`sync\` are not run: a public repo"
    echo "carries no \`@spec\`/\`@ac\` annotations, so coverage is 0% by"
    echo "construction and gating on it would measure nothing."
    echo
    echo "| Repo | Assert/file | Extraction | parse | resolve | check | dup ids | orphans | struct conflicts | dup ACs |"
    echo "|------|------------|------------|-------|---------|-------|---------|---------|------------------|---------|"
    for row in "${ROUND_TRIP[@]}"; do
        IFS='|' read -r name ratio extraction prc rrc crc dups orph sc dac <<< "${row}"
        echo "| ${name} | ${ratio} | ${extraction} | ${prc} | ${rrc} | ${crc} | ${dups} | ${orph} | ${sc} | ${dac} |"
    done
    echo
    echo "## Findings"
    echo
    echo "- Crashes (non-zero exit / non-JSON output): **${CRASH_COUNT}** of ${#ROW_DATA[@]}"
    echo "- Total validation errors across extraction diagnostics: **${TOTAL_ERRORS}**"
    echo "- Repos extracting **zero** assertions: **${ZERO_EXTRACTION}**"
    echo "- Repos whose generated specs fail \`resolve\`: **${RESOLVE_FAIL}**"
    echo "- Repos whose generated specs fail \`check\`: **${CHECK_FAIL}**"
    echo
    echo "### How to read these"
    echo
    echo "**Workspaces the pipeline accepts: ${PIPELINE_CLEAN} of ${#ROUND_TRIP[@]}.**"
    echo "The harness measured only crashes and extraction"
    echo "diagnostics before this column existed. Extraction is not the same"
    echo "property as usability, and earlier runs reported the first while the"
    echo "release notes read it as the second."
    echo
    echo "**Zero crashes is not correct extraction.** A repo the adapters"
    echo "understand nothing about exits 0 and emits valid JSON, scoring"
    echo "identically to one handled perfectly. The assertions-per-file column and"
    echo "the ZERO marker exist so that case is visible rather than averaged away."
    echo
    echo "**The struct conflicts column cannot be non-zero here, and its zero"
    echo "means nothing.** The scan iterates dependency edges with"
    echo "relationship \`requires\`, and \`reverse\` emits no \`depends_on\`"
    echo "at all: every generated workspace in this run resolves to 0"
    echo "dependencies. The scan has no input, so it reports no conflicts"
    echo "whether or not any exist. The column is kept because it will start"
    echo "meaning something the day \`reverse\` emits dependency edges."
    echo
    echo "**A run over reverse output therefore cannot supply the number"
    echo "\`bugs/SP-SP-004\` needs.** That defect is P0 and its"
    echo "false-positive rate has only ever"
    echo "been measured against the corpus the checker authors wrote"
    echo "themselves. Measuring it on foreign code needs a corpus of specs"
    echo "with real dependency edges, which reverse-compiled drafts are not."
    echo "Reading a zero here as evidence would be reading the absence of a"
    echo "measurement as a passing one."
    echo
    echo "A second reason a zero can be empty: \`check\` refuses structural"
    echo "checks while \`resolve\` has errors, so a row with a non-zero"
    echo "resolve rc reports zeros because nothing was looked for."
    echo
    echo "**Orphan constraints are what \`check\` finds when it does run**, and"
    echo "a non-zero count in that column is not by itself a failure."
    echo "\`reverse\` writes constraints that no acceptance criterion"
    echo "references, which is an expected state of a draft. They are reported"
    echo "at the tier default, meaning info on the tier-3 specs \`reverse\`"
    echo "writes, so they do not fail the gate."
    echo
    echo "That was not true before \`bugs/SP-SP-062\` was fixed on 2026-08-23."
    echo "\`reverse\` stamped \`enforcement: error\` on every constraint it"
    echo "generated, which overrode the tier default. In the v0.14.1 run both"
    echo "repositories that cleared \`resolve\` failed \`check\` on it, 37 and"
    echo "61 diagnostics, all \`orphan_constraint\`, none at info. If a row"
    echo "shows resolve 0 and check non-zero, read the log: the cause is"
    echo "something other than orphan constraints."
    echo
    echo "Per-repo logs, generated specs, and JSON output: \`${PER_REPO_LOG}/\`"
    echo "and \`${WORKDIR}/generated/\`"
} > "${RESULTS_FILE}"

echo
echo "wrote ${RESULTS_FILE}"
echo "crashes:              ${CRASH_COUNT}/${#ROW_DATA[@]}"
echo "extraction errors:    ${TOTAL_ERRORS}"
echo "zero-extraction repos: ${ZERO_EXTRACTION}"
echo "resolve failures:      ${RESOLVE_FAIL}"
echo "check failures:        ${CHECK_FAIL}"
echo "clean round trips:     ${PIPELINE_CLEAN}/${#ROUND_TRIP[@]}"
