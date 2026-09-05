-- One page produces one complete snapshot. Relative URLs stay source-owned.
local body, err = http_get(TARGET_URL)
if err then return false, err end
local links = html_attr_all(body, "article.Box-row h2 a", "href")
local items = {}
for index, path in ipairs(links or {}) do
    local row = "article.Box-row:nth-of-type(" .. index .. ")"
    local name = path:gsub("^/", "")
    local description = html_find(body, row .. " p") or ""
    local language = html_find(body, row .. " [itemprop='programmingLanguage']") or "Unknown"
    local stars = html_find(body, row .. " a[href$='/stargazers']") or "0"
    local today = html_find(body, row .. " span.float-sm-right") or "0"
    local clean_stars = stars:gsub(",", ""):gsub("%s", "")
    local today_number = today:gsub(",", ""):match("%d+")
    items[#items + 1] = {
        name = name, url = url_resolve(TARGET_URL, path), rank = index,
        stars = tonumber(clean_stars) or 0, stars_today = tonumber(today_number) or 0,
        description = description, language = language
    }
end
if #items == 0 then return false, "GitHub trending rows were not found" end
return true, {items = items, meta = {source = "GitHub", scope = "daily Go trending", url = TARGET_URL}}
