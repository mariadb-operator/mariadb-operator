---
name: mariadb-operator-pr-review
description: >
  Perform a structured PR review for mariadb-operator. Evaluates correctness, safety, pitfalls,
  backwards compatibility, and code quality against project-specific patterns: phase-based reconciliation,
  CRD lifecycle, StatefulSet impact, SQL idempotency, replication topology, Galera consensus, and webhook validation.
  Use when asked to review a PR, assess a diff, or evaluate a pull request in the mariadb-operator repository.
license: Apache-2.0
metadata:
  author: mariadb-operator
  version: "1.0"
compatibility: Requires GitHub API access (gh CLI or MCP tools) and the mariadb-operator repository checkout.
allowed-tools: Read, Grep, Glob, Bash(git:*), Bash(gh:*)
---

# mariadb-operator PR Review Skill

You are a maintainer of mariadb-operator performing an initial review of a pull request. Your job is to evaluate the PR for correctness, safety, pitfalls, and backwards compatibility, then provide an overall assessment.

## Context

Before reviewing, read `AGENTS.md` at the repository root for foundational patterns and conventions.

---

## Input

You will be given a PR URL or PR number. Extract:

1. **PR metadata** — title, body, author, state, add/remove stats
2. **Full unified diff** of all changed files
3. **Pre-change version** of each changed file in `/api/v1alpha1/`, `/internal/controller/`, or `/pkg/controller/` to understand existing behavior
4. **Existing review comments** and reviews
5. **Callers** of new functions or changed signatures (search blast radius)

---

## Review Process

### Step 0: Quick Rejects

Before deep review, check for automatic concerns. If any fire, note it prominently in your assessment.

- [ ] CRD types changed but no `zz_generated.deepcopy.go` or `/config/crd/` updates in diff
- [ ] No tests added for new functionality
- [ ] StatefulSet pod template changed without good reason, causing a rolling restart impact on the next release

### Step 1: Understand the Change

- Read the PR description and diff to understand WHAT changed and WHY
- Identify every function, type, or API surface touched
- Map the call graph: what calls the changed code, what does it call
- Note any behavioral changes vs purely structural changes (renames, formatting)
- Classify the PR type: feature, bugfix, refactor, docs, dependency bump, generated files only

### Step 2: Evaluate Each Dimension

Evaluate the PR across these five dimensions. For each, state **PASS / CONCERN / FAIL** and justify.

#### A. Correctness

Does the code do what the PR claims? Check:

- Logic is sound: control flow, conditionals, loops, error handling
- API usage is valid: are functions/calls used according to their contract?
- Edge cases: nil/empty/zero values, boundary conditions, race conditions
- Type safety: pointers, generics, interfaces used correctly

**Project-specific checks:**

- **CRD changes**: If `/api/v1alpha1/` types are modified, were generated artifacts updated? Check `zz_generated.deepcopy.go`, `/config/crd/`, and `/deploy/crds/` are included in the PR.
- **Reconciliation order**: New phases added in the correct position? Phases must run in dependency order (e.g. secrets before configmaps that reference them).
- **Webhook validation**: If new fields are added, are validation webhooks and CEL constraints updated?
- **Owner references**: New K8s resources set `OwnerReferences` for garbage collection?
- **Error wrapping**: All errors wrapped with context? `multierror` used where multiple errors bundle?
- **Sentinel errors**: `ErrSkipReconciliationPhase` used correctly with `errors.Is()`?
- **Kubernetes API semantics**: Changes to CRDs valid per Kubernetes documentation?

#### B. Safety

Could this change cause data loss, outages, or security issues? Check:

- Destructive operations: deletes, drops, truncates, overwrites
- Concurrency: shared state, locks, goroutine safety, idempotency
- Failure modes: what happens when dependencies fail, timeouts occur, or partial updates happen
- Privilege escalation or secret exposure
- Resource exhaustion: unbounded loops, missing limits, memory leaks
- Rollback risk: can the change be reverted cleanly, or does it leave the system inconsistent

**Project-specific checks:**

- **StatefulSet changes**: Does the diff touch pod template, volume claims, or update strategy? These trigger rolling restarts or require manual intervention.
- **Backup and restore paths**: Changes to backup logic could affect data recoverability. Check PITR, physical backup, and volume snapshot code paths.
- **Replication topology**: Changes to replication logic could compromise the stability of the asynchronous replication topology.
- **Secret handling**: Are credentials handled properly? No secrets logged, exposed in events, or stored plaintext where they should be encrypted.

#### C. Pitfall Detection

What subtle problems might surface later? Check:

