#!/usr/bin/env bash
# Skills Hub CLI — Agent Skill 脚本
# 让 AI Agent 通过命令行调用 Skills Hub API
set -euo pipefail

# --- 依赖检查 ---
check_deps() {
    local missing=""
    command -v curl >/dev/null 2>&1 || missing="${missing} curl"
    command -v jq >/dev/null 2>&1   || missing="${missing} jq"
    command -v unzip >/dev/null 2>&1 || missing="${missing} unzip"
    if [ -n "$missing" ]; then
        echo "错误: 缺少必要依赖:${missing}"
        echo "请先安装: apt install${missing}  或  brew install${missing}"
        exit 1
    fi
}
check_deps

# --- 配置 ---
API_KEY="${SKILLS_HUB_API_KEY:-}"
BASE_URL="${SKILLS_HUB_BASE_URL:-https://skills-hub.example.com}"
API_BASE="${BASE_URL}/api/v1"

# 没有 API Key 就报错退出
check_auth() {
    if [ -z "$API_KEY" ]; then
        echo "错误: 未设置 SKILLS_HUB_API_KEY 环境变量"
        echo "请在 Skills Hub 个人中心生成 API Key 后设置:"
        echo "  export SKILLS_HUB_API_KEY=\"shk_your_key_here\""
        exit 1
    fi
}

# 统一的 HTTP GET 请求，stdout 只输出 body，错误走 stderr
api_get() {
    local path="$1"
    check_auth

    local http_code body
    # 把响应体写到临时文件，http_code 拿状态码，stderr 保持独立
    local tmp_body
    tmp_body=$(mktemp)
    http_code=$(curl -sS -w '%{http_code}' \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Accept: application/json" \
        -o "$tmp_body" \
        "${API_BASE}${path}" 2>/dev/null) || {
        rm -f "$tmp_body"
        echo "错误: 无法连接 ${API_BASE}${path} (网络错误)" >&2
        return 1
    }
    body=$(cat "$tmp_body")
    rm -f "$tmp_body"

    if [ "$http_code" -ge 400 ]; then
        # 尝试从 JSON 中提取错误信息
        local api_err
        api_err=$(echo "$body" | jq -r '.error // empty' 2>/dev/null) || true
        if [ -n "$api_err" ]; then
            echo "错误: ${api_err} (HTTP ${http_code})" >&2
        else
            echo "错误: API 返回 HTTP ${http_code}" >&2
        fi
        return 1
    fi

    echo "$body"
}

# --- 搜索技能 ---
cmd_search() {
    local query="" category="" sort="" page="1" per_page="20"

    # 第一个非 flag 参数当作搜索词
    while [ $# -gt 0 ]; do
        case "$1" in
            --category) category="$2"; shift 2 ;;
            --sort)     sort="$2"; shift 2 ;;
            --page)     page="$2"; shift 2 ;;
            --per-page) per_page="$2"; shift 2 ;;
            -*)         echo "未知参数: $1"; exit 1 ;;
            *)          query="$1"; shift ;;
        esac
    done

    local params="?page=${page}&per_page=${per_page}"
    [ -n "$query" ]    && params="${params}&q=$(urlencode "$query")"
    [ -n "$category" ] && params="${params}&category=$(urlencode "$category")"
    [ -n "$sort" ]     && params="${params}&sort=${sort}"

    local result
    result=$(api_get "/search${params}") || exit 1

    local total
    total=$(echo "$result" | jq -r '.total // 0')
    echo "共找到 ${total} 个技能:"
    echo ""
    echo "$result" | jq -r '.skills[]? | "  \(.name) (v\(.version // "-"))  [\(.category // "无分类")]  评分:\(.rating // 0)  slug:\(.slug)"'

    if [ "$total" = "0" ]; then
        echo "  (无结果)"
    fi
}

