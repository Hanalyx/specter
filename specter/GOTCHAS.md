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

## 13. `process.arch` ≠ `runner.arch` — case matters

**Symptom:** VS Code extension downloads URL `.../specter_0.6.6_linux_x64.tar.gz` which 404s. The actual published asset is `..._linux_amd64.tar.gz`.

**Cause:** The extension had one `normaliseArch` function serving two callers with different case conventions:
- VS Code's `runner.arch` (GitHub Actions context) uses uppercase: `X64`, `ARM64`, `IA32`.
- Node's `process.arch` (what the extension host actually passes) uses lowercase: `x64`, `arm64`, `ia32`.

The switch was case-sensitive with only uppercase cases. Lowercase values fell through to default (`return arch.toLowerCase()`) and produced URLs with `x64` in the path — which goreleaser never emits (it uses `amd64`).

**What's in place now:** `normaliseArch` lowercases its input before switching. Tests cover both uppercase and lowercase inputs.

**Rule:** when writing a normaliser that straddles two case conventions, normalise case at the top of the function. Don't depend on callers feeding the "right" case.

---

## 14. Context was extensible pre-v0.7.0 and silently dropped data

**Symptom (pre-v0.7.0):** A user writes `context.role: "public API"` or `context.callers: [...]` in their spec. `specter parse` passes. Downstream tools see only the declared fields (`system`, `feature`, etc.) — the extra keys are gone.

**Cause:** Schema had `context.additionalProperties: true` ("extras are OK"), but `SpecContext` (types.go) was a closed struct. yaml.v3's default non-strict unmarshal silently discarded anything the struct didn't declare. The schema's promise and the types' behavior disagreed, and the disagreement was invisible.

**What's in place now (v0.7.0+):** `context.additionalProperties: false`. Unknown context keys are rejected at parse time with a named error. SPEC_SCHEMA_REFERENCE.md no longer advertises the "add custom keys" escape hatch.

**Rule:** when a JSON Schema declares `additionalProperties: true` on an object that types.go parses into a closed struct, the mismatch causes silent data loss. Either make the schema strict (preferred, as of v0.7.0) or parse into `map[string]interface{}` in the struct (used for `inputs` and `expected_output` which ARE free-form by design).

---

## 15. Release asset names do NOT match `uname` output

**Symptom:** Docs (and old extension code) that used `$(uname -s)_$(uname -m)` or `specter_Linux_x86_64.tar.gz` hit 404. On install, users get a 9-byte `Not Found` response written to disk and chmodded +x.

**Cause:** Four different arch/OS vocabularies coexist in this project:

| Source | OS on Linux amd64 | Arch on Linux amd64 |
|---|---|---|
| `uname -s` / `uname -m` | `Linux` | `x86_64` |
| Node `process.platform` / `process.arch` | `linux` | `x64` |
| VS Code `runner.os` / `runner.arch` | `Linux` | `X64` |
| **Go `GOOS` / `GOARCH` (asset names)** | **`linux`** | **`amd64`** |

goreleaser uses `name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"` — Go's values — so every asset is `specter_<version>_<linux|darwin|windows>_<amd64|arm64>.<ext>`. None of the reporter conventions match directly; every install caller must translate.

**What's in place now:** all install snippets in README.md, QUICKSTART.md, GETTING_STARTED.md translate `uname` → `GOOS`/`GOARCH` and resolve version via the GitHub API. The VS Code extension's `normaliseArch` lowercases its input so it handles both uppercase (runner.arch) and lowercase (process.arch). The composite GitHub Action in `.github/actions/specter-sync/action.yml` also does the translation.

