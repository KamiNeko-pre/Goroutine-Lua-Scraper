# LuaSpider

> Go 驱动的 Lua 规则多源采集平台

LuaSpider 把**任务调度、并发执行、可靠投递和结果持久化**放在 Go 中，把不同网站的解析逻辑放在 Lua 规则中。新增采集来源时，只需要增加受信任的 Lua 规则和对应元数据，不需要改动 Worker、队列或存储主链路。

当前仓库包含一条可运行的 Redis Streams 持久化任务链路、两个示例来源（GitHub Trending 与 Hacker News）以及一个用于快照分析、规则健康检查和任务排障的 Web 工作台。

## 能做什么

- 通过 HTTP API 创建异步采集任务，并使用 `request_key` 保证客户端重试不会重复创建同一任务。
- 使用固定数量 Worker 执行任务，限制并发度，避免每个请求无限创建 goroutine。
- 用 Lua 描述 HTTP 请求后的 DOM 提取、字段转换和统一 JSON 结果，不同来源共享 Go 执行引擎。
- 用 MySQL 保存任务与结果，用 Redis Streams Consumer Group 在独立进程之间传递任务 ID。
- 用 MySQL Outbox 把“任务创建”和“待投递记录”放在同一个事务中，Publisher 可重试投递。
- 用状态 CAS、执行租约和随机运行令牌隔离并发抢占；Worker 中断后由 `XAUTOCLAIM` 接管空闲消息。
- 通过分析工作台查看最新快照、字段分布、数值趋势、快照差异、规则健康度和任务事件链路。

## 架构

```text
                          MySQL source of truth
                         ┌──────────────────────┐
POST /api/v1/tasks ─────▶│ CrawlTask + Outbox   │
                         └──────────┬───────────┘
                                    │ publisher polls unpublished rows
                                    ▼
                         ┌──────────────────────┐
                         │ Redis Stream         │
                         │ Consumer Group       │
                         └──────────┬───────────┘
                                    │ XREADGROUP / XAUTOCLAIM
                                    ▼
                         ┌──────────────────────┐
                         │ Worker processes     │
                         │ bounded concurrency  │
                         └──────────┬───────────┘
                                    │ task-local Lua VM + Context
                                    ▼
              ┌─────────────────────┴─────────────────────┐
              │ HTTP fetch -> Lua parse -> JSON validation │
              └─────────────────────┬─────────────────────┘
                                    ▼
                         ┌──────────────────────┐
                         │ MySQL result +       │
                         │ terminal task state  │
                         └──────────┬───────────┘
                                    │ commit succeeds first
                                    ▼
                                  XACK
```

### 关键设计

**规则与执行引擎解耦**

Go 只提供受限的 `http_get`、DOM 选择器和 URL 处理能力。Lua 规则返回统一的 `items` / `meta` 结构，Worker 不需要理解每个网站的字段。每个任务使用独立 Lua VM，避免脚本全局状态在任务之间串扰；Context 超时会传入 Lua VM，网络请求和脚本执行共享任务截止时间。

**有界并发与背压**

Worker 数量由配置决定。HTTP 接口只负责校验和创建任务，慢网络与 Lua 执行不会阻塞请求线程；队列或下游不可用时由状态和错误信息暴露问题，而不是无界堆积任务。

**Outbox + Redis Streams**

任务记录和 Outbox 记录在同一个 MySQL 事务中提交。Publisher 扫描 `published_at IS NULL` 的记录，将任务 ID 写入 Redis Stream 后再标记 Outbox；发布失败时记录仍可被下一轮重试。Redis 只承载任务引用，MySQL 保留任务完整状态。

**至少一次投递与故障恢复**

Worker 在 `queued -> running` 时使用带旧状态条件的 CAS，并写入租约和运行令牌。处理完成后，结果和终态在同一 MySQL 事务中提交，成功后才 `XACK`。因此系统实现的是“至少一次投递 + 幂等终态提交”，不是 Exactly Once；外部网站请求在故障恢复时可能重复发生，但过期 Worker 不能提交新结果。

