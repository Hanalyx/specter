# Bootstrap your specs

## 1. Install the Specter CLI

**macOS / Linux:**
```bash
curl -Lo specter.tar.gz https://github.com/Hanalyx/specter/releases/latest/download/specter_$(uname -s)_$(uname -m).tar.gz
tar xzf specter.tar.gz && sudo mv specter /usr/local/bin/
specter --version
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri https://github.com/Hanalyx/specter/releases/latest/download/specter_Windows_x86_64.zip -OutFile specter.zip
Expand-Archive specter.zip; Move-Item specter\specter.exe C:\Windows\System32\
```

## 2. Generate draft specs from your code

Point Specter at your source directory — it generates one `.spec.yaml` per module automatically:

```bash
specter reverse src/        # TypeScript / JavaScript
specter reverse app/        # Python / Django / FastAPI
specter reverse ./          # Go
```

This creates a `specs/` directory. Every spec will have `gap: true` — that's expected. It means Specter extracted the structure but needs your intent to complete it.

## 3. Initialize the workspace manifest

```bash
specter init
```

Creates `specter.yaml` — required for this extension to activate. Once it exists, the **Sp** icon appears in the activity bar and the Coverage panel shows your specs.

> **Already have specs in another format?** See the [migration guide](https://github.com/Hanalyx/specter/blob/main/specter/docs/GETTING_STARTED.md#what-specter-is-not) for converting Gherkin, OpenAPI, or plain text specs into Specter's schema.
