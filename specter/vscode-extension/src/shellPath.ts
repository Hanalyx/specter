// @spec spec-vscode
//
// Pure helpers for "Specter: Add CLI to Shell PATH" command.
// Kept free of vscode / fs imports so the detection and idempotency logic
// can be unit-tested without mocks.

import * as path from 'path';

export interface DetectShellInput {
  /** $SHELL env var (e.g. "/usr/bin/bash"). */
  shell: string;
  /** process.platform value ("linux" | "darwin" | "win32" | ...). */
  platform: string;
  /** os.homedir() result. */
  home: string;
}

export interface ShellConfig {
  /** Absolute path to the rc file we should append to. */
  rcFile: string;
  /** The export line to add, in the shell's own syntax. */
  exportLine: string;
  /** Canonical shell name ("bash" | "zsh" | "fish"). */
  shell: 'bash' | 'zsh' | 'fish';
}

/** Marker written above the export line so we can identify our own edits. */
export const SPECTER_MARKER =
  '# Added by Specter VS Code extension — https://marketplace.visualstudio.com/items?itemName=Hanalyx.specter-vscode';

/**
 * Resolves which rc file to append to and which export syntax to use, given
 * the user's shell and platform. Returns null for unknown shells so the
 * command can fall back to showing a manual command.
 *
 * Bash on Linux: .bashrc. Bash on macOS: .bash_profile (macOS bash sessions
 * are login shells by default, which source .bash_profile not .bashrc).
 */
export function detectShellConfig(input: DetectShellInput, binDir: string): ShellConfig | null {
  const shellName = path.basename(input.shell || '').toLowerCase();

  switch (shellName) {
    case 'bash': {
      const rcFile =
        input.platform === 'darwin'
          ? path.join(input.home, '.bash_profile')
          : path.join(input.home, '.bashrc');
      return {
        shell: 'bash',
        rcFile,
        exportLine: `export PATH="${binDir}:$PATH"`,
      };
    }
    case 'zsh':
      return {
        shell: 'zsh',
        rcFile: path.join(input.home, '.zshrc'),
        exportLine: `export PATH="${binDir}:$PATH"`,
      };
    case 'fish':
      return {
        shell: 'fish',
        rcFile: path.join(input.home, '.config', 'fish', 'config.fish'),
        exportLine: `fish_add_path ${binDir}`,
      };
    default:
      return null;
  }
}

/**
 * Returns true if `binDir` is referenced by any non-comment line in `contents`.
 *
 * This is intentionally permissive — any live reference to the bin dir means
 * we should not append again, regardless of whether it was added by us or by
 * the user. Re-running the command must be idempotent.
 */
export function isPathAlreadyPresent(contents: string, binDir: string): boolean {
  for (const rawLine of contents.split('\n')) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    if (line.includes(binDir)) return true;
  }
  return false;
}

/**
 * Formats the block to append. Blank line before so we don't merge into
 * whatever's above, marker comment so the edit is identifiable, then the
 * export line.
 */
export function formatAppendBlock(exportLine: string): string {
  return `\n${SPECTER_MARKER}\n${exportLine}\n`;
}

/**
 * Pure decision: should we prompt the user to add the CLI to their PATH?
 *
 * `rcContents` is the current content of the detected shell's rc file, or
 * null when the file does not exist on disk. `dismissed` is whatever the
 * caller pulled out of persistent state for the "don't show again" flag.
 *
 * We prompt only when all three are true:
 *   - the rc file exists (we don't create a shell config on someone's
 *     behalf without invitation)
 *   - the bin dir is not already referenced in it
 *   - the user has not previously opted out
 */
export function shouldPromptAddPath(
  rcContents: string | null,
  binDir: string,
  dismissed: boolean,
): boolean {
  if (dismissed) return false;
  if (rcContents === null) return false;
  return !isPathAlreadyPresent(rcContents, binDir);
}
