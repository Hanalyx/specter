// @spec spec-vscode
//
// C-27 as of 7.0.0: the private CLI copy fetches the version package.json
// declares under specterCli.default, not the extension's own version. The
// new function is reached through require() so the red is a runtime
// failure rather than a build failure under strict ts-jest.

import * as fs from 'fs';
import * as path from 'path';

// eslint-disable-next-line @typescript-eslint/no-var-requires, @typescript-eslint/no-explicit-any
const mod: any = require('../binaryDiscovery');

const PKG = { version: '0.15.2', specterCli: { default: '0.15.1', range: '>=0.15.0 <0.16.0' } };

// @ac AC-83
describe('[spec-vscode/AC-83] privateVersionFor: the declared default, not the extension version', () => {
  it('exports privateVersionFor', () => {
    expect(typeof mod.privateVersionFor).toBe('function');
  });

  it('an empty setting yields specterCli.default, even when the extension version differs', () => {
    expect(mod.privateVersionFor('', PKG)).toBe('0.15.1');
  });

  it('the latest setting is passed through for the caller to resolve', () => {
    expect(mod.privateVersionFor('latest', PKG)).toBe('latest');
  });

  it('a pinned setting is used as is', () => {
    expect(mod.privateVersionFor('0.15.0', PKG)).toBe('0.15.0');
  });

  it('a build without the field cannot resolve a private copy and names the field', () => {
    expect(() => mod.privateVersionFor('', { version: '0.15.2', specterCli: { range: '>=0.15.0 <0.16.0' } })).toThrow(/specterCli\.default/);
    expect(() => mod.privateVersionFor('', { version: '0.15.2' })).toThrow(/specterCli\.default/);
  });
});

// @ac AC-83
describe('[spec-vscode/AC-83] package.json declares the CLI the repository ships', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'package.json'), 'utf8'));
  const repoVersion = fs.readFileSync(path.join(__dirname, '..', '..', '..', 'VERSION'), 'utf8').trim();

  it('specterCli.default is a plain MAJOR.MINOR.PATCH', () => {
    expect(pkg.specterCli).toBeDefined();
    expect(pkg.specterCli.default).toMatch(/^\d+\.\d+\.\d+$/);
  });

  it('specterCli.default equals the repository VERSION file', () => {
    expect(pkg.specterCli.default).toBe(repoVersion);
  });

  it('specterCli.default satisfies specterCli.range', () => {
    expect(mod.satisfiesRange(pkg.specterCli.default, pkg.specterCli.range)).toBe(true);
  });
});
