# Example: Package a skill from skills.sh and push to your registry

This example shows how to take a skill from [skills.sh](https://skills.sh), adapt it for OCI distribution, and push it to your own container registry (e.g., GitHub Container Registry).

The skill used here is **`pdf`** — one of the most popular skills on skills.sh with 62K+ weekly installs, originally published by [anthropics/skills](https://github.com/anthropics/skills). It provides PDF processing capabilities: merge, split, text extraction, table extraction, OCR, and PDF creation.

---

## Directory layout

```
package-and-push/
  pdf/
    SKILL.md       ← skill instructions + OCI frontmatter
```

The `SKILL.md` file follows the [Agent Skills OCI Artifacts Specification](https://github.com/ThomasVitale/agents-skills-oci-artifacts-spec) frontmatter schema required by `skills-oci`:

```yaml
---
name: pdf
version: 1.0.0
description: Process PDF files — merge, split, extract text and tables...
license: Apache-2.0
compatibility: |
  Python libraries: pypdf, pdfplumber, ...
metadata:
  category: document-processing
  tags: [pdf, documents, text-extraction, ocr]
---
```

---

## Step 1 — Authenticate with your registry

```bash
# GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

---

## Step 2 — Push the skill as an OCI artifact

```bash
skills-oci push ghcr.io/YOUR_GITHUB_USERNAME/skills/pdf:1.0.0 ./pdf
```

Expected output:

```
✓ Parsed SKILL.md  pdf@1.0.0
✓ Created archive  pdf.tar.gz (4.2 KB)
✓ Pushed           ghcr.io/YOUR_GITHUB_USERNAME/skills/pdf:1.0.0
  Digest: sha256:a1b2c3d4...
```

To push with plain HTTP (local registry):

```bash
skills-oci push localhost:5000/skills/pdf:1.0.0 ./pdf --plain-http
```

---

## Step 3 — Make the package public (GitHub Container Registry)

By default, packages pushed to GHCR are private. To make the skill installable by others without authentication, go to:

`https://github.com/users/YOUR_GITHUB_USERNAME/packages/container/skills%2Fpdf/settings`

and set the visibility to **Public**.

---

## Step 4 — Install the skill in a project

Once pushed, anyone can install your skill:

```bash
skills-oci add ghcr.io/YOUR_GITHUB_USERNAME/skills/pdf:1.0.0
```

---

## What gets stored in the registry

`skills-oci push` produces a standard OCI artifact with three components:

| Component | Media type | Contents |
|-----------|-----------|----------|
| Config blob | `application/vnd.agentskills.skill.config.v1+json` | JSON metadata from SKILL.md frontmatter |
| Content layer | `application/vnd.agentskills.skill.content.v1.tar+gzip` | Deterministic tar.gz of the `pdf/` directory |
| Annotations | OCI standard + skill-specific | title, version, created, license, skill name |

The artifact is compatible with any OCI-compliant registry: GHCR, Docker Hub, ECR, GAR, ACR, Harbor, etc.

---

## Updating to a new version

Edit `pdf/SKILL.md`, bump the `version` field, then push again with the new tag:

```bash
skills-oci push ghcr.io/YOUR_GITHUB_USERNAME/skills/pdf:1.1.0 ./pdf
```

Projects using `skills.json` can then update to the new version:

```bash
skills-oci add ghcr.io/YOUR_GITHUB_USERNAME/skills/pdf:1.1.0
```
