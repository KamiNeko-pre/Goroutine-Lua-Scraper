const SUCCESS_STATUS = "succeeded";

function asObject(value) {
  return value !== null && typeof value === "object" ? value : null;
}

function parseJSON(value) {
  if (typeof value !== "string") {
    return value;
  }
  return JSON.parse(value);
}

function normalizeItem(value) {
  return asObject(value) ?? { value };
}

function pathParts(path) {
  return String(path ?? "")
    .replace(/^\$\.?/, "")
    .split(".")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function readPath(value, path) {
  if (!path) {
    return value;
  }
  let current = value;
  for (const part of pathParts(path)) {
    if (current === null || current === undefined) {
      return undefined;
    }
    current = current[part];
  }
  return current;
}

export function extractItems(payload) {
  try {
    const parsed = parseJSON(payload);
    if (Array.isArray(parsed)) {
      return { items: parsed.map(normalizeItem), meta: {}, legacy: false, error: null };
    }

    if (!asObject(parsed)) {
      return {
        items: [],
        meta: {},
        legacy: false,
        error: { code: "invalid_payload", message: "快照不是 JSON 对象或数组" }
      };
    }

    if (Array.isArray(parsed.items)) {
      return {
        items: parsed.items.map(normalizeItem),
        meta: asObject(parsed.meta) ?? {},
        legacy: false,
        error: null
      };
    }

    if (parsed.url || parsed.name || parsed.full_name || parsed.title) {
      return { items: [parsed], meta: {}, legacy: true, error: null };
    }

    return {
      items: [],
      meta: {},
      legacy: false,
      error: { code: "missing_items", message: "快照中没有 items 数组" }
    };
  } catch (error) {
    return {
      items: [],
      meta: {},
      legacy: false,
      error: { code: "invalid_json", message: error instanceof Error ? error.message : "快照 JSON 无法解析" }
    };
  }
}

export function normalizeSnapshot(record) {
  const payload = record?.payload ?? record?.data ?? null;
  const extracted = payload === null || payload === ""
    ? { items: [], meta: {}, legacy: false, error: null }
    : extractItems(payload);

  return {
    taskId: Number(record?.task_id ?? record?.taskId ?? 0),
    ruleId: String(record?.rule_id ?? record?.target ?? record?.ruleId ?? "unknown"),
    status: String(record?.status ?? "unknown").toLowerCase(),
    finishedAt: record?.finished_at ?? record?.finishedAt ?? null,
    items: extracted.items,
    meta: extracted.meta,
    legacy: extracted.legacy,
    parseError: extracted.error,
    failureCode: record?.failure_code ?? record?.failureCode ?? "",
    lastError: record?.last_error ?? record?.lastError ?? "",
    rawPayload: payload
  };
}

function itemKey(item, rule) {
  const value = readPath(item, rule?.item_key ?? rule?.itemKey);
  if (value === undefined || value === null || value === "") {
    return null;
  }
  return String(value);
}

function comparableItem(item) {
  return stableJSON(item);
}

export function stableJSON(value) {
  if (Array.isArray(value)) return `[${value.map(stableJSON).join(",")}]`;
  if (value && typeof value === "object") return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${stableJSON(value[key])}`).join(",")}}`;
  return JSON.stringify(value);
}

export function compareSnapshots(previous, current, rule = {}) {
  const previousItems = previous?.items ?? [];
  const currentItems = current?.items ?? [];
  const previousMap = new Map();
  const currentMap = new Map();
  const unkeyed = [];

  for (const item of previousItems) {
    const key = itemKey(item, rule);
    if (key !== null) {
      previousMap.set(key, item);
    }
  }

  for (const item of currentItems) {
    const key = itemKey(item, rule);
    if (key === null) {
      unkeyed.push(item);
      continue;
    }
    currentMap.set(key, item);
  }

  const added = [];
  const changed = [];
  const removed = [];

  for (const [key, item] of currentMap) {
    if (!previousMap.has(key)) {
      added.push({ key, item });
      continue;
    }
    if (comparableItem(previousMap.get(key)) !== comparableItem(item)) {
      changed.push({ key, before: previousMap.get(key), item });
    }
  }

  for (const [key, item] of previousMap) {
    if (!currentMap.has(key)) {
      removed.push({ key, item });
    }
  }

  return {
    added,
    removed,
    changed,
    unkeyed,
    countDelta: currentItems.length - previousItems.length
  };
}

function parseDuration(value) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value * 60 * 1000;
  }
  const match = String(value ?? "24h").trim().match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d)$/i);
  if (!match) {
    return 24 * 60 * 60 * 1000;
  }
  const amount = Number(match[1]);
  const unit = match[2].toLowerCase();
  const multipliers = { ms: 1, s: 1000, m: 60 * 1000, h: 60 * 60 * 1000, d: 24 * 60 * 60 * 1000 };
  return amount * multipliers[unit];
}

function timestamp(value) {
  const time = value ? new Date(value).getTime() : NaN;
  return Number.isFinite(time) ? time : null;
}

