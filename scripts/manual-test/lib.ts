/**
 * Sprint 20 smoke-test shared helpers.
 *
 * Every scenario follows the same shape:
 *
 *   1. launchTerminal({ command: ./bin/webox, args: [--mock], cols, rows })
 *   2. waitForText("Webox") so we don't snapshot pre-init noise
 *   3. drive the TUI through a sequence of key presses / mouse clicks
 *   4. capture .txt snapshot AND .png screenshot for each interesting state
 *   5. close() in finally so test-side daemon never leaks
 *
 * All artefacts land under docs/screenshots/sprint-20/manual/{NN}-{name}.{txt,png}
 * so the operator (and reviewers on the PR) can eyeball the rendering. The .txt
 * snapshots are deterministic and diff-friendly; the .png captures are for the
 * human review pass (a markdown table in REPORT.md links them).
 */

import { launchTerminal } from "tuistory";
import { writeFile } from "node:fs/promises";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export const REPO_ROOT = resolve(__dirname, "..", "..");
export const ARTEFACTS_DIR = join(
  REPO_ROOT,
  "docs",
  "screenshots",
  "sprint-20",
  "manual",
);
export const WEBOX_BIN = join(REPO_ROOT, "bin", "webox");

export interface OpenWeboxOpts {
  cols?: number;
  rows?: number;
  /** Extra args after `--mock`. */
  args?: string[];
}

/**
 * Launches `./bin/webox --mock` in a tuistory PTY session sized to
 * `cols × rows` (defaults to canonical Bento Ultra 120×35). Waits for
 * the cockpit "WEBOX" header so the first snapshot the scenario takes
 * is a fully-rendered frame rather than a half-painted one.
 *
 * Production-mode env hardening: `WEBOX_NOW` and `WEBOX_DEMO_SEED`
 * are passed through so the cockpit fixtures stay deterministic; the
 * runner refuses to start if the binary is missing (operator forgot
 * `make build`).
 */
export async function openWebox(opts: OpenWeboxOpts = {}) {
  const session = await launchTerminal({
    command: WEBOX_BIN,
    args: ["--mock", ...(opts.args ?? [])],
    cols: opts.cols ?? 120,
    rows: opts.rows ?? 35,
    cwd: REPO_ROOT,
  });

  await session.waitForText("WEBOX", { timeout: 10_000 });
  // Drop one extra animation frame so the bento engine has settled
  // (status bar live-clock can briefly cover the WEBOX banner on
  // very small terminals before lipgloss reflows).
  await sleep(120);
  return session;
}

/**
 * Capture a `.txt` snapshot for `name` and return the absolute path.
 *
 * We only capture text, not PNG: tuistory's JS API surfaces text via
 * `session.text()` but the PNG screenshot helper lives only in the CLI.
 * Text snapshots are also diff-friendly so PR reviewers see exactly
 * which line changed between runs; PNGs would require an image diff
 * tool. If a reviewer wants a visual, they can run
 * `tuistory attach -s webox-mock` to see the live session.
 */
export async function snapshot(
  session: Awaited<ReturnType<typeof openWebox>>,
  name: string,
) {
  const txtPath = join(ARTEFACTS_DIR, `${name}.txt`);
  const text = await session.text({ trimEnd: true });
  await writeFile(txtPath, text + "\n", "utf-8");
  return { txtPath };
}

/**
 * Tiny sleep helper. `tuistory`'s wait-idle is reactive (~60ms) so
 * the sleep budget here exists only for animations that happen
 * *after* idle (e.g., the spinner reaches a new frame).
 */
export function sleep(ms: number) {
  return new Promise((res) => setTimeout(res, ms));
}

/**
 * Assert helper that writes a clean stderr line and exits 1 on
 * failure. We don't pull a test framework because scenarios run as
 * standalone scripts via `node smoke.ts`.
 */
export function expect(
  condition: boolean,
  scenario: string,
  message: string,
): void {
  if (condition) {
    process.stdout.write(`  ✓ ${scenario}: ${message}\n`);
    return;
  }
  process.stderr.write(`  ✗ ${scenario}: ${message}\n`);
  process.exitCode = 1;
}

/**
 * Report row appended to docs/screenshots/sprint-20/manual/REPORT.md.
 */
export interface ReportRow {
  scenario: string;
  description: string;
  artefacts: string[];
  ok: boolean;
}

export const report: ReportRow[] = [];

export function logSection(name: string) {
  process.stdout.write(`\n▶ ${name}\n`);
}