# --- 技能详情 ---
cmd_info() {
    local slug="${1:-}"
    if [ -z "$slug" ]; then
        echo "用法: skills-hub info <skill-slug>"
        exit 1
    fi

    local result
    result=$(api_get "/skills/${slug}") || exit 1

    echo "$result" | jq -r '
        "名称: \(.name)",
        "版本: \(.version // "-")",
        "作者: \(.author // "-")",
        "评分: \(.rating // 0) (\(.rating_count // 0) 人评分)",
        "下载: \(.download_count // 0) 次",
        "分类: \(.category // "-")",
        "关键词: \((.keywords // []) | join(", "))",
        "",
        "--- README ---",
        ""
    '
    # readme 是 HTML，用 sed 粗略去标签给终端看
    echo "$result" | jq -r '.readme // "暂无文档"' | sed 's/<[^>]*>//g' | sed '/^$/d'
}

# --- 下载安装 ---
cmd_install() {
    local slug="${1:-}"
    local target_dir="${SKILLS_DIR:-./skills}"

    shift || true
    while [ $# -gt 0 ]; do
        case "$1" in
            --dir) target_dir="$2"; shift 2 ;;
            *)     echo "未知参数: $1"; exit 1 ;;
        esac
    done

    if [ -z "$slug" ]; then
        echo "用法: skills-hub install <skill-slug> [--dir ./skills]"
        exit 1
    fi

    check_auth

    # 先拿详情确认 skill 存在，同时拿到 id（下载接口用 id 不是 slug）
    local detail
    if ! detail=$(api_get "/skills/${slug}"); then
        echo "错误: 技能 '${slug}' 不存在或无权访问"
        exit 1
    fi
    local skill_name skill_id
    skill_name=$(echo "$detail" | jq -r '.name // ""')
    skill_id=$(echo "$detail" | jq -r '.id // 0')

    if [ "$skill_id" = "0" ] || [ -z "$skill_id" ]; then
        echo "错误: 无法获取技能 ID"
        exit 1
    fi

    echo "正在下载: ${skill_name} (${slug})..."

    mkdir -p "${target_dir}"

    # 下载 ZIP，错误信息走 stderr
    local tmp_zip http_code
    tmp_zip=$(mktemp /tmp/skill-XXXXXX.zip)
    http_code=$(curl -sS -w '%{http_code}' \
        -H "Authorization: Bearer ${API_KEY}" \
        -o "$tmp_zip" \
        "${API_BASE}/download/${skill_id}" 2>/dev/null) || {
        rm -f "$tmp_zip"
        echo "错误: 下载失败 (网络错误)"
        exit 1
    }

    if [ "$http_code" -ge 400 ]; then
        rm -f "$tmp_zip"
        echo "错误: 下载失败 (HTTP ${http_code})"
        exit 1
    fi

    # 解压到目标目录
    local extract_dir="${target_dir}/${slug}"
    mkdir -p "$extract_dir"
    if ! unzip -o -q "$tmp_zip" -d "$extract_dir" 2>/dev/null; then
        rm -f "$tmp_zip"
        echo "错误: 解压失败，文件可能不是有效的 ZIP"
        exit 1
    fi
    rm -f "$tmp_zip"

    echo "已安装到: ${extract_dir}"
    echo "文件列表:"
    find "$extract_dir" -type f | head -20 | while read -r f; do
        echo "  ${f}"
    done
}