function sortNewestFirst(snapshots) {
  return [...snapshots].sort((left, right) => (timestamp(right.finishedAt) ?? 0) - (timestamp(left.finishedAt) ?? 0));
}

export function evaluateRuleHealth(snapshots, rule = {}, now = new Date()) {
  const ruleSnapshots = sortNewestFirst((snapshots ?? []).filter((snapshot) => snapshot.ruleId === rule.id));
  if (ruleSnapshots.length === 0) {
    return {
      ruleId: rule.id,
      status: "unknown",
      reason: "暂无历史运行记录",
      latestTaskId: null,
      latestSuccessAt: null,
      itemCount: 0,
      consecutiveFailures: 0
    };
  }

  let consecutiveFailures = 0;
  for (const snapshot of ruleSnapshots) {
    if (snapshot.status === SUCCESS_STATUS) {
      break;
    }
    consecutiveFailures += 1;
  }

  const latest = ruleSnapshots[0];
  const latestSuccess = ruleSnapshots.find((snapshot) => snapshot.status === SUCCESS_STATUS);
  const latestSuccessTime = timestamp(latestSuccess?.finishedAt);
  const latestTime = timestamp(latest.finishedAt);
  const age = latestTime === null ? Infinity : Math.max(0, now.getTime() - latestTime);
  const interval = parseDuration(rule.schedule);
  const itemCount = latestSuccess?.items.length ?? 0;
  let status = "healthy";
  let reason = "执行成功，数据新鲜且条数正常";

  if (latest.status !== SUCCESS_STATUS) {
    status = "failing";
    reason = latest.lastError || latest.failureCode || "最近一次运行失败";
  } else if (latest.parseError) {
    status = "failing";
    reason = `结果解析失败：${latest.parseError.message}`;
  } else if (age > interval * 2) {
    status = "failing";
    reason = "最近一次成功快照已明显过期";
  } else if (itemCount < Number(rule.min_items ?? 0)) {
    status = "degraded";
    reason = `有效数据 ${itemCount} 条，低于最低阈值 ${rule.min_items} 条`;
  } else if (age > interval * 0.75) {
    status = "degraded";
    reason = "快照接近预期更新周期";
  }

  return {
    ruleId: rule.id,
    status,
    reason,
    latestTaskId: latest.taskId,
    latestSuccessAt: latestSuccess?.finishedAt ?? null,
    itemCount,
    consecutiveFailures,
    latestFailureAt: latest.status === SUCCESS_STATUS ? null : latest.finishedAt,
    latestSnapshot: latest
  };
}

function dateBucket(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "未知";
  }
  return date.toISOString().slice(5, 10);
}

export function buildTrendStats(trend = []) {
  const ordered = [...trend].sort((left, right) => left.bucket.localeCompare(right.bucket));
  const latest = ordered.at(-1) ?? null;
  const previous = ordered.at(-2) ?? null;
  const latestCount = Number(latest?.itemCount ?? 0);
  const previousCount = Number(previous?.itemCount ?? 0);
  const delta = latest && previous ? latestCount - previousCount : 0;
  const deltaRate = previousCount > 0 ? (delta / previousCount) * 100 : null;
  const changeCount = Number(latest?.changeCount ?? 0);

  return {
    latestBucket: latest?.bucket ?? null,
    latestCount,
    previousCount,
    delta,
    deltaRate,
    changeCount,
    changeRate: latestCount > 0 ? (changeCount / latestCount) * 100 : 0
  };
}

