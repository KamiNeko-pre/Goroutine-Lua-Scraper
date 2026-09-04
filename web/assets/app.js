import {
  buildOverview,
  normalizeSnapshot,
  readPath
} from "./analysis.js";
import {
  fetchRules,
  fetchSnapshots,
  fetchTaskEvents
} from "./api.js";

const DEMO_MODE = new URLSearchParams(window.location.search).get("demo") === "1";

const state = {
  loading: true,
  error: null,
  filters: { ruleId: "all", range: "7d" },
  rules: [],
  snapshots: [],
  overview: null,
  selectedTaskId: null,
  events: [],
  traceLoading: false
};

const demoRules = [
  {
    id: "github_trending_go",
    name: "GitHub Go 趋势",
    source: "GitHub",
    schedule: "6h",
    min_items: 3,
    item_key: "url",
    display: { title: "name", url: "url", score: "stars", rank: "rank" }
  },
  {
    id: "hackernews_top",
    name: "Hacker News 热榜",
    source: "Hacker News",
    schedule: "3h",
    min_items: 4,
    item_key: "url",
    display: { title: "title", url: "url", score: "points", rank: "rank" }
  },
  {
    id: "public_source_demo",
    name: "待接入公开来源",
    source: "其他来源",
    schedule: "12h",
    min_items: 1,
    item_key: "url",
    display: { title: "title", url: "url" }
  }
];

const demoSnapshots = [
  {
    task_id: 1042,
    target: "github_trending_go",
    status: "succeeded",
    finished_at: "2026-09-05T07:42:00Z",
    payload: JSON.stringify({
      items: [
        { name: "gin-gonic/gin", url: "https://github.com/gin-gonic/gin", stars: 82000, rank: 1, extra: { language: "Go" } },
        { name: "charmbracelet/bubbletea", url: "https://github.com/charmbracelet/bubbletea", stars: 29000, rank: 2, extra: { language: "Go" } },
        { name: "go-kratos/kratos", url: "https://github.com/go-kratos/kratos", stars: 22000, rank: 3, extra: { language: "Go" } },
        { name: "jackc/pgx", url: "https://github.com/jackc/pgx", stars: 13000, rank: 4, extra: { language: "Go" } }
      ],
      meta: { source: "GitHub", collected_at: "2026-09-05T07:42:00Z" }
    })
  },
  {
    task_id: 1041,
    target: "hackernews_top",
    status: "succeeded",
    finished_at: "2026-09-05T06:55:00Z",
    payload: JSON.stringify({
      items: [
        { title: "A practical guide to Go concurrency", url: "https://news.ycombinator.com/item?id=1", points: 412, rank: 1, extra: { comments: 98 } },
        { title: "Building reliable event pipelines", url: "https://news.ycombinator.com/item?id=2", points: 287, rank: 2, extra: { comments: 64 } },
        { title: "What we learned from a failed migration", url: "https://news.ycombinator.com/item?id=3", points: 231, rank: 3, extra: { comments: 51 } },
        { title: "The shape of modern developer tools", url: "https://news.ycombinator.com/item?id=4", points: 189, rank: 4, extra: { comments: 32 } }
      ],
      meta: { source: "Hacker News", collected_at: "2026-09-05T06:55:00Z" }
    })
  },
  {
    task_id: 1038,
    target: "github_trending_go",
    status: "succeeded",
    finished_at: "2026-09-04T07:40:00Z",
    payload: JSON.stringify({
      items: [
        { name: "gin-gonic/gin", url: "https://github.com/gin-gonic/gin", stars: 81500, rank: 1, extra: { language: "Go" } },
        { name: "go-kratos/kratos", url: "https://github.com/go-kratos/kratos", stars: 21800, rank: 2, extra: { language: "Go" } },
        { name: "spf13/cobra", url: "https://github.com/spf13/cobra", stars: 37000, rank: 3, extra: { language: "Go" } }
      ],
      meta: { source: "GitHub", collected_at: "2026-09-04T07:40:00Z" }
    })
  },
  {
    task_id: 1037,
    target: "hackernews_top",
    status: "failed",
    finished_at: "2026-09-04T06:52:00Z",
    failure_code: "lua_execution",
    last_error: "selector not found"
  },
  {
    task_id: 1031,
    target: "hackernews_top",
    status: "succeeded",
    finished_at: "2026-09-03T06:50:00Z",
    payload: JSON.stringify({
      items: [
        { title: "A practical guide to Go concurrency", url: "https://news.ycombinator.com/item?id=1", points: 405, rank: 1, extra: { comments: 93 } },
        { title: "How to design useful metrics", url: "https://news.ycombinator.com/item?id=5", points: 156, rank: 2, extra: { comments: 27 } },
        { title: "The shape of modern developer tools", url: "https://news.ycombinator.com/item?id=4", points: 170, rank: 3, extra: { comments: 29 } },
        { title: "Making small systems observable", url: "https://news.ycombinator.com/item?id=6", points: 142, rank: 4, extra: { comments: 22 } }
      ],
      meta: { source: "Hacker News", collected_at: "2026-09-03T06:50:00Z" }
    })
  }
];

