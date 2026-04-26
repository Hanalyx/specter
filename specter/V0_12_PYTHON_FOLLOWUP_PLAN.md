# v0.12 Python adoption follow-up plan

Working document. Picks up where the v0.11 cycle's Wave D left off. Two GH issues + one bigger ergonomic remain. Each can land independently after the v0.11 stack merges.

## Scope

| # | Issue | Effort | Depends on |
|---|---|---|---|
| 1 | GH #77 — language-aware `specter explain` | ~half day | PR #73 (explain bundle) merged |
| 2 | GH #80 — source-only diagnostic hint under `--strict` | ~half day | PR #82 (settings-hardening) merged |
| 3 | GH #79 — `pytest-specter` plugin (the "better fix") | ~3 days | None; stand-alone |

Each is its own PR. Order is flexible. Land #1 and #2 first because they're spec-bump-shaped (concrete contract); #3 is a separate Python package, larger lift.

---

## Item 1 — GH #77: language-aware `specter explain`

### Problem

`specter explain my-spec:AC-01` shows a Go `// @spec` example even when the project is clearly Python. The detection logic exists (`detectAnnotationLanguages` in `cmd/specter/main.go`) but only fires when `.py` files are found in test discovery. A Python project where test discovery walks the wrong directory falls back to the Go default.

### Spec delta

