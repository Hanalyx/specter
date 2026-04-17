# GOTCHAS

Non-obvious traps that have already bitten this codebase. Read before releasing.

---

## 1. `vsce package` does NOT compile TypeScript

**Symptom:** VSIX ships with stale `out/*.js` from a previous build. The `package.json` inside the VSIX reports the new version, but the running extension executes old code.

**How it bit us:** Shipped v0.6.5 VSIX containing v0.5.x-era compiled JS. Fix code was in `src/`, build was never run, `out/` was from the day before. Every user install re-ran the old broken auto-downloader, which wrote the 9-byte string `Not Found` (the body of a 404 response) into `~/.specter/bin/specter` and chmodded it +x.

**What's in place now:**
- `vscode:prepublish` hook in `vscode-extension/package.json` — `vsce` runs this automatically before packaging and will fail the package if build fails.
- `package` script is `npm run build && npx @vscode/vsce package` for defense in depth.

**If you're packaging manually:** always run `npm run build` first and verify `out/extension.js` mtime is newer than any `src/*.ts` file. Quick check:
```bash
# Should return nothing (no source newer than compiled output)
find src -name '*.ts' -newer out/extension.js
```

---

## 2. `jest` compiles TypeScript on the fly — it does NOT write to `out/`

**Symptom:** Tests pass, you assume the build is good, ship broken VSIX.

**How it bit us:** Same incident as #1. `jest` uses `ts-jest` which transforms in memory. Passing tests prove the source is correct but say nothing about what's on disk in `out/`.

**Rule:** test passing ≠ build current. Before any packaging step, run `npm run build` explicitly.

---

## 3. Node's `https.get` does NOT follow redirects or check status

**Symptom:** Download succeeds silently but the bytes are an HTTP error body (`Not Found`, HTML error page, etc).

**How it bit us:** Pre-v0.6.0 download code called `https.get(url, ...)` and wrote the body straight to disk. GitHub Releases returns:
- 302 redirect to `release-assets.githubusercontent.com/...` (actual asset) — the 302 body is empty, so this fails silently
- 404 with `content-type: text/plain` body `Not Found` if the URL is wrong (e.g., `vlatest/specter_latest_...` when the config defaults to literal string `"latest"`)

**Rule:** any `https.get` on GitHub (or any CDN) MUST follow redirects and check status. See `vscode-extension/src/binaryDiscovery.ts:httpsGet` for the correct pattern — follows up to 5 redirects, rejects on status >= 400, has a 30s timeout.

---

## 4. `cfg.get('version', 'latest')` gives you the literal string `"latest"`

**Symptom:** URLs like `.../releases/download/vlatest/specter_latest_linux_amd64.tar.gz` — not valid.

**How it bit us:** Pre-v0.6.0 code used the config value directly in the URL. `"latest"` is a keyword convention, not a real tag.

**Rule:** resolve `"latest"` to a real semver via the GitHub API before building download URLs. See `resolveLatestVersion()` in `binaryDiscovery.ts`.

---

## 5. A bare `go build` inside the package drops a binary at the repo root

**Symptom:** Stray `./specter` binary appears next to `go.mod`. Looks identical to `./bin/specter` but is stale.

**How it bit us:** Someone ran `go build` in `specter/` (no `-o` flag). Go named the output after the package dir: `./specter`. It got committed. When both `./specter` and `./bin/specter` existed, running one versus the other produced different results — the old one lacked gap-exclusion logic, the new one had it. Took non-trivial debugging to notice they were different binaries.

**What's in place now:** `/specter` is in `.gitignore`. Canonical build path is `make build` → `bin/specter`.

**Rule:** don't run bare `go build`. Use `make build` or `go build -o bin/specter ./cmd/specter/`.

---

## 6. Duplicate files risk silent drift

**Symptom:** Two identical-looking files in different paths, only one actually used, the other slowly drifts.

**How it bit us:** `internal/schema/spec-schema.json` and `internal/parser/spec-schema.json` existed side-by-side. Only the parser one was loaded (via `//go:embed`). The schema one was an abandoned copy. They had drifted (parser version had a `coverage_threshold` field the schema one lacked).

**What's in place now:** The orphan duplicate in `internal/schema/` is deleted. Canonical schema is `internal/parser/spec-schema.json`.

**Rule:** one canonical copy. If you find a file that looks like a mirror, verify which is actually loaded (grep for `//go:embed`, import statements) and delete the dead one.

---

## 7. `specter.yaml` being optional means silent defaults

