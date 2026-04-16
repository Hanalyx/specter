// @spec spec-vscode

import * as path from 'path';

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
 * AC-01 — Walks up the directory tree from `filePath` until it finds
 * `specter.yaml` or reaches the filesystem root.  Returns the manifest path
 * or null if none is found.
 *
 * @param filePath  The file to start from (its directory is the first candidate).
 * @param exists    Injectable predicate so callers can supply a mock FS.
 */
export function resolveManifestPath(
  filePath: string,
  exists: (p: string) => boolean,
): string | null {
  let dir = path.dirname(filePath);

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
