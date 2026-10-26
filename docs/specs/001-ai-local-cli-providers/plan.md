# Local CLI AI Providers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Claude Code, Codex, and OpenCode CLI providers with shared safe execution and user-initiated official login, then submit a verified PR for #19.

**Architecture:** Keep HTTP providers intact. An optional `agent.SessionProvider` runs a native CLI loop through a scoped MCP callback owned by Motty; common code owns cancellation, bounds, and audit. Registry adapters own commands and JSONL dialects; login is an explicitly started sensitive local terminal, never an AI tool.

**Tech Stack:** Go 1.26, existing Viva UI, SQLite, standard-library subprocess/HTTP/JSON, existing risk Gate and MCP server.

**Spec:** `docs/specs/001-ai-local-cli-providers/design.md`, v0.2, approved by the user in this chat on 2026-10-26 (`确认`).

## Global Constraints

- Preserve existing HTTP provider, Gate, Ask escalation, history, and vault behavior.
- No arbitrary shell templates, credential copying, automatic login/model replay, CLI installation, or upgrades.
- CLI paths stay local across sync; new SQLite column defaults to empty.
- Per-request loopback MCP, exact allowlist, private temporary configuration, no secret arguments.
- Bound probes to 30 seconds, login to 15 minutes, sends to 30 minutes; JSONL line 1 MiB, output 16 MiB, stderr 64 KiB.
- Fail closed when required CLI execution isolation is unavailable; record tested versions and unverified runtime/platform cases.
- Push only to `caichangqing1120/motty`, PR only to `t8y2/motty:main`; no merge/release/deploy.

## Review Focus

- Imported CLI providers must retain the destination machine's path rather than exporting or clearing it (Task 1).
- A JSONL final message followed by nonzero exit cannot commit a successful answer (Tasks 3/4).
- Hidden MCP tools, token reuse, and repeated tool logs cannot bypass or duplicate Gate execution (Tasks 2/3).
- Inherited CLI MCP/plugins/hooks cannot execute outside the scoped callback (Task 4).
- Closing/deleting/editing a provider during login cannot apply stale success or leak auth output to AI context (Task 6).

## File Map

- `internal/ai/store/provider.go` or existing `store.go`: provider kinds/path and migration.
- `internal/ai/agent/session.go`: optional native interface/orchestration and audit.
- `internal/ai/mcp/server.go`: optional scoped tool specs/dispatcher, existing defaults unchanged.
- `internal/ai/cli/{provider,process,adapters,events,auth}.go`: registry, process lifetime, per-CLI commands/parsing, auth commands/status.
- Platform-specific `internal/ai/cli/process_{unix,windows}.go`: tree cancellation.
- `internal/workbench/ai_cli*.go`, `ai_presets.go`, `ai_settings*.go`, `ai_state.go`, `ai_sync.go`: settings, routing, sensitive login lifecycle.
- `internal/i18n/ai_settings.go`: matching English/Chinese strings.
- Focused package tests and assistant website docs accompany their owning tasks.

### Task 1: Provider Persistence And Sync

**Interfaces:** `store.IsCLIKind(string) bool`, `store.ValidProviderKind(string) bool`, `Provider.CLIPath string`.

- [ ] Add failing tests for three kinds, schema-v2 migration, unknown-kind rejection, empty CLI HTTP fields, and path-preserving sync/import/adoption.
- [ ] Run `go test ./internal/ai/store ./internal/workbench -run 'CLI|Provider|Migration' -count=1`; confirm tests fail for missing behavior.
- [ ] Implement schema v3 and shared validation; exclude path from records and preserve destination paths. Keep old API-key migration intact. Use independent `ai-cli-provider` records so old clients cannot tombstone CLI subtypes; scope imports/deletions by collection and preserve unknown future subtypes.
- [ ] Repeat focused tests; require PASS, then record progress.

### Task 2: Scoped MCP Dispatch

**Interfaces:** extend `mcp.Config` with optional `Tools []tools.Spec` and `Dispatch func(context.Context,string,[]byte) gate.Result`; nil Tools preserves external defaults, non-nil empty exposes nothing.

- [ ] Add failing tests for exact scoped tools including Ask escalation, hidden/unknown call refusal, empty lists, and existing default Gate dispatch.
- [ ] Run `go test ./internal/ai/mcp -count=1`, confirm the scoped tests fail.
- [ ] Enforce the scoped list in both listing and dispatch; preserve authentication/loopback/shutdown protections.
- [ ] Repeat package tests and race tests; require PASS.

### Task 3: Native Agent Session Contract

