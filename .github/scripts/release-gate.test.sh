#!/usr/bin/env bash
# Tests for release-gate.sh. Each case builds a fake GitHub API from fixture
# files and runs the gate against it. No network access and no real
# repository state are involved.
#
# Run: bash .github/scripts/release-gate.test.sh

set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
gate="$here/release-gate.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

REPO_NAME="Hanalyx/specter"
GOOD_SHA="58c1e93dc4fad83522ec22a21ba8537e7211c7be"
OTHER_SHA="1111111111111111111111111111111111111111"
TAG_OBJ="f2e63fd7beb0e17b121deeda9e139962d7638f1d"

# The fake gh answers `gh api <endpoint>` from $FIXTURES/<key>.json, where
# the key is the endpoint with / ? & = replaced by _. A missing file is a
# 404, which gh reports by exiting nonzero.
mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
[ "$1" = "api" ] || exit 2
key=$(printf '%s' "$2" | tr '/?&=' '____')
[ -f "$FIXTURES/$key.json" ] || { echo "HTTP 404" >&2; exit 1; }
cat "$FIXTURES/$key.json"
EOF
chmod +x "$work/bin/gh"

fixture() {
  local key
  key=$(printf '%s' "$1" | tr '/?&=' '____')
  printf '%s' "$2" >"$FIXTURES/$key.json"
}

# A repository where tag v0.15.1 is annotated, points at GOOD_SHA, sits on
# main, has VERSION 0.15.1, and has one successful upstream push run.
good_repo() {
  FIXTURES="$work/fx-$1"
  rm -rf "$FIXTURES" && mkdir -p "$FIXTURES"
  export FIXTURES
  fixture "repos/$REPO_NAME/git/ref/tags/v0.15.1" \
    "{\"object\":{\"type\":\"tag\",\"sha\":\"$TAG_OBJ\"}}"
  fixture "repos/$REPO_NAME/git/tags/$TAG_OBJ" \
    "{\"object\":{\"type\":\"commit\",\"sha\":\"$GOOD_SHA\"}}"
  fixture "repos/$REPO_NAME/compare/main...$GOOD_SHA" '{"status":"behind"}'
  fixture "repos/$REPO_NAME/contents/specter/VERSION?ref=$GOOD_SHA" \
    "{\"content\":\"$(printf '0.15.1\n' | base64)\"}"
  fixture "repos/$REPO_NAME/actions/workflows/pre-release-test.yml/runs?event=push&head_sha=$GOOD_SHA&status=success&per_page=100" \
    "{\"workflow_runs\":[{\"head_branch\":\"v0.15.1\",\"head_repository\":{\"full_name\":\"$REPO_NAME\"},\"path\":\".github/workflows/pre-release-test.yml\",\"conclusion\":\"success\"}]}"
}

run_env() {
  env -i PATH="$work/bin:/usr/bin:/bin" FIXTURES="$FIXTURES" \
    REPO="$REPO_NAME" GITHUB_OUTPUT="$work/out" "$@" bash "$gate"
}

push_run() {
  run_env GATE_EVENT=workflow_run RUN_EVENT=push RUN_CONCLUSION=success \
    RUN_HEAD_REPO="$REPO_NAME" RUN_HEAD_BRANCH=v0.15.1 RUN_HEAD_SHA="$GOOD_SHA" \
    RUN_WORKFLOW_PATH=.github/workflows/pre-release-test.yml "$@"
}

dispatch_run() {
  run_env GATE_EVENT=workflow_dispatch DISPATCH_REF=refs/heads/main DISPATCH_TAG=v0.15.1 "$@"
}

pass=0
fail=0

# expect <name> <allow|reject> <substring of the output> -- <command...>
expect() {
  local name=$1 want=$2 needle=$3
  shift 4
  : >"$work/out"
  local out code
  out=$("$@" 2>&1)
  code=$?
  local ok=1
  if [ "$want" = "allow" ]; then
    [ $code -eq 0 ] && grep -q "tag=v0.15.1" "$work/out" && grep -q "sha=$GOOD_SHA" "$work/out" || ok=0
  else
    [ $code -ne 0 ] && [ ! -s "$work/out" ] || ok=0
  fi
  grep -qF -- "$needle" <<<"$out" || ok=0
  if [ $ok -eq 1 ]; then
    pass=$((pass + 1))
    echo "ok   $name"
  else
    fail=$((fail + 1))
    echo "FAIL $name (exit $code)"
    echo "     $out"
  fi
}

# Accepted paths.
good_repo allow-push
expect "tag push that passed the suite" allow "ALLOWED: v0.15.1" -- push_run
good_repo allow-dispatch
expect "manual dispatch of a tested tag" allow "ALLOWED: v0.15.1" -- dispatch_run
good_repo allow-identical
fixture "repos/$REPO_NAME/compare/main...$GOOD_SHA" '{"status":"identical"}'
expect "tag at the tip of main" allow "ALLOWED" -- push_run

