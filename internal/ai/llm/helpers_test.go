package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-SECRET-key-123"

// collect drains ch with a deadline.
func collect(t *testing.T, ch <-chan Event) []Event {
	t.Helper()
	var evs []Event
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return evs
			}
			evs = append(evs, ev)
		case <-timeout:
			t.Fatal("timed out collecting events")
		}
	}
}

func terminal(t *testing.T, evs []Event) Event {
	t.Helper()
	if len(evs) == 0 {
		t.Fatal("no events")
	}
	n := 0
	for _, e := range evs {
		if e.Kind == EventDone || e.Kind == EventError {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one terminal event, got %d: %+v", n, evs)
	}
	last := evs[len(evs)-1]
	if last.Kind != EventDone && last.Kind != EventError {
		t.Fatalf("terminal event not last: %+v", evs)
	}
	return last
}

func text(evs []Event, k EventKind) string {
	var sb strings.Builder
	for _, e := range evs {
		if e.Kind == k {
			sb.WriteString(e.Text)
		}
	}
	return sb.String()
}

func toolCalls(evs []Event) []ToolCall {
	var out []ToolCall
	for _, e := range evs {
		if e.Kind == EventToolCall {
			out = append(out, *e.ToolCall)
		}
	}
	return out
}

// sseServer serves body as an SSE stream, optionally one byte per flush.
func sseServer(t *testing.T, body string, bytewise bool, check func(*http.Request, []byte)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			b := readAll(r)
			check(r, b)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		if !bytewise {
			w.Write([]byte(body))
			return
		}
		for i := 0; i < len(body); i++ {
			w.Write([]byte{body[i]})
			fl.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readAll(r *http.Request) []byte {
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return []byte(sb.String())
}

func waitGoroutines(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	buf = buf[:runtime.Stack(buf, true)]
	t.Fatalf("goroutines did not exit: base %d now %d\n%s", base, runtime.NumGoroutine(), buf)
}

var bg = context.Background()

func runtimeGoroutines() int { return runtime.NumGoroutine() }
