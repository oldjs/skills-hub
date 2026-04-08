#Requires -Version 5.1
<#
.SYNOPSIS
    Skills Hub CLI - OpenClaw Agent Skill (PowerShell)
.DESCRIPTION
    Windows 版 Skills Hub CLI，功能与 skills-hub.sh 完全一致。
    通过 Skills Hub API 搜索、浏览、下载和上传 OpenClaw 技能。
#>

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command = "help",
    [Parameter(Position = 1, ValueFromRemainingArguments)]
    [string[]]$Arguments
)

$ErrorActionPreference = "Stop"

# --- 配置 ---
$ApiKey = $env:SKILLS_HUB_API_KEY
$BaseUrl = if ($env:SKILLS_HUB_BASE_URL) { $env:SKILLS_HUB_BASE_URL.TrimEnd("/") } else { "https://skills-hub.example.com" }
$ApiBase = "$BaseUrl/api/v1"

function Test-Auth {
    if (-not $ApiKey) {
        Write-Error "错误: 未设置 SKILLS_HUB_API_KEY 环境变量`n请在 Skills Hub 个人中心生成 API Key 后设置:`n  `$env:SKILLS_HUB_API_KEY = 'shk_your_key_here'"
        exit 1
    }
}

# --- 统一 HTTP GET，返回 PSObject ---
function Invoke-ApiGet {
    param([string]$Path)
    Test-Auth
    $url = "$ApiBase$Path"
    try {
        $resp = Invoke-WebRequest -Uri $url -Headers @{
            "Authorization" = "Bearer $ApiKey"
            "Accept"        = "application/json"
        } -UseBasicParsing -ErrorAction Stop
        return $resp.Content | ConvertFrom-Json
    }
    catch {
        # 尝试从响应体提取错误信息
        $statusCode = $_.Exception.Response.StatusCode.value__
        $body = $null
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $body = $reader.ReadToEnd() | ConvertFrom-Json
            $reader.Close()
        } catch {}
        if ($body -and $body.error) {
            Write-Error "错误: $($body.error) (HTTP $statusCode)"
        } elseif ($statusCode) {
            Write-Error "错误: API 返回 HTTP $statusCode ($url)"
        } else {
            Write-Error "错误: 无法连接 $url - $($_.Exception.Message)"
        }
        exit 1
    }
}

# --- URL 编码 ---
function ConvertTo-UrlEncoded {
    param([string]$Text)
    return [System.Uri]::EscapeDataString($Text)
}

# --- 搜索技能 ---
function Invoke-Search {
    param([string[]]$Args)
    $query = ""; $category = ""; $sort = ""; $page = "1"; $perPage = "20"
    $i = 0
    while ($i -lt $Args.Count) {
        switch ($Args[$i]) {
            "--category" { $category = $Args[$i + 1]; $i += 2 }
            "--sort"     { $sort = $Args[$i + 1]; $i += 2 }
            "--page"     { $page = $Args[$i + 1]; $i += 2 }
            "--per-page" { $perPage = $Args[$i + 1]; $i += 2 }
            default {
                if ($Args[$i].StartsWith("-")) {
                    Write-Error "未知参数: $($Args[$i])"; exit 1
                }
                $query = $Args[$i]; $i++
            }
        }
    }

    $params = "?page=$page&per_page=$perPage"
    if ($query)    { $params += "&q=$(ConvertTo-UrlEncoded $query)" }
    if ($category) { $params += "&category=$(ConvertTo-UrlEncoded $category)" }
    if ($sort)     { $params += "&sort=$sort" }

    $result = Invoke-ApiGet "/search$params"
    $total = if ($result.total) { $result.total } else { 0 }
    Write-Host "共找到 $total 个技能:`n"

    if ($result.skills) {
        foreach ($s in $result.skills) {
            $ver = if ($s.version) { $s.version } else { "-" }
            $cat = if ($s.category) { $s.category } else { "无分类" }
            $rating = if ($s.rating) { $s.rating } else { 0 }
            Write-Host "  $($s.name) (v$ver)  [$cat]  评分:$rating  slug:$($s.slug)"
        }
    }
    if ($total -eq 0) { Write-Host "  (无结果)" }
}

