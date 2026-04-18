# skills-oci

A CLI tool for packaging, pushing, and managing AI agent skills as OCI artifacts, following the [Agent Skills OCI Artifacts Specification](https://github.com/ThomasVitale/agents-skills-oci-artifacts-spec).

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for an interactive terminal experience.

## Installation

### Homebrew

```bash
brew install salaboy/tap/skills-oci
```

### Go install

```bash
go install github.com/salaboy/skills-oci@latest
```

### Build from source

```bash
git clone https://github.com/salaboy/skills-oci.git
cd skills-oci
go build -o skills-oci .
```

## What is a Skill?

A skill is a directory containing a `SKILL.md` file with YAML frontmatter that describes what the skill does, along with optional supporting files like scripts and references. Here is an example skill directory:

```
my-skill/
  SKILL.md
  scripts/
    create-pr.sh
  references/
    REFERENCE.md
```

The `SKILL.md` file uses YAML frontmatter to declare metadata:

```markdown
---
name: manage-pull-requests
version: 1.0.0
description: A skill for managing pull requests using the forgejo-cli.
license: Apache-2.0
compatibility: |
  Requires forgejo-cli.
  Agent must have network access to the Forgejo API.
metadata:
  category: development-tools
  tags: [git, forgejo, pull-requests, automation]
---

# Manage Pull Requests

Instructions and documentation for the skill go here...
```

## Packaging and Pushing Skills

The `push` command packages a skill directory into an OCI artifact and pushes it to a container registry. The CLI reads the `SKILL.md` frontmatter to build the artifact config and annotations automatically.

See [`examples/package-and-push/`](examples/package-and-push/) for a complete walkthrough using the popular `pdf` skill from [skills.sh](https://skills.sh/hot).

### Push to a registry

```bash
skills-oci push ghcr.io/myorg/skills/my-skill:1.0.0 ./my-skill
```

### Push to a local registry (plain HTTP)

```bash
skills-oci push localhost:5000/my-skill:1.0.0 ./my-skill --plain-http
```

If no tag is provided in `NAME[:TAG]`, the artifact is tagged as `latest`.

### What gets pushed

The CLI creates a standard OCI artifact with:

- **Config blob** (`application/vnd.agentskills.skill.config.v1+json`) — JSON metadata extracted from the SKILL.md frontmatter (name, version, description, license, compatibility, etc.)
- **Content layer** (`application/vnd.agentskills.skill.content.v1.tar+gzip`) — A deterministic tar.gz archive of the skill directory, rooted at `<skill-name>/`
- **Annotations** — Standard OCI annotations (`org.opencontainers.image.title`, `.version`, `.created`, `.licenses`) plus skill-specific ones (`io.agentskills.skill.name`)

The artifact is compatible with any OCI-compliant registry (GHCR, ECR, GAR, ACR, Docker Hub, Harbor, etc.).

## Installing Skills

The `add` command pulls a skill artifact from a registry, extracts it into `.agents/skills/`, creates symlinks in `.claude/skills/`, `.codex/skills/`, `.cursor/skills/`, and `.gemini/skills/`, and updates the project manifest files.

### Install a skill

```bash
skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0
```

### Install from a local registry

```bash
skills-oci add localhost:5000/my-skill:1.0.0 --plain-http
```

### Install to a custom directory

```bash
skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0 --output ./custom/skills
```

After installation, the skill is extracted and ready for use:

```
my-project/
  .agents/
    skills/
      manage-pull-requests/
        SKILL.md
        scripts/
          create-pr.sh
  skills.json
  skills.lock.json
```

## Managing Skills with skills.json

The CLI automatically manages two manifest files in your project directory:

### skills.json

A declarative manifest that declares which skills your project requires. It is created and updated automatically when you run `skills-oci add` or `skills-oci remove`.

```json
{
  "skills": [
    {
      "name": "manage-pull-requests",
      "source": "ghcr.io/myorg/skills/manage-pull-requests",
      "version": "1.0.0"
    },
    {
      "name": "go-pro-skills",
      "source": "ghcr.io/myorg/skills/go-pro-skills",
      "version": "2.0.0"
    }
  ]
}
```

Each entry contains:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill identifier used for local references |
| `source` | Yes | OCI repository reference (without tag or digest) |
| `version` | No | OCI tag to install (should follow semver) |

### skills.lock.json

A lock file that records the exact OCI digests and metadata of installed skills, ensuring reproducible installs across environments. This file should be committed to version control.

```json
{
  "lockfileVersion": 1,
  "generatedAt": "2026-04-02T08:11:09Z",
  "skills": [
    {
      "name": "manage-pull-requests",
      "path": ".agents/skills/manage-pull-requests",
      "source": {
        "registry": "ghcr.io",
        "repository": "myorg/skills/manage-pull-requests",
        "tag": "1.0.0",
        "digest": "sha256:bc6708cbbc37adb919157f04d31e601e68f4b9c24b35c655079da87ad0e30f86",
        "ref": "ghcr.io/myorg/skills/manage-pull-requests:1.0.0@sha256:bc6708cb..."
      },
      "installedAt": "2026-04-02T08:11:09Z"
    }
  ]
}
```

The lock file pins each skill to an immutable digest, so installations are reproducible regardless of whether mutable tags (like `latest` or `1.0`) have been updated.

### Removing a skill

```bash
skills-oci remove --name manage-pull-requests
```

This removes the skill from `skills.json`, `skills.lock.json`, and deletes the extracted directory.

## Using with Claude Code (Hook Integration)

`skills-oci` can be configured as a Claude Code `SessionStart` hook so that skills are automatically installed every time a Claude Code session starts. This means your project's skills are always present without any manual steps.

### How it works

1. **Declare skills** in `skills.json` at the root of your project.
2. **Register the hook** using `skills-oci register`. This writes a `SessionStart` hook into `.claude/settings.json` that runs `skills-oci install --plain` on every session start.
3. **Start Claude Code** — the hook fires, reads `skills.json`, and pulls any missing skills into `.claude/skills/`. Skills already present are skipped, so subsequent starts are fast.

```
Project start → Claude Code launches
                      │
                      ▼
              SessionStart hook fires
                      │
                      ▼
          skills-oci install --plain
                      │
                 reads skills.json
                      │
            ┌─────────┴──────────┐
            ▼                    ▼
     skill missing?        already present?
    pull from registry         skip
            │
     extract to .claude/skills/
```

### Setup

**Step 1 — Install skills-oci**

```bash
brew install salaboy/tap/skills-oci
```

**Step 2 — Declare your skills**

Create a `skills.json` in your project root (or add skills interactively via `skills-oci add <NAME[:TAG]>`):

```json
{
  "skills": [
    {
      "name": "manage-pull-requests",
      "source": "ghcr.io/salaboy/skills/manage-pull-requests",
      "version": "1.0.0"
    }
  ]
}
```

**Step 3 — Register the hook**

```bash
skills-oci register
```

This creates or updates `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/skills-oci install --plain",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

**Step 4 — Commit both files**

```bash
git add skills.json skills.lock.json .claude/settings.json
git commit -m "add skills-oci hook for Claude Code"
```

Any team member who clones the repo and opens Claude Code will automatically get the skills installed on their first session start.

### Example

See [`examples/claude-code-hooks/`](examples/claude-code-hooks/) for a minimal project showing the `skills.json` and the resulting `.claude/settings.json`.

### Updating skills

To add or update a skill, run `skills-oci add <NAME[:TAG]>` (or edit `skills.json` directly) and commit the updated manifest. The hook will install the new skill on the next session start.

To remove a skill:

```bash
skills-oci remove --name manage-pull-requests
```

## Interactive TUI

By default, the CLI runs with an interactive terminal UI that shows progress through each phase with spinners and styled output. To disable the TUI (for CI/CD pipelines or scripting), use the `--plain` flag:

```bash
skills-oci push ghcr.io/myorg/skills/my-skill:1.0.0 ./my-skill --plain
skills-oci add ghcr.io/myorg/skills/my-skill:1.0.0 --plain
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--plain` | Disable interactive TUI, use plain text output |
| `--plain-http` | Use HTTP instead of HTTPS for registry connections |

## Authentication

The CLI uses your existing Docker credentials from `~/.docker/config.json` and any configured credential helpers. Log in to your registry before pushing or pulling:

```bash
# GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Docker Hub
docker login

# AWS ECR
aws ecr get-login-password | docker login --username AWS --password-stdin <account>.dkr.ecr.<region>.amazonaws.com
```

## License

[Apache License 2.0](LICENSE)
