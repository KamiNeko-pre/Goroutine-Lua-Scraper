-- Smoke rule for a real non-GitHub page: Hacker News front page.
-- It returns the first ranked story using the LuaSpider rule contract.

local body, request_err = http_get(TARGET_URL)
if request_err then
    return false, "request Hacker News: " .. request_err
end

local title, title_err = html_find(body, "span.titleline > a")
if title_err then
    return false, "find top story title: " .. title_err
end

local score, _ = html_find(body, "span.score")
if not score then
    score = "0"
else
    score = string.match(score, "%d+") or "0"
end

return true, {
    url = TARGET_URL,
    stars = score,
    description = title,
    fission_urls = {},
}