# --- 技能详情 ---
function Invoke-Info {
    param([string[]]$Args)
    $slug = if ($Args.Count -gt 0) { $Args[0] } else { "" }
    if (-not $slug) { Write-Error "用法: skills-hub info <skill-slug>"; exit 1 }

    $result = Invoke-ApiGet "/skills/$slug"
    $ver = if ($result.version) { $result.version } else { "-" }
    $author = if ($result.author) { $result.author } else { "-" }
    $rating = if ($result.rating) { $result.rating } else { 0 }
    $ratingCount = if ($result.rating_count) { $result.rating_count } else { 0 }
    $downloads = if ($result.download_count) { $result.download_count } else { 0 }
    $cat = if ($result.category) { $result.category } else { "-" }
    $keywords = if ($result.keywords) { $result.keywords -join ", " } else { "" }

    Write-Host "名称: $($result.name)"
    Write-Host "版本: $ver"
    Write-Host "作者: $author"
    Write-Host "评分: $rating ($ratingCount 人评分)"
    Write-Host "下载: $downloads 次"
    Write-Host "分类: $cat"
    Write-Host "关键词: $keywords"
    Write-Host "`n--- README ---`n"
    # readme 是 HTML，粗略去标签
    $readme = if ($result.readme) { $result.readme } else { "暂无文档" }
    $readme -replace '<[^>]+>', '' -replace '(?m)^\s*$\n', '' | Write-Host
}

# --- 下载安装 ---
function Invoke-Install {
    param([string[]]$Args)
    $slug = ""; $targetDir = if ($env:SKILLS_DIR) { $env:SKILLS_DIR } else { "./skills" }
    $i = 0
    # 第一个非 flag 参数是 slug
    while ($i -lt $Args.Count) {
        switch ($Args[$i]) {
            "--dir" { $targetDir = $Args[$i + 1]; $i += 2 }
            default {
                if ($Args[$i].StartsWith("-")) { Write-Error "未知参数: $($Args[$i])"; exit 1 }
                $slug = $Args[$i]; $i++
            }
        }
    }
    if (-not $slug) { Write-Error "用法: skills-hub install <skill-slug> [--dir ./skills]"; exit 1 }

    Test-Auth

    # 先查详情拿 id
    $detail = Invoke-ApiGet "/skills/$slug"
    $skillId = $detail.id
    $skillName = $detail.name
    if (-not $skillId -or $skillId -eq 0) { Write-Error "错误: 无法获取技能 ID"; exit 1 }

    Write-Host "正在下载: $skillName ($slug)..."

    # 下载 ZIP 到临时文件
    $tmpZip = [System.IO.Path]::GetTempFileName() + ".zip"
    try {
        Invoke-WebRequest -Uri "$ApiBase/download/$skillId" -Headers @{
            "Authorization" = "Bearer $ApiKey"
        } -OutFile $tmpZip -UseBasicParsing -ErrorAction Stop
    } catch {
        Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue
        Write-Error "错误: 下载失败 - $($_.Exception.Message)"
        exit 1
    }

    # 解压
    $extractDir = Join-Path $targetDir $slug
    if (-not (Test-Path $extractDir)) { New-Item -ItemType Directory -Path $extractDir -Force | Out-Null }
    try {
        Expand-Archive -Path $tmpZip -DestinationPath $extractDir -Force
    } catch {
        Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue
        Write-Error "错误: 解压失败，文件可能不是有效的 ZIP"
        exit 1
    }
    Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue

    Write-Host "已安装到: $extractDir"
    Write-Host "文件列表:"
    Get-ChildItem -Path $extractDir -Recurse -File | Select-Object -First 20 | ForEach-Object {
        Write-Host "  $($_.FullName)"
    }
}