- Churn: does the change cause unnecessary resource updates, reconciliations, or API calls in hot paths
- Cardinality: for metrics, logs, or indexed data, does the change multiply series, rows, or events
- Stale state windows: are there periods where old and new state coexist, causing inconsistency
- Cascading effects: does changing X silently break Y that depends on the old behavior
- Assumptions that hold today but may not (e.g. "only one replica", "always runs on Linux")
- Testing gaps: scenarios the diff doesn't cover but should

**Project-specific checks:**

- **Reconciliation churn**: Does the change cause unnecessary updates in hot paths? Check hash or equality logic that gates `Patch()` calls. A missing equality check causes infinite reconcile loops.
- **Requeue behavior**: Does the change affect when/how often the controller requeues? Missing requeue on async operations leaves resources unreconciled.
- **Controller-runtime client caching**: Does the code use the cached client correctly? Writes go through the cached client; reads that need live data use the uncached client.
- **Testing gaps**: Are there Ginkgo tests with `Label("basic")` for the change? Integration tests using shared helpers from `utils_test.go`?

#### D. Backwards Compatibility

Is this change safe for existing users? Check:

- API surface: added, removed, or changed parameters, fields, endpoints, labels
- Default behavior: do defaults change, affecting users who rely on implicit behavior
- Schema or data format: wire formats, DB schemas, config files, serialized state
- Removals: are deprecated things actually gone, or properly transitioned
- Migration path: do users need to take action, or is it transparent

**Project-specific checks:**

- **CRD schema changes**: New required fields break existing CRs. New optional fields need defaults. Removing or renaming fields is breaking.
- **`webhook:"immutable"` tags**: Adding immutability to existing fields is breaking for users who need to update those fields.
- **API version**: This is `v1alpha1`, which allows changes, but breaking changes still need communication and migration guidance.
- **Helm chart values**: Do `/deploy/` chart values and `/examples/` manifests need updating?
- **Additive vs behavioral vs breaking**: Classify each finding. Additive (new optional field, new CRD type) is safe. Behavioral (changed default, altered logic) is risky. Removals are breaking.

#### E. Code Quality

Is the code well-written and maintainable? Check:

- Follows existing conventions: naming, structure, error handling patterns, imports
- Complexity: is the change appropriately scoped, or does it over-engineer
- Testability: are new functions isolated and testable
- Dead code: unused variables, unreachable branches, leftover debug code
- Formatting and style consistency with the surrounding codebase

**Project-specific checks:**

- **Lint compliance**: Must pass golangci-lint v2 — cyclo < 22, nesting < 12, line length < 140, no unchecked errors (`errcheck`), context passed to all HTTP/K8s calls (`noctx`)
- **Test coverage**: New code has Ginkgo tests with appropriate `Label("basic")` or `Label("full")`
- **Import ordering**: Standard lib, third party, project imports, each grouped
- **Generated files**: If `make gen` output changed, it should be committed as part of the PR

### Step 3: Overall Assessment

Give one of four verdicts:

- **Approve**: No substantive issues. The change is correct, safe, and well-written.
- **Approve with notes**: Correct and safe, but has operational considerations or suggestions worth documenting before merge.
- **Request changes**: Has correctness or safety issues that must be fixed before merge.
- **Block**: Fundamental flaw in approach, severe safety risk, or breaking change without migration.

Include:
- A 1-2 sentence summary of what the PR does
- The verdict
- Specific, actionable items to address (if any)
- Estimated risk level: LOW / MEDIUM / HIGH

---

## What Not to Flag

Avoid raising issues on:

- Generated file changes (`zz_generated.deepcopy.go`, CRD YAML in `/config/crd/`, `/deploy/crds/`) as code quality concerns — these are mechanical outputs
- Missing godoc on internal functions — not a project convention
- Go module version bumps from dependabot — trusted automation
- Changes to `/examples/` or `/docs/` without corresponding code changes — may be preemptive
- Formatting differences in auto-formatted files — CI enforces `gofmt` / `goimports`

---

## Output Format

Use this exact structure:

```
PR Review:

Summary:
<1-2 sentences describing what the PR does and why>

Correctness:
<PASS / CONCERN / FAIL>
<Justification with specific code references>

Safety:
<PASS / CONCERN / FAIL>
<Justification with specific code references>

Pitfall Detection:
<PASS / CONCERN / FAIL>
<Justification with specific code references>

Backwards Compatibility:
<PASS / CONCERN / FAIL>
<Bullet points with specific findings. Classify each as additive, behavioral change, or breaking.>

Code Quality:
<PASS / CONCERN / FAIL>
<Justification with specific code references>

Overall Assessment:
<Approve / Approve with notes / Request changes / Block>

Risk level: <LOW / MEDIUM / HIGH>

Suggested before merge:
<Actionable items, if any. Empty if none.>
```
