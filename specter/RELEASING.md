# Releasing Specter

This document defines the gate that must be satisfied before any `vsce publish` (VS Code extension) or `git push origin vX.Y.Z` (CLI release).

It exists because the v0.8.0 → v0.8.3 extension thrash (four patch releases in one day, each fixing a bug the first real user hit on install) proved that running `make check` and `vsce package` is not sufficient to ship. Marketplace is a distribution channel, not a test harness.

---

## The gate

Every step is mandatory. Skipping a step is not a shortcut — it's a commitment to ship a broken version.

### 1. CI green

`make check`, dogfood, and extension jest all pass in GitHub Actions. Not optional, not sufficient alone.

### 2. Install the packaged VSIX locally

```bash
code --install-extension specter-vscode-X.Y.Z.vsix --force
```

CI building the VSIX is not the same as VS Code loading it. The v0.6.5 → v0.6.6 bug (stale `out/` directory shipped in the VSIX) was invisible to CI.

### 3. Reload, then open a known-working test workspace

The `specter/` repo itself is a good choice (14 dogfood specs). The Coverage sidebar should populate with real entries, not the empty-state message.

### 4. Reload, then open a known-failing test workspace

A directory whose `.spec.yaml` files fail Specter's schema (e.g. specs written in an older custom schema). The Coverage sidebar should show the empty-state message, the status bar should be in the error state, and the Output channel should contain the parse errors.

### 5. Exercise every changed code path

If the change touches:
- **Binary resolution** — delete `~/.specter/bin/specter` and watch the re-download.
- **Tree rendering** — verify both empty and populated states.
- **CLI flag handling** — invoke the affected command from the integrated terminal.
- **Activation flow** — open a workspace that matches the activation trigger but wasn't open when VS Code started.

If a change doesn't obviously route through one of these, write down the path it does exercise and test that explicitly.

### 6. Check the Output channel

`View → Output → Specter`. No unexplained red entries during normal use.

### 7. Human verifies and signs off

No AI / automated "I think it works" substitutes here. The person cutting the release reproduces the change in a live VS Code window and confirms the behavior matches intent. This is the step that has repeatedly failed during v0.8.x and is the reason this gate exists.

### 8. Only then run `vsce publish`

For non-trivial changes — schema shifts, new UI surfaces, binary-download code, new CLI flags — publish as `--pre-release` first and keep it on the pre-release channel long enough for issues to surface before promoting to stable.

Stable-only publishes are reserved for small, low-risk patches.

```bash
# Non-trivial — prefer pre-release first
npx vsce publish --packagePath specter-vscode-X.Y.Z.vsix --pre-release

# Small low-risk patch (after pre-release has baked, or for trivial fixes)
npx vsce publish --packagePath specter-vscode-X.Y.Z.vsix
```

---

## The helper

`make release-check` automates the first half of the gate — it runs `make prerelease` (check + vulncheck + dogfood + cross-compile + VSIX package), then prints this checklist to remind the operator of steps 2–7. It does **not** run `vsce publish` under any circumstance. That stays manual so the operator cannot forget to verify.

```bash
make release-check
# ... prints the checklist, then exits. You then perform steps 2–7 by hand
#     before running vsce publish.
```

---

## Related docs

- `GOTCHAS.md` #18 — the failure mode this gate exists to prevent.
- `BACKLOG.md` — `@vscode/test-electron` headless integration tests are queued as the proper long-term backstop for the human verification step.
- `CHANGELOG.md` — release history.
