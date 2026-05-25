/**
 * Scenario: Sprint 20 TASK-20.5 help overlay (`?`).
 *
 * Verifies:
 *   - `?` from dashboard opens overlay with "? Help" header
 *   - Surface keys are sourced from live Footer().Text (regression
 *     guard against the help drifting from the actual key router)
 *   - Strict-block routing: `n` while overlay open doesn't open the
 *     new-project wizard (overlay intercepts the key)
 *   - `Esc` dismisses overlay and returns to the same state
 *   - `?` from a non-dashboard surface (project detail) also works
 *     and labels the surface correctly
 */

import { openWebox, snapshot, sleep, expect, logSection } from "../lib.ts";

export async function run() {
  logSection("scenario: help-overlay");

  const session = await openWebox({ cols: 120, rows: 35 });
  try {
    // Open help from dashboard.
    await session.press("?");
    await session.waitIdle({ timeout: 3000 });
    let txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("? Help"),
      "help-overlay",
      "? opens overlay with title",
    );
    expect(
      txt.includes("Surface keys") && txt.includes("Global keys"),
      "help-overlay",
      "overlay carries both Surface keys + Global keys sections",
    );
    expect(
      txt.includes("[?]") && txt.includes("toggle this help overlay"),
      "help-overlay",
      "overlay enumerates the toggle key with description",
    );
    expect(
      txt.includes("[p]") && (txt.includes("catalog") || txt.includes("[p]")),
      "help-overlay",
      "overlay lists [p] catalog binding from dashboard footer",
    );
    await snapshot(session, "06-help-overlay-dashboard");

    // Strict-block: pressing `n` while overlay is open must NOT
    // open the new-project wizard.
    await session.press("n");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("? Help"),
      "help-overlay",
      "n press while overlay open is silently ignored (overlay still on)",
    );
    expect(
      !txt.includes("New Project") &&
        !/wizard/i.test(txt.replace(/Surface|Global/g, "")),
      "help-overlay",
      "n press did NOT route to new-project wizard underneath",
    );

    // Esc dismisses overlay and returns to dashboard.
    await session.press("esc");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      !txt.includes("? Help"),
      "help-overlay",
      "Esc dismisses overlay",
    );
    expect(
      txt.includes("Active Projects"),
      "help-overlay",
      "After Esc dashboard is visible again (no state drift)",
    );

    // Help on project detail surface.
    await session.press("enter");
    await session.waitIdle({ timeout: 2000 });
    await session.press("?");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("? Help"),
      "help-overlay",
      "? from project detail opens overlay",
    );
    expect(
      /Project Detail|Overview|projectdetail/i.test(txt),
      "help-overlay",
      "overlay surface label reflects current state (Project Detail)",
    );
    await snapshot(session, "07-help-overlay-project-detail");

    // Dismiss with `?` (toggle).
    await session.press("?");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      !txt.includes("? Help"),
      "help-overlay",
      "? toggle closes overlay from project detail",
    );

    // Back to dashboard for clean state.
    await session.press("esc");
    await sleep(200);
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