`spec-explain` 1.1.0 → 1.2.0 (assumes PR #73 merged first).

- **C-13**: When `explain` shows an annotation example for an uncovered AC, the example MUST teach the dual-channel pattern for the detected language. For Python specifically, the example MUST include the autouse-fixture conftest pattern (or the documented `pytest-specter` plugin invocation if available) — not just the `# @spec` source comment, because source comments alone don't satisfy `coverage --strict`.
- **AC-11**: `specter explain my-spec:AC-01` in a project where `discoverTestFiles` returns at least one `.py` file prints (a) the source-comment annotation, (b) the conftest autouse-fixture pattern with `pytest.mark.spec(...)` invocation, (c) the required `pytest.ini` setting (`junit_logging = system-out`). The Go example is NOT shown.
- **AC-12**: Same scenario for `.go` test files. Convention A `t.Run` example shown; Python example NOT shown.
- **AC-13**: Empty test discovery falls back to a generic example that explicitly notes the dual-channel requirement and links to the v0.11 explainer.

### Files touched

- `specter/specs/spec-explain.spec.yaml` — version bump, add C-13, AC-11..AC-13.
- `cmd/specter/main.go` — `explainDetailMode` (around line 1735) — extend the per-language case statement. Python case grows from ~5 lines to ~30 (autouse fixture + pytest.ini snippet).
- `cmd/specter/explain_bundle_test.go` (or new `explain_python_test.go`) — three tests: AC-11 (Python full pattern), AC-12 (Go regression — Python example NOT shown), AC-13 (empty discovery generic example).

### Open question

How much should `explain` print? The full pytest setup is ~30 lines. Two options:

- **Inline**: print the full pattern. Cost: longer output. Benefit: copy-pasteable.
- **Reference**: print the source-comment + `pytest.mark.spec(...)` pattern, then point at `specter explain pytest-setup` (a new sub-surface) for the conftest. Cost: extra command. Benefit: keeps `explain my-spec:AC-01` short.

Recommend inline for v0.12 first cut. Promote to reference if user feedback says output is too long.

---

## Item 2 — GH #80: source-only diagnostic hint under `--strict`

### Problem

When `coverage --strict` finds source annotations (`// @ac` or `# @ac` in a test file) but no matching `.specter-results.json` entry for that AC, the report demotes the AC silently. The user sees 0% coverage with no signal that the issue is "test exists, runtime annotation missing." The dual-channel design is invisible.

### Spec delta

`spec-coverage` 1.11.0 → 1.12.0 (assumes PR #82 merged first).

- **C-28**: Under `--strict`, when an annotated AC has at least one matching source-file annotation but no matching results-file entry, `specter coverage` MUST emit a per-AC stderr hint of the form:

  ```
  hint: AC-01 has source annotation in tests/foo.py:42 but no matching pass in .specter-results.json
        — did your test runner emit Convention A/B annotations? See docs/explainer/v0.10-ci-gated-coverage.md
  ```

  One hint per affected AC, printed before the coverage table. Limit to the first 5 affected ACs to keep CI logs compact; suppress with `--quiet`.

- **AC-31**: Workspace where `.specter-results.json` exists but is empty AND test files have `// @ac AC-01` source comments: `coverage --strict` exits with the threshold-failure code AND prints the per-AC hint for AC-01 above the table.
- **AC-32**: When the same AC has both source annotation AND a matching results entry: no hint (the hint is a diagnostic for the missing-runtime-channel case only).
- **AC-33**: `--quiet` suppresses the hints. `--json` includes them in a top-level `diagnostic_hints` array.

### Files touched

- `specter/specs/spec-coverage.spec.yaml` — version bump, add C-28, AC-31..AC-33.
- `internal/coverage/coverage.go` — `BuildCoverageReportStrict` already has the data needed (`AnnotationMatch` per file/spec/AC + `ResultsFile` entries). Add a helper `DiagnoseSourceOnlyACs` that returns `[]SourceOnlyHint` containing (spec_id, ac_id, source_files []string).
- `cmd/specter/main.go` (coverage command) — print the hints above the table when `--strict` and not `--quiet`. Include in JSON output when `--json`.
- `internal/coverage/coverage_test.go` — three new tests covering AC-31..AC-33.

### Notes

The `tests/foo.py:42` precision (file + line) requires preserving line numbers through the annotation extraction. Currently `AnnotationMatch` carries the file path but not the line. Two options:

- **Cheap**: print the file path only, no line. Less useful but a 5-minute change.
- **Better**: extend `AnnotationMatch` with `Line int` and update the extractor in `internal/coverage/coverage.go` to track it. Touches `coverage.ExtractAnnotations` callers (parse loop, sync, watch).

Recommend the better path. The line number is the difference between "useful hint" and "vague signal."

---

## Item 3 — GH #79: `pytest-specter` plugin (the "better fix")

### Problem

The v0.11 cheap fix (regex broadening) means a pytest user can write `print("# @spec my-spec")` from a test and ingest extracts cleanly. But that's manual: every test needs the `print` call, or a hand-written `conftest.py` autouse fixture.

The plugin formalizes the autouse pattern, ships it on PyPI, and gives Python users `pip install pytest-specter` + a marker decorator:

```python
import pytest

@pytest.mark.spec("my-spec", "AC-01")
def test_user_create_valid():
    ...
```

The plugin handles the conftest autouse fixture, the `pytest.ini` setting (`junit_logging = system-out`), and the marker registration. Test runners need zero Specter-specific code; just the import and the decorator.

### Scope

- Separate Python package, hosted in the Specter repo under `python/pytest-specter/` or in a sibling repo.
- Standard `pyproject.toml` build, published to PyPI as `pytest-specter`.
- Three files of substance: `pytest_specter/__init__.py` (plugin entrypoint), `pytest_specter/_emit.py` (the autouse fixture), `tests/test_plugin.py` (verification against a real pytest run).
- README documenting installation (`pip install pytest-specter`) + usage (`@pytest.mark.spec(...)`) + the resulting JUnit `<system-out>` shape.

### Spec delta

This is a sibling tool, not a Specter feature. No spec bump in the main Specter repo. A new `spec-pytest-plugin.spec.yaml` could live under the plugin's own directory if we want to dogfood SDD on the plugin itself — defer that decision until the plugin is closer to release.

### Files touched

New files only. No changes to the Specter Go binary or specs.

```
python/
  pytest-specter/
    pyproject.toml
    pytest_specter/
      __init__.py
      _emit.py
    tests/
      test_plugin.py
    README.md
    LICENSE
```

### Test plan

- Unit: marker registration works, autouse fixture only fires for marked tests.
- Integration: `pytest --junitxml=out.xml` on a fixture project produces a JUnit file whose `<system-out>` carries `// @spec ...` and `// @ac ...` lines for marked tests.
- E2E: `specter ingest --junit out.xml && specter coverage` produces 100% coverage for the marked ACs.

### Decisions to settle

1. **Where does the plugin live?** Same repo (`python/pytest-specter/`) keeps the dogfood loop tight; separate repo (`Hanalyx/pytest-specter`) makes pip install + PyPI release lifecycle cleaner. Recommend separate repo to keep release cadences independent.
2. **Marker syntax.** `pytest.mark.spec("my-spec", "AC-01")` (positional) vs `pytest.mark.spec(spec_id="my-spec", ac_id="AC-01")` (keyword). Recommend support both; positional is shorter for the common case.
3. **Multi-AC tests.** A single test that covers AC-01 AND AC-02 — does the marker accept a list? Recommend yes: `pytest.mark.spec("my-spec", ["AC-01", "AC-02"])` emits two `// @ac` lines.

---

## Suggested merge order

1. Land v0.11 stack (PRs #73, #74, #81, #82, #83) and tag v0.11.0.
2. Open `feat/python-explain-aware` (Item 1) targeting `release/v0.12` or `main`. Small PR, ~half day.
3. Open `feat/coverage-source-only-hint` (Item 2) targeting same base. Small PR, ~half day.
4. Tag v0.12.0 once both land. Items 1 + 2 are the bulk of v0.12's Python adoption story.
5. Spin up `pytest-specter` repo (Item 3) on its own cadence. Initial release can pre-date or post-date Specter v0.12 — they're decoupled.

---

## What this plan does not cover

- VS Code surface for `settings.strictness` (sidebar showing strictness-level state). Tracked in BACKLOG separately; not Python-specific.
- Flake handling (`--deny-flaky`). Deferred from v0.10.
- Full per-language `init --ai <tool>` templates (Python-specific instruction body). The current template is language-agnostic; per-language variants are a v0.13 candidate.

When these three Python items land, the adoption story for Python pytest projects matches the Go story: write a marker, run pytest, run ingest, run `coverage --strict`, ship.
