/**
 * Scenario: Sprint 20 TASK-20.1 mouse left-click hit testing.
 *
 * We can't use `session.click(pattern)` directly because Webox
 * dashboard rows are inside a styled lipgloss panel and the
 * tuistory `click` API resolves to text matches that may collide
 * with the status bar. Instead we use `clickAt(x, y)` with
 * coordinates from the known bento layout (Sprint 20 TASK-20.1
 * `bento.LayoutMap`):
 *
 *   - Projects tile lives in the top-left quadrant on Ultra 120x35,
 *     roughly x∈[2, 58], y∈[2, 11].
 *   - Click anywhere inside that tile drills into the selected
 *     project's detail.
 *   - Click on the status bar (y=0) is a no-op.
 *
 * We assert by checking the post-click text. If the layout map
 * changes in a future sprint, this scenario is the early-warning
 * canary; update the (x, y) targets then.
 */

import { openWebox, snapshot, expect, logSection, sleep } from "../lib.ts";

export async function run() {
  logSection("scenario: mouse-click");

  const session = await openWebox({ cols: 120, rows: 35 });
  try {
    let txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "mouse-click",
      "dashboard renders before any clicks",
    );
    await snapshot(session, "16-mouse-before-click");

    // Click on the status bar (row 0) — should be a no-op per
    // Sprint 20 layout-aware contract.
    await session.clickAt(10, 0);
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "mouse-click",
      "click on status bar is a no-op (stayed on dashboard)",
    );

    // Click inside the Projects tile — drill into Project Detail.
    // The Projects tile on Ultra 120x35 starts around y=3 (after
    // the status bar at y=0 and tile border at y=1-2). We pick
    // the centre of the first row inside the tile body.
    await session.clickAt(10, 5);
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      /Project Detail|Overview/i.test(txt),
      "mouse-click",
      "click on Projects tile drills into Project Detail",
    );
    await snapshot(session, "17-mouse-after-projects-click");

    // Click on Project Detail returns to dashboard.
    await session.clickAt(30, 10);
    await session.waitIdle({ timeout: 3000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "mouse-click",
      "click on Project Detail body returns to dashboard",
    );

    await sleep(150);
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
