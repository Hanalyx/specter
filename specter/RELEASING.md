# Releasing Specter

This document defines the gate that must be satisfied before any `vsce publish` (VS Code extension) or `git push origin vX.Y.Z` (CLI release).

It exists because the v0.8.0 → v0.8.3 extension thrash (four patch releases in one day, each fixing a bug the first real user hit on install) proved that running `make check` and `vsce package` is not sufficient to ship. Marketplace is a distribution channel, not a test harness.

---

## The gate

Every step is mandatory. Skipping a step is not a shortcut. It is a commitment to ship a broken version.

### 1. CI green

`make check`, dogfood, and extension jest all pass in GitHub Actions. Not optional, not sufficient alone.

### 2. Install the packaged VSIX locally

```bash
code --install-extension specter-vscode-X.Y.Z.vsix --force
```

CI building the VSIX is not the same as VS Code loading it. The v0.6.5 → v0.6.6 bug (stale `out/` directory shipped in the VSIX) was invisible to CI.

### 3. Reload, then open a known-working test workspace

The `specter/` repo itself is a good choice (15 dogfood specs as of v0.10.0, after `spec-ingest` was added). The Coverage sidebar should populate with real entries, not the empty-state message.

### 4. Reload, then open a known-failing test workspace

A directory whose `.spec.yaml` files fail Specter's schema (e.g. specs written in an older custom schema). The Coverage sidebar should show the empty-state message, the status bar should be in the error state, and the Output channel should contain the parse errors.

### 5. Exercise every changed code path

If the change touches:
- **Binary resolution**: run **Specter: Re-download CLI** and watch the download into `~/.specter/cli/`. Deleting the private copy alone does not always trigger a download, because the extension first uses an in-range CLI from `specter.binaryPath`, PATH, or `~/.specter/bin/specter`.
- **Tree rendering**: verify both empty and populated states.
- **CLI flag handling**: invoke the affected command from the integrated terminal.
- **Activation flow**: open a workspace that matches the activation trigger but wasn't open when VS Code started.

If a change doesn't obviously route through one of these, write down the path it does exercise and test that explicitly.

### 6. Check the Output channel

`View → Output → Specter`. No unexplained red entries during normal use.

### 7. Human verifies and signs off

No AI / automated "I think it works" substitutes here. The person cutting the release reproduces the change in a live VS Code window and confirms the behavior matches intent. This is the step that has repeatedly failed during v0.8.x and is the reason this gate exists.

### 8. Only then run `vsce publish`

For non-trivial changes (schema shifts, new UI surfaces, binary-download code, new CLI flags), publish as `--pre-release` first and keep it on the pre-release channel long enough for issues to surface before promoting to stable.

Stable-only publishes are reserved for small, low-risk patches.

```bash
# Non-trivial: prefer pre-release first
npx vsce publish --packagePath specter-vscode-X.Y.Z.vsix --pre-release

# Small low-risk patch (after pre-release has baked, or for trivial fixes)
npx vsce publish --packagePath specter-vscode-X.Y.Z.vsix
```

---

## CLI release tags

A pushed tag starts the CLI release. The Pre-Release Test Suite runs on the tag, and `release.yml` builds and signs only after a read-only gate job allows it. The gate lives in `.github/scripts/release-gate.sh`. It allows a release only when all of these hold:

- The tag is an annotated tag named `vX.Y.Z`, or `vX.Y.Z-suffix` for a pre-release. Create it with `git tag -a`.
- The tag points at a commit on `main`.
- `specter/VERSION` at that commit matches the tag without its `v`.
- The Pre-Release Test Suite passed on the push of that tag, at that commit.

A manual dispatch of `release.yml` must run from `main` and passes through the same gate.

### Tag protection

Two repository rulesets cover `refs/tags/v*`:

- **Release tags: creation by the release operator only.** Only the release operator can create a `v*` tag.
- **Release tags: immutable.** Nobody can move or delete a `v*` tag. This ruleset has no bypass.

The GitHub release is bound to the tag name, not to the commit that was built. A tag that moved during a release would leave the release page pointing at a different commit than the binaries. The immutable ruleset is what prevents that.

### Recovering a release tag

Moving or deleting a release tag is a recovery step, not routine work. Do it only when a tag was pushed to the wrong commit and no release was published from it.

1. Record why in an issue, with the tag, the wrong commit, and the right one.
2. In Settings, Rules, Rulesets, set **Release tags: immutable** to Disabled.
3. Delete or move the tag, and push the corrected tag.
4. Set the ruleset back to Active. Confirm with `gh api repos/Hanalyx/specter/rulesets` that both rulesets read `active`.

The ruleset history records who disabled and re-enabled it, and when. Do not move a tag that already has a published release. Cut a new patch version instead.

### Release build tools

`release.yml` and `release-snapshot.yml` install GoReleaser and syft through `.github/scripts/install-release-tools.sh`. It downloads each archive from a fixed release URL and checks it against a pinned SHA-256 before extracting it. To upgrade a tool:

1. Download the new release's checksums file and its signature.
2. Verify the signature with `cosign verify-blob` against the tool's own release workflow identity. The script's header names each identity.
3. In one change, update the script: the version, the archive's hash from the verified checksums file, and the identity in the header. GoReleaser's identity names its release tag, so it changes with every version. Verify with `--certificate-oidc-issuer https://token.actions.githubusercontent.com`.
4. Open the change as a pull request. The snapshot pre-flight runs on any change under `.github/scripts/`, so a wrong hash fails there, not on a release tag.

---

## The helper

`make release-check` automates the first half of the gate. It runs `make prerelease` (check + vulncheck + dogfood + cross-compile + VSIX package), then prints this checklist to remind the operator of steps 2–7. It does **not** run `vsce publish` under any circumstance. That stays manual so the operator cannot forget to verify.

```bash
make release-check
# ... prints the checklist, then exits. You then perform steps 2–7 by hand
#     before running vsce publish.
```

---

## Related docs

- `GOTCHAS.md` #18: the failure mode this gate exists to prevent.
- `CHANGELOG.md`: release history.
