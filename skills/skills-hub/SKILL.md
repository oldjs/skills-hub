---
name: skills-hub
description: Connects to the Skills Hub marketplace to search, browse, download, and upload OpenClaw agent skills. Use this skill when users want to find skills by keyword or category, install a skill from the marketplace, publish their own skill, or check platform stats.
---

# Skills Hub

This skill connects to the Skills Hub marketplace, allowing you to search, browse, download, and publish OpenClaw agent skills on behalf of the user.

## When to Use This Skill

Use this skill when the user:

- Asks to find or search for a skill (e.g., "find a skill for web scraping", "search for GitHub skills")
- Wants to install/download a specific skill from the marketplace
- Wants to upload/publish a skill they built to the marketplace
- Asks what categories of skills are available
- Asks about platform stats (how many skills, users, etc.)
- Mentions Skills Hub by name
- Wants to explore what skills exist for a particular domain

## Prerequisites

This skill requires two environment variables:

- `SKILLS_HUB_API_KEY` — API Key generated from Skills Hub account settings (format: `shk_xxxx`)
- `SKILLS_HUB_BASE_URL` — (optional) Base URL of the Skills Hub instance, defaults to `https://skills-hub.example.com`

If `SKILLS_HUB_API_KEY` is not set, inform the user that they need to generate one from their Skills Hub account page (Account -> API Key Management).

## How to Help Users

### Step 1: Understand What They Need

When a user mentions skills or the marketplace, identify:

1. **The action** — Are they searching, installing, publishing, or browsing?
2. **The target** — What keyword, slug, category, or directory are they referring to?
3. **Any constraints** — Do they want a specific category, minimum rating, or sorting preference?

### Step 2: Search for Skills

If the user wants to find skills, run the search command:

```bash
./skills/skills-hub/skills-hub.sh search "query" [--category category] [--sort rating|newest]
```

Examples:
- User says "find me a browser automation skill" -> `./skills/skills-hub/skills-hub.sh search "browser automation"`
- User says "what AI skills are available?" -> `./skills/skills-hub/skills-hub.sh search "ai"`
- User says "show me the highest rated skills" -> `./skills/skills-hub/skills-hub.sh search "" --sort rating`

### Step 3: Get Details Before Installing

Before recommending a skill for installation, always check its details first:

```bash
./skills/skills-hub/skills-hub.sh info <slug>
```

Verify:
1. **Rating** — Prefer skills with rating >= 3.0
2. **Description** — Make sure it actually matches what the user needs
3. **Version** — Prefer skills with a version number (indicates maintenance)

Present the skill information to the user with: name, description, rating, version, and download count.

### Step 4: Install Skills

If the user confirms they want to install a skill:

```bash
./skills/skills-hub/skills-hub.sh install <slug> [--dir ./skills]
```

This downloads the skill ZIP and extracts it to the target directory. The default directory is `./skills`.

After installation, tell the user where the skill was installed and list the key files.

### Step 5: Publish Skills

If the user wants to publish a skill they've created:

```bash
./skills/skills-hub/skills-hub.sh publish <directory-path> [--tenant-id tenant_id]
```

Before publishing, verify:
1. The directory contains a `SKILL.md` file
2. The SKILL.md has proper frontmatter (name, description at minimum)

After publishing, inform the user of the slug and that the skill is pending review.

### Step 6: Browse Categories

To show the user what's available:

```bash
./skills/skills-hub/skills-hub.sh categories
```

### Step 7: Platform Stats

For general platform overview:

```bash
./skills/skills-hub/skills-hub.sh stats
```

## Command Reference

| Command | Description |
|---------|-------------|
| `skills-hub.sh search "query"` | Search skills by keyword, supports `--category`, `--sort`, `--page`, `--per-page` |
| `skills-hub.sh info <slug>` | Get full skill details including README |
| `skills-hub.sh install <slug>` | Download and extract skill to local directory, supports `--dir` |
| `skills-hub.sh publish <dir>` | Package and upload a skill directory, supports `--tenant-id` |
| `skills-hub.sh categories` | List all categories with skill counts |
| `skills-hub.sh stats` | Show platform-wide statistics |

## Common Skill Categories

When searching, consider these domains:

| Category | Example Queries |
|----------|----------------|
| browser | web scraping, browser automation, headless |
| agent | ai agent, assistant, chatbot |
| api | rest api, http, webhook |
| automation | workflow, scheduling, pipeline |
| code | code generation, refactoring, analysis |
| database | sql, nosql, migration |
| deploy | docker, kubernetes, ci-cd |

## When Things Go Wrong

- **API Key not set** — Tell the user to visit their Skills Hub account page and generate a key
- **Search returns no results** — Suggest broader keywords or different categories; offer to help build the skill from scratch
- **Download fails** — Check if the slug is correct; the skill may have been removed or not yet approved
- **Upload fails** — Verify the directory has SKILL.md; check file size is under 10MB
- **Network error** — Check if SKILLS_HUB_BASE_URL is reachable