**Rule:** when writing any install-command example, never use `uname -m`, `uname -s`, `process.arch`, or `runner.arch` raw in the URL. Always translate. And never omit the version — there is no version-less alias (goreleaser could be configured to emit one but currently doesn't).

---

## 16. `path.dirname()` on a directory path returns the PARENT, not the directory

**Symptom:** VS Code extension reports "no specter.yaml found in this workspace" even though `specter.yaml` is at the workspace root. `specter init` produces the file, a reload doesn't help, running `specter init --force` doesn't help.

**Cause:** `resolveManifestPath(filePath, exists)` in `activation.ts` started its search by calling `path.dirname(filePath)`. The function was designed to be called with a FILE path (e.g. `/project/specs/auth.spec.yaml`), where `dirname` correctly strips the filename to leave `/project/specs/`. All unit tests passed file paths.

But the real caller in `setupFolder` passes `folder.uri.fsPath` — a DIRECTORY path like `/home/user/project`. `path.dirname` doesn't check whether the input is a directory; it treats it syntactically. `path.dirname("/home/user/project")` returns `/home/user` — the PARENT directory. The resolver then searched `/home/user/project/../specter.yaml`, `/home/user/specter.yaml`, etc., walking all the way to `/` — **never checking `/home/user/project/specter.yaml` itself**.

This shipped in spec-vscode v1.0 and affected every user whose `specter.yaml` was at the workspace root — the canonical location the docs explicitly recommend. The unit tests covered the designed calling convention (file paths) but didn't cover the actual calling convention in production (directory paths).

**What's in place now (v0.8.1+):** `resolveManifestPath` takes an optional third argument `isDirectory: (p: string) => boolean` so callers handling workspace folders can say "this path IS the starting directory." The runtime caller supplies a `statSync(...).isDirectory()` probe. Two regression tests pin both shapes.

**Rule:** `path.dirname` operates on strings, not filesystem types. If a function accepts "a path" without saying which kind, assume at least one real caller will hand it a directory. Either restrict the contract (name it `resolveManifestFromFile`) or accept both and branch on an explicit predicate.

---

## 17. Extension hallucinated CLI flags the binary never accepted

**Symptom (v0.8.1):** Coverage run fails with `error: unknown flag: --manifest`. Also affected `parse` and `check` calls from the extension.

**Cause:** `SpecterClient` in `client.ts` was built assuming the CLI had flags it never shipped:

- `specter parse --json --manifest <path>` — no `--manifest` flag exists
- `specter check --json --manifest <path>` — same
- `specter coverage --json --manifest <path> [--spec <id>]` — same, plus `--spec` doesn't exist either
- `specter diff --json --base <ref> <file>` — no `--base`, no `--json`; diff is two positional args `<path>[@<ref>]` and emits human-readable text

Five fabricated flags in production code. The CLI rejects unknown flags (Cobra default), so every invocation threw, which `runCoverageForFolder`'s try/catch surfaced as "no coverage data" in the sidebar. Users saw empty coverage state regardless of whether their specs were valid.

**Why it slipped:** no integration test in the extension ever invoked the real CLI binary. All tests were unit-level, against TypeScript mocks of the CLI's contract. The mocks described what the extension *wanted* the CLI to accept, not what it actually accepted.

**What's in place now (v0.8.2+):** The `--manifest` / `--spec` / `--base` / `--json` (on diff) flags are stripped from `SpecterClient`. Instead, `execFile` is called with `cwd: path.dirname(manifestPath)` so the CLI's own `findManifest` walk-up finds the correct `specter.yaml`. New `client.test.ts` integration suite spawns the real built CLI against a tmpdir workspace and asserts every public method runs without throwing. The suite is skipped when `bin/specter` doesn't exist (pre-build) so CI requires a build step first.

**Rule:** any code that shells out to another binary gets at least one integration test that spawns the real binary. Mocks describe intent, not contract. If CI doesn't exercise the real-binary path, every flag is a guess.

---

## 18. Marketplace is NOT a test harness — humans must verify before publish

**Symptom:** Four patch releases of the VS Code extension in a single day (v0.8.0 → v0.8.1 → v0.8.2 → v0.8.3), each one fixing a bug that the first real user hit the moment they installed. Users see the auto-update notification every few hours and each install reveals another regression. Damages credibility.

**Cause:** The pre-publish loop was:
1. Write code
2. Run `make check` + `npx jest`
3. `vsce package` + `vsce publish`
4. Wait for a user to install and report whatever broke

Step 4 was the test. Unit tests covered roughly 30% of the extension's actual user-facing surface — specifically the pure-function parts. The un-covered 70% (binary resolution, tree rendering, CLI invocation, activation race conditions, cwd assumptions) is where every one of the four v0.8.x bugs lived.

The lesson isn't "write more unit tests" alone — the real gap was the absence of a human-verified end-to-end test in a real VS Code window against a real workspace before every publish. No mock, no CI substitute, no "I'm confident it works" — an actual install + reload + click-through.

**What's in place now (v0.8.3+):**
- `RELEASING.md` documents an 8-step gate. Mandatory before any `vsce publish`.
- `make release-check` packages the VSIX and prints the checklist. It does not run `vsce publish` — that stays manual so the operator cannot forget step 7 (human sign-off).
- `BACKLOG.md` queues `@vscode/test-electron` headless integration tests for v0.9 — the proper long-term backstop.

**Rule:** Marketplace is a distribution channel, not a test harness. Every publish must ship behavior that has already been verified by a person in a live VS Code window. If that person is me or an AI agent, the verification must be demonstrated (screenshot, recorded session, or reproducible script) — not asserted. The first user to install should be the hundred-and-first person to see the feature work, not the first.

---

## 19. VS Code extension and CLI versions must stay in lockstep

**Symptom:** Extension v0.6.5 queries `api.github.com/.../releases/latest`, gets back e.g. v0.6.5, then tries to download `.../releases/download/v0.6.5/...`. If the GitHub release for that tag doesn't exist yet, you get 404 "Not Found" in the body (see #3).

**Rule:** when bumping version:
1. Bump `VERSION` (CLI) AND `vscode-extension/package.json` (extension) together.
2. Push the git tag first so the GitHub Release goes live.
3. Only THEN publish the VSIX to Marketplace. If the VSIX is live before the tag, the first user to install gets "Not Found" written to `~/.specter/bin/specter`.
