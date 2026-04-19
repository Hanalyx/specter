// @spec spec-vscode

import * as path from 'path';

/**
 * AC-40: gate `specter parse` invocations to real spec files. Returns
 * true when `fsPath`'s basename ends with `.spec.yaml` (double extension,
 * e.g. `auth.spec.yaml`), false for anything else — including the
 * project manifest `specter.yaml`, generic .yaml config, or files whose
 * name happens to match `specter.yaml` but not `*.spec.yaml`.
 *
 * Used by the on-change and on-save hooks so the manifest doesn't get
 * parsed against the spec schema, which would produce spurious
 * "Missing required field 'spec'" / "Unknown field 'settings'" diagnostics.
 */
export function isSpecFilePath(fsPath: string): boolean {
  const base = path.basename(fsPath);
  return base.endsWith('.spec.yaml') && base !== '.spec.yaml';
}

/**
 * AC-01 — Returns true when the workspace contains specter.yaml or at least
 * one *.spec.yaml file (double extension only, not openapi-spec.yaml).
 */
export function shouldActivate(workspaceFiles: string[]): boolean {
  return workspaceFiles.some(f => {
    const base = path.basename(f);
    return base === 'specter.yaml' || base.endsWith('.spec.yaml');
  });
}

/**
 * AC-01 — Walks up the directory tree looking for `specter.yaml`. Returns
 * the first match or null at filesystem root.
 *
 * Two supported input shapes:
 *   - A FILE path (e.g. `/project/specs/auth.spec.yaml`) — searches the
 *     file's containing directory first, then walks up. `isDirectory`
 *     must return false for this path.
 *   - A DIRECTORY path (e.g. a VS Code `workspaceFolder.uri.fsPath` like
 *     `/home/user/project`) — searches the directory itself first, then
 *     walks up. `isDirectory` must return true for this path.
 *
 * `isDirectory` is injectable so callers can supply a mock FS. Defaults
 * to a predicate that always returns false, matching the file-path
 * calling convention used by the existing test suite. The single
 * runtime caller (setupFolder) passes a real FS check — this is what
 * fixes the pre-v0.8.1 bug where folder paths were being dirname'd into
 * their parent before the search even started.
 */
export function resolveManifestPath(
  startPath: string,
  exists: (p: string) => boolean,
  isDirectory: (p: string) => boolean = () => false,
): string | null {
  // If we're handed a directory, start from IT. Otherwise start from the
  // file's parent directory. Tested via unit test; bug-reproducing test
  // included in the suite so this regression cannot come back quietly.
  let dir = isDirectory(startPath) ? startPath : path.dirname(startPath);

  while (true) {
    const candidate = path.join(dir, 'specter.yaml');
    if (exists(candidate)) return candidate;

    const parent = path.dirname(dir);
    if (parent === dir) {
      // Reached filesystem root
      return null;
    }
    dir = parent;
  }
}

/**
 * AC-22 — Returns a stable key for a workspace folder used to map
 * workspace folders to their SpecterClient instances.  Normalises trailing
 * slashes so `/project` and `/project/` map to the same key.
 */
export function createClientKey(folderPath: string): string {
  return folderPath.replace(/\/+$/, '');
}
