# Generate draft specs

The Specter CLI downloads automatically when this extension activates — no manual installation needed.

## Point Specter at your source code

Open the integrated terminal (**Ctrl+`**) and run:

```bash
specter reverse src/        # TypeScript / JavaScript
specter reverse app/        # Python / Django / FastAPI
specter reverse ./          # Go
```

This creates a `specs/` directory with one `.spec.yaml` per module. Every spec will have `gap: true` on some ACs — that means Specter extracted the structure but needs your intent to complete it.

## Initialize the workspace manifest

```bash
specter init
```

Creates `specter.yaml` — required for this extension to activate fully. Once it exists, the **Sp** icon appears in the activity bar and the Coverage panel shows your specs.

> **Already have specs in another format?** See the [migration guide](https://github.com/Hanalyx/specter/blob/main/specter/docs/GETTING_STARTED.md#what-specter-is-not) for converting Gherkin, OpenAPI, or plain text specs into Specter's schema.

> **Auto-download disabled?** If your environment blocks downloads (corporate proxy, air-gapped network), set `specter.autoDownload` to `false` and configure `specter.binaryPath` to point to a manually installed binary. Download binaries from [GitHub Releases](https://github.com/Hanalyx/specter/releases).