**Interfaces:** `agent.SessionProvider.RunSession(context.Context,llm.Request,func(context.Context,llm.ToolCall) gate.Result,func(llm.Event)) error`. CLI providers also implement existing `llm.Provider` for model discovery; their HTTP Stream method rejects misuse.

- [ ] Add fake-session tests for Agent approval/denial, Ask escalation/decline, tool-free retries, max steps, redaction, cancellation, usage, and exactly-once completed history.
- [ ] Run `go test ./internal/ai/agent -run 'Session|Native' -count=1`; confirm RED.
- [ ] Select native orchestration after common request defaults/history preparation. Keep callback execution serialized, audit authoritative, and parser tool events non-executable.
- [ ] Run all agent tests and `-race`; require PASS including legacy HTTP tests.

### Task 4: Shared CLI Runtime And Three Adapters

**Interfaces:** `cli.New(kind,path string) llm.Provider`; registry adapters implement command building, supported capabilities, metadata, and normalized events. `RunSession` starts scoped MCP and owns full cleanup.

- [ ] Write helper-process/parser tests for command arguments/paths, safe config, fragmented JSONL, final-versus-delta deduplication, missing completion, nonzero exit, oversized output, bounded stderr, timeout, cancellation, and request token isolation.
- [ ] Confirm RED using `go test ./internal/ai/cli -count=1`.
- [ ] Establish exact CLI control semantics from installed help and official/source documentation. Implement non-interactive Claude/Codex/OpenCode adapters with default/manual models, capability rejection, native-tool/hook/plugin isolation, and only scoped MCP. Do not infer security from DBX flags alone.
- [ ] Use private cwd/config, argument vectors, read-only probes, environment-only tokens, process-tree reaping, bounded concurrent readers, and safe categorized errors.
- [ ] Add inherited-MCP/hook/plugin sentinel integration tests and verify actual suppression, not only argument strings. Run package tests/race; require PASS.

### Task 5: Native Settings And Send Routing

**Interfaces:** existing `State.aiProvider`, provider forms/presets, and `aiFetchModels` consume Tasks 1/4 without changing HTTP call sites.

- [ ] Add failing form/preset tests for CLI kinds, path editor, no API-key/Endpoint controls, edit roundtrip, localized default models, unknown-kind errors, and vault-free sends.
- [ ] Confirm RED using focused workbench tests.
- [ ] Add three presets/icons, CLI form variants/focus/input handling, provider descriptions, routing, local-path persistence, and English/Chinese strings. Model refresh does not send a model prompt.
- [ ] Run workbench/i18n tests plus all affected packages; require PASS.

### Task 6: Explicit Official Login

**Interfaces:** CLI auth command/status builders return argument vectors and structured states. `State` owns one sensitive local-login session per auth context; background status results are identity-checked before UI updates.

- [ ] Write failing tests for explicit-click-only auth launch, local targeting, commands/status, duplicate starts, interactive input, sensitive-output exclusion, cancel/timeout, stale completion/provider deletion, API-key/custom-provider preservation, and no original-request replay.
- [ ] Confirm RED with focused CLI/workbench auth tests.
- [ ] Implement official auth commands in a dedicated local terminal/dialog with PTY input, not a normal AI-visible SSH/session terminal. Keep browser ownership with the user, no password/token form, no credential-file reads. Status unknown does not block normal sends.
- [ ] Add UI login/recheck/cancel actions and classified errors, optional supported Codex device flow, guard stale results, and clean up on removal/window shutdown. Do not execute a real user login as a test.
- [ ] Run focused and race tests; require PASS.

### Task 7: Product Verification, Review, And PR

- [ ] Update assistant English/Chinese docs, compatibility and auth guidance, native step counting, and honest verification evidence.
- [ ] Run `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test ./... -count=1`, `go test -race ./...`; inspect fresh results, distinguish baseline failures.
- [ ] Build the real native app and validate settings and login controls without logging into accounts; make only non-destructive model smoke calls where available/authorized, record gaps rather than claiming mocked runtime acceptance.
- [ ] Request a whole-branch independent review; repair findings with regression tests and repeat affected verification.
- [ ] Recheck git diff/secret leakage/target/authorization; create `codex/issue-19-ai-local-cli`, commit, push fork branch, open PR with `Fixes #19`, and attach its URL to this chat.

## Self-Review

All design sections map to Tasks 1-7, including the login addition. The five review-focus cases have explicit owning tests. The native interface uses existing llm and Gate types, MCP remains backward-compatible, and no private checkout path belongs in product configuration. Execution is inline per foundation policy; no new implementation-choice confirmation is needed.
