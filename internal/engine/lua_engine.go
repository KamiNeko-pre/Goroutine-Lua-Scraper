package engine

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/fsnotify/fsnotify"
	lua "github.com/yuin/gopher-lua"
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
	"io"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// routeCache 以 host 为键缓存最近一次可用的网络路径：true 表示代理，false 表示直连。
// sync.Map 适合多个 worker 高频读写且键集合不固定的场景。
var routeCache sync.Map

const maxHTTPResponseBodyBytes int64 = 4 * 1024 * 1024

// TaskQuene 是抓取任务的全局缓冲队列。名称沿用现有代码，实际含义是 Queue。
var TaskQuene chan string

var (
	// ScriptCache 保存最近一次成功读取的 Lua 规则。
	// 读取脚本和热更新会并发发生，必须用读写锁保护。
	ScriptCache string
	ScriptMutex sync.RWMutex
)

func InitLuaEngine(scriptPath string) {
	// 先完成一次同步加载，确保第一个任务到来时已有可执行规则。
	reloadScript(scriptPath)

	watcher, err := fsnotify.NewWatcher()

	if err != nil {
		logger.Log.Error("创建文件监听失败", zap.Error(err))
		return
	}

	err = watcher.Add(scriptPath)

	if err != nil {
		logger.Log.Fatal("监听脚本目录失败", zap.Error(err))
	}

	// 文件监听放在独立 goroutine 中，避免阻塞 HTTP 服务和后台 worker。
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					logger.Log.Info("检测到规则脚本发生修改，触发热重载", zap.String("文件", event.Name))
					reloadScript(scriptPath)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Log.Error("监控保安发生异常", zap.Error(err))

			}

		}
	}()
}

func reloadScript(path string) {
	// 只有完整读到新脚本后才替换缓存；读取失败时继续使用旧规则，
	// 防止编辑器写文件的中间态导致线上规则被清空。
	ScriptMutex.Lock()
	defer ScriptMutex.Unlock()

	data, err := os.ReadFile(path)

	if err != nil {
		logger.Log.Error("重新读取脚本失败, 维持旧规则", zap.Error(err))
		return
	}
	ScriptCache = string(data)
	logger.Log.Info("规则热更新成功")
}
func doDirectRequest(ctx context.Context, targetURL string, timeout time.Duration) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create direct request: %w", err)
	}
	client := &http.Client{
		Timeout:       timeout,
		CheckRedirect: validateRedirect,
	}
	return client.Do(request)
}

func validateTargetURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse target URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", parsedURL.Scheme)
	}

	host := strings.ToLower(parsedURL.Hostname())
	if host == "" {
		return nil, fmt.Errorf("target URL must include a host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("localhost targets are not allowed")
	}

	ip, err := netip.ParseAddr(host)
	if err == nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return nil, fmt.Errorf("non-public IP target %q is not allowed", host)
	}

	return parsedURL, nil
}

func validateRedirect(request *http.Request, _ []*http.Request) error {
	if _, err := validateTargetURL(request.URL.String()); err != nil {
		return fmt.Errorf("reject redirect target: %w", err)
	}
	return nil
}

func readResponseBody(body io.Reader, contentLength int64) ([]byte, error) {
	if contentLength > maxHTTPResponseBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d byte limit", maxHTTPResponseBodyBytes)
	}

	limitedBody := io.LimitReader(body, maxHTTPResponseBodyBytes+1)
	data, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > maxHTTPResponseBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d byte limit", maxHTTPResponseBodyBytes)
	}
	return data, nil
}

func HttpGet(L *lua.LState) int {
	// 这是暴露给 Lua 的 Go 函数。返回值是压入 Lua 栈的数量：
	// 成功时返回 body；失败时返回 nil 和错误信息，供 Lua 脚本自行处理。
	targetUrl := L.CheckString(1)

	ctx := L.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := config.Get()

	proxyStr := cfg.App.Proxy

	parsedURL, err := validateTargetURL(targetUrl)

	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("URL validation failed: " + err.Error()))
		return 2
	}
	host := strings.ToLower(parsedURL.Hostname())

	useProxy := false

	// 已有路径记录时直接复用，减少每次请求先直连再回退代理的额外延迟。
	if val, ok := routeCache.Load(host); ok {
		useProxy = val.(bool)
	}
	var resp *http.Response
	var reqErr error
	proxyTimeout := time.Duration(cfg.Engine.HTTPTimeoutProxy) * time.Second
	if useProxy && proxyStr != "" {
		resp, reqErr = doProxyRequest(ctx, targetUrl, proxyStr, proxyTimeout)
	} else {
		resp, reqErr = doDirectRequest(ctx, targetUrl, time.Duration(cfg.Engine.HTTPTimeoutDirect)*time.Second)
		// 首次访问优先直连；直连失败且配置了代理时才降级到代理，
		// 成功后的选择会写入缓存，供同一 host 后续请求使用。
		if reqErr != nil && proxyStr != "" {
			logger.Log.Warn("直连失败, 启动代理并记录至路由表", zap.String("host", host))
			resp, reqErr = doProxyRequest(ctx, targetUrl, proxyStr, proxyTimeout)
			if reqErr == nil {
				routeCache.Store(host, true)
			}
		} else if reqErr == nil {
			routeCache.Store(host, false)
		}

	}
	if reqErr != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(reqErr.Error()))
		return 2
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		L.Push(lua.LNil)
		L.Push(lua.LString(fmt.Sprintf("HTTP 状态异常: %d", resp.StatusCode)))
		return 2
	}

	body, err := readResponseBody(resp.Body, resp.ContentLength)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(string(body)))
	return 1
}