**从采集到分析**

工作台通过只读 API 获取已结束任务的快照，按规则元数据解释字段，支持排行、趋势、快照差异、字段质量和任务事件追踪。GitHub 与 Hacker News 是示例来源，同一套结果契约可以承载其他 Lua 规则。

## 技术栈

| 层次 | 技术 |
| --- | --- |
| HTTP 服务 | Go、Gin |
| 并发与执行 | goroutine、channel、Worker Pool、`context.Context` |
| 规则引擎 | gopher-lua、goquery、fsnotify |
| 任务投递 | Redis Streams、Consumer Group、`XAUTOCLAIM` |
| 持久化 | MySQL、GORM、事务与条件更新 |
| 日志与诊断 | zap、任务事件、pprof |
| 展示 | 原生 HTML/CSS/JavaScript、ECharts、Lucide |
| 本地运行 | Docker Compose |

## 项目结构

```text
cmd/
  server/       HTTP API、静态工作台和本地入口
  publisher/    Outbox -> Redis Stream 发布进程
  worker/       Redis Consumer Group 消费与故障接管进程
configs/
  config.yaml   脱敏后的默认配置模板
  rules.yaml    规则与展示元数据
internal/
  engine/       Lua VM、HTTP/DOM 能力和结果契约
  handler/      HTTP 请求、任务、分析和事件接口
  queue/        Redis 客户端、Stream 和发布操作
  repository/   MySQL 模型、事务、CAS、Outbox 和查询
  service/      Outbox 发布服务
  task/         任务状态与领域模型
  worker/       Consumer、Reclaimer 和消息处理器
  router/       API 与静态页面路由
scripts/
  *.lua         GitHub、Hacker News 等示例规则
web/
  index.html    分析工作台
  assets/       页面逻辑、样式和本地依赖
```

## 快速启动

需要 Docker Desktop。公开 Compose 使用本地 MySQL 和 Redis，密码只从未提交的 `.env` 读取。

```bash
cp .env.example .env
docker compose up --build -d
```

Windows PowerShell 可使用：

```powershell
Copy-Item .env.example .env
docker compose up --build -d
```

打开 `http://localhost:8080/` 查看工作台。提交一条 GitHub Trending 任务：

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"request_key":"demo-github-001","target":"github_trending_go","url":"https://github.com/trending/go?since=daily"}'
```

提交响应中的任务 ID 可用于轮询：

```bash
curl http://localhost:8080/api/v1/tasks/1
curl http://localhost:8080/api/v1/tasks/1/result
```

Hacker News 示例规则对应 `target: hackernews_top`。停止服务但保留本地数据：

```bash
docker compose down
```

## 新增规则

1. 在 `scripts/` 增加一个受信任的 Lua 文件。
2. 让脚本调用 `http_get(TARGET_URL)`，并返回 `true, {items = {...}, meta = {...}}`；失败时返回 `false, "error message"`。
3. 在 `configs/rules.yaml` 增加规则 ID、脚本路径、目标 URL、最小条数、唯一键和展示字段映射。
4. 使用该规则 ID 作为创建任务请求的 `target`。

规则脚本运行在受限库环境中，当前规则目录是本地受信任配置，不提供未审查脚本的在线上传、权限系统或多租户隔离。

## 验证

```bash
go test ./...
go vet ./...
go build ./...
node --test web/assets/analysis.test.mjs web/assets/workbench.test.mjs
```

`docker-compose.yml` 是本地单节点演示环境：Redis 和 MySQL 的进程可以拆开部署，多个 Worker 可以加入同一个 Consumer Group；它不等同于数据库或 Redis 的基础设施高可用集群。任务投递语义、状态转换和恢复边界以代码为准，README 不把本地测试结果包装成生产指标。
