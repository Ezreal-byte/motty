# Local CLI AI Providers

Version: 0.2 (approved for implementation)
Date: 2026-10-26
Issue: https://github.com/t8y2/motty/issues/19
Base: upstream `main`, commit `124ff7f05b89cd1c57642143a58269dfa177aefa`

## Intent And Approval Context

Use an installed, authenticated AI CLI from Motty's built-in assistant without
supplying another API key. The user selected a shared runtime and Claude Code,
Codex, and OpenCode in the first PR, and requested issue creation, `/claim`,
implementation, and a PR. Issue #19 is created and the bot confirmed the claim.
The user also confirmed including an explicit one-click CLI login entry in
this first PR. This adds user-initiated official authorization, not automatic
sign-in or Motty-managed credentials.
Approval of the approach and this v0.2 design is recorded in the chat; the user confirmed implementation on 2026-10-26.

## Approaches

1. Shared subprocess runtime and provider adapters (selected): common lifetime,
   bounded output, scoped tools, and audit handling; adapter-specific commands,
   capabilities, model metadata, and event parsing.
2. Three independent integrations (rejected): repeats security/platform fixes.
3. Arbitrary command templates (rejected): unreliable quoting and permissions.

DBX is a behavioral reference. Motty keeps its Viva UI, Go implementation,
tools, and risk Gate; it does not copy DBX's application architecture.

## User Experience And Data

- Add `Claude Code CLI`, `Codex CLI`, and `OpenCode CLI` to provider presets.
  Show name and optional executable path; hide HTTP protocol, Endpoint, and
  API Key. Explain that installation is external and authentication belongs to
  the CLI; offer the login entry described below for configured CLI providers.
- Empty path selects `claude`, `codex`, or `opencode`. A path denotes one
  executable, not a command line. Keep current dialog/focus/select conventions.
  An existing CLI provider edits its name/path, not its CLI type. Existing
  HTTP editing remains unchanged.
- Saving configuration does not require installation/login. Refresh/send
  reports missing executables, incompatible versions, and login failures.
  Every CLI has `default`, displayed with a localized label and no model flag.
  Reuse manual model entry, enable/disable, and default model selection.
- Add distinct kinds `claude-code-cli`, `codex-cli`, `opencode-cli` and a shared
  kind validator across persistence, sync, UI, icons, and dispatch. An unknown
  kind must not silently become OpenAI.
- Migrate SQLite schema v2 with a `cli_path` text column defaulting to empty;
  expose `Provider.CLIPath`, do not overload `BaseURL`. Existing rows and vault
  key migrations retain their behavior. CLI rows have no HTTP URL/key/HasKey
  and never prompt to unlock the API-key vault.
- Sync provider identity/type/enabled/models through separate `ai-cli-provider`
  records (`ai_cli_providers` collection); HTTP keeps its existing `ai-provider`
  records and `ai_providers` collection. Old clients preserve the unknown CLI
  root without attempting to import it. HTTP and CLI collections are independently
  authoritative; an absent CLI collection in a legacy backup does not delete
  local CLI providers. Unknown future provider subtypes stay opaque and unowned.
  Paths are machine-local: exclude them from sync/export, preserve the local
  destination path when importing updates, and use empty on first restore.
  Provider deletion removes its local path with the SQLite row.
- No environment editor in the first PR. CLI authentication/provider/model
  configuration stays with each CLI. Motty does not open, copy, migrate,
  log, sync, or rewrite credential files. Add English/Chinese UI and docs.

## User-Initiated Login

For a saved CLI provider, show an authentication status and a `Sign in` action
(localized in Chinese). A clear login-required send error also links to that
provider's sign-in action. Missing executable errors point to installation/path
guidance instead. Unknown status is not the same as signed out.

1. Check authentication off the UI thread through a supported non-generative
   CLI status command, without reading its credential files. Represent
   checking, unknown/not verifiable, signed out, configured/authenticated,
   signing in, failed, and canceled states accurately. Display provider-specific
   wording: configured credentials do not establish remote model access.
2. Clicking `Sign in` explicitly starts the selected installed executable's
   official auth command in a task-owned local login terminal. It is never an
   SSH/remote terminal, an AI tool request, or an arbitrary command supplied by
   model output. Use argument vectors and retain the same local CLI home and
   authentication context as subsequent requests.
