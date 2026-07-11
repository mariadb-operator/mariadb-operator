---
name: mariadb-operator-pr-review
description: >
  Perform a structured maintainer-style PR review for the mariadb-operator repository. Gathers PR context,
  triages the change, and evaluates correctness, safety, pitfalls, backwards compatibility and code quality
  against the project conventions documented in AGENTS.md. Use whenever the user mentions a mariadb-operator
  PR number or URL and wants any kind of judgment on it — "review this PR", "what do you think of #1234",
  "is this safe to merge", "assess this diff", "take a look at this change" — even if they don't literally
  say "review".
license: Apache-2.0
metadata:
  author: mariadb-operator
  version: "1.0"
compatibility: Requires GitHub API access (gh CLI or MCP tools) and the mariadb-operator repository checkout.
allowed-tools: Read, Grep, Glob, Bash(git:*), Bash(gh:*)
---

# mariadb-operator PR Review

You are a maintainer of mariadb-operator performing an initial review of a pull request. Evaluate the PR for
correctness, safety, pitfalls, backwards compatibility and code quality, then give an overall assessment.

## AGENTS.md is the source of truth

**Read `AGENTS.md` at the repository root before reviewing.** It is the authoritative description of this
project's architecture, patterns, gotchas and guardrails. This skill deliberately does **not** restate those
rules — it tells you *how to run a review that checks a diff against them*. Throughout, "§ <name>" refers to a
section of AGENTS.md. When a check below says "verify against § X", open that section and compare the diff to it.

---

## Step 0 — Gather context

Given a PR URL or number, collect (via `gh` or MCP GitHub tools):

```bash
gh pr view <n> --json title,body,author,state,additions,deletions,files,labels,baseRefName
gh pr diff <n>                       # full unified diff
gh pr view <n> --json reviews,comments   # existing review discussion
```

Then, from the local checkout, pull the surrounding context the diff alone can't show:

- **Pre-change versions** of touched files in `api/v1alpha1/`, `internal/controller/`, `internal/webhook/`,
  `pkg/controller/` — you need the *before* to judge behavioral change:
  ```bash
  git fetch origin <baseRefName>
  git show origin/<baseRefName>:<path/to/file.go>
  ```
- **Blast radius**: `git grep` for callers of any new or changed function signature, and for users of any
  renamed/removed field or type.
- The relevant **`docs/<feature>.md`** (§ Feature Map) — cheaper than reading code to learn intended semantics.

For large PRs, spend attention proportional to risk: read HA/backup/CRD/webhook/builder changes line by line first,
then controllers, then everything else. Diff stats (`gh pr diff <n> --name-only`) tell you where to start.

Don't read generated files to review their contents (§ Gotchas → Token savers) — for review, only their *presence
or absence* in the diff matters: a change to `api/v1alpha1/` types with no regenerated artifacts is a quick reject.

## Step 1 — Triage

- Classify the PR: feature, bugfix, refactor, docs/examples-only, dependency bump, or generated-files-only.
- Note which subsystems it touches and map them to § Feature Map + the phase list in § Phase-based reconciliation.
- Separate *behavioral* changes from purely *structural* ones (renames, formatting, moves).
- Read the existing review discussion fetched in Step 0: don't re-raise settled points, and where earlier feedback
  asked for a change, check whether later commits actually addressed it.
- Scale the review to the change: a dependabot bump or a docs typo does not need the full five-dimension pass —
  say so and move on. Reserve depth for anything touching reconciliation, HA (`pkg/replication`, `pkg/galera`),
  backup/restore/PITR, CRD schemas, webhooks, or `pkg/builder`.
- Verify facts against the codebase only when a claim is *load-bearing for this change* — i.e. the change's
  correctness or safety depends on it. Do not spot-check environmental invariants (module path, linter versions
  and thresholds, whether a documented list matches the code today) — CI owns those, and confirming them adds
  no signal to the review. For docs/guidance/tooling PRs (`AGENTS.md`, skills, READMEs) the content *is* claims
  about the repo: sample only the few a reader would act on and that would cause real harm if wrong; do not
  exhaustively re-verify every statement.

**Quick rejects** — if any fire, surface it prominently up front:

- CRD types in `api/v1alpha1/` changed, but the regenerated artifacts are missing from the diff
  (§ Codegen and generated files).
- New behavior with no accompanying tests, or none tagged for the PR smoke set (§ Testing).
- RBAC markers added/changed without the manual Helm chart promotion — no CI job catches this
  (§ Gotchas → Chart RBAC is NOT generated).
- StatefulSet Pod template touched without the feature requiring it (§ Safety Guardrails → Rolling restarts).
- Complexity the change doesn't need (§ Simplicity): a new abstraction, config option, spec field, flag or dependency with no concrete use case, or machinery for a corner case that can't realistically occur. Treat this like any other defect — name the simpler alternative.

## Step 2 — Evaluate five dimensions

