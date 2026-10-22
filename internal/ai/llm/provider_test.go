package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---- stream body builders ----

func oaData(s string) string { return "data: " + s + "\n\n" }

func oaText(parts ...string) string {
	var sb strings.Builder
	for _, p := range parts {
		b, _ := json.Marshal(p)
		sb.WriteString(oaData(`{"choices":[{"delta":{"content":` + string(b) + `}}]}`))
	}
	return sb.String()
}

func oaFinish(reason string) string {
	return oaData(`{"choices":[{"delta":{},"finish_reason":"`+reason+`"}]}`) +
		oaData(`{"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3}}`) + oaData("[DONE]")
}

func anEv(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }

func anStart() string {
	return anEv("message_start", `{"type":"message_start","message":{"usage":{"input_tokens":11,"output_tokens":1}}}`) +
		anEv("ping", `{"type":"ping"}`)
}

func anTextBlock(i int, parts ...string) string {
	s := anEv("content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}`, i))
	for _, p := range parts {
		b, _ := json.Marshal(p)
		s += anEv("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":%s}}`, i, b))
	}
	return s + anEv("content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, i))
}

func anToolBlock(i int, id, name, args string, chunk int) string {
	s := anEv("content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":%q,"name":%q,"input":{}}}`, i, id, name))
	for _, p := range splitRunes(args, chunk) {
		part, _ := json.Marshal(p)
		s += anEv("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%s}}`, i, part))
	}
	return s + anEv("content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, i))
}

func anEnd(reason string) string {
	return anEv("message_delta", `{"type":"message_delta","delta":{"stop_reason":"`+reason+`"},"usage":{"output_tokens":9}}`) +
		anEv("message_stop", `{"type":"message_stop"}`)
}

func oaToolChunks(idx int, id, name, args string, chunk int) string {
	s := oaData(fmt.Sprintf(`{"choices":[{"delta":{"tool_calls":[{"index":%d,"id":%q,"type":"function","function":{"name":%q,"arguments":""}}]}}]}`, idx, id, name))
	for _, p := range splitRunes(args, chunk) {
		part, _ := json.Marshal(p)
		s += oaData(fmt.Sprintf(`{"choices":[{"delta":{"tool_calls":[{"index":%d,"function":{"arguments":%s}}]}}]}`, idx, part))
	}
	return s
}

type proto struct {
	name string
	new  func(Config) Provider
	text func(parts ...string) string // full successful text stream
	tool func(calls ...[3]string) string
}

var protos = []proto{
	{"openai", NewOpenAI,
		func(parts ...string) string { return oaText(parts...) + oaFinish("stop") },
		func(calls ...[3]string) string {
			s := ""
			for i, c := range calls {
				s += oaToolChunks(i, c[0], c[1], c[2], 2)
			}
			return s + oaFinish("tool_calls")
		}},
	{"anthropic", NewAnthropic,
		func(parts ...string) string { return anStart() + anTextBlock(0, parts...) + anEnd("end_turn") },
		func(calls ...[3]string) string {
			s := anStart()
			for i, c := range calls {
				s += anToolBlock(i, c[0], c[1], c[2], 2)
			}
			return s + anEnd("tool_use")
		}},
}

func (p proto) provider(srv *httptest.Server, mutate ...func(*Config)) Provider {
	cfg := Config{BaseURL: srv.URL, APIKey: testKey, Client: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}}
	for _, m := range mutate {
		m(&cfg)
	}
	return p.new(cfg)
}

var simpleReq = Request{Model: "m", Messages: []Message{{Role: RoleUser, Text: "hi"}}}