const demoEvents = {
  1042: [
    { stage: "accepted", level: "info", message: "任务已受理", created_at: "2026-09-05T07:40:02Z" },
    { stage: "published", level: "info", message: "已写入 Redis Stream", created_at: "2026-09-05T07:40:03Z" },
    { stage: "claimed", level: "info", message: "worker-02 获取任务租约", created_at: "2026-09-05T07:40:03Z" },
    { stage: "lua_finished", level: "info", message: "Lua 返回 4 条数据", created_at: "2026-09-05T07:42:00Z" },
    { stage: "mysql_committed", level: "info", message: "结果快照已提交", created_at: "2026-09-05T07:42:00Z" },
    { stage: "acked", level: "info", message: "Redis 消息已确认", created_at: "2026-09-05T07:42:00Z" }
  ],
  1041: [
    { stage: "accepted", level: "info", message: "任务已受理", created_at: "2026-09-05T06:53:01Z" },
    { stage: "claimed", level: "info", message: "worker-01 获取任务租约", created_at: "2026-09-05T06:53:02Z" },
    { stage: "lua_finished", level: "info", message: "Lua 返回 4 条数据", created_at: "2026-09-05T06:55:00Z" },
    { stage: "mysql_committed", level: "info", message: "结果快照已提交", created_at: "2026-09-05T06:55:00Z" },
    { stage: "acked", level: "info", message: "Redis 消息已确认", created_at: "2026-09-05T06:55:00Z" }
  ],
  1037: [
    { stage: "accepted", level: "info", message: "任务已受理", created_at: "2026-09-04T06:50:01Z" },
    { stage: "claimed", level: "info", message: "worker-01 获取任务租约", created_at: "2026-09-04T06:50:02Z" },
    { stage: "failed", level: "error", code: "lua_execution", message: "selector not found", created_at: "2026-09-04T06:52:00Z" }
  ]
};

const stageLabels = {
  accepted: "任务受理",
  outbox_created: "创建 Outbox",
  published: "投递 Stream",
  claimed: "Worker 获取",
  lua_finished: "Lua 执行完成",
  result_validated: "结果校验",
  mysql_committed: "结果入库",
  completed: "任务完成",
  failed: "执行失败",
  acked: "消息确认"
};

const healthLabels = {
  healthy: "健康",
  degraded: "需关注",
  failing: "失败",
  unknown: "暂无数据"
};

function $(selector) {
  return document.querySelector(selector);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function formatNumber(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value) || 0);
}

function formatDate(value, withTime = true) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("zh-CN", withTime
    ? { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }
    : { month: "2-digit", day: "2-digit" }).format(date);
}

