# Discover your specs

Specter works from `.spec.yaml` files — structured specifications that describe what your code must do.

If you have existing code without specs, run **specter reverse** to generate draft specs automatically from your source files:

```bash
specter reverse --lang go ./internal/...
```

This produces a draft `.spec.yaml` for each package it finds. Review each file and fill in the `gap: true` acceptance criteria that need manual intent.

> **Tip:** Specter's own specs live in `specs/` — open one to see the format.