func TestPlainText(t *testing.T) {
	for _, p := range protos {
		for _, bytewise := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bytewise=%v", p.name, bytewise), func(t *testing.T) {
				srv := sseServer(t, p.text("Hel", "lo ", "你好🙂", " world"), bytewise, nil)
				ch, err := p.provider(srv).Stream(bg, simpleReq)
				if err != nil {
					t.Fatal(err)
				}
				evs := collect(t, ch)
				done := terminal(t, evs)
				if done.Kind != EventDone || text(evs, EventText) != "Hello 你好🙂 world" {
					t.Fatalf("got %+v", evs)
				}
				if p.name == "openai" && (done.Usage != Usage{7, 3} || done.StopReason != "stop") {
					t.Fatalf("done %+v", done)
				}
				if p.name == "anthropic" && (done.Usage != Usage{11, 9} || done.StopReason != "end_turn") {
					t.Fatalf("done %+v", done)
				}
			})
		}
	}
}

func TestReasoning(t *testing.T) {
	oa := oaData(`{"choices":[{"delta":{"reasoning_content":"think "}}]}`) +
		oaData(`{"choices":[{"delta":{"reasoning_content":"hard","content":null}}]}`) + oaText("answer") + oaFinish("stop")
	an := anStart() + anEv("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`) +
		anEv("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think "}}`) +
		anEv("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hard"}}`) +
		anEv("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc"}}`) +
		anEv("content_block_stop", `{"type":"content_block_stop","index":0}`) + anTextBlock(1, "answer") + anEnd("end_turn")
	for i, body := range []string{oa, an} {
		p := protos[i]
		srv := sseServer(t, body, false, nil)
		ch, err := p.provider(srv).Stream(bg, simpleReq)
		if err != nil {
			t.Fatal(err)
		}
		evs := collect(t, ch)
		terminal(t, evs)
		if text(evs, EventReasoning) != "think hard" || text(evs, EventText) != "answer" {
			t.Fatalf("%s: %+v", p.name, evs)
		}
	}
}

func TestParallelToolCalls(t *testing.T) {
	args1 := `{"path":"/etc/hosts","lines":[1,2,3],"note":"héllo 🙂"}`
	args2 := `{"cmd":"ls -la"}`
	for _, p := range protos {
		for _, bytewise := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bytewise=%v", p.name, bytewise), func(t *testing.T) {
				body := p.tool([3]string{"call_a", "read", args1}, [3]string{"call_b", "run", args2})
				srv := sseServer(t, body, bytewise, nil)
				ch, err := p.provider(srv).Stream(bg, simpleReq)
				if err != nil {
					t.Fatal(err)
				}
				evs := collect(t, ch)
				done := terminal(t, evs)
				want := []ToolCall{{"call_a", "read", args1}, {"call_b", "run", args2}}
				if got := toolCalls(evs); !reflect.DeepEqual(got, want) || done.Kind != EventDone {
					t.Fatalf("got %+v", evs)
				}
				if !strings.Contains(done.StopReason, "tool") {
					t.Fatalf("stop %q", done.StopReason)
				}
			})
		}
	}
}

func TestOpenAIToolCallsInterleavedAndEmptyArgs(t *testing.T) {
	body := oaData(`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"two","arguments":"{\"x\":"}},{"index":0,"id":"a","function":{"name":"one"}}]}}]}`) +
		oaData(`{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"1}"}}]}}]}`) + oaFinish("tool_calls")
	srv := sseServer(t, body, false, nil)
	ch, _ := NewOpenAI(Config{BaseURL: srv.URL}).Stream(bg, simpleReq)
	evs := collect(t, ch)
	terminal(t, evs)
	want := []ToolCall{{"a", "one", "{}"}, {"b", "two", `{"x":1}`}}
	if got := toolCalls(evs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestOpenAINoDoneAfterFinish(t *testing.T) {
	srv := sseServer(t, oaText("a")+oaData(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`), false, nil)
	ch, _ := NewOpenAI(Config{BaseURL: srv.URL}).Stream(bg, simpleReq)
	evs := collect(t, ch)
	if terminal(t, evs).Kind != EventDone {
		t.Fatalf("%+v", evs)
	}
}

func TestTruncatedStreamIsError(t *testing.T) {
	bodies := []string{oaText("a"), anStart() + anTextBlock(0, "a")}
	for i, b := range bodies {
		srv := sseServer(t, b, false, nil)
		ch, _ := protos[i].provider(srv).Stream(bg, simpleReq)
		evs := collect(t, ch)
		if terminal(t, evs).Kind != EventError {
			t.Fatalf("%s: %+v", protos[i].name, evs)
		}
	}
}

