/**
 * Scenario: Sprint 20 TASK-20.4 Project Detail Env Diff + Database
 * tabs unstubbed.
 *
 * Verifies pressing `2` / `3` on project detail no longer raises
 * the misleading "v0.2" alert (Sprint 20 silenced this) and that
 * the new read-only views actually render.
 */

import { openWebox, snapshot, expect, logSection } from "../lib.ts";

export async function run() {
  logSection("scenario: project-detail");

  const session = await openWebox({ cols: 120, rows: 35 });
  try {
    // Drill into project detail.
    await session.press("enter");
    await session.waitIdle({ timeout: 3000 });
    let txt = await session.text({ trimEnd: true });
    expect(
      /Project Detail|Overview/i.test(txt),
      "project-detail",
      "Enter drills from dashboard into Project Detail",
    );
    expect(
      txt.includes("[1] Overview") &&
        txt.includes("[2] Env Diff") &&
        txt.includes("[3] Database") &&
        txt.includes("[4] Logs"),
      "project-detail",
      "tab strip carries all 4 labels",
    );
    await snapshot(session, "12-project-detail-overview");

    // Tab 2 — Env Diff.
    await session.press("2");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      /Env Diff|Managed Secrets/i.test(txt),
      "project-detail",
      "2 switches to Env Diff tab",
    );
    expect(
      !/v0\.2 unlocked|unlocked in v0\.2|tab available in v0\.2/i.test(txt),
      "project-detail",
      "Env Diff no longer carries 'unlocked in v0.2' placeholder",
    );
    await snapshot(session, "13-project-detail-env-diff");

    // Tab 3 — Database.
    await session.press("3");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      /Database|mysql|psql/i.test(txt),
      "project-detail",
      "3 switches to Database tab with cheatsheet",
    );
    await snapshot(session, "14-project-detail-database");

    // Tab 4 — Logs.
    await session.press("4");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      /Live Logs|Logs/i.test(txt),
      "project-detail",
      "4 switches to Logs tab",
    );
    await snapshot(session, "15-project-detail-logs");

    // Tab back to dashboard from Overview (Sprint 20 alias).
    // The Logs tab owns its own key router (live-log scroll
    // semantics), so we first walk back to Overview before
    // testing the Tab → dashboard alias.
    await session.press("1");
    await session.waitIdle({ timeout: 2000 });
    await session.press("tab");
    await session.waitIdle({ timeout: 2000 });
    txt = await session.text({ trimEnd: true });
    expect(
      txt.includes("Active Projects"),
      "project-detail",
      "Tab from Overview returns to dashboard (Sprint 20 alias)",
    );
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
