/**
 * Scenario: Bento Mode switching on terminal resize.
 *
 * Verifies the Sprint 13 Bento layout engine flips between Tiny,
 * Standard, Ultra, and UltraPlus tiers when the operator resizes
 * the terminal. Regression guard for Sprint 20 TASK-20.3 (Standard
 * mode mini-bento redesign) and the implicit contract that every
 * resize redraws within one tea.Cmd tick.
 */

import { openWebox, snapshot, expect, logSection } from "../lib.ts";

export async function run() {
  logSection("scenario: bento-modes");

  const session = await openWebox({ cols: 160, rows: 45 });
  try {
    // UltraPlus baseline (160×45).
    let txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects") && txt.includes("CI/CD"),
      "bento-modes",
      "UltraPlus 160x45 renders Projects + CI/CD tiles",
    );
    await snapshot(session, "01-bento-ultraplus-160x45");

    // Drop into Ultra (120×35).
    await session.resize({ cols: 120, rows: 35 });
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "bento-modes",
      "Ultra 120x35 still renders Projects tile after shrink",
    );
    await snapshot(session, "02-bento-ultra-120x35");

    // Drop into Standard (100×30) — Sprint 20 TASK-20.3 redesign:
    // expects mini CI/CD + Live Log strips.
    await session.resize({ cols: 100, rows: 30 });
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "bento-modes",
      "Standard 100x30 keeps Projects panel",
    );
    expect(
      /SERVER/.test(txt),
      "bento-modes",
      "Standard 100x30 shows compact server overview",
    );
    await snapshot(session, "03-bento-standard-100x30");

    // Tiny (60×18) — should show the resize-hint fallback, NOT
    // the bogus "[r] to redraw" string from pre-Sprint-20.
    await session.resize({ cols: 60, rows: 18 });
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      !txt.includes("[r] to redraw"),
      "bento-modes",
      "Tiny fallback no longer advertises non-existent [r] keybinding",
    );
    expect(
      /resize|Tiny|tiny/i.test(txt),
      "bento-modes",
      "Tiny fallback prompts operator to resize",
    );
    await snapshot(session, "04-bento-tiny-60x18");

    // Back up to Ultra — verify the cockpit recovers without
    // requiring quit/restart (regression for Sprint 19
    // resize-state leak).
    await session.resize({ cols: 120, rows: 35 });
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "bento-modes",
      "Resize Tiny → Ultra recovers cockpit without restart",
    );
    await snapshot(session, "05-bento-ultra-after-tiny");
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
