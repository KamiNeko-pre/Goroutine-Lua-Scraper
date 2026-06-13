package engine

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/fsnotify/fsnotify"
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"
	"gorm.io/gorm/clause"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	//"github.com/spf13/viper"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

var routeCache sync.Map

var TaskQuene = make(chan string, 1000)

var (
	ScriptCache string
	ScriptMutex sync.RWMutex
)

func InitLuaEngine(scriptPath string) {
	reloadScript(scriptPath)

	watcher, err := fsnotify.NewWatcher()

	if err != nil {
		logger.Log.Info("创建文件监听保安失败", zap.Error(err))
	}

	err = watcher.Add(scriptPath)

	if err != nil {
		logger.Log.Fatal("监听脚本目录失败", zap.Error(err))
	}

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

func HttpGet(L *lua.LState) int {

	targetUrl := L.CheckString(1)

	cfg := config.Get()

	proxyStr := cfg.App.Proxy

	parseUrl, err := url.Parse(targetUrl)

	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("URL解析失败: " + err.Error()))
		return 2
	}
	host := parseUrl.Hostname()

	useProxy := false

	if val, ok := routeCache.Load(host); ok {
		useProxy = val.(bool)
	}
	var resp *http.Response
	var reqErr error
	if useProxy && proxyStr != "" {
		resp, reqErr = doProxyRequest(targetUrl, proxyStr)
	} else {
		directClient := &http.Client{Timeout: 5 * time.Second}
		resp, reqErr = directClient.Get(targetUrl)
		if reqErr != nil && proxyStr != "" {
			logger.Log.Warn("直连失败, 启动代理并记录至路由表", zap.String("host", host))
			resp, reqErr = doProxyRequest(targetUrl, proxyStr)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(string(body)))
	return 1
}

func doProxyRequest(targetUrl, proxyStr string) (*http.Response, error) {
	proxyURL, _ := url.Parse(proxyStr)
	proxyClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	return proxyClient.Get(targetUrl)

}

func RunLuaScript(scriptPath string, targetUrl string) error {
	ScriptMutex.RLock()
	scriptText := ScriptCache
	ScriptMutex.RUnlock()
	if scriptText == "" {
		return fmt.Errorf("内存中无有效规则")
	}
	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("http_get", L.NewFunction(HttpGet))
	L.SetGlobal("html_find", L.NewFunction(HtmlFind))
	L.SetGlobal("TARGET_URL", lua.LString(targetUrl))
	if err := L.DoString(scriptText); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "\n") {
			errMsg = strings.Split(errMsg, "\n")[0]
		}
		return fmt.Errorf("Lua 沙箱奔溃: %v", errMsg)
	}
	top := L.GetTop()
	if top < 2 {
		return fmt.Errorf("Lua 脚本没有按规范返回[布尔值，数据/错误信息]")
	}
	isSuccess := lua.LVAsBool(L.Get(1))
	if !isSuccess {
		errMsg := L.Get(2).String()
		return fmt.Errorf("%s", errMsg)
	}
	luaData, ok := L.Get(2).(*lua.LTable)
	if !ok {
		return fmt.Errorf("Lua 脚本返回的成功数据不是一个Table")
	}
	finalUrl := luaData.RawGetString("url").String()
	starStr := luaData.RawGetString("stars").String()
	desc := luaData.RawGetString("description").String()
	var fissionURLs []string
	fissionValue := luaData.RawGetString("fission_urls")
	if fissionTable, ok := fissionValue.(*lua.LTable); ok {
		fissionTable.ForEach(func(key lua.LValue, value lua.LValue) {
			if value.Type() == lua.LTString {
				fissionURLs = append(fissionURLs, value.String())
			}
		})
	}
	starStr = strings.ReplaceAll(starStr, "k", "000")
	starStr = strings.ReplaceAll(starStr, ".", "")
	starInt, _ := strconv.Atoi(strings.TrimSpace(starStr))

	repoName := strings.Replace(finalUrl, "https://github.com/", "", 1)
	repoRecord := repository.GithubRepo{
		Name:        repoName,
		Stars:       starInt,
		Description: desc,
	}
	result := repository.DB.Where(repository.GithubRepo{Name: repoName}).Assign(repoRecord).FirstOrCreate(&repoRecord)
	if result.Error != nil {
		return fmt.Errorf("数据入库失败: %v", result.Error)
	}
	if len(fissionURLs) > 0 {
		var newRepos []repository.GithubRepo
		for _, furl := range fissionURLs {
			fName := strings.Replace(furl, "https://github.com/", "", 1)
			newRepos = append(newRepos, repository.GithubRepo{Name: fName})
		}

		// 这就是核心装甲：批量把几百个链接砸向 SQLite。
		// 如果名字(Name)已存在(uniqueIndex)，SQLite 会在底层物理弹开它，完全不会报错或锁死。
		dbErr := repository.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&newRepos).Error

		if dbErr != nil {
			logger.Log.Warn("裂变链接入库遭遇部分异常", zap.Error(dbErr))
		} else {
			logger.Log.Info("📡 裂变雷达生效", zap.Int("发现并抛入蓄水池的新链接数", len(newRepos)))
		}
	}
	logger.Log.Info("数据已保存至数据库!", zap.String("仓库", repoName), zap.Int("Stars", starInt))
	return nil
}

func HtmlFind(L *lua.LState) int {

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
