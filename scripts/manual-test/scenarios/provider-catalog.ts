/**
 * Scenario: Sprint 20 TASK-20.2 Provider Catalog (`p`).
 *
 * Verifies the embedded preset registry is exposed in the TUI with
 * deterministic ordering, cursor navigation, detail toggle, and
 * clipboard-copy hint surfacing. Clipboard backend itself isn't
 * exercised in headless CI (we'd need pbcopy/xsel/xclip installed
 * and accessible); the test only checks that the *hint* line
 * surfaces on `c`.
 */

import { openWebox, snapshot, expect, logSection, sleep } from "../lib.ts";

export async function run() {
  logSection("scenario: provider-catalog");

  const session = await openWebox({ cols: 140, rows: 45 });
  try {
    // Open catalog.
    await session.press("p");
    await session.waitIdle({ timeout: 3000 });
    let txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Provider Catalog"),
      "provider-catalog",
      "p opens Provider Catalog screen",
    );
    expect(
      txt.includes("Poland"),
      "provider-catalog",
      "catalog groups by region (Poland present)",
    );
    expect(
      txt.includes("smallhost-devil"),
      "provider-catalog",
      "smallhost-devil preset is listed",
    );
    expect(
      /VERIFIED|verified/.test(txt),
      "provider-catalog",
      "verified status pill present",
    );
    await snapshot(session, "08-catalog-default");

    // Cursor down — should move pill to the next entry.
    await session.press("down");
    await session.waitIdle({ timeout: 2000 });
    await session.press("down");
    await session.waitIdle({ timeout: 2000 });
    await snapshot(session, "09-catalog-cursor-moved");

    // Enter expands deep-dive.
    await session.press("enter");
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Detail") || txt.includes("panel"),
      "provider-catalog",
      "Enter expands the detail strip",
    );
    await snapshot(session, "10-catalog-detail-expanded");

    // Copy briefing — at minimum the hint line surfaces (clipboard
    // backend may be missing in headless mode, then we expect a
    // remediation hint).
    await session.press("c");
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      /clipboard|copied|briefing/i.test(txt),
      "provider-catalog",
      "c surfaces clipboard hint (success or remediation)",
    );
    await snapshot(session, "11-catalog-copy-hint");

    // Esc returns to dashboard.
    await session.press("esc");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "provider-catalog",
      "Esc returns from catalog to dashboard",
    );
    await sleep(100);
  } finally {
    session.close();
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  run().catch((e) => {
    console.error(e);
    process.exit(1);
  });
}
