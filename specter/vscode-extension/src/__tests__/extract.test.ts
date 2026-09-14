// @spec spec-vscode
//
// C-34 names the private copy ~/.specter/cli/specter-<version>. The plan
// targets that path, and the plan's tests prove it. This test proves the
// download actually lands there: extractBinary is fed a real tar.gz whose
// member is named "specter", as goreleaser produces, and asked to place it
// at a versioned target. An independent review found the tar branch left
// the file as <dir>/specter and then chmod'ed a path that did not exist, so
// every private download on Linux and macOS failed with ENOENT.

import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { extractBinary } from '../binaryDiscovery';

const describeWithTar = process.platform === 'win32' ? describe.skip : describe;

/** Builds a tar.gz in tmp whose single member is an executable named "specter". */
function archiveWithSpecter(tmp: string, body: string): Buffer {
  const src = path.join(tmp, 'src');
  fs.mkdirSync(src, { recursive: true });
  fs.writeFileSync(path.join(src, 'specter'), body, { mode: 0o755 });
  const archive = path.join(tmp, 'specter_0.15.0_linux_amd64.tar.gz');
  execFileSync('tar', ['czf', archive, '-C', src, 'specter']);
  return fs.readFileSync(archive);
}

// @ac AC-81
describeWithTar('[spec-vscode/AC-81] extractBinary places the archive member at the versioned private path', () => {
  it('lands the binary at ~/.specter/cli/specter-<version>, executable, and leaves no stray "specter"', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-extract-'));
    try {
      const data = archiveWithSpecter(tmp, '#!/bin/sh\necho specter version 0.15.0\n');
      const target = path.join(tmp, 'home', '.specter', 'cli', 'specter-0.15.0');

      await extractBinary(data, 'tar.gz', target);

      expect(fs.existsSync(target)).toBe(true);
      expect(fs.readFileSync(target, 'utf8')).toContain('specter version 0.15.0');
      expect(fs.statSync(target).mode & 0o111).not.toBe(0);
      // The member name must not survive beside the versioned file, or a
      // later plan would find a stale unversioned binary the extension
      // does not track.
      expect(fs.existsSync(path.join(path.dirname(target), 'specter'))).toBe(false);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  it('a second version extracts beside the first without disturbing it', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-extract-'));
    try {
      const dir = path.join(tmp, 'cli');
      await extractBinary(archiveWithSpecter(path.join(tmp, 'a'), 'A'), 'tar.gz', path.join(dir, 'specter-0.15.0'));
      await extractBinary(archiveWithSpecter(path.join(tmp, 'b'), 'B'), 'tar.gz', path.join(dir, 'specter-0.15.1'));
      expect(fs.readFileSync(path.join(dir, 'specter-0.15.0'), 'utf8')).toBe('A');
      expect(fs.readFileSync(path.join(dir, 'specter-0.15.1'), 'utf8')).toBe('B');
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });
});
