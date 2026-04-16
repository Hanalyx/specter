// @spec spec-vscode
//
// Tests for extension activation logic and multi-root workspace isolation.

import { shouldActivate, resolveManifestPath, createClientKey } from '../activation';
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
