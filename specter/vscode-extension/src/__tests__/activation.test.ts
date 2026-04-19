// @spec spec-vscode
//
// Tests for extension activation logic and multi-root workspace isolation.

import { shouldActivate, resolveManifestPath, createClientKey, isSpecFilePath } from '../activation';
import * as path from 'path';

// ---------------------------------------------------------------------------
// AC-01: Extension activates only in workspaces with specter.yaml or *.spec.yaml
// ---------------------------------------------------------------------------

// @ac AC-01
describe('shouldActivate', () => {
  it('returns true when specter.yaml exists in workspace root', () => {
    const workspaceFiles = ['/project/specter.yaml', '/project/src/main.go'];
    expect(shouldActivate(workspaceFiles)).toBe(true);
  });

  it('returns true when at least one .spec.yaml file exists', () => {
    const workspaceFiles = ['/project/src/main.go', '/project/specs/auth.spec.yaml'];
    expect(shouldActivate(workspaceFiles)).toBe(true);
  });

  it('returns false when workspace has only generic YAML files', () => {
    const workspaceFiles = ['/project/docker-compose.yml', '/project/.github/workflows/ci.yaml'];
    expect(shouldActivate(workspaceFiles)).toBe(false);
  });

  it('returns false for an empty workspace', () => {
    expect(shouldActivate([])).toBe(false);
  });

  it('does not activate for .yaml files that happen to contain "spec" in their name', () => {
    const workspaceFiles = ['/project/openapi-spec.yaml'];
    // Only *.spec.yaml (double extension) or specter.yaml trigger activation
    expect(shouldActivate(workspaceFiles)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// AC-01: Manifest discovery — walk up from file to find specter.yaml
// ---------------------------------------------------------------------------

// @ac AC-01
describe('resolveManifestPath', () => {
  const mockFsExistsAt = (manifestPath: string) =>
    (p: string) => p === manifestPath;

  it('finds specter.yaml in the same directory as the file', () => {
    const found = resolveManifestPath(
      '/project/specs/auth.spec.yaml',
      mockFsExistsAt('/project/specs/specter.yaml'),
    );
    expect(found).toBe('/project/specs/specter.yaml');
  });

  it('walks up to find specter.yaml in a parent directory', () => {
    const found = resolveManifestPath(
      '/project/specs/auth/auth.spec.yaml',
      mockFsExistsAt('/project/specter.yaml'),
    );
    expect(found).toBe('/project/specter.yaml');
  });

  it('returns null when no specter.yaml found up to filesystem root', () => {
    const found = resolveManifestPath('/project/specs/auth.spec.yaml', () => false);
    expect(found).toBeNull();
  });

  it('stops walking at filesystem root and does not loop', () => {
    // Should terminate without infinite loop
    const found = resolveManifestPath('/', () => false);
    expect(found).toBeNull();
  });

  // Regression — pre-v0.8.1 bug. If resolveManifestPath is called with a
  // directory path (as the extension's setupFolder does with
  // folder.uri.fsPath) and no `isDirectory` predicate, it would dirname()
  // up one level and miss a specter.yaml sitting at the start path.
  //
  // The real caller now passes an isDirectory probe; this test pins that
  // shape so the bug can't sneak back.
  it('finds specter.yaml at the directory itself when isDirectory predicate is supplied', () => {
    const found = resolveManifestPath(
      '/home/user/project',                       // a directory path, no trailing slash
      p => p === '/home/user/project/specter.yaml',
      p => p === '/home/user/project',            // isDirectory: yes for this path
    );
    expect(found).toBe('/home/user/project/specter.yaml');
  });

  it('without isDirectory predicate, directory paths get their parent searched (backwards compat)', () => {
    // Matches the pre-v0.8.1 behaviour for callers who still pass a file path
    // and leave isDirectory off. The found manifest is one directory up.
    const found = resolveManifestPath(
      '/home/user/project/spec.yaml',             // file path — not a directory
      p => p === '/home/user/project/specter.yaml',
      // no third arg — defaults to "not a directory"
    );
    expect(found).toBe('/home/user/project/specter.yaml');
  });
});

// ---------------------------------------------------------------------------
// AC-22: Multi-root workspace — each folder has its own isolated client key
// ---------------------------------------------------------------------------

// @ac AC-22
describe('createClientKey', () => {
  it('produces distinct keys for different workspace folders', () => {
    const key1 = createClientKey('/workspace/project-a');
    const key2 = createClientKey('/workspace/project-b');
    expect(key1).not.toBe(key2);
  });

  it('produces the same key for the same folder path (idempotent)', () => {
    expect(createClientKey('/workspace/project-a')).toBe(createClientKey('/workspace/project-a'));
  });

  it('normalizes trailing slashes so /project and /project/ are the same client', () => {
    expect(createClientKey('/workspace/project/')).toBe(createClientKey('/workspace/project'));
  });
});

// ---------------------------------------------------------------------------
// AC-40: parse-on-edit hooks gate on isSpecFilePath
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-40
describe('isSpecFilePath', () => {
  it('accepts a double-extension .spec.yaml file', () => {
    expect(isSpecFilePath('/project/specs/auth.spec.yaml')).toBe(true);
  });

  it('rejects the project manifest specter.yaml', () => {
    expect(isSpecFilePath('/project/specter.yaml')).toBe(false);
  });

  it('rejects generic .yaml and .yml files', () => {
    expect(isSpecFilePath('/project/.github/workflows/ci.yml')).toBe(false);
    expect(isSpecFilePath('/project/config.yaml')).toBe(false);
  });

  it('rejects a bare ".spec.yaml" filename (no stem) as a guard edge case', () => {
    // Pathological input; treating it as a spec would crash the parser
    // on an empty spec name. The predicate rejects it to stay safe.
    expect(isSpecFilePath('/project/.spec.yaml')).toBe(false);
  });

  it('matches on basename, not substring, so "openapi-spec.yaml" is rejected', () => {
    // A file whose basename ends with "spec.yaml" but not ".spec.yaml".
    expect(isSpecFilePath('/project/docs/openapi-spec.yaml')).toBe(false);
  });

  it('accepts a nested spec file regardless of depth', () => {
    expect(isSpecFilePath('/project/specs/domain/a/b/c/foo.spec.yaml')).toBe(true);
  });
});
