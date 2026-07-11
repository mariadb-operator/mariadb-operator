---
name: mariadb-operator-comment
description: >
  Post a comment (or a formal PR review) to a GitHub issue or pull request in mariadb-operator/mariadb-operator.
  Use whenever the user wants to publish something to GitHub for this repo — "comment on issue #123", "post this
  as a PR comment", "leave a note on #456", "reply on the PR" — and especially when they want the output of the
  mariadb-operator-pr-review skill delivered to GitHub instead of just shown in chat, e.g. "review PR #1234 and
  post it as a comment" or "post the review results to the PR". Always confirms the exact comment text with the
  user before publishing — never posts autonomously.
license: Apache-2.0
metadata:
  author: mariadb-operator
  version: "1.0"
compatibility: >
  Requires either the project-scoped GitHub MCP server (tools named mcp__github-mariadb-operator__*) or the
  gh CLI authenticated via a GITHUB_TOKEN_MARIADB_OPERATOR environment variable.
allowed-tools: Bash(gh:*), AskUserQuestion
---

# mariadb-operator Comment

Publish a comment or PR review to `mariadb-operator/mariadb-operator` (or another repo the user names). This
skill only handles *delivery* — if the content is a code review, run `mariadb-operator-pr-review` first (or use
its output if already produced in this conversation) and pass the result in here for formatting and posting.

## Never post without confirmation

**This is the one rule that matters more than anything else below.** Posting to GitHub is public, hard to
fully undo (edits and deletions leave an audit trail, and people may already have read a notification), and
visible to the whole team. Before calling any tool that actually publishes something:

1. Render the exact text you are about to post, in full — not a paraphrase or a summary of it.
2. State exactly where it's going: `owner/repo#number`, and whether it's a plain comment, a review comment, or
   a formal review (with its verdict, e.g. "Request changes").
3. Ask the user to confirm with `AskUserQuestion` (options like "Post it" / "Edit first" / "Cancel"). Treat
   silence, an ambiguous reply, or moving on to a different topic as "no" — do not post speculatively.
4. Only after an explicit go-ahead, call the posting tool or `gh` command.

If the user asks you to "just post it" up front, still show the rendered text and get one explicit confirmation
before the tool call — the instruction to post is not itself the confirmation of *what* gets posted, since you
may have formatted or summarized content since they last saw it.

## Step 1 — Resolve the target

Default repo is `mariadb-operator/mariadb-operator` unless the user names another one. Get the issue/PR number
from the user's message or a pasted URL (`github.com/<owner>/<repo>/(issues|pull)/<n>`). If it's ambiguous
whether something is an issue or a PR, it usually doesn't matter for a plain comment (GitHub PRs are issues
under the hood, so the same comment endpoint works for both) — it only matters if you're posting a *formal PR
review* (Step 3), which requires an actual pull request.

## Step 2 — Assemble the comment body

Two common sources:

- **Direct ask**: the user gives you the text, or a short instruction to write one (e.g. "tell them the fix
  looks good but ask about the migration path"). Draft it in the tone of a maintainer comment: concise,
  specific, no filler.
- **From `mariadb-operator-pr-review` output**: if a structured review (verdict table + per-dimension
  findings) already exists in this conversation, or you're asked to produce one, its Markdown output format is
  already written to drop straight into a GitHub comment — use it close to verbatim rather than re-summarizing
  it into something vaguer. Do not silently drop findings to shorten it; if you trim, tell the user what you cut.

Keep the rendered body in Markdown exactly as it will appear on GitHub — this is what you show the user in
Step 4, and what you post in Step 5, so there should be no gap between the two.

## Step 3 — Pick plain comment vs. formal review

- **Plain comment** (default): one comment on the issue/PR timeline. Use this for a review delivered as a
  single consolidated write-up (the normal `mariadb-operator-pr-review` output), status updates, questions, or
  anything that isn't anchored to specific diff lines. Simpler, and simplicity is the right default here — only
  reach for a formal review when the findings genuinely need per-line anchoring.
- **Formal PR review with inline comments**: only when the user explicitly wants findings attached to their
  exact lines in the diff (e.g. "leave inline comments on each finding"), and each finding has a concrete
  `file:line`. This requires a pending review that inline comments are added to before submission — see Step 5.
  Don't reach for this by default; a single well-formatted comment covers the common case and is easier for the
  user to review before you post it.

## Step 4 — Show the user exactly what will be posted, then stop and ask

Render, verbatim:

```
Target: <owner>/<repo>#<number> (<issue|PR>)
Mode: <plain comment | formal review: VERDICT>

--- comment body ---
<full rendered Markdown>
--- end ---
```

Then call `AskUserQuestion` asking whether to post, edit, or cancel. Do not proceed past this point without an
explicit "post it" answer. This is the hard gate described above — nothing in Step 5 runs before it.

## Step 5 — Post

**Prefer the project-scoped GitHub MCP tools** (names like `mcp__github-mariadb-operator__add_issue_comment`,
`mcp__github-mariadb-operator__pull_request_review_write`, `mcp__github-mariadb-operator__add_comment_to_pending_review`).
These may show up as deferred tools — if so, load their schema with `ToolSearch` (e.g.
`ToolSearch({query: "select:mcp__github-mariadb-operator__add_issue_comment", max_results: 1})`) before calling
them. If that MCP server isn't connected, fall back to the plain `mcp__github__*` tools if they're pointed at
the right repo, otherwise use the `gh` CLI.

**Plain comment:**
- MCP: `add_issue_comment` with `owner`, `repo`, `issue_number`, `body` — works for both issues and PRs.
- gh CLI fallback, authenticated with the project token rather than the ambient `gh auth` session:
  ```bash
  GH_TOKEN="$GITHUB_TOKEN_MARIADB_OPERATOR" gh issue comment <n> --repo mariadb-operator/mariadb-operator --body "<text>"
  # or for a PR:
  GH_TOKEN="$GITHUB_TOKEN_MARIADB_OPERATOR" gh pr comment <n> --repo mariadb-operator/mariadb-operator --body "<text>"
  ```
  Pass the body via `--body-file` (write it to the scratchpad directory first) instead of `--body` when it's
  long or has quoting-sensitive characters — safer than fighting shell escaping.

**Formal review with inline comments:**
- MCP: use `pull_request_review_write` to create a pending review, `add_comment_to_pending_review` once per
  finding (each needs `path`, `line`, and the finding's body), then submit the pending review with its overall
  verdict (comment / approve / request changes) via `pull_request_review_write`.
- There's no clean `gh` CLI equivalent for multi-inline-comment reviews — if MCP isn't available and the user
  wants inline comments, say so and offer the plain-comment fallback instead of improvising something fragile.

After posting, confirm back to the user with a link or reference to what was just published — don't just say
"done".

## What this skill does not do

- It does not decide *what* to say about a PR — that's `mariadb-operator-pr-review` (or the user's own words).
  This skill only formats and delivers.
- It does not edit or delete existing comments unless the user explicitly asks for that, and the same
  confirmation gate applies before any edit/delete too.
