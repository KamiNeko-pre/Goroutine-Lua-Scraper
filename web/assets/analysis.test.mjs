import assert from "node:assert/strict";
import test from "node:test";

import {
  buildOverview,
  buildInsightBrief,
  buildTrendStats,
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

test("comparison includes custom fields and ignores object property order", () => {
  const previous = { items: [{url:"x", extra:{price:10, count:2}}] };
  assert.equal(compareSnapshots(previous, {items:[{url:"x",extra:{price:11,count:2}}]}, rule).changed.length, 1);
  assert.equal(compareSnapshots(previous, {items:[{url:"x",extra:{price:10,count:3}}]}, rule).changed.length, 1);
  assert.equal(compareSnapshots(previous, {items:[{extra:{count:2,price:10},url:"x"}]}, rule).changed.length, 0);
});

test("invalid successful payload is not healthy even with zero minimum", () => {
  const broken = normalizeSnapshot({task_id:4,target:rule.id,status:"succeeded",finished_at:new Date().toISOString(),payload:"{invalid"});
  assert.notEqual(evaluateRuleHealth([broken], {...rule,min_items:0}).status, "healthy");
});

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

test("buildTrendStats distinguishes a real zero-valued previous point", () => {
  const stats = buildTrendStats([
    { bucket: "09-04", itemCount: 0, changeCount: 0 },
    { bucket: "09-05", itemCount: 2, changeCount: 1 }
  ]);
  assert.equal(stats.previousCount, 0);
  assert.equal(stats.delta, 2);
  assert.equal(stats.latestCount, 2);
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

test("buildInsightBrief turns aggregates into actionable conclusions", () => {
  const first = snapshot(1, "2026-09-04T08:00:00Z", [
    { title: "A", url: "https://a", rank: 1 },
    { title: "B", url: "https://b", rank: 2 }
  ]);
  const second = snapshot(2, "2026-09-05T08:00:00Z", [
    { title: "A", url: "https://a", rank: 2 },
    { title: "C", url: "https://c", rank: 1 }
  ]);
  const overview = buildOverview([first, second], [rule], new Date("2026-09-05T08:20:00Z"));
  const brief = buildInsightBrief(overview, [{ ...rule, name: "GitHub Go 趋势" }]);

  assert.equal(brief.movement.value, "GitHub Go 趋势");
  assert.match(brief.movement.detail, /3 项/);
  assert.equal(brief.coverage.value, "1 个来源");
  assert.equal(brief.trend.latestCount, 2);
  assert.equal(brief.trend.delta, 0);

  const briefWithHistoricalFailure = buildInsightBrief({
    ...overview,
    summary: { ...overview.summary, runCount: 3, failedRuns: 1 },
    health: [{ ...overview.health[0], status: "healthy" }]
  }, [{ ...rule, name: "GitHub Go 趋势" }]);
  assert.equal(briefWithHistoricalFailure.attention.detail, "存在历史失败运行，建议查看运行追踪");
});