# Untrusted triggers.
good_repo pr
expect "pull request run" reject "event is 'pull_request'" -- push_run RUN_EVENT=pull_request
good_repo fork
expect "fork pull request run" reject "event is 'pull_request'" -- \
  push_run RUN_EVENT=pull_request RUN_HEAD_REPO=attacker/specter
good_repo fork-push
expect "push run from a fork repository" reject "came from 'attacker/specter'" -- \
  push_run RUN_HEAD_REPO=attacker/specter
good_repo failed
expect "upstream suite failed" reject "conclusion is 'failure'" -- push_run RUN_CONCLUSION=failure
good_repo other-workflow
expect "a different workflow with the same name" reject "upstream workflow is" -- \
  push_run RUN_WORKFLOW_PATH=.github/workflows/evil.yml
good_repo dispatch-branch
expect "dispatch from a non-main ref" reject "must run from main" -- \
  dispatch_run DISPATCH_REF=refs/heads/v0.15.1
good_repo dispatch-notag
expect "dispatch without a tag" reject "needs a tag input" -- dispatch_run DISPATCH_TAG=
good_repo event
expect "unsupported event" reject "unsupported event" -- run_env GATE_EVENT=push

# Branches and tags.
good_repo branch
expect "branch named like a release, no such tag" reject "does not exist" -- \
  push_run RUN_HEAD_BRANCH=v0.15.9
good_repo name
expect "branch name that is not a release tag" reject "is not a release tag name" -- \
  push_run RUN_HEAD_BRANCH=v-anything
good_repo inject
expect "shell metacharacters in the branch name" reject "is not a release tag name" -- \
  push_run 'RUN_HEAD_BRANCH=v1.0.0;touch${IFS}/tmp/pwned'
good_repo missing
rm -f "$FIXTURES"/repos_Hanalyx_specter_git_ref_tags_v0.15.1.json
expect "tag missing" reject "does not exist" -- push_run
good_repo lightweight
fixture "repos/$REPO_NAME/git/ref/tags/v0.15.1" \
  "{\"object\":{\"type\":\"commit\",\"sha\":\"$GOOD_SHA\"}}"
expect "lightweight tag" reject "must be annotated" -- push_run
good_repo mismatch
expect "tag points at a different commit than was tested" reject "but the tested commit is $OTHER_SHA" -- \
  push_run RUN_HEAD_SHA="$OTHER_SHA"
good_repo moved
fixture "repos/$REPO_NAME/git/tags/$TAG_OBJ" \
  "{\"object\":{\"type\":\"commit\",\"sha\":\"$OTHER_SHA\"}}"
expect "tag moved after the suite ran" reject "resolves to $OTHER_SHA" -- push_run

# Release branch policy.
good_repo ahead
fixture "repos/$REPO_NAME/compare/main...$GOOD_SHA" '{"status":"ahead"}'
expect "commit not merged to main" reject "is not on main (compare status: ahead)" -- push_run
good_repo diverged
fixture "repos/$REPO_NAME/compare/main...$GOOD_SHA" '{"status":"diverged"}'
expect "commit on a diverged branch" reject "compare status: diverged" -- push_run
good_repo version
fixture "repos/$REPO_NAME/contents/specter/VERSION?ref=$GOOD_SHA" \
  "{\"content\":\"$(printf '0.15.0\n' | base64)\"}"
expect "VERSION does not match the tag" reject "is '0.15.0', which does not match v0.15.1" -- push_run

# Manual dispatch needs the same upstream evidence as the automatic path.
good_repo dispatch-untested
fixture "repos/$REPO_NAME/actions/workflows/pre-release-test.yml/runs?event=push&head_sha=$GOOD_SHA&status=success&per_page=100" \
  '{"workflow_runs":[]}'
expect "dispatch of a tag the suite never passed" reject "no successful push run" -- dispatch_run
good_repo dispatch-prrun
fixture "repos/$REPO_NAME/actions/workflows/pre-release-test.yml/runs?event=push&head_sha=$GOOD_SHA&status=success&per_page=100" \
  "{\"workflow_runs\":[{\"head_branch\":\"v0.15.1\",\"head_repository\":{\"full_name\":\"attacker/specter\"},\"path\":\".github/workflows/pre-release-test.yml\",\"conclusion\":\"success\"}]}"
expect "dispatch where the only passing run came from a fork" reject "no successful push run" -- dispatch_run
good_repo dispatch-missing
rm -f "$FIXTURES"/repos_Hanalyx_specter_git_ref_tags_v0.15.1.json
expect "dispatch of a missing tag" reject "does not exist" -- dispatch_run

echo
echo "$pass passed, $fail failed"
[ $fail -eq 0 ]