# --- 上传发布 ---
function Invoke-Publish {
    param([string[]]$Args)
    $srcDir = ""; $tenantId = ""
    $i = 0
    while ($i -lt $Args.Count) {
        switch ($Args[$i]) {
            "--tenant-id" { $tenantId = $Args[$i + 1]; $i += 2 }
            default {
                if ($Args[$i].StartsWith("-")) { Write-Error "未知参数: $($Args[$i])"; exit 1 }
                $srcDir = $Args[$i]; $i++
            }
        }
    }

    if (-not $srcDir -or -not (Test-Path $srcDir -PathType Container)) {
        Write-Error "用法: skills-hub publish <目录路径> [--tenant-id 租户ID]`n目录内必须包含 SKILL.md 文件"
        exit 1
    }
    $skillMdPath = Join-Path $srcDir "SKILL.md"
    if (-not (Test-Path $skillMdPath)) {
        Write-Error "错误: $srcDir 目录下没有 SKILL.md 文件"
        exit 1
    }

    Test-Auth
    Write-Host "正在打包: $srcDir..."

    # 打包 ZIP（排除 .git 等）
    $tmpZip = [System.IO.Path]::GetTempFileName() + ".zip"
    try {
        # 收集要打包的文件（排除 .git、__pycache__、.DS_Store）
        $files = Get-ChildItem -Path $srcDir -Recurse -File |
            Where-Object { $_.FullName -notmatch '[\\/]\.git[\\/]' -and $_.FullName -notmatch '__pycache__' -and $_.Name -ne '.DS_Store' }
        # 用 .NET 的 ZipFile 打包
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $zip = [System.IO.Compression.ZipFile]::Open($tmpZip, 'Create')
        $basePath = (Resolve-Path $srcDir).Path
        foreach ($f in $files) {
            $entryName = $f.FullName.Substring($basePath.Length + 1).Replace("\", "/")
            [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $f.FullName, $entryName) | Out-Null
        }
        $zip.Dispose()
    } catch {
        Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue
        Write-Error "错误: 打包失败 - $($_.Exception.Message)"
        exit 1
    }

    Write-Host "正在上传..."

    $uploadUrl = "$ApiBase/upload"
    if ($tenantId) { $uploadUrl += "?tenant_id=$tenantId" }

    try {
        # 用 .NET HttpClient 上传 multipart/form-data
        Add-Type -AssemblyName System.Net.Http
        $client = New-Object System.Net.Http.HttpClient
        $client.DefaultRequestHeaders.Add("Authorization", "Bearer $ApiKey")
        $content = New-Object System.Net.Http.MultipartFormDataContent
        $fileBytes = [System.IO.File]::ReadAllBytes($tmpZip)
        $fileContent = New-Object System.Net.Http.ByteArrayContent(,$fileBytes)
        $fileContent.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::Parse("application/zip")
        $content.Add($fileContent, "zipfile", "skill.zip")
        $resp = $client.PostAsync($uploadUrl, $content).Result
        $body = $resp.Content.ReadAsStringAsync().Result
        $client.Dispose()
        Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue

        if (-not $resp.IsSuccessStatusCode) {
            $errObj = $body | ConvertFrom-Json -ErrorAction SilentlyContinue
            $errMsg = if ($errObj -and $errObj.error) { $errObj.error } else { "HTTP $($resp.StatusCode.value__)" }
            Write-Error "错误: 上传失败 - $errMsg"
            exit 1
        }

        $result = $body | ConvertFrom-Json
        Write-Host "上传成功!"
        Write-Host "  Slug: $($result.slug)"
        Write-Host "  状态: $($result.review_status)"
        if ($result.message) { Write-Host $result.message }
    } catch {
        Remove-Item -Force $tmpZip -ErrorAction SilentlyContinue
        Write-Error "错误: 上传失败 - $($_.Exception.Message)"
        exit 1
    }
}

# --- 浏览分类 ---
function Invoke-Categories {
    param([string[]]$Args)
    $tenantId = ""
    $i = 0
    while ($i -lt $Args.Count) {
        switch ($Args[$i]) {
            "--tenant-id" { $tenantId = $Args[$i + 1]; $i += 2 }
            default { Write-Error "未知参数: $($Args[$i])"; exit 1 }
        }
    }
    $params = ""
    if ($tenantId) { $params = "?tenant_id=$tenantId" }

    $result = Invoke-ApiGet "/categories$params"
    Write-Host "技能分类:`n"
    if ($result.categories) {
        foreach ($c in $result.categories) {
            Write-Host "  $($c.name) ($($c.count) 个技能)"
        }
    }
}

# --- 平台统计 ---
function Invoke-Stats {
    $result = Invoke-ApiGet "/stats"
    Write-Host "Skills Hub 平台统计:`n"
    Write-Host "  技能总数:   $($result.total_skills)"
    Write-Host "  用户总数:   $($result.total_users)"
    Write-Host "  租户总数:   $($result.total_tenants)"
    Write-Host "  评论总数:   $($result.total_comments)"
    Write-Host "  评分总数:   $($result.total_ratings)"
}

# --- 帮助 ---
function Show-Help {
    Write-Host @"
Skills Hub CLI - OpenClaw Agent Skill (PowerShell)

用法: skills-hub.ps1 <命令> [参数]

命令:
  search <关键词>     搜索技能
  info <slug>         查看技能详情
  install <slug>      下载并安装技能
  publish <目录>      打包上传技能
  categories          浏览所有分类
  stats               查看平台统计
  help                显示此帮助

环境变量:
  SKILLS_HUB_API_KEY   API Key (必需)
  SKILLS_HUB_BASE_URL  服务地址 (默认 https://skills-hub.example.com)
  SKILLS_DIR            技能安装目录 (默认 ./skills)
"@
}

# --- 入口 ---
switch ($Command) {
    "search"     { Invoke-Search $Arguments }
    "info"       { Invoke-Info $Arguments }
    "install"    { Invoke-Install $Arguments }
    "publish"    { Invoke-Publish $Arguments }
    "categories" { Invoke-Categories $Arguments }
    "stats"      { Invoke-Stats }
    { $_ -in "help", "--help", "-h" } { Show-Help }
    default {
        Write-Host "未知命令: $Command"
        Write-Host "运行 'skills-hub.ps1 help' 查看可用命令"
        exit 1
    }
}