export function buildOverview(snapshots, rules, now = new Date()) {
  const normalized = (snapshots ?? []).map((snapshot) => snapshot.taskId === undefined ? normalizeSnapshot(snapshot) : snapshot);
  const successful = normalized.filter((snapshot) => snapshot.status === SUCCESS_STATUS);
  const health = (rules ?? []).map((rule) => evaluateRuleHealth(normalized, rule, now));
  const ruleById = new Map((rules ?? []).map((rule) => [rule.id, rule]));
  const sortedSuccessful = sortNewestFirst(successful);
  const trendMap = new Map();
  const sourceMap = new Map();
  const changes = [];

  for (const snapshot of sortedSuccessful) {
    const bucket = dateBucket(snapshot.finishedAt);
    const trend = trendMap.get(bucket) ?? { bucket, itemCount: 0, changeCount: 0, runs: 0 };
    trend.itemCount += snapshot.items.length;
    trend.runs += 1;
    trendMap.set(bucket, trend);

    const rule = ruleById.get(snapshot.ruleId) ?? { id: snapshot.ruleId, source: snapshot.ruleId, item_key: "url" };
    const source = rule.source ?? snapshot.ruleId;
    const sourceSummary = sourceMap.get(source) ?? { source, itemCount: 0, runs: 0, ruleIds: new Set() };
    sourceSummary.itemCount += snapshot.items.length;
    sourceSummary.runs += 1;
    sourceSummary.ruleIds.add(snapshot.ruleId);
    sourceMap.set(source, sourceSummary);
  }

  const byRule = new Map();
  for (const snapshot of sortedSuccessful) {
    const history = byRule.get(snapshot.ruleId) ?? [];
    history.push(snapshot);
    byRule.set(snapshot.ruleId, history);
  }

  for (const [ruleId, history] of byRule) {
    const rule = ruleById.get(ruleId) ?? { id: ruleId, item_key: "url" };
    for (let index = 0; index < history.length - 1; index += 1) {
      const comparison = compareSnapshots(history[index + 1], history[index], rule);
      trendMap.get(dateBucket(history[index].finishedAt)).changeCount += comparison.added.length + comparison.removed.length + comparison.changed.length;
      changes.push({ ruleId, taskId: history[index].taskId, finishedAt: history[index].finishedAt, ...comparison });
    }
  }

  const latestByRule = new Map();
  for (const snapshot of sortedSuccessful) {
    if (!latestByRule.has(snapshot.ruleId)) {
      latestByRule.set(snapshot.ruleId, snapshot);
    }
  }
  const latestItems = [...latestByRule.values()].flatMap((snapshot) => snapshot.items.map((item) => ({
    ...item,
    __taskId: snapshot.taskId,
    __ruleId: snapshot.ruleId,
    __finishedAt: snapshot.finishedAt
  })));

  const sourceList = [...sourceMap.values()]
    .map((source) => ({ ...source, ruleIds: [...source.ruleIds] }))
    .sort((left, right) => right.itemCount - left.itemCount);

  return {
    snapshots: normalized,
    successfulSnapshots: successful,
    summary: {
      itemCount: successful.reduce((total, snapshot) => total + snapshot.items.length, 0),
      runCount: normalized.length,
      successfulRuns: successful.length,
      failedRuns: normalized.length - successful.length,
      changeCount: changes.reduce((total, change) => total + change.added.length + change.removed.length + change.changed.length, 0),
      healthyRules: health.filter((entry) => entry.status === "healthy").length
    },
    trend: [...trendMap.values()],
    sources: sourceList,
    health,
    changes: changes.sort((left, right) => (timestamp(right.finishedAt) ?? 0) - (timestamp(left.finishedAt) ?? 0)),
    latestItems
  };
}

export function buildInsightBrief(overview, rules = []) {
  const summary = overview?.summary ?? {};
  const changes = overview?.changes ?? [];
  const health = overview?.health ?? [];
  const sources = overview?.sources ?? [];
  const ruleById = new Map((rules ?? []).map((rule) => [rule.id, rule]));
  const changeByRule = new Map();

  for (const change of changes) {
    const total = change.added.length + change.removed.length + change.changed.length;
    const current = changeByRule.get(change.ruleId) ?? 0;
    changeByRule.set(change.ruleId, current + total);
  }

  const leadingChange = [...changeByRule.entries()]
    .sort((left, right) => right[1] - left[1])[0];
  const leadingRule = leadingChange ? ruleById.get(leadingChange[0]) : null;
  const attention = health
    .filter((entry) => entry.status === "failing" || entry.status === "degraded")
    .sort((left, right) => {
      if (left.status === right.status) return 0;
      return left.status === "failing" ? -1 : 1;
    })[0] ?? null;
  const attentionRule = attention ? ruleById.get(attention.ruleId) : null;
  const trend = buildTrendStats(overview?.trend ?? []);
  const topSource = sources[0] ?? null;
  const sourceCount = sources.length;
  const itemCount = Number(summary.itemCount ?? 0);
  const failedRuns = Number(summary.failedRuns ?? 0);

  let headline = "当前窗口还没有足够的快照形成变化结论。";
  if (attention && attention.status === "failing") {
    headline = `${attentionRule?.name ?? attention.ruleId} 最近一次运行失败，建议先检查规则健康。`;
  } else if (leadingChange) {
    headline = `${leadingRule?.name ?? leadingChange[0]} 出现 ${leadingChange[1]} 项变化，是当前窗口最值得查看的信号。`;
  } else if (failedRuns > 0) {
    headline = `当前窗口有 ${failedRuns} 次失败运行，虽然最近结果已恢复，也建议检查失败记录。`;
  } else if (topSource) {
    headline = `${topSource.source} 贡献了当前窗口最多的有效数据，采集结果暂未出现明显变化。`;
  }

  const attentionDetail = attention
    ? `${attentionRule?.name ?? attention.ruleId}：${attention.reason}`
    : failedRuns > 0 ? "存在历史失败运行，建议查看运行追踪" : "当前窗口没有失败运行";

  return {
    headline,
    scope: `${Number(summary.runCount ?? 0)} 次运行 · ${sourceCount} 个来源`,
    movement: {
      value: leadingRule?.name ?? "暂无明显变化",
      detail: leadingChange ? `${leadingChange[1]} 项新增、下线或字段变化` : "需要至少两次成功快照"
    },
    coverage: {
      value: `${sourceCount} 个来源`,
      detail: `${itemCount} 条有效数据 · ${Number(summary.successfulRuns ?? 0)} 次成功运行`
    },
    attention: {
      value: failedRuns > 0 ? `${failedRuns} 次失败` : attention ? "规则需关注" : "运行稳定",
      detail: attentionDetail
    },
    trend
  };
}