function rangeStart(range) {
  const milliseconds = { "24h": 24 * 60 * 60 * 1000, "7d": 7 * 24 * 60 * 60 * 1000, "30d": 30 * 24 * 60 * 60 * 1000 };
  return new Date(Date.now() - (milliseconds[range] ?? milliseconds["7d"]));
}

function activeSnapshots() {
  const from = rangeStart(state.filters.range).getTime();
  return state.snapshots.filter((snapshot) => {
    if (state.filters.ruleId !== "all" && snapshot.ruleId !== state.filters.ruleId) {
      return false;
    }
    const timestamp = snapshot.finishedAt ? new Date(snapshot.finishedAt).getTime() : 0;
    return timestamp >= from;
  });
}

function setConnectionStatus(text, live) {
  const status = $(".sync-status");
  if (!status) return;
  status.innerHTML = `<span class="status-dot ${live ? "is-live" : ""}" aria-hidden="true"></span>${escapeHTML(text)}`;
}

function renderLoading() {
  state.loading = true;
  for (const element of document.querySelectorAll("[data-summary]")) {
    element.textContent = "--";
  }
  $("#last-sync").textContent = "正在读取";
  $("#data-window").textContent = "分析窗口：读取中";
  $("#trend-chart").innerHTML = '<div class="empty-state compact">正在读取快照</div>';
  $("#source-list").innerHTML = '<div class="empty-state compact">正在读取来源</div>';
  $("#items-table-wrap").innerHTML = '<div class="empty-state">正在整理数据</div>';
  $("#health-list").innerHTML = '<div class="empty-state compact">正在检查规则</div>';
}

function normalizeRules(rules) {
  return (rules ?? []).map((rule) => ({
    id: String(rule.id ?? rule.rule_id ?? "unknown"),
    name: String(rule.name ?? rule.id ?? "未命名规则"),
    source: String(rule.source ?? rule.id ?? "未知来源"),
    schedule: rule.schedule ?? "24h",
    min_items: Number(rule.min_items ?? 0),
    item_key: rule.item_key ?? rule.itemKey ?? "url",
    display: rule.display ?? {}
  }));
}

function normalizeRecords(records) {
  return (records ?? []).map(normalizeSnapshot);
}

function demoData() {
  return { rules: demoRules, snapshots: demoSnapshots };
}

async function loadDashboard() {
  renderLoading();
  state.error = null;
  try {
    const data = DEMO_MODE
      ? demoData()
      : await (async () => {
        const range = rangeStart(state.filters.range).toISOString();
        const [rules, snapshots] = await Promise.all([
          fetchRules(),
          fetchSnapshots({ ruleId: state.filters.ruleId, from: range, limit: 200 })
        ]);
        return { rules, snapshots };
      })();

    state.rules = normalizeRules(data.rules);
    state.snapshots = normalizeRecords(data.snapshots);
    state.overview = buildOverview(activeSnapshots(), state.rules, new Date());
    state.loading = false;
    setConnectionStatus(DEMO_MODE ? "演示数据" : "数据连接正常", true);
    renderRuleFilter();
    renderOverview();
    if (state.selectedTaskId === null) {
      state.selectedTaskId = state.overview.snapshots[0]?.taskId ?? null;
    }
    await loadTrace(state.selectedTaskId);
  } catch (error) {
    state.loading = false;
    state.error = error instanceof Error ? error : new Error("读取分析数据失败");
    setConnectionStatus("数据连接异常", false);
    renderError(state.error.message);
  }
}

function renderRuleFilter() {
  const select = $("#rule-filter");
  const previous = state.filters.ruleId;
  select.innerHTML = '<option value="all">全部规则</option>' + state.rules.map((rule) =>
    `<option value="${escapeHTML(rule.id)}">${escapeHTML(rule.name)}</option>`).join("");
  select.value = state.rules.some((rule) => rule.id === previous) ? previous : "all";
}

