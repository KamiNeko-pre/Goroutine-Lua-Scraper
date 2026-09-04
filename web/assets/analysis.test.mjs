import assert from "node:assert/strict";
import test from "node:test";

import {
  buildOverview,
  compareSnapshots,
  evaluateRuleHealth,
  extractItems,
  normalizeSnapshot
} from "./analysis.js";

const rule = {
  id: "github_trending_go",
  source: "GitHub",
  schedule: "1h",
  min_items: 2,
  item_key: "url"
};

function snapshot(taskId, finishedAt, items, status = "succeeded", extra = {}) {
  return normalizeSnapshot({
    task_id: taskId,
    target: rule.id,
    status,
    finished_at: finishedAt,
    payload: status === "succeeded" ? JSON.stringify({ items }) : "",
    ...extra
  });
}

test("extractItems reads the common items envelope", () => {
  const result = extractItems(JSON.stringify({ items: [{ title: "Go", url: "https://go.dev" }], meta: { source: "GitHub" } }));
  assert.equal(result.error, null);
  assert.equal(result.items.length, 1);
  assert.equal(result.meta.source, "GitHub");
});

test("extractItems wraps the legacy single GitHub object", () => {
  const result = extractItems({ url: "https://github.com/golang/go", stars: 100 });
  assert.equal(result.legacy, true);
  assert.equal(result.items.length, 1);
  assert.equal(result.items[0].stars, 100);
});

test("extractItems returns a structured error for invalid JSON", () => {
  const result = extractItems("{broken");
  assert.equal(result.items.length, 0);
  assert.equal(result.error.code, "invalid_json");
});

test("compareSnapshots uses item_key and ignores array reorder", () => {
  const previous = snapshot(1, "2026-09-05T08:00:00Z", [
    { title: "A", url: "https://a", rank: 1 },
    { title: "B", url: "https://b", rank: 2 }
  ]);
  const current = snapshot(2, "2026-09-05T09:00:00Z", [
    { title: "B", url: "https://b", rank: 2 },
    { title: "A", url: "https://a", rank: 1 }
  ]);
  const result = compareSnapshots(previous, current, rule);
  assert.equal(result.added.length, 0);
  assert.equal(result.removed.length, 0);
  assert.equal(result.changed.length, 0);
});

test("compareSnapshots identifies additions, removals and ranking changes", () => {
  const previous = snapshot(1, "2026-09-05T08:00:00Z", [
    { title: "A", url: "https://a", rank: 1 },
    { title: "B", url: "https://b", rank: 2 }
  ]);
  const current = snapshot(2, "2026-09-05T09:00:00Z", [
    { title: "A", url: "https://a", rank: 2 },
    { title: "C", url: "https://c", rank: 1 }
  ]);
  const result = compareSnapshots(previous, current, rule);
  assert.equal(result.added[0].key, "https://c");
  assert.equal(result.removed[0].key, "https://b");
  assert.equal(result.changed[0].key, "https://a");
  assert.equal(result.countDelta, 0);
});

test("evaluateRuleHealth detects low item count and consecutive failures", () => {
  const failed = snapshot(3, "2026-09-05T10:00:00Z", [], "failed", {
    failure_code: "lua_execution",
    last_error: "selector not found"
  });
  const low = snapshot(2, "2026-09-05T09:00:00Z", [{ title: "A", url: "https://a" }]);
  const result = evaluateRuleHealth([failed, low], rule, new Date("2026-09-05T10:30:00Z"));
  assert.equal(result.status, "failing");
  assert.equal(result.consecutiveFailures, 1);
  assert.equal(result.reason, "selector not found");

  const degraded = evaluateRuleHealth([low], rule, new Date("2026-09-05T09:30:00Z"));
  assert.equal(degraded.status, "degraded");
  assert.match(degraded.reason, /低于最低阈值/);
});

test("evaluateRuleHealth reports unknown when a rule has no history", () => {
  const result = evaluateRuleHealth([], rule, new Date("2026-09-05T10:30:00Z"));
  assert.equal(result.status, "unknown");
  assert.equal(result.latestTaskId, null);
});

test("buildOverview aggregates runs, sources and changes", () => {
  const first = snapshot(1, "2026-09-04T08:00:00Z", [
    { title: "A", url: "https://a", rank: 1 },
    { title: "B", url: "https://b", rank: 2 }
  ]);
  const second = snapshot(2, "2026-09-05T08:00:00Z", [
    { title: "A", url: "https://a", rank: 2 },
    { title: "C", url: "https://c", rank: 1 }
  ]);
  const overview = buildOverview([first, second], [rule], new Date("2026-09-05T08:20:00Z"));
  assert.equal(overview.summary.itemCount, 4);
  assert.equal(overview.summary.runCount, 2);
  assert.equal(overview.summary.changeCount, 3);
  assert.equal(overview.sources[0].source, "GitHub");
  assert.equal(overview.health[0].status, "healthy");
  assert.equal(overview.latestItems.length, 2);
  assert.deepEqual(overview.latestItems.map((item) => item.__taskId), [2, 2]);
});
