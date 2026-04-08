# Skills Hub

[![CI](https://github.com/oldjs/skills-hub/actions/workflows/ci.yml/badge.svg)](https://github.com/oldjs/skills-hub/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](Dockerfile)

[English](README.md) | [中文](README.zh-CN.md)

**Skills Hub** is a multi-tenant skill marketplace for [OpenClaw](https://clawhub.ai), built with Go, SQLite, and Redis. It aggregates public skills from ClawHub while allowing tenants to upload, rate, comment on, and manage private skills through a unified web interface and REST API.

![Home Page](docs/screenshots/home-placeholder.svg)

## Features

**Core**
- Full-text search (FTS5) with BM25 relevance ranking
- Advanced filters: rating, date range, author, source, multi-category
- Skill detail pages with Markdown rendering, ratings, and threaded comments
- Skill version history with admin rollback
- Bookmark (favorites) and curated skill collections
- Skill leaderboard (top-rated + recently active)
- Notification system (review results, comment replies)

**Multi-Tenancy**
- Tenant isolation for all data (skills, ratings, comments)
- Personal tenant auto-created on registration
- Invite system with role-based access (owner / admin / member)
- Tenant switching in the navigation bar

**Authentication & Security**
- Email verification code login (no passwords)
- QQ and Gmail address restrictions with Gmail dot/alias normalization
- CAPTCHA on all forms
- CSRF protection, security headers (CSP, HSTS, X-Frame-Options)
- Brute-force protection: 5 failures lock for 15 minutes
- API Key authentication (SHA-256 hashed at rest)
- Multi-layer rate limiting (API key, IP, user, search)

**Admin Dashboard**
- 30-day registration and skill growth trend charts
- Pending skill review queue with batch approve/reject
- Comment moderation, user management, tenant management
- Action log with CSV export
- Editable email templates

**Developer Ecosystem**
- REST API v1 with 6 endpoints (search, detail, download, upload, categories, stats)
- OpenAPI 3.0 spec at `/api/v1/openapi.json` + Swagger UI at `/api/v1/docs`
- Agent Skill (Bash + PowerShell) for CLI / AI agent integration

**SEO & Performance**
- Server-side rendering (Go templates) - crawlable out of the box
- Public search and skill detail pages (no login required to browse)
- Meta tags, Open Graph, Twitter Cards, canonical URLs
- Dynamic sitemap.xml and robots.txt
- JSON-LD structured data (SoftwareApplication, BreadcrumbList, WebSite)
- HTTP cache headers (ETag, Last-Modified, Cache-Control)
- Redis query caching for homepage listings
- Dark mode with system preference detection

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.21, `net/http`, `log/slog` |
| Database | SQLite (WAL mode) via `modernc.org/sqlite` |
| Cache / Session | Redis 7 |
| Templates | Go `html/template` |
| Markdown | Goldmark + Chroma + Bluemonday |
| Frontend | Tailwind CSS + vanilla JavaScript |
| Charts | Chart.js (admin dashboard) |
| Container | Docker, Docker Compose |
| CI | GitHub Actions + golangci-lint |

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/oldjs/skills-hub.git
cd skills-hub
cp .env.example .env
docker compose up -d --build
```

Open [http://localhost:8080](http://localhost:8080). The first registered user becomes the platform admin.

### Local Development

Prerequisites: Go 1.21+, Redis 7+

```bash
go mod download
cp .env.example .env

# Start Redis (if not running)
redis-server &

# Run the server
go run .
```

### Initial Sync

To populate skills from ClawHub on first start:

```bash
go run . --sync
```

## Configuration

See [`.env.example`](.env.example) for all options.

| Variable | Default | Description |
|----------|---------|-------------|
| `DEV_MODE` | `false` | Set `true` for local dev (auto-creates user + tenant, auto-login) |
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | `./skills.db` | SQLite database file path |
| `REDIS_URL` | `127.0.0.1:6379` | Redis address |
| `COOKIE_SECURE` | `false` | Set `true` in production (HTTPS) |
| `TRUST_PROXY_HEADERS` | `false` | Set `true` behind a reverse proxy |
| `PLATFORM_ADMIN_EMAILS` | | Comma-separated admin emails |
| `RESEND_API_KEY` | | [Resend](https://resend.com) API key for email |
| `MAIL_FROM` | `noreply@example.com` | Email sender address |
| `SITE_URL` | `https://skills-hub.example.com` | Canonical URL for SEO |

## API

All API v1 endpoints require `Authorization: Bearer <api_key>`. Generate API keys at `/account`.

Interactive docs: **`/api/v1/docs`** (Swagger UI)

```bash
# Search skills
curl -H "Authorization: Bearer shk_xxx" "http://localhost:8080/api/v1/search?q=browser"

# Skill detail
curl -H "Authorization: Bearer shk_xxx" "http://localhost:8080/api/v1/skills/my-skill"

# Download ZIP
curl -L -H "Authorization: Bearer shk_xxx" "http://localhost:8080/api/v1/download/123" -o skill.zip

# Upload skill
curl -X POST -H "Authorization: Bearer shk_xxx" -F "zipfile=@skill.zip" "http://localhost:8080/api/v1/upload"

# Categories
curl -H "Authorization: Bearer shk_xxx" "http://localhost:8080/api/v1/categories"

# Platform stats
curl -H "Authorization: Bearer shk_xxx" "http://localhost:8080/api/v1/stats"
```

Full OpenAPI 3.0 spec: [`/api/v1/openapi.json`](http://localhost:8080/api/v1/openapi.json)

## Agent Skill

Skills Hub ships with a built-in agent skill at `skills/skills-hub/` for CLI and AI agent integration.

**Linux / macOS:**
```bash
./skills/skills-hub/skills-hub.sh search "browser automation"
./skills/skills-hub/skills-hub.sh install my-skill --dir ./skills
./skills/skills-hub/skills-hub.sh publish ./my-skill-dir
```

**Windows (PowerShell):**
```powershell
./skills/skills-hub/skills-hub.ps1 search "browser automation"
./skills/skills-hub/skills-hub.ps1 install my-skill --dir ./skills
```

Set `SKILLS_HUB_API_KEY` and optionally `SKILLS_HUB_BASE_URL` before use.

## Project Structure

```
.
├── db/                  # Database access, migrations, FTS, caching
├── handlers/            # HTTP handlers, middleware, auth, templates
├── models/              # Domain models
├── security/            # Markdown rendering and input sanitization
├── skills/              # Built-in agent skill (Bash + PowerShell)
├── static/              # Frontend assets (JS, CSS)
├── templates/           # HTML templates (18 pages)
├── Dockerfile           # Multi-stage production build
├── docker-compose.yml   # Local dev stack (app + Redis)
└── .github/workflows/   # CI pipeline
```

## Health Check

```bash
curl http://localhost:8080/healthz
# {"status":"ok","db":"ok","redis":"ok"}
```

## Development

```bash
# Run tests
go test ./...

# Build
go build ./...

# Format
gofmt -w .

# Lint (requires golangci-lint)
golangci-lint run
```

## On AI-Assisted Programming

Let's be honest: I'm mass-producing code with AI, and it's a weird position to be in.

On one hand, I genuinely worry that AI will make human programmers obsolete — including me. Every time I watch Cursor or Claude spit out a working feature in seconds that would have taken me an hour, a small part of my professional identity dies a little. That's a real anxiety I carry. I've spent years learning to code, and now a machine does it faster (and sometimes better, which hurts even more).

On the other hand, I absolutely love coding with AI. It's like having a tireless pair-programming partner who never judges your dumb questions and never needs a coffee break. I get more done in a weekend than I used to in a month. The dopamine hit of shipping features at this speed is genuinely addictive. I've become the person I used to mock — the one who says "it's not about writing code, it's about knowing what to build."

So yeah, the irony isn't lost on me: I'm using the very thing I fear to build the things I love. If AI does replace us all someday, at least I'll have had fun on the way out. And if it doesn't, well, I'll have shipped a lot of code.

This entire project was built with heavy AI assistance (Claude, mostly). The architecture decisions, the code, even parts of this README — AI had its fingerprints on all of it. I'm not ashamed of that. I'm a little scared of it. But mostly, I'm just grateful the tools exist.

## License

MIT
