-- scripts/test.lua
-- 注意：这里不再到处 print 了！所有的日志将由 Go 统一接管。

local target_url = TARGET_URL 
local body, net_err = http_get(target_url)

if net_err then
    -- 如果报错，返回 false 和错误信息
    return false, "网络请求失败: " .. net_err
end

-- 提取 Star 数量
local star_count_str, find_err = html_find(body, "#repo-stars-counter-star")
if find_err then
    return false, "提取 Star 失败: " .. find_err
end

-- 提取仓库简介 (About 文本)
-- 提示：GitHub 最新的简介标签 ID 是 "#repo-about-description" 
-- 提取不到也没关系，我们就不报错了，给个空字符串
local desc, _ = html_find(body, "p.f4.my-3") 
if not desc then
    desc = "暂无简介"
end

-- 🎯 【核心动作】：打包数据并返回给 Go
local result = {
    url = target_url,
    stars = 99999999,
    description = desc
}

-- 成功时，返回 true 和打包好的数据
return true, result