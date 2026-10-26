package agent

import (
	"context"
	"errors"
	"motty/internal/ai/gate"
	"motty/internal/ai/llm"
	"motty/internal/ai/tools"
	"strings"
	"testing"
)

type nativeTestProvider struct {
	run func(context.Context, llm.Request, func(context.Context, llm.ToolCall) gate.Result, func(llm.Event)) error
}

func (*nativeTestProvider) Models(context.Context) ([]string, error) { return []string{"default"}, nil }
func (*nativeTestProvider) Stream(context.Context, llm.Request) (<-chan llm.Event, error) {
	return nil, errors.New("native session must not use HTTP loop")
}
func (p *nativeTestProvider) RunSession(c context.Context, r llm.Request, call func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
	return p.run(c, r, call, emit)
}

func TestNativeSessionExecutesOnceRedactsAndPersistsCompletedCalls(t *testing.T) {
	b := &fakeBackend{out: "password=secret"}
	cfg := newCfg(&fakeProvider{}, b, true)
	cfg.RedactToolResults = true
	cfg.Provider = &nativeTestProvider{run: func(ctx context.Context, r llm.Request, invoke func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
		c := llm.ToolCall{ID: "one", Name: tools.NameTerminalExec, Arguments: `{"command":"ls"}`}
		res := invoke(ctx, c)
		if strings.Contains(res.Text, "secret") {
			t.Error("unredacted result")
		}
		emit(llm.Event{Kind: llm.EventToolCall, ToolCall: &c}) // diagnostic only; not an instruction to execute again.
		emit(text("finished"))
		emit(done(2, 3))
		return nil
	}}
	var events []Event
	msgs, err := Run(t.Context(), cfg, nil, "hello", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if len(b.execs) != 1 || len(msgs) != 4 || msgs[2].Role != llm.RoleTool || msgs[3].Text != "finished" {
		t.Fatalf("exec=%v msgs=%+v", b.execs, msgs)
	}
	for _, m := range msgs {
		if strings.Contains(m.Text, "secret") {
			t.Fatal("secret in history")
		}
	}
	starts, ends := 0, 0
	for _, e := range events {
		if e.Kind == EventToolStart {
			starts++
		}
		if e.Kind == EventToolDone {
			ends++
		}
	}
	if starts != 1 || ends != 1 {
		t.Fatal(starts, ends)
	}
}

func TestNativeSessionFailureKeepsToolPairButNotFalseFinal(t *testing.T) {
	b := &fakeBackend{}
	cfg := newCfg(&fakeProvider{}, b, true)
	cfg.Provider = &nativeTestProvider{run: func(ctx context.Context, r llm.Request, invoke func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
		invoke(ctx, llm.ToolCall{Name: tools.NameTerminalExec, Arguments: `{"command":"ls"}`})
		emit(text("unverified"))
		emit(done(1, 1))
		return errors.New("exit 7")
	}}
	msgs, err := Run(t.Context(), cfg, nil, "hello", nil)
	if err == nil || len(msgs) != 3 || msgs[2].Role != llm.RoleTool {
		t.Fatal(msgs, err)
	}
}

func TestNativeSessionAskEscalationNeverExecutes(t *testing.T) {
	b := &fakeBackend{}
	cfg := newCfg(&fakeProvider{}, b, true)
	cfg.Mode = ModeAsk
	cfg.Provider = &nativeTestProvider{run: func(ctx context.Context, r llm.Request, invoke func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
		if len(r.Tools) != 1 || r.Tools[0].Name != tools.NameRequestAgentMode {
			t.Error(r.Tools)
		}
		denied := invoke(ctx, llm.ToolCall{Name: tools.NameTerminalExec, Arguments: `{"command":"touch no"}`})
		if !denied.IsError {
			t.Error("Ask allowed exec")
		}
		invoke(ctx, llm.ToolCall{Name: tools.NameRequestAgentMode, Arguments: `{"reason":"inspect"}`})
		return ctx.Err()
	}}
	escalation := 0
	msgs, err := Run(t.Context(), cfg, nil, "inspect", func(e Event) {
		if e.Kind == EventAgentModeRequest {
			escalation++
		}
	})
	if err != nil || len(b.execs) != 0 || escalation != 1 || len(msgs) != 1 {
		t.Fatal(msgs, err, b.execs, escalation)
	}
}

func TestNativeSessionBudgetAndNoTools(t *testing.T) {
	for _, noTools := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "declined"}[noTools], func(t *testing.T) {
			b := &fakeBackend{}
			cfg := newCfg(&fakeProvider{}, b, true)
			cfg.MaxSteps = 1
			if noTools {
				cfg.Mode = ModeAsk
				cfg.NoTools = true
			}
			cfg.Provider = &nativeTestProvider{run: func(ctx context.Context, r llm.Request, invoke func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
				if noTools && len(r.Tools) != 0 {
					t.Error("tools after decline")
				}
				invoke(ctx, llm.ToolCall{Name: tools.NameTerminalExec, Arguments: `{"command":"ls"}`})
				res := invoke(ctx, llm.ToolCall{Name: tools.NameTerminalExec, Arguments: `{"command":"pwd"}`})
				if !res.IsError {
					t.Error("budget bypass")
				}
				emit(text("summary"))
				emit(done(0, 0))
				return nil
			}}
			_, err := Run(t.Context(), cfg, nil, "question", nil)
			want := 1
			if noTools {
				want = 0
			}
			if err != nil || len(b.execs) != want {
				t.Fatal(err, b.execs)
			}
		})
	}
}

func TestNativeSessionRequiresTerminalEvent(t *testing.T) {
	cfg := Config{Provider: &nativeTestProvider{run: func(ctx context.Context, r llm.Request, call func(context.Context, llm.ToolCall) gate.Result, emit func(llm.Event)) error {
		emit(text("partial"))
		return nil
	}}}
	msgs, err := Run(t.Context(), cfg, nil, "hello", nil)
	if err == nil || len(msgs) != 1 {
		t.Fatal(msgs, err)
	}
}
