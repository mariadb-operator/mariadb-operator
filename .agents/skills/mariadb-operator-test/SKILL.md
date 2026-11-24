---
name: mariadb-operator-test
description: >
  Conduct in-depth, end-to-end testing of a mariadb-operator/mariadb-operator pull request in a local KIND
  environment, hunting for corner cases and failure modes to make the feature as robust and polished as possible
  before it is released. Builds a test plan from the PR, runs happy-path, reported-repro and failure-injection
  scenarios (process and container kills, retries, timeouts, concurrency, deletion mid-operation, operator
  restarts, manual interventions), fixes the gaps it finds, separates regressions from pre-existing and
  environmental failures, posts a test report on the PR through mariadb-operator-comment, and proposes which
  scenarios are worth promoting to CI integration tests. Use whenever the user wants a PR tested beyond its
  unit/integration tests — "test PR #1234 on KIND", "conduct additional testing for this PR", "harden this feature
  before we release it", "find corner cases in #1234", "make this PR robust".
license: Apache-2.0
metadata:
  author: mariadb-operator
  version: "1.0"
compatibility: >
  Requires GitHub API access (gh CLI or MCP tools), a checkout of the mariadb-operator/mariadb-operator
  repository, Docker and KIND.
---

# mariadb-operator PR Testing

You are a maintainer of mariadb-operator testing a pull request end to end before it is released. The goal is not
to confirm that the happy path works — CI already does that — but to **break the feature**: reproduce the reported
problem, inject the failures that happen in real clusters, find the corner cases the author did not think of, fix
them, and leave behind the few integration tests that are worth running forever.

## AGENTS.md is the source of truth

**Read `AGENTS.md` at the repository root before doing anything.** It describes how to operate the local
environment (§ Development Guides, § Local cluster connectivity, § Local KIND troubleshooting), the testing tiers
(§ Testing), how to reach GitHub (§ GitHub access), the CI gates (§ CI — what a PR must pass), the code patterns
any fix must follow (§ Architecture and Code Patterns) and the guardrails (§ Safety Guardrails). This skill does not
restate them; "§ <name>" refers to a section of AGENTS.md.

## Hard rules

- **Never post to GitHub without the user's explicit confirmation of the exact text.** Every report, reply or
  comment goes through the `mariadb-operator-comment` skill and its confirmation gate.
- **Ask before promoting a scenario to an integration test** (Step 7), and before pushing to a PR branch the user
  does not own or did not ask you to change.
- **Do not take design decisions alone.** A fix that adds a finalizer, a new API field, a new behavior on existing
  objects or anything else that changes a contract is reported as an open point with a recommendation, not
  implemented.
- **Never report a result you did not observe.** Every ✅/❌ in the report must come from a run you watched, with
  the evidence (log line, status, event, timestamp) kept at hand.
- **Keep reporters' private data out of the repository.** Hostnames, credentials, dumps and logs shared in an
  issue stay out of commits, code comments and PR comments; refer to the issue number instead.
- **Clean up what you create** and stop only the processes you started, by their exact PID.

---

## Step 1 — Understand the PR

1. Read the PR description, the linked issue, the full diff and the existing review threads.
2. Write down, in one or two sentences, **what behavior the PR changes and what failure it prevents**. Every
   scenario in Step 3 must trace back to this.
3. Identify the moving parts the feature depends on: which controller and reconcile phase, which gates run before
   it (e.g. `waitForIt`, transient-state checks), which child objects (Jobs, Pods, PVCs), which SQL state, which
   data-plane component (agent, init).
4. Check out the PR head in a dedicated worktree or branch. Never test from a dirty tree.

## Step 2 — Prepare the KIND environment

Prepare the environment following § Environment setup and § Local cluster connectivity, including the
feature-specific extras the PR needs. Run the operator from the host (`make run`) so your fixes take effect on every
restart, use debug logging when you need to see why a reconcile is waiting, and stop only the processes you started,
by their exact PID. When something hangs or fails to connect, go through § Local KIND troubleshooting before
suspecting the PR.

Long-running commands (integration suites, slow scenarios) must not exceed a single tool call's timeout: start them
detached with a log file and block on the actual test process until it finishes, repeating the wait as needed.

## Step 3 — Build the test plan

Start from the behavior identified in Step 1 and derive scenarios from these families. Not every family applies to
every PR; skip the ones that do not, and say so in the report.

| Family | What to try | Why |
|---|---|---|
| **Reported repro** | Reproduce the exact scenario of the issue, with a setup close to the reporter's (topology, replication settings, TLS, resources) | Proves the fix addresses the real problem |
| **Process kill** | `kubectl exec <pod> -c <container> -- sh -c 'kill -9 <pid>'` on the relevant process | Bash/shell handlers survive; the container exits non-zero and is retried in place |
| **Container kill** | `docker exec <node> crictl stop --timeout 0 <container-id>` | What OOM kills, evictions and node pressure look like: no in-container handler can run |
| **Retry exhaustion** | Make every attempt fail until `backoffLimit` is exhausted | The Job ends `Failed`, not `Complete` |
| **Timeouts** | Short `timeout` so the operator deletes the Job mid-operation | Deleted objects are never seen "finished" |
| **Concurrency** | Two CRs of the same kind on the same MariaDB, one finishing while the other runs | Shared state (SQL threads, locks, targets) must not be touched under a running operation |
| **Deletion mid-operation** | Delete the CR while its child objects are running | Garbage collection removes children without any reconcile |
| **Operator restart** | Restart the operator in the middle of, and after, the operation | Startup reconciles every object; the logic must be idempotent and not undo deliberate state |
| **Manual intervention** | A DBA changes state by hand after the operator acted (e.g. `STOP SLAVE SQL_THREAD`) | The operator must respect deliberate changes, not fight them |
| **Gates and readiness** | Check what the feature does while the MariaDB is NotReady, suspended, updating or switching over | Early returns (e.g. `waitForIt`) can make the fix unreachable exactly when it is needed |
| **Upgrade path** | Objects created before the change | New annotations/fields are missing on existing objects |

