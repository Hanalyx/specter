#!/usr/bin/env bash
# Release gate. Decides whether release.yml may build and sign a release.
#
# Runs in a read-only job before any privileged step. It reads only the
# GitHub API and the event payload, and it never executes code from the
# commit being released. A release is allowed only when all of these hold:
#
#   1. The trigger is trusted. For workflow_run: a successful push run of
#      pre-release-test.yml in this repository. For workflow_dispatch: the
#      workflow runs from the release branch.
#   2. The tag is an annotated tag in this repository, named vX.Y.Z or
#      vX.Y.Z-pre, and it resolves to the exact commit that was tested.
#   3. That commit is reachable from the release branch (tags on main only).
#   4. specter/VERSION at that commit matches the tag.
#   5. For workflow_dispatch: a successful push run of pre-release-test.yml
#      exists for that tag at that commit.
#
# On success it writes tag= and sha= to $GITHUB_OUTPUT. On any failure it
# prints the reason and exits 1.
#
# Inputs come from the environment, never from interpolation into a shell
# line, because the branch name in a workflow_run payload is set by whoever
# opened the pull request.
#
#   GATE_EVENT          workflow_run | workflow_dispatch
#   REPO                owner/name of this repository
#   RUN_EVENT           workflow_run.event
#   RUN_CONCLUSION      workflow_run.conclusion
#   RUN_HEAD_REPO       workflow_run.head_repository.full_name
#   RUN_HEAD_BRANCH     workflow_run.head_branch (the tag name on a tag push)
#   RUN_HEAD_SHA        workflow_run.head_sha
#   RUN_WORKFLOW_PATH   workflow_run.path
#   DISPATCH_TAG        inputs.tag
#   DISPATCH_REF        github.ref of the dispatched run
#
# Tests: .github/scripts/release-gate.test.sh

set -euo pipefail

RELEASE_BRANCH="main"
UPSTREAM_WORKFLOW=".github/workflows/pre-release-test.yml"
TAG_PATTERN='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'

reject() {
  echo "release gate: REJECTED: $*" >&2
  exit 1
}

api() {
  gh api "$1" 2>/dev/null
}

: "${GATE_EVENT:?}" "${REPO:?}"

case "$GATE_EVENT" in
  workflow_run)
    [ "${RUN_EVENT:-}" = "push" ] ||
      reject "upstream run event is '${RUN_EVENT:-}', not push"
    [ "${RUN_CONCLUSION:-}" = "success" ] ||
      reject "upstream run conclusion is '${RUN_CONCLUSION:-}', not success"
    [ "${RUN_HEAD_REPO:-}" = "$REPO" ] ||
      reject "upstream run came from '${RUN_HEAD_REPO:-}', not $REPO"
    [ "${RUN_WORKFLOW_PATH:-}" = "$UPSTREAM_WORKFLOW" ] ||
      reject "upstream workflow is '${RUN_WORKFLOW_PATH:-}', not $UPSTREAM_WORKFLOW"
    tag="${RUN_HEAD_BRANCH:-}"
    tested_sha="${RUN_HEAD_SHA:-}"
    ;;
  workflow_dispatch)
    [ "${DISPATCH_REF:-}" = "refs/heads/$RELEASE_BRANCH" ] ||
      reject "manual dispatch must run from $RELEASE_BRANCH, not '${DISPATCH_REF:-}'"
    tag="${DISPATCH_TAG:-}"
    [ -n "$tag" ] || reject "manual dispatch needs a tag input"
    tested_sha=""
    ;;
  *)
    reject "unsupported event '$GATE_EVENT'"
    ;;
esac

[[ "$tag" =~ $TAG_PATTERN ]] || reject "'$tag' is not a release tag name"

# Resolve the tag through the repository's own refs. A branch that shares
# the name does not count, and a fork cannot create a tag here.
ref_json=$(api "repos/$REPO/git/ref/tags/$tag") ||
  reject "tag $tag does not exist in $REPO"
ref_type=$(jq -r '.object.type' <<<"$ref_json")
ref_obj=$(jq -r '.object.sha' <<<"$ref_json")
[ "$ref_type" = "tag" ] ||
  reject "tag $tag is a lightweight tag; release tags must be annotated"
tag_json=$(api "repos/$REPO/git/tags/$ref_obj") ||
  reject "annotated tag object for $tag could not be read"
[ "$(jq -r '.object.type' <<<"$tag_json")" = "commit" ] ||
  reject "tag $tag does not point at a commit"
tag_sha=$(jq -r '.object.sha' <<<"$tag_json")

if [ "$GATE_EVENT" = "workflow_run" ]; then
  [ "$tag_sha" = "$tested_sha" ] ||
    reject "tag $tag resolves to $tag_sha, but the tested commit is $tested_sha"
else
  runs_json=$(api "repos/$REPO/actions/workflows/pre-release-test.yml/runs?event=push&head_sha=$tag_sha&status=success&per_page=100") ||
    reject "could not list upstream runs for $tag_sha"
  matches=$(jq --arg tag "$tag" --arg repo "$REPO" --arg path "$UPSTREAM_WORKFLOW" \
    '[.workflow_runs[] | select(.head_branch == $tag and .head_repository.full_name == $repo and .path == $path and .conclusion == "success")] | length' \
    <<<"$runs_json")
  [ "$matches" -gt 0 ] ||
    reject "no successful push run of $UPSTREAM_WORKFLOW exists for $tag at $tag_sha"
fi

# "behind" or "identical" means the commit is reachable from the release
# branch. "ahead" or "diverged" means it is not on it.
compare_json=$(api "repos/$REPO/compare/$RELEASE_BRANCH...$tag_sha") ||
  reject "could not compare $tag_sha against $RELEASE_BRANCH"
status=$(jq -r '.status' <<<"$compare_json")
case "$status" in
  behind | identical) ;;
  *) reject "commit $tag_sha is not on $RELEASE_BRANCH (compare status: $status)" ;;
esac

version_json=$(api "repos/$REPO/contents/specter/VERSION?ref=$tag_sha") ||
  reject "specter/VERSION could not be read at $tag_sha"
version=$(jq -r '.content | @base64d' <<<"$version_json" | tr -d '[:space:]')
[ "v$version" = "$tag" ] ||
  reject "specter/VERSION at $tag_sha is '$version', which does not match $tag"

echo "release gate: ALLOWED: $tag at $tag_sha"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  {
    echo "tag=$tag"
    echo "sha=$tag_sha"
  } >>"$GITHUB_OUTPUT"
fi
