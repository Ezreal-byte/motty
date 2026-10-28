package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type probeFixture struct {
	Version, Help, Features, Config string
	Extra                           map[string]string
	VersionExit                     int
	VersionDelay                    time.Duration
}

func TestProbeCLIHelper(t *testing.T) {
	raw := os.Getenv("MOTTY_TEST_PROBE")
	if raw == "" {
		return
	}
	var fixture probeFixture
	if json.Unmarshal([]byte(raw), &fixture) != nil {
		os.Exit(2)
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if output, ok := fixture.Extra[strings.Join(args, " ")]; ok {
		fmt.Println(output)
		os.Exit(0)
	}
	switch {
	case len(args) == 1 && args[0] == "--version":
		time.Sleep(fixture.VersionDelay)
		if fixture.VersionExit != 0 {
			os.Exit(fixture.VersionExit)
		}
		fmt.Println(fixture.Version)
	case len(args) >= 2 && args[0] == "features" && args[1] == "list":
		fmt.Println(fixture.Features)
	case len(args) >= 2 && args[0] == "debug" && args[1] == "config":
		fmt.Println(fixture.Config)
	case len(args) > 0 && args[len(args)-1] == "--help":
		fmt.Println(fixture.Help)
	default:
		os.Exit(3)
	}
	os.Exit(0)
}

func probeCommand(t *testing.T, fixture probeFixture) command {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return command{program: exe, args: []string{"-test.run=^TestProbeCLIHelper$", "--"}, env: []string{"MOTTY_TEST_PROBE=" + string(raw)}, dir: t.TempDir()}
}

func probeFixtureError(t *testing.T, kind string, fixture probeFixture) error {
	t.Helper()
	c := probeCommand(t, fixture)
	return probe(t.Context(), kind, &c)
}

func TestProbeUsesCapabilitiesRatherThanVersionWhitelist(t *testing.T) {
	for _, tc := range []struct {
		kind, version, help string
	}{
		{"codex-cli", "codex-cli 0.162.0", "--ephemeral --json --sandbox"},
		{"codex-cli", "codex-cli 0.163.0", "--ephemeral --json --sandbox"},
		{"codex-cli", "codex-cli 0.165.0", "--ephemeral --json --sandbox"},
		{"codex-cli", "codex-cli 1.0.0", "--ephemeral --json --sandbox"},
		{"claude-code-cli", "2.2.0 (Claude Code)", "--settings --setting-sources --strict-mcp-config --tools --disable-slash-commands --include-partial-messages --no-session-persistence --print --output-format --mcp-config --permission-mode --allowedTools"},
		{"claude-code-cli", "3.0.0 (Claude Code)", "--settings --setting-sources --strict-mcp-config --tools --disable-slash-commands --include-partial-messages --no-session-persistence --print --output-format --mcp-config --permission-mode --allowedTools"},
		{"opencode-cli", "2.0.0", "--pure"},
	} {
		t.Run(tc.kind+"/"+tc.version, func(t *testing.T) {
			fixture := probeFixture{Version: tc.version, Help: tc.help, Features: "shell_tool stable false\njs_repl removed false\nhooks stable false", Config: `{"permission":"deny"}`}
			if err := probeFixtureError(t, tc.kind, fixture); err != nil {
				t.Fatalf("capable CLI rejected: %v", err)
			}
		})
	}
}

func TestProbeRejectsIneffectiveCodexControls(t *testing.T) {
	for _, features := range []string{
		"shell_tool stable true",
		"shell_tool stable false\njs_repl experimental true",
		"shell_tool stable false\nhooks stable true",
		"shell_tool stable false\nnot a feature table",
		"shell_tool stable false\nshell_tool stable true",
	} {
		t.Run(features, func(t *testing.T) {
			fixture := probeFixture{Version: "codex-cli 0.163.0", Help: "--ephemeral --json --sandbox", Features: features}
			if err := probeFixtureError(t, "codex-cli", fixture); err == nil {
				t.Fatal("ineffective isolation controls accepted")
			}
		})
	}
}

func TestProbeToleratesRemovedFeaturesAndHeaderRows(t *testing.T) {
	for _, features := range []string{
		"shell_tool removed false\njs_repl removed false",
		"Feature Stage Enabled\nshell_tool stable false",
		"js_repl removed false",
	} {
		fixture := probeFixture{Version: "codex-cli 0.165.0", Help: "--ephemeral --json --sandbox", Features: features}
		if err := probeFixtureError(t, "codex-cli", fixture); err != nil {
			t.Fatalf("%q rejected: %v", features, err)
		}
	}
}

// A flag named only in a description is prose: the session would fail on it.
func TestProbeIgnoresAFlagMentionedOnlyInHelpProse(t *testing.T) {
	help := "--settings --setting-sources --strict-mcp-config --tools --disable-slash-commands --include-partial-messages --no-session-persistence --print --output-format --permission-mode --allowedTools\n  --strict-mcp-config  reject the --mcp-config flag"
	fixture := probeFixture{Version: "2.2.0 (Claude Code)", Help: help, Features: "shell_tool stable false"}
	err := probeFixtureError(t, "claude-code-cli", fixture)
	if err == nil || !strings.Contains(err.Error(), "--mcp-config") {
		t.Fatalf("prose counted as a capability: %v", err)
	}
}

func TestProbeReportsVersionAndMissingCapabilityWithoutRawOutput(t *testing.T) {
	fixture := probeFixture{Version: "PRIVATE_TOKEN\ncodex-cli 0.165.0", Help: "--json --sandbox"}
	err := probeFixtureError(t, "codex-cli", fixture)
	if err == nil || !strings.Contains(err.Error(), "0.165.0") || !strings.Contains(err.Error(), "--ephemeral") || strings.Contains(err.Error(), "PRIVATE_TOKEN") {
		t.Fatalf("missing safe capability diagnostics: %v", err)
	}
}

func TestProbeStillRequiresOpenCodeAppliedPermission(t *testing.T) {
	fixture := probeFixture{Version: "2.0.0", Help: "--pure", Config: `{"permission":"allow"}`}
	if err := probeFixtureError(t, "opencode-cli", fixture); err == nil || !strings.Contains(err.Error(), "permission=deny") {
		t.Fatalf("permission was not checked: %v", err)
	}
}

func TestProbeDoesNotMistakeSimilarFlagForRequiredCapability(t *testing.T) {
	fixture := probeFixture{Version: "codex-cli 0.163.0", Help: "--ephemeral-extra --json --sandbox", Features: "shell_tool stable false"}
	if err := probeFixtureError(t, "codex-cli", fixture); err == nil || !strings.Contains(err.Error(), "--ephemeral") {
		t.Fatalf("missing exact flag accepted: %v", err)
	}
}

func TestProbeDoesNotEchoUnrecognizedVersionOutput(t *testing.T) {
	fixture := probeFixture{Version: "PRIVATE_TOKEN invalid output", Help: "--json --sandbox"}
	err := probeFixtureError(t, "codex-cli", fixture)
	if err == nil || !strings.Contains(err.Error(), "--ephemeral") || strings.Contains(err.Error(), "PRIVATE_TOKEN") {
		t.Fatalf("capability check has no safe diagnostic: %v", err)
	}
}

func TestVersionMetadataDoesNotGateCapableCLI(t *testing.T) {
	for _, fixture := range []probeFixture{
		{Version: "development build", Help: "--ephemeral --json --sandbox", Features: "shell_tool stable false"},
		{VersionExit: 7, Help: "--ephemeral --json --sandbox", Features: "shell_tool stable false"},
	} {
		if err := probeFixtureError(t, "codex-cli", fixture); err != nil {
			t.Fatalf("version metadata blocked capability detection: %v", err)
		}
	}
}

func TestSharedProbeRunsNewAdapterDeclarationsWithoutRuntimeBranches(t *testing.T) {
	for _, policy := range []string{"deny", "allow"} {
		t.Run(policy, func(t *testing.T) {
			fixture := probeFixture{Extra: map[string]string{"about": "custom-agent 23.4.5", "usage": "--isolated", "policy": policy}}
			c := probeCommand(t, fixture)
			a := adapter{
				program: "custom-agent", version: []string{"about"}, help: []string{"usage"}, required: []string{"--isolated"},
				checks: []capabilityCheck{{label: "policy", args: []string{"policy"}, validate: func(raw []byte) checkFailure {
					if strings.TrimSpace(string(raw)) != "deny" {
						return checkFailure{reason: "not_applied", detail: "native-execution=deny"}
					}
					return checkFailure{}
				}}},
			}
			err := probeAdapter(t.Context(), a, &c)
			if c.version != "23.4.5" || (err == nil) != (policy == "deny") {
				t.Fatalf("adapter declaration ignored: version=%q error=%v", c.version, err)
			}
			if err != nil {
				diagnostic, ok := ParseCompatibilityDiagnostic(err.Error())
				if !ok || diagnostic.Program != "custom-agent" || diagnostic.Command != "policy" || diagnostic.Detail != "native-execution=deny" {
					t.Fatalf("generic diagnostics lost adapter metadata: %v", err)
				}
			}
		})
	}
}

func TestSlowOptionalVersionDoesNotConsumeCapabilityBudget(t *testing.T) {
	fixture := probeFixture{VersionDelay: time.Minute, Help: "--ephemeral --json --sandbox", Features: "shell_tool stable false"}
	c := probeCommand(t, fixture)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := probe(ctx, "codex-cli", &c); err != nil {
		t.Fatalf("optional version lookup blocked capable CLI: %v", err)
	}
}

func TestProbePreservesParentCancellationAndDeadline(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	for _, ctx := range []context.Context{canceled, expired} {
		c := probeCommand(t, probeFixture{})
		if err := probe(ctx, "codex-cli", &c); !errors.Is(err, ctx.Err()) {
			t.Fatalf("lost parent context error %v: %v", ctx.Err(), err)
		}
	}
}
