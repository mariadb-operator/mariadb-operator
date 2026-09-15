---
name: mariadb-operator-release-notes
description: >
  Create the release notes and upgrade guide for a mariadb-operator release. Given the release PR (titled
  "Release <version>", head branch `release-<version>`) whose body lists every PR included in the release, it
  gathers each PR, groups the changes by relevance, and produces `docs/releases/RELEASE_<version>_HEADER.md.gotmpl`
  and `docs/releases/UPGRADE_<version>.md` in the format the previous releases use, then opens a PR targeting
  `release-<version>`. If no release PR is provided it asks for the new version and infers the changes from git
  history since the last tag. Use whenever the user wants release notes, an upgrade/update guide, or docs for a
  new mariadb-operator version — "create the release notes for 26.10.0", "write the upgrade guide", "document
  this release", "prepare the release PR docs" — even if they don't mention a release PR.
license: Apache-2.0
metadata:
  author: mariadb-operator
  version: "1.0"
compatibility: Requires GitHub API access (gh CLI or MCP tools) and the mariadb-operator repository checkout.
allowed-tools: Read, Grep, Glob, Write, Edit, Bash(git:*), Bash(gh:*)
---

# mariadb-operator Release Notes

Produce the two release documentation artifacts for a new version and deliver them as a PR against the release
branch:

- `docs/releases/RELEASE_<version>_HEADER.md.gotmpl` — the release notes header
- `docs/releases/UPGRADE_<version>.md` — the upgrade guide

## How release notes are built

`.github/workflows/release.yml` runs goreleaser on the release tag. It looks for
`docs/releases/RELEASE_${VERSION}_HEADER.md.gotmpl` (falling back to the generic
`RELEASE_HEADER.md.gotmpl`) and **prepends its rendered content to the auto-generated "What's Changed"
changelog**. Consequences:

- The filename must match the tag exactly: tag `26.10.0` → `RELEASE_26.10.0_HEADER.md.gotmpl`.
- The header is a **template**: use `{{ .ProjectName }}` for the project name, never hardcode it.
- Do **not** write a full commit/PR changelog in the header — goreleaser appends the complete one. The header
  carries the narrative: highlights grouped into sections, each item linking its PR.
- Verify the exact tag-to-file lookup in `.github/workflows/release.yml` before relying on it.

## GitHub credentials

Whenever this skill calls GitHub — fetching the release PR, its linked PRs, or opening the docs PR — pick the
access method in this order, falling through only when the previous one is unavailable:

1. **Project-scoped GitHub MCP tools** (names like `mcp__github-mariadb-operator__*`).
2. **`gh` CLI with the project-specific token**, if the MCP server isn't connected. Use
   `GITHUB_MARIADB_OPERATOR_TOKEN` explicitly (`GH_TOKEN="$GITHUB_MARIADB_OPERATOR_TOKEN" gh ...`) rather than
   the ambient `gh auth` session.
3. **Generic GitHub MCP tools** (`mcp__github__*`), if neither of the above is available.
4. **`gh` CLI with default credentials** (plain `gh auth`) as a last resort.

---

## Step 0 — Gather the input

**Preferred input: the release PR.** The user provides the release PR (titled `Release <version>`, head branch
`release-<version>`, base `main`). Its body is the ordering and scope authority: it lists every PR in the
release, typically grouped by merge status ("merged into main", "merged into this branch", "in review").

```bash
gh pr view <release-pr> --json title,body,headRefName,baseRefName
```

Parse the body into a list of PR links with their stated status. Respect explicit user directives verbatim, for
example: "include #X even though it hasn't been merged yet" (document it, and flag in the PR body that it must
merge before the release is cut) or "omit #Y for now" (drop it entirely — say so in the PR body).

**Fallback: no release PR provided.** Ask the user for the new version to release (e.g. `26.10.0`). Then infer
the change set from git history:

```bash
git fetch --tags origin main release-<version>
LAST_TAG=$(git describe --tags --abbrev=0 release-<version> 2>/dev/null || git describe --tags --abbrev=0 origin/main)
git log --oneline ${LAST_TAG}..origin/release-<version>                 # what changed
git log --merges --pretty='%h %s' ${LAST_TAG}..origin/release-<version> # merge commits → PRs
```

Map merge commits back to PR numbers (commit subjects and the `pull/` refs in commit bodies), and confirm the
`release-<version>` branch exists on the remote before proceeding. If the history is ambiguous (squashed
merges, rebases), say so and list the commits you could not attribute to a PR.

## Step 1 — Read the included PRs

For every PR in the release (excluding the ones the user told you to omit), fetch:

```bash
gh pr view <n> --json title,body,author,state,mergedAt
```

Classify each: **feature** (new capability, new spec field), **bugfix**, **improvement** (perf, tooling,
CI), **docs**, or **toolchain** (dependency/tool bumps). Note the author — community contributors are thanked
by name in the Community section.

Then determine the **data-plane impact**, which decides the upgrade guide content:

```bash
git diff --stat ${LAST_TAG}..origin/release-<version> -- \
  cmd/init cmd/agent pkg/controller/replication/config.go pkg/galera/config \
  pkg/environment pkg/builder/container_builder.go pkg/command
```

Any change here (agent/init behavior, rendered config, env vars, backup/restore CLIs, default images) means the
[data-plane](../../docs/data_plane.md) must be updated to the new version. Also check whether the release bumps
the default `MariaDB` image (`RELATED_IMAGE_MARIADB_VERSION` in the `Makefile`) — that belongs in the notes.

## Step 2 — Group into sections