function renderOverview() {
  if (!state.overview) return;
  const { summary } = state.overview;
  $("[data-summary=\"itemCount\"]").textContent = formatNumber(summary.itemCount);
  $("[data-summary=\"runCount\"]").textContent = formatNumber(summary.runCount);
  $("[data-summary=\"changeCount\"]").textContent = formatNumber(summary.changeCount);
  $("[data-summary=\"healthyRules\"]").textContent = `${summary.healthyRules}/${state.rules.length}`;
  $("[data-summary-foot=\"itemCount\"]").textContent = `${summary.successfulRuns} 次成功运行贡献`;
  $("[data-summary-foot=\"runCount\"]").textContent = summary.failedRuns ? `${summary.failedRuns} 次需要排查` : "没有失败运行";
  $("[data-summary-foot=\"changeCount\"]").textContent = "基于相邻成功快照";
  $("[data-summary-foot=\"healthyRules\"]").textContent = summary.healthyRules === state.rules.length ? "全部规则正常" : "点击右侧查看原因";
  $("#last-sync").textContent = formatDate(state.overview.snapshots[0]?.finishedAt);
  $("#data-window").textContent = `分析窗口：最近 ${state.filters.range === "24h" ? "24 小时" : state.filters.range === "30d" ? "30 天" : "7 天"}`;
  $("#trend-range").textContent = `${state.overview.trend.length} 个时间点`;
  $("#source-count").textContent = `${state.overview.sources.length} 个来源`;
  renderTrend();
  renderSources();
  renderHealth();
  renderItems();
}

function renderTrend() {
  const chart = $("#trend-chart");
  const trend = [...state.overview.trend].sort((left, right) => left.bucket.localeCompare(right.bucket));
  if (!trend.length) {
    chart.innerHTML = '<div class="empty-state compact">当前窗口没有可分析的成功快照</div>';
    return;
  }
  const max = Math.max(...trend.map((entry) => entry.itemCount), 1);
  chart.innerHTML = trend.map((entry) => {
    const volumeHeight = Math.max(4, Math.round((entry.itemCount / max) * 100));
    const changeHeight = Math.max(3, Math.round((entry.changeCount / max) * 100));
    return `<div class="trend-column">
      <div class="bar-stage" title="${escapeHTML(entry.bucket)}：${formatNumber(entry.itemCount)} 条数据，${formatNumber(entry.changeCount)} 项变化">
        <span class="bar bar-volume" style="height:${volumeHeight}%"></span>
        <span class="bar bar-change" style="height:${changeHeight}%"></span>
      </div>
      <span class="bar-label">${escapeHTML(entry.bucket)}</span>
    </div>`;
  }).join("");
}

function renderSources() {
  const list = $("#source-list");
  if (!state.overview.sources.length) {
    list.innerHTML = '<div class="empty-state compact">当前窗口没有来源数据</div>';
    return;
  }
  const max = Math.max(...state.overview.sources.map((source) => source.itemCount), 1);
  list.innerHTML = state.overview.sources.map((source, index) => `<button class="source-row source-row-button" type="button" data-source="${escapeHTML(source.source)}">
    <span class="source-row-top"><span class="source-name">${escapeHTML(source.source)}</span><span class="source-number">${formatNumber(source.itemCount)} 条</span></span>
    <span class="source-track"><span class="source-fill ${index === 1 ? "is-secondary" : index > 1 ? "is-tertiary" : ""}" style="width:${Math.max(4, Math.round((source.itemCount / max) * 100))}%"></span></span>
  </button>`).join("");
}