**Symptom:** User puts specs in `src/specs/`, runs `specter sync`, sees "No .spec.yaml files found", has no idea why.

**Cause:** When no `specter.yaml` is found, Specter silently uses `specs_dir: specs` default. Nothing tells the user this happened.

**Partial mitigation:** `specter doctor` reports manifest status. Full mitigation is pending (see improvement backlog in the session log).

**Rule:** if you're editing manifest-loading code, preserve (or add) a clear hint when defaults are in play. `discoverSpecs` should be able to say "I looked at `specs/` and found 0 files".

---

## 8. VS Code does NOT reload extension code on update or reinstall

**Symptom:** User updates extension, sees new version number in Marketplace panel, still gets old behavior. Status bar shows something from old code.

**Cause:** VS Code's extension host stays running. Updating an extension stages new files on disk; the live process keeps running old code until the window reloads. Uninstall + reinstall has the same issue unless VS Code prompts "Reload to finish".

**Rule:** when debugging "did my fix ship?", always have the user Developer: Reload Window before concluding it didn't. If you're publishing critical extension fixes, consider bumping the major-activation-event trigger or asking users to reload in the release notes.

---

## 9. CI's `pull_request` trigger does NOT fire on title edits

**Symptom:** You edit a PR title (e.g., to satisfy a conventional-commit check), then rerun failed jobs — they still see the old title.

**Cause:** Default `pull_request` trigger types are `opened, synchronize, reopened`. `edited` is NOT included. Reruns use the original event payload.

**Workaround:** push an empty commit (`git commit --allow-empty -m "ci: retrigger"`) to force a `synchronize` event. Or add `types: [opened, synchronize, reopened, edited]` to the workflow trigger.

---

## 10. Env vars in a separate terminal do NOT reach Claude Code's Bash tool

**Symptom:** You `export VSCE_PAT=...` in one terminal, tell Claude Code to publish, Claude's shell reports VSCE_PAT unset.

**Cause:** Each terminal has its own shell env. Claude Code's Bash tool launches a non-interactive bash subprocess that inherits from the Claude Code parent, not from any other terminal. A `.bash_profile` export doesn't help either — non-interactive shells don't source it.

**How to bridge:**
- Type `! export VAR=value` as a message to Claude (the `!` prefix sends it to Claude's own shell), OR
- Put the export in `.bashrc` (non-interactive shells DO source this on some distros), OR
- Have Claude `source ~/.bash_profile` explicitly inside the Bash call.

---

## 11. `gap: true` ACs were silently exempt from coverage (pre-v0.6.5)

**Symptom:** A reverse-compiled spec with 5 gap ACs and zero tests would show `100%` coverage status PASS.

**Cause:** Coverage denominator excluded gap ACs entirely. A 100%-gap spec had `totalACs == 0`, which was treated as "exempt from threshold".

**What's in place now:** v0.6.5+ counts every declared AC, including gaps. A gap AC without a test annotation is uncovered. `spec-coverage` spec C-09/AC-09 pin this.

**Rule:** `gap: true` is a triage marker for human review. It does not buy a pass.

---

## 12. `constraint.enforcement` / `constraint.type` were parsed-but-unused (pre-v0.6.5)

**Symptom:** Author sets `enforcement: error` on a tier-3 spec's constraint expecting it to become an error; stays info.

**Cause:** The checker used hardcoded `orphanSeverityByTier`. The constraint's own `enforcement` field was ignored.

**What's in place now:** v0.6.5+ lets `constraint.enforcement` override the tier default. `constraint.type` shows up inline in check output. `trust_level` (parsed-but-useless) was removed.

**Rule:** before adding a new field to the schema, decide what part of the pipeline consumes it. If nothing does, don't add it — or clearly mark it "documentation only, not enforced."

---

## 13. VS Code extension and CLI versions must stay in lockstep

**Symptom:** Extension v0.6.5 queries `api.github.com/.../releases/latest`, gets back e.g. v0.6.5, then tries to download `.../releases/download/v0.6.5/...`. If the GitHub release for that tag doesn't exist yet, you get 404 "Not Found" in the body (see #3).

**Rule:** when bumping version:
1. Bump `VERSION` (CLI) AND `vscode-extension/package.json` (extension) together.
2. Push the git tag first so the GitHub Release goes live.
3. Only THEN publish the VSIX to Marketplace. If the VSIX is live before the tag, the first user to install gets "Not Found" written to `~/.specter/bin/specter`.
