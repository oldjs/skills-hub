# P4 Development Roadmap

> Generated from P4 planning session. Implementation order and priorities are suggestions — adjust as needed.

---

## Batch 1: P0 — Must-Have (Fill Critical Gaps)

| # | Feature | Description | Priority | Effort |
|---|---------|-------------|----------|--------|
| 1 | **Notification Center Page** | Notification API exists (`/api/notifications`, `/api/notifications/read`, `/api/notifications/read-all`) but no frontend UI. The `#notifications` anchor in layout.html points to nothing. Add a notifications section to the account page with read/mark-all-read actions. | P0 | Small |
| 2 | **Custom Error Pages (404/500/403)** | All errors currently use bare `http.Error` text responses. Create unified error templates matching the existing visual style, with a "Back to Home" link. Register custom error handlers in `main.go`. | P0 | Small |
| 3 | **Health Check `/healthz`** | Check SQLite DB + Redis connectivity, return JSON status. Required for Docker/K8s deployment readiness. | P0 | Tiny |
| 4 | **Login/Register Brute-Force Protection** | `UserLogin`/`UserRegister` POST endpoints have no rate limiting (only the verification code sender does). Add IP + email dimension lockout mechanism, e.g. 5 failures → 15-minute lockout. | P0 | Small |

**Estimated total:** ~1 afternoon

---

## Batch 2: P1 — Important (UX + Observability + Tech Debt)

| # | Feature | Description | Priority | Effort |
|---|---------|-------------|----------|--------|
| 5 | **Public Skill Pages** | Currently all pages require `RequireAuth`, blocking crawlers and making SEO useless. Skill detail page + search page should allow anonymous browsing (rating/comment/upload still require login). | P1 | Medium |
| 6 | **Structured Logging** | Replace all `log.Printf` with Go 1.21 `slog`. Add `request_id`, duration, and status code per request. Generate tracing headers. | P1 | Medium |
| 7 | **Split `auth.go`** | Currently 688 lines mixing session, CSRF, middleware, and handlers. Refactor into `session.go`, `csrf.go`, `auth_handlers.go`. | P1 | Small |
| 8 | **HTTP Cache Headers** | Add `ETag` / `Last-Modified` to skill detail pages. Add short TTL `Cache-Control` to search results. CDN-friendly. | P1 | Small |
| 9 | **Skill Leaderboard** | Weekly/monthly leaderboard based on rating growth, downloads, and comment activity. Show Top 10 on homepage or a dedicated leaderboard page. | P1 | Medium |

---

## Batch 3: P2 — Enhancement (Testing + Performance + Operations)

| # | Feature | Description | Priority | Effort |
|---|---------|-------------|----------|--------|
| 10 | **Test Coverage** | Currently only 68 lines of tests. Cover at minimum: login flow, API v1 authentication, FTS search, rating/comment CRUD, email validation rules. | P2 | Large |
| 11 | **Load More / Infinite Scroll** | Homepage and search currently use traditional pagination. Add an optional "Load More" button for better mobile UX (keep pagination mode as alternative). | P2 | Medium |
| 12 | **Enhanced Admin Dashboard** | Add 7-day/30-day registration curves, skill growth trends, active user rankings. Use SQLite aggregate queries, render with Chart.js CDN. | P2 | Medium |
| 13 | **Database Query Optimization** | Homepage and search currently run 2 SQL queries per request (count + data). Cache popular query results in Redis for 60s. | P2 | Medium |
| 14 | **Local Static Assets** | Tailwind CDN + Font Awesome CDN + Google Fonts all loaded externally. Bundle into `/static/` to eliminate external dependencies and enable offline usage. | P2 | Medium |

---

## Batch 4: P3 — Nice-to-Have (Community + Operations Details)

| # | Feature | Description | Priority | Effort |
|---|---------|-------------|----------|--------|
| 15 | **Skill Collections/Lists** | Users can create collections like "My AI Toolbox", add multiple skills, and share publicly. Similar to GitHub Stars Lists. | P3 | Large |
| 16 | **Comment Voting** | Add "Helpful" / "Not Helpful" voting on comments. Pin highest-voted comments to top. | P3 | Medium |
| 17 | **Comment Sort Options** | Comments currently fixed in reverse chronological order. Add "Newest / Oldest / Most Popular" toggle. | P3 | Small |
| 18 | **Batch Review** | Skill review page should support multi-select for one-click batch approve/reject to reduce repetitive operations. | P3 | Small |
| 19 | **Action Log Export** | `admin_action_logs` should support CSV export for compliance and audit trail. | P3 | Small |
| 20 | **Email Template Management** | Verification code email template is currently hardcoded in Go. Move to admin-editable templates in the backend. | P3 | Medium |

---

## Suggested Implementation Order

```
Batch 1 (P0):     1 → 2 → 3 → 4              Fill critical gaps
Batch 2 (P1):     5 → 6 → 7 → 8 → 9          UX + observability + tech debt
Batch 3 (P2):     10 → 11 → 12 → 13 → 14     Testing + performance + operations
Batch 4 (P3):     15 → 16 → 17 → 18 → 19 → 20 Community + ops details
```

## Notes

- All features should maintain the existing Tailwind + vanilla JS stack (no framework migration).
- Follow the established patterns in `handlers/`, `db/`, and `templates/`.
- Each feature should be a separate commit with a clear message.
- Run `go vet ./...` and `go build` before each commit.
- Update `README.md` when user-facing features are added.
