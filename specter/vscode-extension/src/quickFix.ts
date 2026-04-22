// @spec spec-vscode
//
// Pure helper identifying parse diagnostics whose "Unknown field '<X>'"
// message names a field that has been removed from the spec schema. The
// CodeAction provider in extension.ts uses this to decide whether to offer
// a "Remove deprecated field" quick-fix.
//
// Kept runtime-free (no vscode imports) so it's testable without a jest
// runtime shim for the vscode module. See __tests__/quickFix.test.ts.

/**
 * Fields removed from the spec schema in earlier versions. The quick-fix
 * only offers itself for fields on this list — new drift (truly unknown
 * fields) is not silently "fixed" by dropping the line; the user must
 * decide.
 *
 * Paired with internal/migrate's `strip-trust-level` rewrite (specter
 * doctor --fix applies the same repair at the CLI layer).
 */
export const KNOWN_REMOVED_FIELDS: readonly string[] = [
  'trust_level', // Removed in v0.6.5
] as const;

/**
 * Matches the standard specter parser "Unknown field 'X'" message. If the
 * extracted X is in KNOWN_REMOVED_FIELDS, returns X; otherwise returns null.
 *
 * AC-50.
 */
export function matchRemovedFieldDiagnostic(message: string): string | null {
  const m = /Unknown field '([^']+)'/.exec(message);
  if (!m) return null;
  const fieldName = m[1];
  if (!KNOWN_REMOVED_FIELDS.includes(fieldName)) return null;
  return fieldName;
}
