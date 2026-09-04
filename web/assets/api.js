const API_ROOT = "/api/v1";

function createAPIError(response, body) {
  const message = body?.message || body?.error || `请求失败（${response.status}）`;
  const error = new Error(message);
  error.status = response.status;
  return error;
}

async function request(path) {
  const response = await fetch(`${API_ROOT}${path}`, {
    headers: { Accept: "application/json" }
  });
  let body = null;
  try {
    body = await response.json();
  } catch {
    body = null;
  }
  if (!response.ok) {
    throw createAPIError(response, body);
  }
  return body?.data ?? body;
}

function queryString(values) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== "" && value !== "all") {
      params.set(key, value);
    }
  }
  return params.toString();
}

export async function fetchSnapshots({ ruleId, from, to, limit = 200 } = {}) {
  const query = queryString({ rule_id: ruleId, from, to, limit });
  const data = await request(`/analysis/snapshots${query ? `?${query}` : ""}`);
  if (Array.isArray(data)) {
    return data;
  }
  if (Array.isArray(data?.items)) {
    return data.items;
  }
  throw new Error("快照接口返回格式不正确");
}

export async function fetchRules() {
  const data = await request("/analysis/rules");
  if (Array.isArray(data)) {
    return data;
  }
  if (Array.isArray(data?.items)) {
    return data.items;
  }
  throw new Error("规则接口返回格式不正确");
}

export async function fetchTaskEvents(taskId) {
  if (!taskId) {
    return [];
  }
  const data = await request(`/tasks/${encodeURIComponent(taskId)}/events`);
  if (Array.isArray(data)) {
    return data;
  }
  if (Array.isArray(data?.events)) {
    return data.events;
  }
  throw new Error("任务事件接口返回格式不正确");
}
