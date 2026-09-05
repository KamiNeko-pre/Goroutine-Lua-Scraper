package engine

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteLuaScriptStopsDeadloopAtDeadline(t *testing.T) {
	scriptText, err := os.ReadFile(filepath.Join("..", "..", "scripts", "deadloop.lua"))
	if err != nil {
		t.Fatalf("read deadloop script: %v", err)
	}

	startedAt := time.Now()
	_, err = executeLuaScript(string(scriptText), "https://example.com", 200*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want wrapped context deadline exceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("Lua execution returned too late: %v", elapsed)
	}
}

func TestExecuteLuaScriptParsesContractAndHtmlFind(t *testing.T) {
	result, err := executeLuaScript(`
local title, findErr = html_find("<article><h1 class='title'>LuaSpider</h1></article>", ".title")
if findErr then
    return false, findErr
end

return true, {
    url = TARGET_URL,
    stars = "12.3k",
    description = title,
    fission_urls = {"https://github.com/golang/go"}
}
`, "https://github.com/example/project", time.Second)
	if err != nil {
		t.Fatalf("execute Lua script: %v", err)
	}
	if result.URL != "https://github.com/example/project" {
		t.Fatalf("URL = %q, want target URL", result.URL)
	}
	if result.Stars != 12300 {
		t.Fatalf("Stars = %d, want 12300", result.Stars)
	}
	if result.Description != "LuaSpider" {
		t.Fatalf("Description = %q, want %q", result.Description, "LuaSpider")
	}
	if len(result.FissionURLs) != 1 || result.FissionURLs[0] != "https://github.com/golang/go" {
		t.Fatalf("FissionURLs = %v, want one Go repository URL", result.FissionURLs)
	}
}

func TestExecuteLuaScriptExposesCollectionAndURLHelpers(t *testing.T) {
	result, err := executeLuaScript(`
local html = [[
<article><a href="/first"> First </a></article>
<article><a href="second">Second</a></article>
]]

local titles, titleErr = html_find_all(html, "article a")
if titleErr then return false, titleErr end
local links, linkErr = html_attr_all(html, "article a", "href")
if linkErr then return false, linkErr end
local absoluteURL, resolveErr = url_resolve(TARGET_URL, links[1])
if resolveErr then return false, resolveErr end

return true, {
    url = absoluteURL,
    stars = tostring(#titles),
    description = titles[1] .. "," .. titles[2],
    fission_urls = {absoluteURL},
}
`, "https://example.com/catalog/index.html", time.Second)
	if err != nil {
		t.Fatalf("execute Lua helper script: %v", err)
	}
	if result.URL != "https://example.com/first" {
		t.Fatalf("URL = %q, want resolved absolute URL", result.URL)
	}
	if result.Stars != 2 {
		t.Fatalf("Stars = %d, want helper result count 2", result.Stars)
	}
	if result.Description != "First,Second" {
		t.Fatalf("Description = %q, want trimmed collection values", result.Description)
	}
}

func TestExecuteLuaScriptReturnsRuleError(t *testing.T) {
	_, err := executeLuaScript(`return false, "selector not found"`, "https://example.com", time.Second)
	if err == nil || !strings.Contains(err.Error(), "selector not found") {
		t.Fatalf("error = %v, want rule error message", err)
	}
}

func TestExecuteLuaScriptDoesNotExposeDangerousLibraries(t *testing.T) {
	_, err := executeLuaScript(`
local forbidden = {
    "os", "io", "debug", "package",
    "dofile", "load", "loadfile", "loadstring", "require", "module",
}

for _, name in ipairs(forbidden) do
    if _G[name] ~= nil then
        return false, "forbidden global exposed: " .. name
    end
end

return true, {
    url = TARGET_URL,
    stars = "0",
    description = "",
    fission_urls = {},
}
`, "https://example.com", time.Second)
	if err != nil {
		t.Fatalf("dangerous libraries should not be exposed: %v", err)
	}
}

func TestHTTPRequestsStopAtContextDeadline(t *testing.T) {
	tests := []struct {
		name string
		run  func(ctx context.Context, serverURL string) (*http.Response, error)
	}{
		{
			name: "direct request",
			run: func(ctx context.Context, serverURL string) (*http.Response, error) {
				return doDirectRequest(ctx, serverURL, time.Second)
			},
		},
		{
			name: "proxy request",
			run: func(ctx context.Context, serverURL string) (*http.Response, error) {
				return doProxyRequest(ctx, "http://example.com", serverURL, time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestStarted := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				close(requestStarted)
				<-r.Context().Done()
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			errCh := make(chan error, 1)
			go func() {
				_, err := tt.run(ctx, server.URL)
				errCh <- err
			}()

			select {
			case <-requestStarted:
			case <-time.After(time.Second):
				t.Fatal("test server did not receive the request")
			}

			startedAt := time.Now()
			err := <-errCh
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want wrapped context deadline exceeded", err)
			}
			if elapsed := time.Since(startedAt); elapsed > time.Second {
				t.Fatalf("request returned too late after deadline: %v", elapsed)
			}
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "public HTTPS domain", rawURL: "https://github.com/golang/go"},
		{name: "public IP", rawURL: "http://8.8.8.8"},
		{name: "unsupported scheme", rawURL: "file:///etc/passwd", wantErr: true},
		{name: "missing host", rawURL: "https:///only-a-path", wantErr: true},
		{name: "localhost", rawURL: "http://localhost:8080", wantErr: true},
		{name: "localhost subdomain", rawURL: "http://api.localhost", wantErr: true},
		{name: "IPv4 loopback", rawURL: "http://127.0.0.1", wantErr: true},
		{name: "IPv6 loopback", rawURL: "http://[::1]", wantErr: true},
		{name: "loopback IPv4", rawURL: "http://127.0.0.1:8080", wantErr: true},
		{name: "link-local IPv4", rawURL: "http://169.254.169.254", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateTargetURL(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTargetURL(%q) error = %v, wantErr %v", tt.rawURL, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRedirect(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/internal", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if err := validateRedirect(request, nil); err == nil {
		t.Fatal("private redirect target should be rejected")
	}
}

func TestReadResponseBodyRejectsOversizedBody(t *testing.T) {
	tooLarge := bytes.Repeat([]byte("a"), int(maxHTTPResponseBodyBytes)+1)
	_, err := readResponseBody(bytes.NewReader(tooLarge), -1)
	if err == nil {
		t.Fatal("oversized response body should be rejected")
	}
}
