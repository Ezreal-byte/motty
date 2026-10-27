package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const anthropicVersion = "2023-06-01"

type anthropic struct {
	base
	baseErr error
}

// NewAnthropic returns a Provider speaking the Anthropic Messages API. An
// empty BaseURL selects https://api.anthropic.com.
func NewAnthropic(cfg Config) Provider {
	u, err := anthropicBase(cfg.BaseURL)
	return &anthropic{base: newBase(cfg, u), baseErr: err}
}

// anthropicBase returns the base without any /v1 or /v1/messages suffix;
// endpoints are appended as /v1/....
func anthropicBase(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "https://api.anthropic.com", nil
	}
	u, err := normalizeBase(raw)
	if err != nil {
		return "", err
	}
	u.Path, _ = cutSuffixFold(u.Path, "/messages")
	u.Path, _ = cutSuffixFold(u.Path, "/v1")
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func (p *anthropic) headers(r *http.Request) {
	r.Header.Set("anthropic-version", anthropicVersion)
	if p.key == "" {
		return
	}
	// Exactly one credential header: relays that accept only a bearer token
	// reject a request that also carries x-api-key.
	if p.authStyle == AuthStyleBearer {
		r.Header.Set("Authorization", "Bearer "+p.key)
		return
	}
	r.Header.Set("x-api-key", p.key)
}

type anBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anMessage struct {
	Role    string    `json:"role"`
	Content []anBlock `json:"content"`
}

type anTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anRequest struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	System      string      `json:"system,omitempty"`
	Messages    []anMessage `json:"messages"`
	Tools       []anTool    `json:"tools,omitempty"`
	Stream      bool        `json:"stream"`
	Temperature *float64    `json:"temperature,omitempty"`
}

const defaultAnthropicMaxTokens = 4096

func (p *anthropic) buildBody(req Request) ([]byte, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	out := anRequest{Model: req.Model, MaxTokens: req.MaxTokens, Stream: true, Temperature: req.Temperature}
	if out.MaxTokens <= 0 {
		out.MaxTokens = defaultAnthropicMaxTokens
	}
	system := []string{}
	if req.System != "" {
		system = append(system, req.System)
	}
	// addUser appends blocks to the trailing user message so that parallel
	// tool results (and a following user turn) share one message.
	addUser := func(b anBlock) {
		if n := len(out.Messages); n > 0 && out.Messages[n-1].Role == "user" {
			out.Messages[n-1].Content = append(out.Messages[n-1].Content, b)
			return
		}
		out.Messages = append(out.Messages, anMessage{Role: "user", Content: []anBlock{b}})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			if m.Text != "" {
				system = append(system, m.Text)
			}
		case RoleUser:
			if m.Text != "" {
				addUser(anBlock{Type: "text", Text: m.Text})
			}
		case RoleTool:
			if m.ToolCallID == "" {
				return nil, errors.New("llm: tool message without ToolCallID")
			}
			addUser(anBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Text})
		case RoleAssistant:
			var blocks []anBlock
			if m.Text != "" {
				blocks = append(blocks, anBlock{Type: "text", Text: m.Text})
			}
			for _, tc := range m.ToolCalls {
				args, err := validateArgs(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("tool call %q: %w", tc.Name, err)
				}
				blocks = append(blocks, anBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: json.RawMessage(args)})
			}
			if len(blocks) > 0 {
				out.Messages = append(out.Messages, anMessage{Role: "assistant", Content: blocks})
			}
		default:
			return nil, fmt.Errorf("llm: unknown role %q", m.Role)
		}
	}
	if len(out.Messages) == 0 {
		return nil, errors.New("llm: request has no non-empty messages")
	}
	out.System = strings.Join(system, "\n\n")
	for _, t := range req.Tools {
		schema, err := validateSchema(t.Schema)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", t.Name, err)
		}
		out.Tools = append(out.Tools, anTool{Name: t.Name, Description: t.Description, InputSchema: schema})
	}
	return json.Marshal(out)
}

func (p *anthropic) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	if p.baseErr != nil {
		return nil, p.baseErr
	}
	body, err := p.buildBody(req)
	if err != nil {
		return nil, err
	}
	resp, err := p.do(ctx, http.MethodPost, p.endpoint+"/v1/messages", body, func(r *http.Request) {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "text/event-stream")
		p.headers(r)
	})
	if err != nil {
		return nil, err
	}
	return p.pump(ctx, resp.Body, &anHandler{}), nil
}

