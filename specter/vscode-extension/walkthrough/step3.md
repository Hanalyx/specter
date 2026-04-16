# Run specter sync

With specs written and tests annotated, run the full pipeline:

```bash
specter sync
```

This runs **parse → resolve → check → coverage** in sequence and exits non-zero if any Tier 1 or Tier 2 spec fails its coverage threshold. Use this as your CI gate.

**What you'll see in VS Code:**

- **Gutter icons** on each AC line in `.spec.yaml` files — green (covered), red (uncovered), grey dash (gap)
- **Status bar** — `Specter: N specs · X% · F failing` — click to open the Insights panel
- **Insights panel** — human-readable health cards for every failing spec with full AC descriptions and actionable next steps
- **Problems panel** — parse and structural errors from `specter check` appear as you save

> **CI integration:** Add `specter sync` to your CI pipeline. It exits 0 only when all Tier 1/2 specs meet their coverage thresholds.

---

**Not sure where to start?** Use the [AI prompts guide](https://github.com/Hanalyx/specter/blob/main/specter/docs/AI_PROMPTS.md) — six ready-to-paste prompts that take you from intent to a passing `specter sync`, one step at a time.
