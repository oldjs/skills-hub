---
name: Skills Hub
description: 连接 Skills Hub 技能市场，搜索、浏览、下载和上传 OpenClaw Agent Skills
version: 1.0.0
author: Skills Hub Team
categories: api, marketplace, skills
keywords: skills, marketplace, search, download, upload, openclaw
---

# Skills Hub

连接 Skills Hub 技能市场的 Agent Skill。让你的 AI Agent 能直接搜索、浏览、下载和上传 OpenClaw 技能，无需手动操作浏览器。

## 功能

- **搜索技能** — 按关键词、分类搜索技能市场
- **查看详情** — 获取技能的完整文档、版本、评分、关键词
- **下载安装** — 下载技能 ZIP 包并自动解压到本地 skills 目录
- **上传发布** — 将本地技能目录打包上传到平台
- **浏览分类** — 查看所有技能分类及数量统计
- **平台统计** — 查看平台整体数据概览

## 配置

使用前需要设置两个环境变量：

```bash
# Skills Hub API Key（在 Skills Hub 个人中心 -> API Key 管理 中生成）
export SKILLS_HUB_API_KEY="shk_your_api_key_here"

# Skills Hub 服务地址（可选，默认 https://skills-hub.example.com）
export SKILLS_HUB_BASE_URL="https://your-skills-hub-instance.com"
```

## 使用场景

当用户说出以下意图时触发此 Skill：

- "搜索一个关于 GitHub 的 skill"
- "帮我找找有没有网页抓取的技能"
- "下载 browser-agent 这个 skill"
- "把我写的 skill 上传到平台"
- "看看平台上有哪些分类"
- "平台上现在有多少技能"

## 命令参考

### 搜索技能

```bash
skills-hub search "关键词" [--category 分类] [--sort rating|newest] [--page 1] [--per-page 20]
```

### 查看技能详情

```bash
skills-hub info <skill-slug>
```

### 下载技能

```bash
skills-hub install <skill-slug> [--dir ./skills]
```

### 上传技能

```bash
skills-hub publish <目录路径> [--tenant-id 租户ID]
```

### 浏览分类

```bash
skills-hub categories [--tenant-id 租户ID]
```

### 平台统计

```bash
skills-hub stats
```
