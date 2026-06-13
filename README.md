# Goroutine-Lua-Scraper

Go 宿主 + Lua 脚本引擎的爬虫框架。Go 提供网络、解析、存储、调度基础设施，Lua 脚本作为业务插件定义抓取规则，支持热重载。

## 工作流

```
POST /api/v1/task → Channel(1000) → 15 Workers → Lua VM → HTTP GET → goquery 解析 → SQLite
                                                         ↑
                                                    fsnotify 热重载
```

## 快速开始

```bash
# 启动
go run cmd/server/main.go

# 提交抓取任务
curl -X POST http://localhost:8080/api/v1/task \
  -H "Content-Type: application/json" \
  -d '{"target":"vue","url":"https://github.com/vuejs/vue"}'

# 查询结果
curl http://localhost:8080/api/v1/task?repo=vuejs/vue
```

## 技术栈

- **Gin** — HTTP API
- **GORM + SQLite** — 数据持久化
- **gopher-lua** — Lua 虚拟机嵌入
- **goquery** — HTML CSS 选择器解析
- **fsnotify** — 脚本文件监听与热重载
- **robfig/cron** — 定时自动更新
- **Viper** — 配置管理（支持热更新）
- **Zap** — 结构化日志

## 项目结构

```
cmd/server/main.go        # 入口：启动配置、日志、数据库、worker 池、HTTP 服务
internal/
  config/config.go         # Viper 配置读取与热更新
  logger/logger.go         # Zap 全局日志
  repository/db.go         # GORM 初始化 + GithubRepo 模型
  engine/lua_engine.go     # Lua 引擎：脚本加载、热重载、HTTP/HTML 函数注入
  scheduler/cron.go        # Cron 定时调度
  handler/task.go          # API 处理器：创建任务、查询结果
  router/router.go         # Gin 路由
scripts/test.lua           # Lua 抓取脚本（GitHub 仓库 Star/描述/裂变链接）
configs/config.yaml        # 配置文件
```

## Lua 脚本能做什么

Go 侧向 Lua 虚拟机注入了两个函数：

- `http_get(url)` — 发起 HTTP GET，返回 HTML body
- `html_find(html, selector)` — 用 CSS 选择器从 HTML 中提取文本

Lua 脚本拿到目标 URL 后，调用这两个函数完成抓取和解析，最后返回 `(true, table)` 或 `(false, error_message)`。

```lua
local body, err = http_get(TARGET_URL)
if err then return false, "网络请求失败: " .. err end

local star = html_find(body, "#repo-stars-counter-star")
-- ...
return true, {url = TARGET_URL, stars = star, description = desc}
```

修改脚本后保存，fsnotify 检测到变更自动热重载，无需重启服务。

## 当前状态

这是 MVP 版本，技术验证完成。后续计划：

- [ ] Lua 执行超时保护（沙箱）
- [ ] 优雅退出
- [ ] 多脚本支持 + 独立调度
- [ ] MySQL + Redis 布隆过滤器 + Redis 任务队列
- [ ] 多数据源（Steam 史低、掘金热榜）
- [ ] pprof 性能剖析
- [ ] 测试与 Docker 化

## License

MIT
