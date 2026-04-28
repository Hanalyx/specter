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
 * AC-51.
 */
export function matchRemovedFieldDiagnostic(message: string): string | null {
  const m = /Unknown field '([^']+)'/.exec(message);
  if (!m) return null;
  const fieldName = m[1];
  if (!KNOWN_REMOVED_FIELDS.includes(fieldName)) return null;
  return fieldName;
}

/**
 * Returns true when `line` represents a YAML `<key>: <value>` form whose
 * value is a single-line scalar safe to delete by removing the whole line.
 * Returns false for shapes that span multiple lines (block scalars `|`/`>`,
 * empty values that introduce a sequence/mapping on the next line) or
 * unterminated quoted strings — deleting just the diagnostic's line on
 * those shapes orphans continuation lines and corrupts the file.
 *
 * AC-53. Mirrors the migrate package's `canSafelyStripTrustLevel`
 * predicate (spec-doctor C-15) so editor and CLI workflows agree on what
 * is safe to auto-repair.
 */
export function isLineSafeToDelete(line: string): boolean {
  const colonIdx = line.indexOf(':');
  if (colonIdx === -1) {
    return false; // not a key:value line
  }
  const valuePart = line.slice(colonIdx + 1);
  const stripped = stripInlineComment(valuePart).trim();

  // Empty value → next-line content (sequence or mapping) → unsafe.
  if (stripped === '') {
    return false;
  }
  // Block-scalar indicator (`|`, `|-`, `|+`, `|2`, `>`, `>-`, `>2`, etc.)
  if (/^[|>]/.test(stripped)) {
    return false;
  }
  // Quoted scalars must be closed on the same line.
  if (stripped.startsWith('"')) {
    // Double-quoted: backslash escape allowed.
    return /^"(\\.|[^"\\])*"$/.test(stripped);
  }
  if (stripped.startsWith("'")) {
    // Single-quoted: only escape is doubled quote (`''`).
    return /^'([^']|'')*'$/.test(stripped);
  }
  // Plain scalar — safe.
  return true;
}

// stripInlineComment removes a trailing `# comment`, respecting double-
// and single-quoted regions so a `#` inside a string isn't misread as a
// comment marker. Returns the input unchanged if no comment is present.
function stripInlineComment(s: string): string {
  let inDouble = false;
  let inSingle = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '\\' && inDouble) {
      i++; // skip the escaped character
      continue;
    }
    if (c === '"' && !inSingle) {
      inDouble = !inDouble;
    } else if (c === "'" && !inDouble) {
      inSingle = !inSingle;
    } else if (c === '#' && !inDouble && !inSingle) {
      // YAML comments must be preceded by whitespace (or be at the line
      // start). Otherwise `#` is part of the scalar.
      if (i === 0 || /\s/.test(s[i - 1])) {
        return s.slice(0, i);
      }
    }
  }
  return s;
}