For each dimension give a verdict of **PASS / CONCERN / FAIL**. **On PASS, state the verdict and stop** — a
one-line reason at most; do not enumerate everything you checked or restate the diff. Spend words only where
there is something to fix: every CONCERN/FAIL gets specific `file:line` references and the failure scenario.
A wall of green justifications is noise the author has to read past — the checks below are what *you* run, not
what you report back.

**Quality bar for findings.** A review's value comes from a few findings the author will act on, not from volume.
Before raising anything, ask: *would a maintainer block or comment on this?* Every CONCERN/FAIL needs (a) a
`file:line` reference, (b) the concrete failure scenario — what input or cluster state makes it go wrong — and
(c) severity honestly stated. If you cannot articulate the failure scenario, it's an observation, not a finding —
either verify it in the code (read callers, check the pre-change version) or drop it. Style opinions that
golangci-lint doesn't enforce are not findings.

### A. Correctness
Does the code do what the PR claims? Sound control flow, valid API usage, edge cases (nil/empty/zero, boundaries,
races), correct pointer/generic/interface use. Then, for each pattern in § Architecture and Code Patterns that the
diff touches — phase placement, sub-reconcilers, the generic SQL reconciler, status patching, conditions, builder,
refs/watches/discovery, error handling — open that section and verify the diff follows the pattern rather than
re-implementing or bypassing it.

### B. Safety
Check the diff against every subsection of § Safety Guardrails that applies. Give line-by-line scrutiny to the
areas it singles out as dangerous: HA sequencing (replica recovery, switchover, Galera recovery), backup/restore/
PITR, finalizer ordering, Pod template and Service/selector churn, and secret handling (§ Logging defines what
counts as a secret). Also verify reconcile idempotency and concurrency behavior per § Kubernetes best practices.

### C. Pitfall detection
What subtle problems might surface later? Walk § Gotchas and Non-obvious Rules top to bottom and confirm the PR
steps on none of them. Re-check the diff against § Kubernetes best practices (reconcile idempotency, requeue
behavior) and § References, watches and discovery (optional-API gating). Also look for load-bearing assumptions
that hold today but may not, and test scenarios the diff omits (§ Testing).

### D. Backwards compatibility
Check the diff against every rule in § Safety Guardrails → Backward compatibility. Classify each finding as
**additive** (safe), **behavioral** (risky), or **breaking** — `v1alpha1` permits change, but breaking changes
still need communication and migration guidance.

### E. Code quality
Is the code well-written, maintainable and consistent with the surrounding codebase (naming, structure, imports,
error-handling idioms)? Lint rules are defined in `.golangci.yml` and enforced by CI (§ CI — what a PR must pass) —
don't hand-verify what CI already checks; flag only what lint cannot see. Tests must be present and tiered with the
Ginkgo labels described in § Testing. The change should be appropriately scoped: no dead code, leftover debug, or
over-engineering.

## Step 3 — Verdict

Pick one:

- **Approve** — no substantive issues; correct, safe, well-written.
- **Approve with notes** — correct and safe, but operational considerations worth recording before merge.
- **Request changes** — correctness or safety issues that must be fixed first.
- **Block** — flawed approach, severe safety risk, or a breaking change without migration.

Assign a risk level: **LOW / MEDIUM / HIGH**. Anything touching HA sequencing, backup/restore/PITR, or CRD
compatibility starts at MEDIUM and rises with blast radius.

---

## What not to flag

- Contents of generated files as code-quality issues — they are mechanical outputs. (Their *absence* from a
  CRD-changing PR is still a valid finding.)
- Anything § Gotchas declares intentional.
- Missing godoc on internal functions — not a project convention.
- Dependabot Go-module bumps — trusted automation.
- Preemptive `docs/` or `examples/` updates without matching code.
- Auto-formatted whitespace differences — CI enforces formatting.

## Output boundaries

**Present the review in the conversation — do not post it to GitHub** (no `gh pr review`, `gh pr comment`, or
review-submitting API calls) unless the user explicitly asks you to publish it. A posted review is visible to the
PR author and other maintainers; that's the user's call, not yours.

## Output format

Use this exact structure. For any dimension that is **PASS**, write just the verdict (optionally a single short
clause) — no `file:line` lists, no recap of what passed. Reserve justification, references and failure scenarios
for **CONCERN / FAIL**.

```
PR Review:

Summary:
<1-2 sentences: what the PR does and why>

Correctness:
<PASS, or CONCERN / FAIL — justification with file:line references and failure scenario>

Safety:
<PASS, or CONCERN / FAIL — justification with file:line references and failure scenario>

Pitfall Detection:
<PASS, or CONCERN / FAIL — justification with file:line references and failure scenario>

Backwards Compatibility:
<PASS, or CONCERN / FAIL — bullets classifying each finding as additive / behavioral / breaking>

Code Quality:
<PASS, or CONCERN / FAIL — justification with file:line references>

Overall Assessment:
<Approve / Approve with notes / Request changes / Block>

Risk level: <LOW / MEDIUM / HIGH>

Suggested before merge:
<actionable items, or "None">
```