3. The CLI launches the official browser authorization where supported. Keep
   local terminal input available for provider selection, one-time codes, and
   documented fallbacks. Do not embed a Motty password/token form or open an
   unrelated fixed sign-in homepage that cannot complete the CLI callback.
4. Recheck supported CLI authentication status after successful auth-command
   exit. Restore the UI and allow explicit retry; do not automatically resend
   the original model request or resume a previously proposed terminal action.

| CLI | Official entry | Login behavior |
| --- | --- | --- |
| Claude Code | `claude auth login` | CLI-managed browser flow, with terminal callback/code fallback when required |
| Codex | `codex login` | Browser flow; an explicit device-code alternative uses `codex login --device-auth` only when supported |
| OpenCode | `opencode auth login` | Interactive provider/auth selection; browser, key, or other methods depend on the provider |

Do not treat arbitrary HTTP 401/403, network failures, unsupported models, or
third-party gateway failures as proof that official account login is needed.
Existing environment/API-key/custom-provider modes continue to work without
forcing a subscription-account browser login. If a CLI cannot report a reliable
status, show unknown and offer manual login/recheck; do not block a normal send
solely for missing status metadata or claim it is authenticated. A signed-out
official-account status is advisory too: a CLI with existing third-party
provider credentials remains usable without official-account sign-in.

Only a user click launches login; settings paint, model refresh, failed sends,
and restoring a provider must never open a browser automatically. Disable
duplicate login starts for the same resolved CLI/auth context. Auth commands
must not execute a model request or install/upgrade the CLI.

Login terminals are sensitive: do not capture their URLs, one-time codes,
typed secrets, or raw auth output into AI context, conversation history,
recordings, sync records, or diagnostic logs. The CLI may save its credentials
using its normal official auth mechanism; Motty never handles those files or
adds the credentials to its own SQLite/vault. A browser opened by the CLI stays
user-owned and is not closed on cancellation.

Allow cancel and timeout (15 minutes) and stop/reap only the task-owned auth
processes on cancel, timeout, provider removal, or window shutdown. Cancellation
must not run logout, erase existing credentials, or claim to roll back a
credential that the CLI already saved. Guard asynchronous completion against
deleted/edited provider or executable/auth-context changes. Browser-opening
failure leaves terminal guidance available; surface failure without false
success. Do not parse login URLs from model output. If Motty offers an explicit
link fallback, validate an adapter-supported HTTPS login host; otherwise leave
the official CLI interaction in the local terminal.

## Architecture

### Native Session Extension

Keep `llm.Provider` and the existing HTTP streaming loop for HTTP providers.
CLIs own an agent loop: do not convert their already-executed MCP events into
pending HTTP function calls, which would execute tools twice.

Define a native-session extension contract in `internal/ai/agent`: model,
system instructions, bounded structured history, exact tool specs, a
Motty-controlled tool callback, and an output emitter. The CLI receives no
unrestricted workbench/backend object. The agent selects its HTTP path or a
native orchestration path; both produce existing `agent.Event` UI events and
`llm.Message` history. The native path owns mode, Gate, audit, redaction,
budgets, and history. The callback is authoritative, not parsed CLI tool logs.

Implement `internal/ai/cli` against this contract, avoiding an agent-to-CLI
import cycle. Its registry maps kinds to adapters. Shared code owns executable
resolution, private working directories, subprocess I/O, process-tree cleanup,
output limits, request MCP lifetime, and error categories. Adapters own command
construction, compatibility checks, model metadata, and JSONL dialects.

### Request-Scoped MCP And Tools

Each send starts a loopback-only MCP endpoint with a new random token, scoped
to that request/window/callback, not the public multi-window AIHub. It works
with external MCP disabled and never changes the external setting/token/server.
Extend the existing MCP server with optional scoped tool specs/dispatch while
preserving current AIHub defaults. Both list and call enforce the allowlist.
Reuse `motty mcp-bridge` for stdio when needed. Tokens travel only in child
environment/private config, never command arguments, prompts, errors, or history.

- Ask exposes only `request_agent_mode`. It cannot inspect/execute sessions or
  access files. Validate the escalation, emit the existing approval card, stop
  the child, and use current accept/decline retry behavior. Declined retries
  expose no tools. Escalation never directly executes a proposed command.
- Agent exposes existing Motty tools. Reuse the same Gate, typed validation,
  risk policy, approval UI, step timeout, denied-result semantics, result
  truncation, and configured redaction. Serialize dispatch within a request.