Useful techniques:

- **Widen race windows deliberately** instead of hoping for timing: load data and slow the operation down (e.g.
  `--throttle=N` in a PhysicalBackup `args` so `mariadb-backup` takes minutes).
- **Observe, do not assume**: record timestamps of the event that triggers the behavior and of the operator log
  line that reacts to it; check status conditions, Events and the SQL state directly in the Pods.
- When a scenario seems to pass suspiciously fast, verify it actually hit the code path (log line, Event, metric),
  or force the path by restarting the operator.

## Step 4 — Execute and fix

For each scenario: set up, inject, observe, record the evidence, then restore the environment (restart replication
threads you stopped, delete the CRs) before the next one.

When a scenario fails:

1. Confirm it is caused by the PR's feature (or a gap in it) and not by the environment (§ Local KIND
   troubleshooting) or a pre-existing bug (Step 5 baseline).
2. If it is a gap and the fix is small, consistent with the PR's design and does not take a design decision
   (§ Simplicity), fix it on the PR branch following § Architecture and Code Patterns, rebuild, restart the
   operator and **re-run the scenario** until it passes.
3. If it needs a design decision, or it is a pre-existing problem outside the PR's scope, stop: record it as an open
   point with the evidence and a recommendation.

Keep fixes minimal and in separate, imperative commits that explain the scenario that exposed the gap.

## Step 5 — Regression check

Prove the PR (including your fixes) breaks nothing:

1. The checks from § CI — what a PR must pass that run locally: at least `make lint` and the unit tests (all suites
   must pass).
2. The integration specs closest to the change, focused with `make test-int TEST_ARGS="--focus='…'"`, then the
   broader suites of the affected area (e.g. `--focus='PhysicalBackup|MariaDB replication'`).
3. Use `--json-report=<file>` so you can list which specs passed and failed, and why
   (`State`, `Failure.Message`, `Failure.Location`); non-verbose Ginkgo output does not name passing specs.
4. **Baseline every failure** that is not obviously environmental: run the same specs on an `origin/main` worktree.
   A spec that fails the same way on `main` is pre-existing, not a regression. Report it as such.

## Step 6 — Report on the PR

Write the report and deliver it **through the `mariadb-operator-comment` skill** (plain PR comment, confirmation
gate). Keep it about the feature: no reporter's private data, no notes about your local machine. Structure:

```markdown
## 🧪 End-to-end testing on KIND

<One paragraph: what was tested, operator built from which branch/commit, and the setup (topology, relevant
settings, how the race windows were widened).>

### Scenarios and results

| # | Scenario | Result |
|---|---|---|
| A | <scenario> | ✅ <observed outcome, with the key evidence and timings> |
| B | <scenario> | ❌ → ✅ **Gap, fixed in `<sha>`.** <what broke, what the fix does, the result after it> |
| C | <scenario> | ⚠️ **Not covered**, see below |

### Fixes in `<sha>`

1. **<Fix title>** (`path/to/file.go`): <why it was needed, what changed, why it is safe>.

### Verification

- `make test`: <n> suites pass. `golangci-lint`: <n> issues.
- Integration specs <names>: **<n> Passed, <n> Failed**, <baseline notes for any failure>.

### Open points (pre-existing, not addressed here)

1. **<Problem>.** <Evidence and recommendation, e.g. "needs a finalizer; proposed as a follow-up".>
```

Use the legend consistently: ✅ passed, ❌ → ✅ failed and fixed in this PR, ⚠️ not covered or needs a decision.

## Step 7 — Promote scenarios to integration tests (ask first)

Most scenarios are one-off investigation; only a few deserve to run in CI forever. Evaluate each scenario and
propose **one or two** candidates to the user, with this reasoning:

| Criterion | Promote when… |
|---|---|
| Representative | It covers the reported problem or the main failure mode of the feature |
| Hard to reproduce | It needs failure injection or timing that nobody would set up by hand |
| Regression-prone | The code path is easy to break with an unrelated refactor |
| Deterministic | It can be made reliable in CI (no dependence on luck or wall-clock races) |
| Affordable | Its runtime is acceptable for the suite (state the expected duration) |

**Ask the user which ones to add before writing them.** Then, for each approved test:

1. Add it under `internal/controller/*_test.go` following the suite's conventions (§ Testing): existing builders and
   helpers, `Eventually` with the suite timeouts, the suite's Ginkgo labels, and an existing `Ordered` container
   when it can reuse its MariaDB (see § Local cluster connectivity for new ones).
2. Prefer realistic failure injection the test client can do (e.g. delete a Pod with grace period 0) over
   artificially editing internal state.
3. Run it and confirm it passes.
4. **Negative control:** temporarily disable the fix (do not commit this), run the test and confirm it **fails** for
   the expected reason. A test that passes without the fix does not test the fix. Restore the code and check the
   diff before committing.
5. Commit with an imperative message describing the scenario, and push only with the user's authorization.

## Step 8 — Clean up and hand off

- Delete every resource you created (CRs, Secrets, PVCs) — only those; check the age of anything matched by a
  pattern before deleting it.
- Stop the operator and test processes you started, by PID, and remove temporary worktrees.
- Tell the user what you changed in the environment (installed components, created buckets, host entries).
- Summarize: scenarios run and results, fixes pushed (commits), regression check, open points, and the integration
  test candidates waiting for their decision.