function renderHealth() {
  const list = $("#health-list");
  if (!state.overview.health.length) {
    list.innerHTML = '<div class="empty-state compact">没有可检查的规则</div>';
    return;
  }
  list.innerHTML = state.overview.health.map((health) => {
    const rule = state.rules.find((entry) => entry.id === health.ruleId);
    return `<div class="health-row">
      <div class="health-row-top">
        <button class="health-rule health-rule-button" type="button" data-rule-id="${escapeHTML(health.ruleId)}">${escapeHTML(rule?.name ?? health.ruleId)}</button>
        <span class="health-badge is-${escapeHTML(health.status)}">${escapeHTML(healthLabels[health.status] ?? health.status)}</span>
      </div>
      <div class="health-row-bottom"><span class="health-reason" title="${escapeHTML(health.reason)}">${escapeHTML(health.reason)}</span><span class="health-count">${formatNumber(health.itemCount)} 条</span></div>
    </div>`;
  }).join("");
}

function changeLookup() {
  const lookup = new Map();
  for (const change of state.overview.changes) {
    for (const entry of change.added) lookup.set(`${change.ruleId}:${entry.key}`, "added");
    for (const entry of change.changed) lookup.set(`${change.ruleId}:${entry.key}`, "changed");
  }
  return lookup;
}

function displayValue(item, rule, field, fallback = "--") {
  const path = rule?.display?.[field];
  const value = path ? readPath(item, path) : item?.[field];
  return value === undefined || value === null || value === "" ? fallback : value;
}

function renderItems() {
  const wrap = $("#items-table-wrap");
  const items = state.overview.latestItems;
  $("#result-count").textContent = `${formatNumber(items.length)} 条`;
  if (!items.length) {
    wrap.innerHTML = '<div class="empty-state">当前窗口没有成功快照，无法生成数据表。</div>';
    return;
  }
  const changes = changeLookup();
  wrap.innerHTML = `<table class="data-table"><thead><tr><th>标题 / 名称</th><th>来源</th><th>排名</th><th>评分</th><th>变化</th><th>采集时间</th></tr></thead><tbody>${items.map((item, index) => {
    const rule = state.rules.find((entry) => entry.id === item.__ruleId);
    const title = displayValue(item, rule, "title", displayValue(item, rule, "name", "未命名数据"));
    const url = displayValue(item, rule, "url", "");
    const score = displayValue(item, rule, "score", displayValue(item, rule, "stars", displayValue(item, rule, "points", "--")));
    const key = rule ? String(readPath(item, rule.item_key) ?? url ?? index) : String(url ?? index);
    const change = changes.get(`${item.__ruleId}:${key}`) ?? "stable";
    return `<tr data-task-id="${escapeHTML(item.__taskId)}">
      <td class="item-title"><button type="button" data-raw-index="${index}" title="查看原始 JSON">${escapeHTML(title)}</button></td>
      <td><span class="item-link" title="${escapeHTML(url)}">${escapeHTML(rule?.source ?? item.__ruleId)}</span></td>
      <td class="item-rank">${escapeHTML(displayValue(item, rule, "rank"))}</td>
      <td class="item-score">${escapeHTML(score)}</td>
      <td><span class="change-badge is-${change}">${change === "added" ? "新增" : change === "changed" ? "变化" : "稳定"}</span></td>
      <td>${escapeHTML(formatDate(item.__finishedAt))}</td>
    </tr>`;
  }).join("")}</tbody></table>`;
}

function renderTrace() {
  const list = $("#trace-list");
  if (state.traceLoading) {
    list.innerHTML = '<div class="empty-state compact">正在读取运行事件</div>';
    return;
  }
  if (!state.selectedTaskId) {
    $("#trace-task").textContent = "未选择任务";
    list.innerHTML = '<div class="empty-state compact">点击一条运行记录查看生命周期</div>';
    return;
  }
  $("#trace-task").textContent = `#${state.selectedTaskId}`;
  if (!state.events.length) {
    list.innerHTML = '<div class="empty-state compact">该任务暂无持久化事件</div>';
    return;
  }
  list.innerHTML = state.events.map((event) => `<div class="trace-row">
    <span class="trace-marker ${event.level === "error" ? "is-error" : ""}" aria-hidden="true"></span>
    <div class="trace-content"><span class="trace-stage">${escapeHTML(stageLabels[event.stage] ?? event.stage)}</span><span class="trace-detail">${escapeHTML(event.message ?? event.code ?? "")} · ${escapeHTML(formatDate(event.created_at ?? event.createdAt))}</span></div>
  </div>`).join("");
}