- Record each actual tool invocation/result and emit start/done once. CLI tool
  log events never execute or duplicate calls. Keep completed tool/result pairs
  on later failure/cancel; do not persist incomplete calls as successful work.
- Native requests consume one configured step per accepted MCP invocation;
  document this stricter native rule. HTTP keeps its current count of model
  turns requesting tools. At the limit refuse execution and request a summary;
  repeated invalid/denied attempts are bounded and cannot keep a run alive.
- Cancel approvals, stop the MCP server, reap the child tree, and remove only
  request-owned artifacts on success, error, cancel, escalation, or window
  shutdown. Request tokens cannot reach another request or window.

### History And Output

Start a fresh ephemeral CLI session per send; replay Motty's trimmed history
with explicit role and tool/result boundaries, not CLI resume files. Tool
results remain untrusted data, never promoted to new executable instructions.
Normalize text, reasoning when available, usage, success, and errors. Avoid
appending final text twice when it repeats deltas. Only the Motty tool callback
writes tool audit/history. Serialize it with reader output to avoid races.

Unknown informational events may be skipped. Invalid relevant JSON, a truncated
stream, missing completion, nonzero exit, or protocol failure cannot report
success. Persist final assistant text only after a validated successful terminal
event and exit. Preserve current context attachment and redaction semantics.

## Adapter Security And Compatibility

Run from a fresh private directory, not the user's project. Spawn an argument
vector, never a user-controlled shell command. Discover PATH/documented common
installations; support explicit paths, tilde expansion, and spaces. On Windows,
resolve recognized npm shims to their executable/Node entry; reject unsupported
scripts rather than interpreting them through a shell.

Retain supported CLI authentication and model/provider routing, but isolate
execution capabilities: native shell/file/web/browser/subagent tools, hooks,
plugins, and unrelated MCP must not become alternate execution channels. Use
request-local strict tool/MCP overrides, not edits to global configuration.
Read-only sandboxing alone is insufficient. Never use dangerous permission or
sandbox bypass flags. An arbitrary replacement binary remains trusted software;
this integration is not an OS sandbox for hostile executables.

Verified Codex compatibility detail: its built-in MCP resource metadata
helpers may remain visible. They address only configured MCP servers; the
sole scoped Motty server advertises no resources and rejects all resource
list/read methods. They are not a local file or alternate execution channel.
Shell, image/file access, goals, questions, browser, and other native execution
tools must remain disabled. Treat any additional model-visible tool as a
compatibility failure in the installed-CLI fixture.

| Adapter | Transport | Required isolation direction |
| --- | --- | --- |
| Claude Code | print, stream-json, partial messages | explicit Motty tools, strict MCP, native tools/hooks/plugins off, no session persistence, retain supported login |
| Codex | exec, json, stdin, ephemeral | native tools/web off, read-only sandbox, only scoped Motty MCP, extra execution channels off, retain supported auth/routing |
| OpenCode | run, json, stdin | pure mode, project config/external skills off, deny-by-default except exact scoped Motty tools, unrelated MCP off, private session storage |

Before model access, capability checks must establish required controls. Fail
closed with an upgrade/configuration hint if they are unavailable; never fall
back to interactive mode or weaker permissions. Establish supported version
floors/flags with official documentation, help, fixtures, and adversarial tests.
Those are implementation facts, not assumptions proven by the DBX reference.

## Bounds, Errors, And Models

- Bound version/help/metadata probes to 30 seconds. Total native send deadline
  is 30 minutes including approvals; tool calls retain existing step timeout.
  Cancellation wins and shutdown never waits indefinitely on pipe readers.
- Limit JSONL lines to 1 MiB, total output to 16 MiB, and concurrent stderr
  buffering to 64 KiB. Bound serialized input and reject oversized input rather
  than silently dropping the latest user turn. Drain both pipes concurrently.
- Classify missing/bad executable, unsupported capabilities, login-required,
  provider failure, timeout, cancellation, and invalid output. Localize hints;
  display only bounded sanitized details, never raw environment/config,
  credentials, full invocation arguments, tokens, or unfiltered stderr.
- All three require default/manual models. OpenCode may enumerate configured
  models through its bounded `models` command. Additional discovery uses only
  supported non-generative CLI metadata; aliases/fallbacks are not discovery.
  Unavailable metadata does not erase enabled/manual/default model choices.
