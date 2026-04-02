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

### Push to a registry

```bash
skills-oci push --ref ghcr.io/myorg/skills/my-skill --path ./my-skill --tag 1.0.0
```

### Push to a local registry (plain HTTP)

```bash
skills-oci push --ref localhost:5000/my-skill --path ./my-skill --tag 1.0.0 --plain-http
```

The `--path` flag defaults to the current directory if omitted. If `--tag` is not provided, the artifact is tagged as `latest`.

### What gets pushed

The CLI creates a standard OCI artifact with:

- **Config blob** (`application/vnd.agentskills.skill.config.v1+json`) — JSON metadata extracted from the SKILL.md frontmatter (name, version, description, license, compatibility, etc.)
- **Content layer** (`application/vnd.agentskills.skill.content.v1.tar+gzip`) — A deterministic tar.gz archive of the skill directory, rooted at `<skill-name>/`
- **Annotations** — Standard OCI annotations (`org.opencontainers.image.title`, `.version`, `.created`, `.licenses`) plus skill-specific ones (`io.agentskills.skill.name`)

The artifact is compatible with any OCI-compliant registry (GHCR, ECR, GAR, ACR, Docker Hub, Harbor, etc.).

## Installing Skills

The `add` command pulls a skill artifact from a registry, extracts it into `.agents/skills/` (or `.claude/skills/` with `--claude`), and updates the project manifest files.

### Install a skill

```bash
skills-oci add --ref ghcr.io/myorg/skills/my-skill:1.0.0
```

### Install from a local registry

```bash
skills-oci add --ref localhost:5000/my-skill:1.0.0 --plain-http
```

### Install to .claude/skills (for Claude Code projects)

```bash
skills-oci add --ref ghcr.io/myorg/skills/my-skill:1.0.0 --claude
```

### Install to a custom directory

```bash
skills-oci add --ref ghcr.io/myorg/skills/my-skill:1.0.0 --output ./custom/skills
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

Or with `--claude`:

```
my-project/
  .claude/
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

When using `--claude`, the `path` field reflects the `.claude/skills/` directory instead.

### Removing a skill

```bash
skills-oci remove --name manage-pull-requests
```

If the skill was installed with `--claude`, pass the flag again:

```bash
skills-oci remove --name manage-pull-requests --claude
```

This removes the skill from `skills.json`, `skills.lock.json`, and deletes the extracted directory.

## Interactive TUI

By default, the CLI runs with an interactive terminal UI that shows progress through each phase with spinners and styled output. To disable the TUI (for CI/CD pipelines or scripting), use the `--plain` flag:

```bash
skills-oci push --ref ghcr.io/myorg/skills/my-skill --path ./my-skill --tag 1.0.0 --plain
skills-oci add --ref ghcr.io/myorg/skills/my-skill:1.0.0 --plain
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--plain` | Disable interactive TUI, use plain text output |
| `--plain-http` | Use HTTP instead of HTTPS for registry connections |
| `--claude` | Use `.claude/skills` instead of `.agents/skills` as the skills directory |

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