func (p *anthropic) Models(ctx context.Context) ([]string, error) {
	if p.baseErr != nil {
		return nil, p.baseErr
	}
	var ids []string
	after := ""
	for page := 0; page < 10; page++ {
		q := url.Values{"limit": {"1000"}}
		if after != "" {
			q.Set("after_id", after)
		}
		resp, err := p.do(ctx, http.MethodGet, p.endpoint+"/v1/models?"+q.Encode(), nil, func(r *http.Request) {
			r.Header.Set("Accept", "application/json")
			p.headers(r)
		})
		if err != nil {
			return nil, err
		}
		var list struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		err = decodeBounded(resp.Body, &list)
		resp.Body.Close()
		if err != nil {
			return nil, p.scrubErr(err)
		}
		for _, d := range list.Data {
			ids = append(ids, d.ID)
		}
		if !list.HasMore || list.LastID == "" || list.LastID == after {
			break
		}
		after = list.LastID
	}
	return sortedUnique(ids), nil
}

type anHandler struct {
	blocks map[int]*anBlockState
	usage  Usage
	stop   string
}

type anBlockState struct {
	tool bool
	id   string
	name string
	args strings.Builder
}

type anEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens  *int `json:"input_tokens"`
		OutputTokens int  `json:"output_tokens"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`
}

func (h *anHandler) event(ev sseEvent, em *emitter) (bool, error) {
	var e anEvent
	if err := json.Unmarshal(ev.Data, &e); err != nil {
		return false, fmt.Errorf("llm: malformed stream event: %w", err)
	}
	if e.Type == "" {
		e.Type = ev.Name
	}
	switch e.Type {
	case "message_start":
		h.usage.InputTokens = e.Message.Usage.InputTokens
		h.usage.OutputTokens = e.Message.Usage.OutputTokens
	case "content_block_start":
		if h.blocks == nil {
			h.blocks = map[int]*anBlockState{}
		}
		b := &anBlockState{tool: e.ContentBlock.Type == "tool_use", id: e.ContentBlock.ID, name: e.ContentBlock.Name}
		h.blocks[e.Index] = b
		if e.ContentBlock.Type == "text" && e.ContentBlock.Text != "" {
			return false, em.send(Event{Kind: EventText, Text: e.ContentBlock.Text})
		}
	case "content_block_delta":
		switch e.Delta.Type {
		case "text_delta":
			if e.Delta.Text != "" {
				return false, em.send(Event{Kind: EventText, Text: e.Delta.Text})
			}
		case "thinking_delta":
			if e.Delta.Thinking != "" {
				return false, em.send(Event{Kind: EventReasoning, Text: e.Delta.Thinking})
			}
		case "input_json_delta":
			if b := h.blocks[e.Index]; b != nil {
				b.args.WriteString(e.Delta.PartialJSON)
			}
		}
	case "content_block_stop":
		b := h.blocks[e.Index]
		delete(h.blocks, e.Index)
		if b == nil || !b.tool {
			return false, nil
		}
		args, err := validateArgs(b.args.String())
		if err != nil {
			return false, fmt.Errorf("tool call %q: %w", b.name, err)
		}
		if b.name == "" || b.id == "" {
			return false, errors.New("llm: tool_use block without id or name")
		}
		return false, em.send(Event{Kind: EventToolCall, ToolCall: &ToolCall{ID: b.id, Name: b.name, Arguments: args}})
	case "message_delta":
		if e.Delta.StopReason != "" {
			h.stop = e.Delta.StopReason
		}
		if e.Usage.InputTokens != nil && *e.Usage.InputTokens > 0 {
			h.usage.InputTokens = *e.Usage.InputTokens
		}
		if e.Usage.OutputTokens > 0 {
			h.usage.OutputTokens = e.Usage.OutputTokens
		}
	case "message_stop":
		return true, em.send(Event{Kind: EventDone, Usage: h.usage, StopReason: h.stop})
	case "error":
		return false, errors.New("llm: provider error: " + errorMessage(e.Error))
	}
	return false, nil
}

func (h *anHandler) eof(*emitter) error {
	return errors.New("llm: stream ended before message_stop")
}