- Refresh/paint never sends a billable model prompt. Version/help/default-model
  availability does not prove login, network, or actual model availability.

## Verification And Delivery

Tests first: migrations, shared kind validation, path locality across sync,
forms/focus, registry dispatch, argument safety, environment controls, dialect
fixtures, output bounds, process cancellation, and exactly-once history. Use
Go helper executables so process tests need neither installed CLIs nor Unix
shell scripts on Windows.

Login tests cover status classification, user-click-only launch, local-versus-
SSH targeting, argument safety, duplicate starts, provider selection/code input,
browser failure, auth cancel/timeout, stale completion, provider removal, status
recheck, API-key/custom-provider compatibility, sensitive-output exclusion, and
no automatic request replay. Test adapters through fake auth/status processes;
do not log into or out of the user's real accounts during automated tests.

Integration coverage must exercise scoped MCP through the native orchestrator
and Gate: Ask isolation and escalation accept/decline, Agent approval and deny,
hidden/malformed tools, budgets, redaction, cancel during approval, concurrent
requests, token isolation, and exactly-once execution. Use inherited MCP/hook/
plugin sentinel configs to demonstrate they cannot execute; matching command
strings alone is insufficient evidence.

Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`.
The repository replaces Viva with `../viva`; establish a compatible isolated
dependency checkout for baseline tests without editing existing user checkouts
or adding a machine-specific product go.mod workaround. Report baseline failures
separately. Verify the real native settings flow and non-destructive sends with
available authenticated CLIs. Do not install/login/upgrade a CLI without separate
authorization to perform that action on this machine. Implementing the sign-in
entry is authorized scope; initiating a real account login is not. OpenCode is
not currently on PATH; mocks are not its smoke test.
Record CLI versions, OS, UI evidence, and unverified platforms in PR evidence;
cross-compilation is not Windows runtime validation.

After approval, create `plan.md` here, implement/review/verify, create a `codex/`
branch, push to the user's Motty fork, and open a PR to `t8y2/motty:main` with
`Fixes #19`. Preserve worktree/evidence. No merge, release, deploy, or closing
an unmerged issue is authorized.

## Non-goals And Builder Latitude

No other CLIs, arbitrary command templates/arguments, environment editor,
automatic installation/login/upgrade, embedded credential management, new API
protocols, external MCP UI redesign,
resume, model effort UI, image input, or Viva changes. A genuine framework
defect is reported separately, not hidden by an application workaround.

The builder chooses Go type/helper names, small file boundaries, tested
compatibility floors, protocol fixtures, and platform process-tree mechanisms.
These choices cannot weaken mode, Gate, credential, sync, or isolation rules.
Report required behavior that cannot be implemented safely instead of omitting
it. The user approved this design on 2026-10-26; implementation follows plan.md.

## Evidence And Owning Sources

- `internal/ai/agent/agent.go`: HTTP loop, mode, Gate, history.
- `internal/ai/mcp/server.go`, `internal/ai/mcp/bridge.go`,
  `cmd/motty/mcp_bridge.go`: scoped server extension and transport.
- `internal/ai/store/store.go`: schema v2/provider persistence.
- `internal/workbench/ai_state.go`, `ai_settings.go`, `ai_settings_paint.go`,
  `ai_presets.go`, `ai_sync.go`, `ai_hub.go`: UI, dispatch, sync, external MCP.
- DBX logical reference: `crates/dbx-core/src/ai_cli_agent.rs`,
  `ai_claude_code_cli.rs`, `ai_codex_cli.rs`, `ai_opencode_cli.rs`.
- [Codex non-interactive mode](https://developers.openai.com/codex/noninteractive)
  and [configuration](https://developers.openai.com/codex/config-reference).
- [Codex authentication](https://developers.openai.com/codex/auth) and
  [Claude Code authentication](https://code.claude.com/docs/en/authentication).
- [OpenCode CLI](https://opencode.ai/docs/cli/),
  [permissions](https://opencode.ai/docs/permissions/), and
  [MCP](https://opencode.ai/docs/mcp-servers/).
- Read-only local help: Claude Code 2.1.309 and Codex CLI 0.163.0.
- Approval delta: the user explicitly requested including the proposed
  one-click official CLI login flow, then approved the full v0.2 design.
