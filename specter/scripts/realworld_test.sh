#!/usr/bin/env bash
# Real-world pre-release test for Specter.
#
# Clones 12 open-source repositories at HEAD (shallow) and runs
# `specter reverse --dry-run --json` against each, capturing:
#   - FilesProcessed / SpecsGenerated / AssertionsFound
#   - Diagnostic count (validation failures)
#   - Exit code (non-zero = crash signal)
#   - Wall time
#
# Mirrors the v0.2.x evidence-base test (docs/IMPROVEMENT_ROADMAP.md).
#
# Usage:
#   scripts/realworld_test.sh [WORKDIR]
#
# WORKDIR defaults to a temp directory; clones land there.
# Results are written to docs/release-testing/v0.13.0-realworld.md.

set -euo pipefail

SPECTER_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPECTER_BIN="${SPECTER_REPO_ROOT}/bin/specter"
WORKDIR="${1:-$(mktemp -d -t specter-rwtest-XXXXXX)}"
RESULTS_DIR="${SPECTER_REPO_ROOT}/docs/release-testing"
RESULTS_FILE="${RESULTS_DIR}/v0.13.0-realworld.md"
PER_REPO_LOG="${WORKDIR}/per-repo"

mkdir -p "${WORKDIR}" "${RESULTS_DIR}" "${PER_REPO_LOG}"

if [[ ! -x "${SPECTER_BIN}" ]]; then
    echo "error: specter binary not found at ${SPECTER_BIN} — run 'make build' first" >&2
    exit 1
fi

SPECTER_VERSION="$(${SPECTER_BIN} --version 2>/dev/null | head -1 || echo unknown)"
HOST_GIT_COMMIT="$(cd "${SPECTER_REPO_ROOT}" && git rev-parse --short HEAD)"

echo "specter: ${SPECTER_VERSION}"
echo "host commit: ${HOST_GIT_COMMIT}"
echo "workdir: ${WORKDIR}"
echo "results: ${RESULTS_FILE}"
echo

# Repo list — name | language | clone URL
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
            echo "  CLONE FAILED — see ${log_file}.clone"
            ROW_DATA+=("${name}|${lang}|CLONE FAILED|-|-|-|-|-")
            return
        fi
    fi

    local commit_sha
    commit_sha="$(cd "${clone_dir}" && git rev-parse --short HEAD)"

    local start_ts end_ts elapsed
    start_ts=$(date +%s)
    local exit_code=0
    (cd "${clone_dir}" && "${SPECTER_BIN}" reverse --dry-run --json . > "${json_file}" 2> "${log_file}") || exit_code=$?
    end_ts=$(date +%s)
    elapsed=$((end_ts - start_ts))

    if [[ ${exit_code} -ne 0 ]] || ! python3 -c "import json; json.load(open('${json_file}'))" 2>/dev/null; then
        echo "  CRASH or non-JSON output — exit ${exit_code}; see ${log_file}"
        ROW_DATA+=("${name}|${lang}|${commit_sha}|CRASH (exit ${exit_code})|-|-|-|${elapsed}s")
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
print(f"{s.get('FilesProcessed', 0)}|{s.get('SpecsGenerated', 0)}|{s.get('AssertionsFound', 0)}|{errors}")
PYEOF
)

    IFS='|' read -r files specs assertions errors <<< "${metrics}"
    echo "  files=${files} specs=${specs} assertions=${assertions} errors=${errors} time=${elapsed}s"

    ROW_DATA+=("${name}|${lang}|${commit_sha}|${files}|${specs}|${assertions}|${errors}|${elapsed}s")
}

for entry in "${REPOS[@]}"; do
    run_one "${entry}" || echo "  (continuing past error)"
done

# Aggregate totals (only over rows with numeric data)
TOTAL_FILES=0
TOTAL_SPECS=0
TOTAL_ASSERTIONS=0
TOTAL_ERRORS=0
CRASH_COUNT=0
for row in "${ROW_DATA[@]}"; do
    IFS='|' read -r _ _ _ files specs assertions errors _ <<< "${row}"
    if [[ "${files}" == CRASH* ]] || [[ "${files}" == "-" ]]; then
        CRASH_COUNT=$((CRASH_COUNT + 1))
        continue
    fi
    TOTAL_FILES=$((TOTAL_FILES + files))
    TOTAL_SPECS=$((TOTAL_SPECS + specs))
    TOTAL_ASSERTIONS=$((TOTAL_ASSERTIONS + assertions))
    TOTAL_ERRORS=$((TOTAL_ERRORS + errors))
done

# Emit the Markdown report
{
    echo "# v0.13.0 Real-World Test Results"
    echo
    echo "Pre-release validation of \`specter reverse --dry-run --json\` against"
    echo "the 12 open-source repositories from the v0.2.x evidence base."
    echo
    echo "- specter: \`${SPECTER_VERSION}\`"
    echo "- host commit: \`${HOST_GIT_COMMIT}\`"
    echo "- workdir: \`${WORKDIR}\`"
    echo "- date: $(date -u +%Y-%m-%d)"
    echo
    echo "## Per-repo results"
    echo
    echo "| Repo | Language | HEAD | Files | Specs | Assertions | Errors | Time |"
    echo "|------|----------|------|-------|-------|------------|--------|------|"
    for row in "${ROW_DATA[@]}"; do
        IFS='|' read -r name lang sha files specs assertions errors time <<< "${row}"
        echo "| ${name} | ${lang} | \`${sha}\` | ${files} | ${specs} | ${assertions} | ${errors} | ${time} |"
    done
    echo "| **TOTAL** | | | **${TOTAL_FILES}** | **${TOTAL_SPECS}** | **${TOTAL_ASSERTIONS}** | **${TOTAL_ERRORS}** | |"
    echo
    echo "## Findings"
    echo
    echo "- Crashes (non-zero exit / non-JSON output): **${CRASH_COUNT}** of ${#ROW_DATA[@]}"
    echo "- Total validation errors across diagnostics: **${TOTAL_ERRORS}**"
    echo
    echo "Per-repo logs and JSON output: \`${PER_REPO_LOG}/\`"
} > "${RESULTS_FILE}"

echo
echo "wrote ${RESULTS_FILE}"
echo "crashes: ${CRASH_COUNT}/${#ROW_DATA[@]}"
echo "total validation errors: ${TOTAL_ERRORS}"