async function loadTrace(taskId) {
  state.selectedTaskId = taskId ?? null;
  state.traceLoading = Boolean(taskId);
  renderTrace();
  if (!taskId) {
    state.events = [];
    state.traceLoading = false;
    renderTrace();
    return;
  }
  try {
    state.events = DEMO_MODE ? (demoEvents[taskId] ?? []) : await fetchTaskEvents(taskId);
  } catch (error) {
    state.events = [{ stage: "failed", level: "error", message: error instanceof Error ? error.message : "读取事件失败" }];
  } finally {
    state.traceLoading = false;
    renderTrace();
  }
}

function renderError(message) {
  for (const element of document.querySelectorAll("[data-summary]")) {
    element.textContent = "--";
  }
  $("#last-sync").textContent = "读取失败";
  $("#data-window").textContent = "分析窗口：暂无真实数据";
  const errorHTML = `<div class="error-state"><span>${escapeHTML(message)}</span><button class="retry-button" id="retry-button" type="button">重新读取</button></div>`;
  $("#trend-chart").innerHTML = errorHTML;
  $("#source-list").innerHTML = '<div class="empty-state compact">接口不可用</div>';
  $("#items-table-wrap").innerHTML = '<div class="empty-state">后端未提供可分析的真实快照。</div>';
  $("#health-list").innerHTML = '<div class="empty-state compact">规则状态不可用</div>';
  $("#result-count").textContent = "0 条";
  $("#retry-button")?.addEventListener("click", loadDashboard);
  renderTrace();
}

function bindEvents() {
  $("#refresh-button").addEventListener("click", loadDashboard);
  $("#rule-filter").addEventListener("change", async (event) => {
    state.filters.ruleId = event.target.value;
    state.overview = buildOverview(activeSnapshots(), state.rules, new Date());
    renderOverview();
    if (!DEMO_MODE) await loadDashboard();
  });
  $("#range-filter").addEventListener("change", async (event) => {
    state.filters.range = event.target.value;
    await loadDashboard();
  });
  $("#open-data-button").addEventListener("click", () => $("#items-panel").scrollIntoView({ behavior: "smooth", block: "start" }));
  $("#close-json-button").addEventListener("click", () => $("#json-dialog").close());
  $("#items-table-wrap").addEventListener("click", async (event) => {
    const rawButton = event.target.closest("[data-raw-index]");
    if (rawButton) {
      const item = state.overview?.latestItems[Number(rawButton.dataset.rawIndex)];
      if (item) {
        $("#json-content").textContent = JSON.stringify(item, null, 2);
        $("#json-dialog").showModal();
      }
      return;
    }
    const row = event.target.closest("tr[data-task-id]");
    if (row) await loadTrace(Number(row.dataset.taskId));
  });
  $("#health-list").addEventListener("click", (event) => {
    const button = event.target.closest("[data-rule-id]");
    if (!button) return;
    state.filters.ruleId = button.dataset.ruleId;
    $("#rule-filter").value = state.filters.ruleId;
    state.overview = buildOverview(activeSnapshots(), state.rules, new Date());
    renderOverview();
    $("#items-panel").scrollIntoView({ behavior: "smooth", block: "start" });
  });
  $("#source-list").addEventListener("click", (event) => {
    const button = event.target.closest("[data-source]");
    if (!button) return;
    const sourceRule = state.rules.find((rule) => rule.source === button.dataset.source);
    if (!sourceRule) return;
    state.filters.ruleId = sourceRule.id;
    $("#rule-filter").value = sourceRule.id;
    state.overview = buildOverview(activeSnapshots(), state.rules, new Date());
    renderOverview();
  });
}

bindEvents();
loadDashboard();
