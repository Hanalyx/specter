# SSRB-105: retire the manifest registry section

Status: ACCEPT
Directed: 2026-08-22, founder, deciding `bugs/SP-SP-054` and `bugs/SP-SP-049` together.
Source: the phase 4 review of v0.15 item 1D-a, which found C-06 mandates behavior no command performs.

## 1. Request

Remove the `registry` section from the manifest schema, and with it the code and
criteria that describe it.

```yaml
# removed
registry:
  - id: checkout
    file: specs/checkout.spec.yaml
    version: "1.0.0"
    status: approved
    tier: 1
    domain: payments
```

## 2. Origin

`spec-manifest` C-06 states, at `enforcement: error`:

> MUST auto-update the registry section from the current set of parsed specs

Measured at `5576cab`:

- `cmd/specter/main.go` contains **zero** occurrences of `registry` in any casing.
- `UpdateRegistry` has zero callers anywhere in the tree, tests included.
- `BuildRegistryFromSpecs` is called only by `UpdateRegistry` and by its own two
  unit tests.
- A passing `sync` leaves `specter.yaml` byte-identical.

**It was never implemented.** `UpdateRegistry` arrived in `6873cc1`, the commit
that added the manifest, and no commit has ever wired it into a command.
`specter init` has never scaffolded a registry block.

So no released Specter has written a registry section into anyone's manifest.

## 3. Universality

This is a removal, so the question inverts: does anyone depend on it?

**Approximately nobody can.** The only ways a registry block exists in a real
workspace are that an operator hand-copied it from
`testdata/manifests/valid/full.specter.yaml`, or wrote one after reading C-06 and
believing the tool maintained it. The second case is a person the current state
has already failed.

The field it would most plausibly serve, `tier`, is a copy of a value the spec
already declares, and `bugs/SP-SP-049` records that the resolution path feeding
it is itself dead.

## 4. Cost

Enumerated before scoping, per the schema-conservatism rule.

**Schema.** `registry` leaves `validTopLevelKeys` (`internal/manifest/manifest.go:21`).

**Code.** `Manifest.Registry` and `type RegistryEntry` (`types.go`),
`BuildRegistryFromSpecs` and `UpdateRegistry` (`registry.go`).

**Spec.** C-06 goes. C-01 drops `registry` from the sections it names. C-13's
`init --refresh` list drops `registry` from the fields it must leave untouched.
AC-09 and AC-10 go. Five objective-scope lines change.

**Tests.** The two `BuildRegistryFromSpecs` unit tests, and the assertion at
`manifest_test.go:47` that `full.specter.yaml` yields two registry entries.

**Fixtures.** The `registry:` block in `testdata/manifests/valid/full.specter.yaml`.

**Not affected.** `spec-parse` line 36 uses "registry" to mean file discovery,
a different sense. Domains, `--scope`, and coverage are untouched.

## 5. Existing coverage

C-06 is the only constraint mandating the behavior, and it has no
implementation. AC-09 and AC-10 assert the builder at unit level and pass, which
is why coverage reports C-06 covered. That is the P1 defect in `SP-SP-054`: the
gate reports a requirement satisfied while nothing implements it, in the corpus
of the tool whose claim is that a requirement without a passing test fails the
build.

## 6. Alternatives

**Implement C-06.** Rejected. The registry is a cache of data derivable from the
specs on every run, and Specter parses every spec on every run regardless, so it
caches nothing worth caching. A persisted copy of the source of truth can go
stale against it, which is the failure mode a type system for specs exists to
prevent. It would also make a CI gate rewrite a user's manifest on every run.

**Leave it inert.** Rejected. It fails the v0.18 pre-lock criterion that every
schema field be consumed deterministically by at least one command, and it
leaves the P1 coverage defect standing.

## 7. Decision

**ACCEPT, delivered as deprecate-then-remove**, following the SSRB-104
precedent rather than a hard break.

### 7.1 v0.15.0: accept and warn

`registry` stays an accepted top-level key. A manifest carrying one parses, and
the validator warns that the section is unmaintained, was never populated by any
Specter version, and will be removed at v1.0.0.

The code, C-06, AC-09, AC-10 and the fixture block go now. Only the key's
acceptance survives.

**Why not a hard removal now.** Dropping `registry` from `validTopLevelKeys`
turns any manifest carrying the block into a parse error with an unknown-key
message, and there is no migration tooling: `doctor --fix` is BETA with a
one-rule table and `specter migrate` is roadmap. The affected population is
probably empty, but "probably empty" plus "no migration path" is not a
combination worth a hard break in a pre-1.0 minor, when the deprecation window
costs one warning.

### 7.2 v1.0.0: remove the key

`registry` leaves `validTopLevelKeys`. This lands in the same release that
removes `settings.strictness` and `--strictness` per SSRB-104, so a workspace
migrating for one migrates for both.

### 7.3 This does not decide tier inheritance

`bugs/SP-SP-049` is a separate question and this brief does not answer it.

Retiring the registry deletes `BuildRegistryFromSpecs`, which is one of the two
call sites of `ResolveTier`. The other, `ResolveTierWithOverrides`, is also
dead. So after this change `ResolveTier` has zero call sites of any kind.

**That makes the SP-SP-049 decision more urgent, not less.** If tier inheritance
is wanted, `ResolveTier` is the starting point and should not be deleted here.
If it is not, `ResolveTier` should be deleted in the same pass rather than left
as the last orphan of a retired subsystem.

`SP-SP-049` carries the recommendation and its reassessment.

## 8. Reconsideration triggers

- An adopter reporting they maintain a registry section by hand and rely on it,
  which would make the field a real interface rather than an unimplemented one.
- A performance case for caching parsed spec metadata, which would be a
  different feature with a different shape and should not reuse this field.

## 9. References

- `bugs/SP-SP-054`, the P1 defect this closes.
- `bugs/SP-SP-049`, the tier cascade. Decided alongside, answered separately.
- `docs/ssrb/SSRB-104.md`, whose deprecate-then-remove shape this follows.
- `docs/SPECTER_LEXICON.md` Part 5, which records that the registry block parses
  and no command reads or regenerates it.
