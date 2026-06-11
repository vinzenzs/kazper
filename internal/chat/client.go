package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	defaultModel     = "claude-sonnet-4-6"
	anthropicVersion = "2023-06-01"

	// Version is baked into the User-Agent header.
	Version = "0.1.0"

	// maxTokensPerTurn caps each upstream turn's output. Generous enough for a
	// few recommendation options with macros; bounds runaway generation.
	maxTokensPerTurn = 2048
)

// clientConfig configures the Anthropic streaming client.
type clientConfig struct {
	APIKey  string
	BaseURL string        // overridable for fixture tests
	Model   string        // overridable for ops
	Timeout time.Duration // per-request; 0 means no client-level timeout (caller uses ctx)
}

// client streams from the Anthropic Messages API. Mirrors internal/vision.Client
// in shape but consumes the streaming (SSE) response.
type client struct {
	baseURL   string
	apiKey    string
	model     string
	userAgent string
	http      *http.Client
}

func newClient(cfg clientConfig) (*client, error) {
	if cfg.APIKey == "" {
		return nil, ErrAPIKeyMissing
	}
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = defaultModel
	}
	return &client{
		baseURL:   base,
		apiKey:    cfg.APIKey,
		model:     model,
		userAgent: fmt.Sprintf("nutrition-chat/%s", Version),
		http:      &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// stream POSTs one turn and parses the SSE response, invoking onText for each
// visible text delta. It returns the finalized turn (content blocks to echo,
// pending client tool calls, stop_reason, usage).
func (c *client) stream(ctx context.Context, req messagesRequest, onText func(string)) (*turnResult, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("chat: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("chat: build request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	httpReq.Header.Set("user-agent", c.userAgent)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &ErrUpstreamUnavailable{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain a little for logs; map any non-200 to upstream_unavailable.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &ErrUpstreamUnavailable{StatusCode: resp.StatusCode}
	}

	return parseSSE(resp.Body, onText)
}

// blockBuilder accumulates one content block from its start + deltas.
type blockBuilder struct {
	typ       string
	id        string
	name      string
	textBuf   strings.Builder
	jsonBuf   strings.Builder // input_json_delta accumulation for tool_use blocks
	rawStart  json.RawMessage // the content_block object from content_block_start
}

// parseSSE consumes the Anthropic event stream and reconstructs the turn.
func parseSSE(r io.Reader, onText func(string)) (*turnResult, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	builders := map[int]*blockBuilder{}
	order := []int{}
	result := &turnResult{}

	var dataLine strings.Builder
	flush := func() error {
		if dataLine.Len() == 0 {
			return nil
		}
		raw := dataLine.String()
		dataLine.Reset()
		return handleSSEData(raw, builders, &order, result, onText)
	}

	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "data:"):
			dataLine.WriteString(strings.TrimSpace(line[len("data:"):]))
		default:
			// `event:` and other field lines are ignored; the data payload
			// carries its own "type" field which is what we switch on.
		}
	}
	if err := sc.Err(); err != nil {
		return nil, &ErrUpstreamUnavailable{Err: err}
	}
	if err := flush(); err != nil {
		return nil, err
	}

	// Finalize content blocks in arrival order.
	var text strings.Builder
	for _, idx := range order {
		b := builders[idx]
		raw, call := b.finalize()
		if b.typ == "text" {
			text.WriteString(b.textBuf.String())
		}
		if raw != nil {
			result.AssistantContent = append(result.AssistantContent, raw)
		}
		if call != nil {
			result.ClientToolCalls = append(result.ClientToolCalls, *call)
		}
	}
	result.Text = text.String()
	return result, nil
}

// handleSSEData processes one decoded `data:` JSON payload.
func handleSSEData(raw string, builders map[int]*blockBuilder, order *[]int, result *turnResult, onText func(string)) error {
	var env struct {
		Type         string          `json:"type"`
		Index        int             `json:"index"`
		ContentBlock json.RawMessage `json:"content_block"`
		Delta        json.RawMessage `json:"delta"`
		Message      json.RawMessage `json:"message"`
		Usage        *Usage          `json:"usage"`
		Error        *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		// Tolerate the occasional non-JSON keepalive line.
		return nil
	}

	switch env.Type {
	case "error":
		msg := "unknown"
		if env.Error != nil {
			msg = env.Error.Type + ": " + env.Error.Message
		}
		return &ErrUpstreamProtocol{Msg: msg}

	case "message_start":
		// Capture input token usage from the opening message.
		var m struct {
			Message struct {
				Usage Usage `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(raw), &m) == nil {
			result.Usage.InputTokens = m.Message.Usage.InputTokens
		}

	case "content_block_start":
		var cb struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(env.ContentBlock, &cb)
		builders[env.Index] = &blockBuilder{
			typ:      cb.Type,
			id:       cb.ID,
			name:     cb.Name,
			rawStart: env.ContentBlock,
		}
		*order = append(*order, env.Index)

	case "content_block_delta":
		b := builders[env.Index]
		if b == nil {
			return nil
		}
		var d struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		}
		_ = json.Unmarshal(env.Delta, &d)
		switch d.Type {
		case "text_delta":
			b.textBuf.WriteString(d.Text)
			if onText != nil && d.Text != "" {
				onText(d.Text)
			}
		case "input_json_delta":
			b.jsonBuf.WriteString(d.PartialJSON)
		}

	case "message_delta":
		var m struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage *Usage `json:"usage"`
		}
		if json.Unmarshal([]byte(raw), &m) == nil {
			if m.Delta.StopReason != "" {
				result.StopReason = m.Delta.StopReason
			}
			if m.Usage != nil {
				result.Usage.OutputTokens = m.Usage.OutputTokens
			}
		}

	case "content_block_stop", "message_stop", "ping":
		// no-op; finalization happens after the stream ends
	}
	return nil
}

// finalize turns the accumulated block into the JSON to echo back, and (for
// client tool_use blocks) a clientToolCall to dispatch.
func (b *blockBuilder) finalize() (json.RawMessage, *clientToolCall) {
	switch b.typ {
	case "text":
		obj := map[string]any{"type": "text", "text": b.textBuf.String()}
		raw, _ := json.Marshal(obj)
		return raw, nil
	case "tool_use":
		input := json.RawMessage("{}")
		if s := strings.TrimSpace(b.jsonBuf.String()); s != "" {
			input = json.RawMessage(s)
		}
		obj := map[string]any{"type": "tool_use", "id": b.id, "name": b.name, "input": input}
		raw, _ := json.Marshal(obj)
		return raw, &clientToolCall{ID: b.id, Name: b.name, Input: input}
	case "server_tool_use":
		input := json.RawMessage("{}")
		if s := strings.TrimSpace(b.jsonBuf.String()); s != "" {
			input = json.RawMessage(s)
		}
		obj := map[string]any{"type": "server_tool_use", "id": b.id, "name": b.name, "input": input}
		raw, _ := json.Marshal(obj)
		return raw, nil
	default:
		// Blocks delivered whole (e.g. web_search_tool_result) — echo verbatim.
		return b.rawStart, nil
	}
}