func doProxyRequest(ctx context.Context, targetURL string, proxyStr string, timeout time.Duration) (*http.Response, error) {
	// 为代理请求单独设置超时，避免代理线路的波动占用直连请求的时间预算。
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}
	if proxyURL.Scheme == "" || proxyURL.Host == "" {
		return nil, fmt.Errorf("proxy URL must include scheme and host")
	}
	proxyClient := &http.Client{
		Timeout:       timeout,
		CheckRedirect: validateRedirect,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create proxy request: %w", err)
	}
	return proxyClient.Do(request)

}

func RunLuaScript(scriptPath string, targetUrl string) error {
	// 每次任务创建独立 Lua 虚拟机，避免脚本全局变量在不同任务之间相互污染。
	// 规则文本从内存缓存读取，不在 worker 热路径中反复访问磁盘。
	ScriptMutex.RLock()
	scriptText := ScriptCache
	ScriptMutex.RUnlock()
	if scriptText == "" {
		return fmt.Errorf("内存中无有效规则")
	}
	timeout := time.Duration(config.Get().Engine.LuaTimeout) * time.Second
	result, err := executeLuaScript(scriptText, targetUrl, timeout)
	if err != nil {
		return err
	}

	repoName := strings.Replace(result.URL, "https://github.com/", "", 1)
	repoRecord := repository.GithubRepo{
		Name:        repoName,
		Stars:       result.Stars,
		Description: result.Description,
		Source:      "github",
	}
	// 以仓库名查询后更新或创建，保证同一仓库重复抓取时覆盖最新数据而不是累积重复记录。
	dbResult := repository.DB.Where(repository.GithubRepo{Name: repoName}).Assign(repoRecord).FirstOrCreate(&repoRecord)
	if dbResult.Error != nil {
		return fmt.Errorf("数据入库失败: %v", dbResult.Error)
	}
	if len(result.FissionURLs) > 0 {
		var newRepos []repository.GithubRepo
		for _, furl := range result.FissionURLs {
			fName := strings.Replace(furl, "https://github.com/", "", 1)
			newRepos = append(newRepos, repository.GithubRepo{Name: fName})
		}

		// 批量插入新发现的链接，并由数据库唯一索引过滤重复仓库。
		// OnConflict DoNothing 保证已存在的任务不会因为重复发现而报错。
		dbErr := repository.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&newRepos).Error

		if dbErr != nil {
			logger.Log.Warn("裂变链接入库遭遇部分异常", zap.Error(dbErr))
		} else {
			logger.Log.Info("📡 裂变雷达生效", zap.Int("发现并抛入蓄水池的新链接数", len(newRepos)))
		}
	}
	logger.Log.Info("数据已保存至数据库!", zap.String("仓库", repoName), zap.Int("Stars", result.Stars))
	return nil
}

type luaScriptResult struct {
	URL         string
	Stars       int
	Description string
	FissionURLs []string
}

func openLuaSandboxLibraries(L *lua.LState) error {
	libraries := []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	}

	for _, library := range libraries {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(library.open),
			NRet:    0,
			Protect: true,
		}, lua.LString(library.name)); err != nil {
			return fmt.Errorf("open Lua library %q: %w", library.name, err)
		}
	}

	// OpenBase provides common helpers, but its file and dynamic-loading entry
	// points do not belong in a user-provided rule sandbox.
	for _, globalName := range []string{
		"dofile",
		"load",
		"loadfile",
		"loadstring",
		"require",
		"module",
		"collectgarbage",
		"_printregs",
		"newproxy",
	} {
		L.SetGlobal(globalName, lua.LNil)
	}

	return nil
}

