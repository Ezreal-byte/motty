# Verification: Local CLI AI Providers

Date: 2026-10-26
Issue: https://github.com/t8y2/motty/issues/19
Design: `design.md`, approved v0.2
Integration base: upstream `main` at `f6b981191e3a7f9cc21c48728bb08b855dc58236`

## Environment

- macOS, amd64; Go 1.26.3.
- Compatible isolated Viva dependency: `4884bf3a8f3de49fe272833a2209215cfae41012`.
- Installed CLI fixtures: Claude Code 2.1.309 and Codex CLI 0.163.0.
- OpenCode is not installed. Its command/config/parser fixtures run, but its real
  process, authentication and model path have NOT been runtime-verified.

## Evidence

- Formatting and `go vet ./...`: PASS.
- `go test ./... -count=1`: PASS after upstream integration.
- `go test -race ./... -count=1`: PASS after upstream integration.
- `MOTTY_TEST_INSTALLED_CLI=1 go test ./internal/ai/cli -run TestInstalledCLIIsolation -count=1 -v`: PASS for both installed CLIs.
- `go test -tags vivahead ./internal/workbench -run TestCLILoginNativeInputAndFormRendering -count=1`: PASS; native Metal/CoreText form rendering and actual native focus/text/key routing exercised through Viva's headless driver.
- Native macOS application build: PASS.
- Windows amd64 changed-package test cross-compilation: PASS after upstream
  integration. This is NOT Windows runtime/ConPTY validation.
- Final independent whole-branch review found two issues; both have regression
  tests observed RED before the fix and GREEN after it (details below).

The installed CLI fixture uses fresh temporary homes, dummy credentials and a
loopback fake model endpoint. It inspects the actual model-visible tool catalog,
requests one real scoped MCP call through the native orchestrator and Gate,
asserts one backend call and one complete audit pair, and checks that inherited
MCP/startup-hook/notification sentinel commands never execute. It does not
contact a real model or log into any user account.

## Regression Coverage

- SQLite v2-to-v3 migration, provider kinds, HTTP regressions, path locality on
  sync/import, CLI form save/edit and visible provider type.
- Scoped MCP lists/calls, hidden tools, empty tool catalogs, no resource channel.
- Native Ask escalation and declined retries, Agent Gate execution, redaction,
  step limits, completion validation and exactly-once tool history.
- Three JSONL dialects, final snapshot deduplication, informational-event types,
  malformed/oversized/truncated output, nonzero exits and cancellation.
- CLI version controls, argument vectors, missing executables, private config,
  secret-free arguments and correct execution/write tool annotations.
- Explicit official login commands, status classification, cross-window auth
  reservations, interactive local PTY input, no automatic login or request replay.
- Sensitive output exclusion; clipboard cannot reach an active SSH session;
  cancellation on removal, synced deletion, stale/pending startup and window
  closure, including a queued callback discarded by the UI.
- Auth timeout stops the process without depending on UI event delivery.
- Higher-priority modal keyboard/clipboard routing and private login scrolling
  remain correct after upstream input/modal changes.
- Signed-out official-account status does not block a native send for any of
  the three CLI kinds, replace its path/default model, inject Motty vault keys,
  or trigger login. English/Chinese signed-out hints explicitly allow existing
  third-party configuration. Installed Claude/Codex fixtures use third-party
  routing without official-account login.

## Review Fixes

- Mixed-version sync: CLI providers now use an independent `ai-cli-provider`
  record family. HTTP/CLI import and deletion scopes are independent; a legacy
  backup without CLI records preserves local CLI configuration. Unknown future
  provider subtypes remain opaque, not deletion-owned. Tests cover first restore,
  lost AI database adoption, machine-local paths, model choices and last-provider
  deletion through the real sync engine. Existing HTTP sync behavior is retained.
- Legacy-source proof: an archive of unchanged upstream `6d4b0c6` plus a test
  harness imported this branch's exported record fixture and ran its real
  `reloadShared()`/save path. Before the fix it tombstoned CLI records; after the
  fix the same legacy code preserved them and still imported HTTP normally.
  `TestCLIRecordsHaveLegacySafeIndependentOwnership` can export the fixture via
  `MOTTY_LEGACY_RECORD_FIXTURE` for replay against an older source checkout.
- Claude mixed content: text deduplication tracks streamed content-block indexes
  for the assistant message rather than resetting on tool/thinking block starts.
  Tests cover streamed text followed by tool use, multiple text blocks,
  snapshot-only blocks, message boundaries and initial nonempty text blocks.

## Compatibility Decisions

- Supported controls are deliberately bounded: Claude Code 2.1.309+ within
  2.1.x, Codex 0.163.x, and OpenCode 1.x with `--pure` capability. Unknown control
  versions fail closed; widening support requires repeating protocol/tool tests.
- Codex exposes protocol-only MCP resource helpers. The only configured request
  server publishes no resources and rejects all resource methods. They are not
  another execution or local-file channel.
- Claude safe mode disables explicit MCP too. The adapter instead retains normal
  user auth/routing settings, disables all hooks and enumerated installed plugins,
  disables deferred tool search, and uses strict request-local MCP/tool selection.

## Limits And Unverified Acceptance

- No real OAuth/device-code account login, user credential changes, billable
  inference, production host operation, deployment, or release was performed.
- Real OpenCode, Windows/ConPTY, and Linux runtime acceptance remain unverified.
- CLI executables are trusted installed software, not hostile binaries inside an
  OS sandbox. CLI authentication helpers and credential management remain owned
  by each CLI.
- One full-suite attempt observed a failure of the unchanged remote-stats test
  `TestWorkerAttachesTheGPULinesBeforeACountersLineToThatSample`; its isolated
  30-repeat run and subsequent complete ordinary/race gates passed. No change
  was made to that module by this feature; latest upstream subsequently adjusted
  that fixture. The initial failure is retained in local evidence.