# --- 上传发布 ---
cmd_publish() {
    local src_dir="${1:-}"
    local tenant_id=""

    shift || true
    while [ $# -gt 0 ]; do
        case "$1" in
            --tenant-id) tenant_id="$2"; shift 2 ;;
            *)           echo "未知参数: $1"; exit 1 ;;
        esac
    done

    if [ -z "$src_dir" ] || [ ! -d "$src_dir" ]; then
        echo "用法: skills-hub publish <目录路径> [--tenant-id 租户ID]"
        echo "目录内必须包含 SKILL.md 文件"
        exit 1
    fi

    # 检查 SKILL.md 存在
    if [ ! -f "${src_dir}/SKILL.md" ]; then
        echo "错误: ${src_dir} 目录下没有 SKILL.md 文件"
        exit 1
    fi

    check_auth

    echo "正在打包: ${src_dir}..."

    # 打包成 ZIP
    local tmp_zip
    tmp_zip=$(mktemp /tmp/skill-upload-XXXXXX.zip)
    if ! (cd "$src_dir" && zip -r -q "$tmp_zip" . -x '*.git*' -x '*__pycache__*' -x '*.DS_Store'); then
        rm -f "$tmp_zip"
        echo "错误: 打包失败"
        exit 1
    fi

    echo "正在上传..."

    # 构建上传 URL
    local upload_url="${API_BASE}/upload"
    [ -n "$tenant_id" ] && upload_url="${upload_url}?tenant_id=${tenant_id}"

    # 上传，先拿 http_code 和 body 分开处理
    local tmp_resp http_code result
    tmp_resp=$(mktemp)
    http_code=$(curl -sS -w '%{http_code}' \
        -H "Authorization: Bearer ${API_KEY}" \
        -F "zipfile=@${tmp_zip};filename=skill.zip" \
        -o "$tmp_resp" \
        "$upload_url" 2>/dev/null) || {
        rm -f "$tmp_zip" "$tmp_resp"
        echo "错误: 上传失败 (网络错误)"
        exit 1
    }
    result=$(cat "$tmp_resp")
    rm -f "$tmp_zip" "$tmp_resp"

    if [ "$http_code" -ge 400 ]; then
        local api_err
        api_err=$(echo "$result" | jq -r '.error // empty' 2>/dev/null) || true
        echo "错误: 上传失败 (HTTP ${http_code})"
        [ -n "$api_err" ] && echo "  原因: ${api_err}"
        exit 1
    fi

    local new_slug review_status
    new_slug=$(echo "$result" | jq -r '.slug // "unknown"')
    review_status=$(echo "$result" | jq -r '.review_status // "unknown"')

    echo "上传成功!"
    echo "  Slug: ${new_slug}"
    echo "  状态: ${review_status}"
    echo "$result" | jq -r '.message // ""'
}

# --- 浏览分类 ---
cmd_categories() {
    local tenant_id=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --tenant-id) tenant_id="$2"; shift 2 ;;
            *)           echo "未知参数: $1"; exit 1 ;;
        esac
    done

    local params=""
    [ -n "$tenant_id" ] && params="?tenant_id=${tenant_id}"

    local result
    result=$(api_get "/categories${params}") || exit 1

    echo "技能分类:"
    echo ""
    echo "$result" | jq -r '.categories[]? | "  \(.name) (\(.count) 个技能)"'
}

# --- 平台统计 ---
cmd_stats() {
    local result
    result=$(api_get "/stats") || exit 1

    echo "Skills Hub 平台统计:"
    echo ""
    echo "$result" | jq -r '
        "  技能总数:   \(.total_skills // 0)",
        "  用户总数:   \(.total_users // 0)",
        "  租户总数:   \(.total_tenants // 0)",
        "  评论总数:   \(.total_comments // 0)",
        "  评分总数:   \(.total_ratings // 0)"
    '
}

# --- URL 编码（安全版，不拼接用户输入到代码字符串） ---
urlencode() {
    python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null \
        || printf '%s' "$1" | jq -sRr @uri 2>/dev/null \
        || printf '%s' "$1"
}

# --- 帮助信息 ---
cmd_help() {
    echo "Skills Hub CLI - OpenClaw Agent Skill"
    echo ""
    echo "用法: skills-hub <命令> [参数]"
    echo ""
    echo "命令:"
    echo "  search <关键词>     搜索技能"
    echo "  info <slug>         查看技能详情"
    echo "  install <slug>      下载并安装技能"
    echo "  publish <目录>      打包上传技能"
    echo "  categories          浏览所有分类"
    echo "  stats               查看平台统计"
    echo "  help                显示此帮助"
    echo ""
    echo "环境变量:"
    echo "  SKILLS_HUB_API_KEY   API Key (必需)"
    echo "  SKILLS_HUB_BASE_URL  服务地址 (默认 https://skills-hub.example.com)"
    echo "  SKILLS_DIR            技能安装目录 (默认 ./skills)"
}

# --- 入口 ---
command="${1:-help}"
shift || true

case "$command" in
    search)     cmd_search "$@" ;;
    info)       cmd_info "$@" ;;
    install)    cmd_install "$@" ;;
    publish)    cmd_publish "$@" ;;
    categories) cmd_categories "$@" ;;
    stats)      cmd_stats "$@" ;;
    help|--help|-h) cmd_help ;;
    *)
        echo "未知命令: ${command}"
        echo "运行 'skills-hub help' 查看可用命令"
        exit 1
        ;;
esac
