# FIXME.md — Codebase Audit Report

Thorough review of logical errors, nil pointer risks, edge cases, and design issues.

---

## Open

### 4. Partial commits pushed on multi-file patch group failure

**File:** `internal/actions/apply_executor.go`

When a patch group spans multiple files, files are processed sequentially. If file N fails after file N-1 already committed changes, the error is returned but the branch may already have partial commits. On the next run, `CheckoutOrCreateBranch` reuses the existing branch with its partial state, compounding the problem.

The `defer` cleanup only checks out the base branch locally — it does not revert pushed commits or clean up the remote branch.

**Note:** Requires architectural redesign (transactional commit/rollback) to fix properly.

---

### 9. PR creation failure leaves pushed branch with no cleanup

**File:** `internal/actions/apply_executor.go`

If `createOrUpdatePullRequest()` fails after the branch has been pushed, the error is returned but the remote branch with its commits remains. The next run will find this branch via `CheckoutOrCreateBranch`, reuse it, and potentially accumulate stale changes.

**Note:** Requires architectural redesign (branch cleanup on failure) to fix properly.

---

### 12. No API rate limit handling

**Files:** All GitHub, Docker, and Helm scraper HTTP calls

HTTP 429 (Too Many Requests) is treated the same as any other error — a generic failure message. No retry logic, no exponential backoff, no `Retry-After` header inspection. Same for GitHub's 403 rate limit responses.

**Note:** Feature addition requiring retry/backoff infrastructure.

---

### 19. New `Repository` created per file in a patch group

**File:** `internal/actions/apply_executor.go`

Each file in a patch group creates a fresh `git.NewRepository()` and runs `DetectRepository()` (which finds git root, gets remote URL, detects base branch). For patch groups with many files in the same repo, this redundantly re-detects the same information and creates independent Repository objects that don't share state.

**Note:** Performance optimization that needs architectural changes.

---

## Resolved

| # | Finding | Resolution |
|---|---------|------------|
| 1 | `log.Fatal()` crashes program on non-fatal error | Replaced with `fmt.Errorf` error returns in all 3 scraper clients |
| 2 | Nil map dereference in Helm index parsing | Added nil check for `index.Entries` before map access |
| 3 | No HTTP client timeout on any API call | Added `Timeout: 30 * time.Second` to all `http.Client` instances |
| 5 | Branch divergence not detected (pull failures swallowed) | Pull failures now return errors instead of being silently swallowed |
| 6 | `GetTargetInfo()` silently discards errors | Both implementations now log warnings on `ReadCurrentVersion()` failure |
| 7 | Terraform regex fails on nested braces | Changed `[^}]*` to `(?s).*?` to cross nested brace boundaries |
| 8 | Subchart regex can match wrong dependency | Rewrote patterns to stay within dependency block boundaries |
| 10 | Pull failures silently swallowed during branch creation | Same fix as #5 — pull failures now return errors |
| 11 | Invalid regex silently skips versions | Regex compiled once before loop; invalid patterns return errors immediately |
| 13 | Map iteration order is non-deterministic | File paths sorted before iteration for deterministic order |
| 14 | Empty auth credentials sent silently | Added warnings when auth type is configured but credentials are empty |
| 15 | `--limit` flag accepts negative values | Added validation in `load`, `compare`, and `apply` commands |
| 16 | Regex recompiled on every loop iteration | Fixed together with #11 — regex compiled once with `regexp.Compile()` |
| 17 | Version comparison returns "no update" for non-semver | Non-semver versions now return `UpdateTypePatch` instead of `UpdateTypeNone` |
| 18 | Wildcard with no matches keeps literal glob pattern | Targets with no matches are now skipped instead of kept |
| 20 | Dead code: `ForcePush()`, `deleteBranch()`, `resetToBaseBranch()` | All three methods removed |
| 21 | `HasUnpushedCommits` uses empty `BranchName` | Added empty `BranchName` guard with descriptive error |
| 22 | `io.ReadAll()` errors discarded in error paths | Both error paths now handle read errors explicitly |
| 23 | Base branch detection can fall back to feature branch | Improved warning message to alert about potential feature branch contamination |