Map the PRs into logical groups **sorted by relevance** (biggest user-facing features first). Typical
section lineup for this project — use only the ones that have content:

- **MariaDB <X.Y> support** — new default server version, compatibility changes
- **Replication topologies** — HA orchestration changes (switchovers, failovers, semi-sync, GTID handling, `read_only`)
- **<New feature> compression** / **Backups** — backup/restore/PITR features
- **Galera improvements** — clustering changes
- **Bugfixes** — user-visible fixes
- **Improvements** — observability, docs, CI, toolchain

Every item is one bullet naming the concrete change, why it matters, and a PR link:
`- Fixed X that could Y ([#1234](https://github.com/mariadb-operator/mariadb-operator/pull/1234))`.
Stop at the change: one or two sentences, no forensics.

## Step 3 — Write the release notes header

Write `docs/releases/RELEASE_<version>_HEADER.md.gotmpl`, following the most recent version's header as the
template (read `docs/releases/RELEASE_<previous>_HEADER.md.gotmpl` first). Structure:

```markdown
**`{{ .ProjectName }}` [<zero-padded short version>](https://github.com/mariadb-operator/mariadb-operator/releases/tag/<version>) is here!** 🦭

<enthusiastic open-source intro; highlight any milestones the user provides, e.g. star count, Docker pulls —
never invent numbers>
<community contributions thank-you paragraph, as in previous releases>

If you're upgrading from previous versions, __do not miss the [UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_<version>.md)__ for a smooth transition.

## <feature section>
...

## Bugfixes
...

## Improvements
...

---

## Community
<same adopters/stars paragraph as previous releases>

## Enterprise
<same Enterprise Operator paragraph as previous releases>
```

Formatting rules (these are the review corrections — apply them up front):

- **Version forms differ by context**: the title link text is zero-padded (`26.10` for `26.10.0`), the
  `releases/tag/` link is not. Keep the two forms consistent with the previous release's header.
- Inline mentions of docs use **relative links** (`./replication.md`); "Refer to the ... docs" lines use
  **absolute `blob/main` links**.
- New spec fields: verify the exact field name and enum values against `api/v1alpha1/` on the release branch
  (or the PR diff when the PR is not merged yet) before writing them — wrong field names in release notes
  ship to every reader.
- A YAML example may accompany a headline feature, mirroring the style of the previous header.
- Do not document omitted PRs; do flag (in the delivery PR body, not the notes) any documented-but-unmerged PR.

## Step 4 — Write the upgrade guide

Write `docs/releases/UPGRADE_<version>.md`, copying the previous guide's structure:

```markdown
# <zero-padded short version> update guide

This guide illustrates, step by step, how to update to `<version>` from previous versions. This guide only
applies if you are updating from a version prior to `<zero-padded>x`, otherwise you may upgrade directly
(see [Helm](../helm.md#updates))

> [!TIP]  (OCI-based installation — same block as previous guides)
> [!CAUTION]  (mariadb-operator-crds in-place upgrade — same block as previous guides)

- The [data-plane](../data_plane.md) must be updated ... `updateStrategy.autoUpdateDataPlane=true` diff block
- Upgrade `mariadb-operator-crds` then `mariadb-operator` helm chart to `<version>` (bash blocks)
- Consider reverting `updateStrategy.autoUpdateDataPlane` back to `false` (diff block)
```

- Include the data-plane step when Step 1's data-plane check found changes; keep the previous guide's exact
  wording otherwise.
- Add release-specific `> [!CAUTION]` / `> [!TIP]` blocks only when the release contains a migration hazard
  (breaking default change, deprecated mechanism, required data-plane feature).
- Helm chart versions in commands are **not** padded (`--version 26.10.0`).

## Step 5 — Verify before pushing

```bash
# filenames match the tag exactly (release.yml lookup)
ls docs/releases/RELEASE_<version>_HEADER.md.gotmpl docs/releases/UPGRADE_<version>.md

# template variables and links are sane
grep -n "{{ .ProjectName }}" docs/releases/RELEASE_<version>_HEADER.md.gotmpl
grep -n "UPGRADE_<version>.md" docs/releases/RELEASE_<version>_HEADER.md.gotmpl
# relative doc links resolve to real files in docs/
grep -o '](\./[a-z_]*\.md' docs/releases/RELEASE_<version>_HEADER.md.gotmpl | sort -u
```

Re-read both files end to end: every PR link must be a PR from the release list, every version string must use
the right form for its context, and the upgrade guide must be applicable to users of the previous release.

## Step 6 — Deliver as a PR

- Branch `feature-release-notes-<version>` from `release-<version>`.
- Commit both files: "Add release notes and upgrade guide for <version>".
- Push and open a PR **targeting `release-<version>`**. The PR body must include
  `Closes MDB-<issue-number>` linking the tracking issue when one exists, and it must list the judgement calls:
  which PRs were included despite being unmerged, which were omitted, and the reasoning behind the
  data-plane requirement.
- Wait for human review before merging — never self-merge release docs.

## Gotchas

- **The release notes can describe unmerged work.** That is normal for a draft docs PR: items are documented
  while their PRs are still open against `release-<version>`. The docs must not ship before the code merges —
  always flag it in the delivery PR body.
- **The generated changelog already lists every PR.** If the user wants a PR mentioned, it belongs in the
  header's grouped sections; do not add a third changelog section to the header.
- **Backport releases exist** (e.g. `release-26.6.1`). The "update to `<version>` from a version prior to
  `<major.minor>.x`" line must match the actual minor series of the release being documented.
- **Never invent milestone numbers.** Stars, pulls, adopters: only what the user provided or that is verifiable
  on the repository/package pages at release time.