func TestInvalidToolJSON(t *testing.T) {
	bodies := []string{
		oaToolChunks(0, "a", "t", `{"x":`, 2) + oaFinish("tool_calls"),
		anStart() + anToolBlock(0, "a", "t", `{"x":`, 2) + anEnd("tool_use"),
		oaToolChunks(0, "a", "t", `[1]`, 2) + oaFinish("tool_calls"),
		anStart() + anToolBlock(0, "a", "t", `"str"`, 2) + anEnd("tool_use"),
	}
	for i, b := range bodies {
		srv := sseServer(t, b, false, nil)
		ch, _ := protos[i%2].provider(srv).Stream(bg, simpleReq)
		evs := collect(t, ch)
		if terminal(t, evs).Kind != EventError || len(toolCalls(evs)) != 0 {
			t.Fatalf("case %d: %+v", i, evs)
		}
	}
}

func TestProviderErrorEvents(t *testing.T) {
	bodies := []string{
		oaText("a") + oaData(`{"error":{"message":"overloaded `+testKey+`"}}`),
		anStart() + anEv("error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded `+testKey+`"}}`),
	}
	for i, b := range bodies {
		srv := sseServer(t, b, false, nil)
		ch, _ := protos[i].provider(srv).Stream(bg, simpleReq)
		evs := collect(t, ch)
		last := terminal(t, evs)
		if last.Kind != EventError || !strings.Contains(last.Err.Error(), "verloaded") || strings.Contains(last.Err.Error(), testKey) {
			t.Fatalf("%s: %+v", protos[i].name, last)
		}
	}
}

func TestHTTPErrorsScrubKey(t *testing.T) {
	for _, p := range protos {
		for _, code := range []int{401, 429, 500} {
			t.Run(fmt.Sprintf("%s/%d", p.name, code), func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
					fmt.Fprintf(w, `{"error":{"message":"Incorrect API key provided: %s.\n\tBearer %s"}}`, testKey, testKey)
					w.Write([]byte(strings.Repeat("x", 100000)))
				}))
				defer srv.Close()
				prov := p.provider(srv)
				_, err := prov.Stream(bg, simpleReq)
				_, err2 := prov.Models(bg)
				for _, e := range []error{err, err2} {
					he, ok := e.(*HTTPError)
					if !ok || he.StatusCode != code {
						t.Fatalf("got %T %v", e, e)
					}
					msg := e.Error()
					if strings.Contains(msg, testKey) || !strings.Contains(msg, fmt.Sprint(code)) || len(msg) > 700 || strings.ContainsAny(msg, "\n\t") {
						t.Fatalf("bad error %q", msg)
					}
				}
			})
		}
	}
}

func TestConnectionErrorHasNoKey(t *testing.T) {
	for _, p := range protos {
		prov := p.new(Config{BaseURL: "http://127.0.0.1:1", APIKey: testKey})
		_, err := prov.Stream(bg, simpleReq)
		if err == nil || strings.Contains(err.Error(), testKey) {
			t.Fatalf("%v", err)
		}
	}
}

func TestMidStreamDisconnect(t *testing.T) {
	for _, p := range protos {
		t.Run(p.name, func(t *testing.T) {
			full := p.text("hello")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Write([]byte(full[:len(full)/2]))
				w.(http.Flusher).Flush()
				panic(http.ErrAbortHandler)
			}))
			defer srv.Close()
			ch, err := p.provider(srv).Stream(bg, simpleReq)
			if err != nil {
				t.Fatal(err)
			}
			evs := collect(t, ch)
			if last := terminal(t, evs); last.Kind != EventError || last.Err == nil {
				t.Fatalf("%+v", evs)
			}
		})
	}
}

// blockingServer sends head then blocks until the client goes away.
func blockingServer(t *testing.T, head string, gone chan<- struct{}) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readAll(r) // lets the server notice a closed connection
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(head))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(gone)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestContextCancelMidStream(t *testing.T) {
	for _, p := range protos {
		t.Run(p.name, func(t *testing.T) {
			base := runtimeGoroutines()
			gone := make(chan struct{})
			head := oaText("a")
			if p.name == "anthropic" {
				head = anStart() + anTextBlock(0, "a")
			}
			srv := blockingServer(t, head, gone)
			ctx, cancel := context.WithCancel(bg)
			ch, err := p.provider(srv).Stream(ctx, simpleReq)
			if err != nil {
				t.Fatal(err)
			}
			if ev := <-ch; ev.Kind != EventText {
				t.Fatalf("%+v", ev)
			}
			cancel()
			start := time.Now()
			evs := collect(t, ch)
			if time.Since(start) > 2*time.Second {
				t.Fatal("cancel was not prompt")
			}
			for _, e := range evs {
				if e.Kind == EventDone || e.Kind == EventError {
					t.Fatalf("terminal event after cancel: %+v", e)
				}
			}
			select {
			case <-gone:
			case <-time.After(5 * time.Second):
				t.Fatal("server never saw the disconnect")
			}
			srv.Close()
			waitGoroutines(t, base)
		})
	}
}

// A consumer that stops reading must not pin the goroutine once ctx ends.
func TestAbandonedConsumerDoesNotLeak(t *testing.T) {
	base := runtimeGoroutines()
	srv := sseServer(t, oaText(strings.Split(strings.Repeat("x", 200), "")...)+oaFinish("stop"), false, nil)
	ctx, cancel := context.WithCancel(bg)
	ch, err := NewOpenAI(Config{BaseURL: srv.URL, Client: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}}).Stream(ctx, simpleReq)
	if err != nil {
		t.Fatal(err)
	}
	<-ch // read one, then abandon with the buffer full
	time.Sleep(100 * time.Millisecond)
	cancel()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				srv.Close()
				waitGoroutines(t, base)
				return
			}
		case <-deadline:
			t.Fatal("channel never closed")
		}
	}
}

func TestSlowConsumer(t *testing.T) {
	parts := strings.Split(strings.Repeat("ab", 60), "")
	srv := sseServer(t, oaText(parts...)+oaFinish("stop"), false, nil)
	ch, _ := NewOpenAI(Config{BaseURL: srv.URL}).Stream(bg, simpleReq)
	var evs []Event
	for ev := range ch {
		evs = append(evs, ev)
		if len(evs)%20 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}
	terminal(t, evs)
	if text(evs, EventText) != strings.Repeat("ab", 60) {
		t.Fatalf("lost text: %q", text(evs, EventText))
	}
}

func TestOversizedLineAndTotal(t *testing.T) {
	big := strings.Repeat("a", 2<<20)
	for _, p := range protos {
		t.Run(p.name+"/line", func(t *testing.T) {
			srv := sseServer(t, "data: "+big+"\n\n", false, nil)
			ch, _ := p.provider(srv).Stream(bg, simpleReq)
			evs := collect(t, ch)
			if terminal(t, evs).Kind != EventError {
				t.Fatalf("%+v", evs)
			}
		})
		t.Run(p.name+"/total", func(t *testing.T) {
			srv := sseServer(t, p.text(strings.Split(strings.Repeat("y", 500), "")...), false, nil)
			ch, _ := p.provider(srv, func(c *Config) { c.MaxResponseBytes = 2000 }).Stream(bg, simpleReq)
			evs := collect(t, ch)
			last := terminal(t, evs)
			if last.Kind != EventError || !strings.Contains(last.Err.Error(), "size limit") {
				t.Fatalf("%+v", last)
			}
		})
	}
}

func TestMalformedChunk(t *testing.T) {
	for i, b := range []string{oaData("{not json"), anStart() + anEv("content_block_delta", "{oops")} {
		srv := sseServer(t, b, false, nil)
		ch, _ := protos[i].provider(srv).Stream(bg, simpleReq)
		if terminal(t, collect(t, ch)).Kind != EventError {
			t.Fatal("want error")
		}
	}
}

func TestBaseURLNormalisation(t *testing.T) {
	oa := []struct{ in, want string }{
		{"", "https://api.openai.com/v1"},
		{"  https://api.openai.com/v1/ ", "https://api.openai.com/v1"},
		{"https://api.openai.com", "https://api.openai.com/v1"},
		{"https://api.openai.com/", "https://api.openai.com/v1"},
		{"http://localhost:11434/v1///", "http://localhost:11434/v1"},
		{"https://api.deepseek.com/v1/chat/completions", "https://api.deepseek.com/v1"},
		{"https://open.bigmodel.cn/api/paas/v4/", "https://open.bigmodel.cn/api/paas/v4"},
		{"https://x.test/openai?key=1#f", "https://x.test/openai"},
	}
	for _, c := range oa {
		got, err := openAIBase(c.in)
		if err != nil || got != c.want {
			t.Errorf("openAIBase(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	an := []struct{ in, want string }{
		{"", "https://api.anthropic.com"},
		{"https://api.anthropic.com/", "https://api.anthropic.com"},
		{"https://api.anthropic.com/v1", "https://api.anthropic.com"},
		{"https://api.anthropic.com/v1/", "https://api.anthropic.com"},
		{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com"},
		{"https://proxy.test/anthropic//", "https://proxy.test/anthropic"},
		{"https://proxy.test/anthropic/v1", "https://proxy.test/anthropic"},
	}
	for _, c := range an {
		got, err := anthropicBase(c.in)
		if err != nil || got != c.want {
			t.Errorf("anthropicBase(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"ftp://x", "not a url", "http://", "://x"} {
		if _, err := openAIBase(bad); err == nil {
			t.Errorf("openAIBase(%q) should fail", bad)
		}
		if _, err := NewAnthropic(Config{BaseURL: bad}).Stream(bg, simpleReq); err == nil {
			t.Errorf("anthropic Stream with base %q should fail", bad)
		}
	}
}

func TestEndpointPaths(t *testing.T) {
	for _, p := range protos {
		for _, suffix := range []string{"", "/", "/v1", "/v1/"} {
			var path atomic.Value
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path.Store(r.URL.Path)
				w.Write([]byte(p.text("x")))
			}))
			prov := p.new(Config{BaseURL: srv.URL + suffix})
			ch, err := prov.Stream(bg, simpleReq)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, ch)
			srv.Close()
			want := "/v1/messages"
			if p.name == "openai" {
				want = "/v1/chat/completions"
			}
			if path.Load() != want {
				t.Errorf("%s base %q: path %v want %s", p.name, suffix, path.Load(), want)
			}
		}
	}
}

func jsonEq(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got invalid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("body mismatch\n got: %s\nwant: %s", got, want)
	}
}

var goldenReq = func() Request {
	temp := 0.25
	return Request{
		Model:       "the-model",
		System:      "be brief",
		MaxTokens:   256,
		Temperature: &temp,
		Tools: []ToolSpec{
			{Name: "read", Description: "read a file", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)},
			{Name: "noargs"},
		},
		Messages: []Message{
			{Role: RoleUser, Text: "look at two files"},
			{Role: RoleAssistant, Text: "ok", ToolCalls: []ToolCall{
				{ID: "c1", Name: "read", Arguments: `{"path":"a"}`},
				{ID: "c2", Name: "noargs"},
			}},
			{Role: RoleTool, ToolCallID: "c1", Text: "AAA"},
			{Role: RoleTool, ToolCallID: "c2", Text: "BBB"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c3", Name: "read", Arguments: `{"path":"b"}`}}},
			{Role: RoleTool, ToolCallID: "c3", Text: "CCC"},
			{Role: RoleUser, Text: "thanks"},
		},
	}
}()

func TestOpenAIRequestGolden(t *testing.T) {
	var got []byte
	var hdr http.Header
	srv := sseServer(t, oaFinish("stop"), false, func(r *http.Request, b []byte) { got, hdr = b, r.Header.Clone() })
	ch, err := NewOpenAI(Config{BaseURL: srv.URL, APIKey: testKey}).Stream(bg, goldenReq)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, ch)
	if hdr.Get("Authorization") != "Bearer "+testKey || hdr.Get("Content-Type") != "application/json" {
		t.Fatalf("headers %v", hdr)
	}
	jsonEq(t, got, `{
	"model":"the-model","stream":true,"stream_options":{"include_usage":true},"max_tokens":256,"temperature":0.25,
	"messages":[
	 {"role":"system","content":"be brief"},
	 {"role":"user","content":"look at two files"},
	 {"role":"assistant","content":"ok","tool_calls":[
	   {"id":"c1","type":"function","function":{"name":"read","arguments":"{\"path\":\"a\"}"}},
	   {"id":"c2","type":"function","function":{"name":"noargs","arguments":"{}"}}]},
	 {"role":"tool","content":"AAA","tool_call_id":"c1"},
	 {"role":"tool","content":"BBB","tool_call_id":"c2"},
	 {"role":"assistant","content":null,"tool_calls":[{"id":"c3","type":"function","function":{"name":"read","arguments":"{\"path\":\"b\"}"}}]},
	 {"role":"tool","content":"CCC","tool_call_id":"c3"},
	 {"role":"user","content":"thanks"}],
	"tools":[
	 {"type":"function","function":{"name":"read","description":"read a file","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}},
	 {"type":"function","function":{"name":"noargs","parameters":{"type":"object","properties":{}}}}]
	}`)
}

func TestAnthropicRequestGolden(t *testing.T) {
	var got []byte
	var hdr http.Header
	srv := sseServer(t, anStart()+anEnd("end_turn"), false, func(r *http.Request, b []byte) { got, hdr = b, r.Header.Clone() })
	ch, err := NewAnthropic(Config{BaseURL: srv.URL, APIKey: testKey}).Stream(bg, goldenReq)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, ch)
	if hdr.Get("x-api-key") != testKey || hdr.Get("anthropic-version") != "2023-06-01" || hdr.Get("Authorization") != "" {
		t.Fatalf("headers %v", hdr)
	}
	jsonEq(t, got, `{
	"model":"the-model","stream":true,"max_tokens":256,"temperature":0.25,"system":"be brief",
	"messages":[
	 {"role":"user","content":[{"type":"text","text":"look at two files"}]},
	 {"role":"assistant","content":[{"type":"text","text":"ok"},
	   {"type":"tool_use","id":"c1","name":"read","input":{"path":"a"}},
	   {"type":"tool_use","id":"c2","name":"noargs","input":{}}]},
	 {"role":"user","content":[
	   {"type":"tool_result","tool_use_id":"c1","content":"AAA"},
	   {"type":"tool_result","tool_use_id":"c2","content":"BBB"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"c3","name":"read","input":{"path":"b"}}]},
	 {"role":"user","content":[
	   {"type":"tool_result","tool_use_id":"c3","content":"CCC"},
	   {"type":"text","text":"thanks"}]}],
	"tools":[
	 {"name":"read","description":"read a file","input_schema":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}},
	 {"name":"noargs","input_schema":{"type":"object","properties":{}}}]
	}`)
}

func TestRequestValidation(t *testing.T) {
	bad := []Request{
		{Messages: simpleReq.Messages},
		{Model: "m"},
		{Model: "m", Messages: []Message{{Role: RoleTool, Text: "x"}}},
		{Model: "m", Messages: []Message{{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "1", Name: "t", Arguments: "[1]"}}}}},
		{Model: "m", Messages: simpleReq.Messages, Tools: []ToolSpec{{Name: "t", Schema: json.RawMessage(`[`)}}},
		{Model: "m", Messages: []Message{{Role: "weird", Text: "x"}}},
	}
	for i, r := range bad {
		for _, p := range protos {
			if _, err := p.new(Config{BaseURL: "http://127.0.0.1:1"}).Stream(bg, r); err == nil || strings.Contains(err.Error(), "connect") {
				t.Errorf("%s case %d: want validation error, got %v", p.name, i, err)
			}
		}
	}
}

func TestModels(t *testing.T) {
	var gotHdr http.Header
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHdr, gotPath = r.Header.Clone(), r.URL.Path
		w.Write([]byte(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":"zeta"},{"id":""},{"id":"mid"}],"has_more":false}`))
	}))
	defer srv.Close()
	for _, p := range protos {
		ids, err := p.provider(srv).Models(bg)
		if err != nil || !reflect.DeepEqual(ids, []string{"alpha", "mid", "zeta"}) {
			t.Fatalf("%s: %v %v", p.name, ids, err)
		}
		if p.name == "openai" && (gotPath != "/v1/models" || gotHdr.Get("Authorization") != "Bearer "+testKey) {
			t.Fatalf("openai %s %v", gotPath, gotHdr)
		}
		if p.name == "anthropic" && (gotPath != "/v1/models" || gotHdr.Get("x-api-key") != testKey || gotHdr.Get("anthropic-version") != "2023-06-01") {
			t.Fatalf("anthropic %s %v", gotPath, gotHdr)
		}
	}
}

func TestAnthropicModelsPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after_id") == "" {
			w.Write([]byte(`{"data":[{"id":"b"}],"has_more":true,"last_id":"b"}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"a"}],"has_more":false,"last_id":"a"}`))
	}))
	defer srv.Close()
	ids, err := NewAnthropic(Config{BaseURL: srv.URL}).Models(bg)
	if err != nil || !reflect.DeepEqual(ids, []string{"a", "b"}) {
		t.Fatalf("%v %v", ids, err)
	}
}

func TestModelsBoundedAndMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" && r.Header.Get("X-Big") == "1" {
			w.Write([]byte(`{"data":[{"id":"` + strings.Repeat("a", 5<<20) + `"}]}`))
			return
		}
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	if _, err := NewOpenAI(Config{BaseURL: srv.URL}).Models(bg); err == nil {
		t.Fatal("want malformed error")
	}
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"` + strings.Repeat("a", 5<<20) + `"}]}`))
	}))
	defer big.Close()
	if _, err := NewAnthropic(Config{BaseURL: big.URL}).Models(bg); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestDefaultClientHasNoOverallTimeout(t *testing.T) {
	c := defaultClient()
	if c.Timeout != 0 {
		t.Fatal("default client must not set an overall timeout")
	}
	tr := c.Transport.(*http.Transport)
	if tr.ResponseHeaderTimeout == 0 || tr.TLSHandshakeTimeout == 0 {
		t.Fatal("missing header/TLS timeouts")
	}
}

func splitRunes(s string, n int) []string {
	var out []string
	r := []rune(s)
	for i := 0; i < len(r); i += n {
		out = append(out, string(r[i:min(i+n, len(r))]))
	}
	return out
}

// The OpenAI handler reuses its chunk struct between events. A field absent
// from the next event must not survive from the previous one.
func TestOpenAIReusedChunkDoesNotLeakFields(t *testing.T) {
	body := oaData(`{"choices":[{"delta":{"content":"a","tool_calls":[{"index":0,"id":"call","function":{"name":"run","arguments":"{}"}}]}}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`) +
		oaData(`{"choices":[{"delta":{"content":"b"},"finish_reason":"stop"}]}`) + oaData("[DONE]")
	ch, err := NewOpenAI(Config{BaseURL: sseServer(t, body, false, nil).URL}).Stream(bg, simpleReq)
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, ch)
	done := terminal(t, evs)
	if got := text(evs, EventText); got != "ab" {
		t.Fatalf("text = %q", got)
	}
	if calls := toolCalls(evs); len(calls) != 1 || calls[0].ID != "call" {
		t.Fatalf("tool calls = %+v", calls)
	}
	if done.Usage != (Usage{5, 1}) || done.StopReason != "stop" {
		t.Fatalf("done = %+v", done)
	}
}