// executeLuaScript owns one Lua VM for one task. It accepts timeout explicitly
// so the production wrapper can use configuration while tests stay fast.
func executeLuaScript(scriptText, targetURL string, timeout time.Duration) (luaScriptResult, error) {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()

	if err := openLuaSandboxLibraries(L); err != nil {
		return luaScriptResult{}, fmt.Errorf("initialize Lua sandbox: %w", err)
	}

	L.SetGlobal("http_get", L.NewFunction(HttpGet))
	L.SetGlobal("html_find", L.NewFunction(HtmlFind))
	L.SetGlobal("html_find_all", L.NewFunction(HtmlFindAll))
	L.SetGlobal("html_attr_all", L.NewFunction(HtmlAttrAll))
	L.SetGlobal("url_resolve", L.NewFunction(URLResolve))
	L.SetGlobal("TARGET_URL", lua.LString(targetURL))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	L.SetContext(ctx)

	if err := L.DoString(scriptText); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return luaScriptResult{}, fmt.Errorf("Lua script execution timed out after %v: %w", timeout, ctx.Err())
		}
		return luaScriptResult{}, fmt.Errorf("Lua sandbox execution failed: %w", err)
	}

	return decodeLuaResult(L)
}

func decodeLuaResult(L *lua.LState) (luaScriptResult, error) {
	if L.GetTop() < 2 {
		return luaScriptResult{}, fmt.Errorf("Lua script must return [boolean, data or error]")
	}
	if !lua.LVAsBool(L.Get(1)) {
		return luaScriptResult{}, fmt.Errorf("%s", L.Get(2).String())
	}

	luaData, ok := L.Get(2).(*lua.LTable)
	if !ok {
		return luaScriptResult{}, fmt.Errorf("Lua script success data must be a table")
	}

	stars, err := parseStarCount(luaData.RawGetString("stars").String())
	if err != nil {
		return luaScriptResult{}, fmt.Errorf("parse repository stars: %w", err)
	}
	result := luaScriptResult{
		URL:         luaData.RawGetString("url").String(),
		Stars:       stars,
		Description: luaData.RawGetString("description").String(),
	}
	fissionValue := luaData.RawGetString("fission_urls")
	if fissionTable, ok := fissionValue.(*lua.LTable); ok {
		fissionTable.ForEach(func(_ lua.LValue, value lua.LValue) {
			if value.Type() == lua.LTString {
				result.FissionURLs = append(result.FissionURLs, value.String())
			}
		})
	}
	return result, nil
}

func parseStarCount(raw string) (int, error) {
	starsText := strings.ToLower(strings.TrimSpace(raw))
	starsText = strings.ReplaceAll(starsText, ",", "")
	multiplier := 1.0
	if strings.HasSuffix(starsText, "k") {
		starsText = strings.TrimSuffix(starsText, "k")
		multiplier = 1000
	}

	stars, err := strconv.ParseFloat(starsText, 64)
	if err != nil {
		return 0, err
	}
	return int(math.Round(stars * multiplier)), nil
}

func HtmlFind(L *lua.LState) int {
	// html_find(html, selector) 负责将 HTML 解析和 CSS 选择器能力提供给 Lua。
	// 与 http_get 一样，失败时返回 nil 和错误信息，成功时返回第一个匹配元素的文本。
	htmlstr := L.CheckString(1)
	selector := L.CheckString(2)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlstr))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("DOM树构建失败: " + err.Error()))
		return 2
	}
	selection := doc.Find(selector).First()

	if selection.Length() == 0 {
		L.Push(lua.LNil)
		L.Push(lua.LString("在网页中未找到匹配的元素: " + selector))
		return 2
	}
	cleanText := strings.TrimSpace(selection.Text())
	L.Push(lua.LString(cleanText))
	return 1

}

func HtmlFindAll(L *lua.LState) int {
	return htmlSelectAll(L, "")
}

func HtmlAttrAll(L *lua.LState) int {
	return htmlSelectAll(L, L.CheckString(3))
}

func htmlSelectAll(L *lua.LState, attribute string) int {
	htmlText := L.CheckString(1)
	selector := L.CheckString(2)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	values := L.NewTable()
	doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
		value := strings.TrimSpace(selection.Text())
		if attribute != "" {
			value, _ = selection.Attr(attribute)
			value = strings.TrimSpace(value)
		}
		values.Append(lua.LString(value))
	})
	L.Push(values)
	return 1
}

func URLResolve(L *lua.LState) int {
	base, err := url.Parse(L.CheckString(1))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	reference, err := url.Parse(L.CheckString(2))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(base.ResolveReference(reference).String()))
	return 1
}
