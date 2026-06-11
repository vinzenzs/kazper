package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Config carries the chat runtime knobs, sourced from the server config.
type Config struct {
	Model              string
	MaxToolRounds      int
	MaxHistoryMessages int
	RequestTimeout     time.Duration
	DietaryPreferences string
	Timezone           string
	// BaseURL overrides the Anthropic endpoint for fixture tests.
	BaseURL string
}

// Service runs the server-side chat agent loop. It is constructed only when an
// Anthropic API key is present; the handler returns 503 when it is nil.
type Service struct {
	client     *client
	dispatcher *dispatcher
	cfg        Config
}

// New builds the chat Service. Returns ErrAPIKeyMissing when apiKey is empty so
// the caller can leave the Service nil and surface 503 chat_unavailable.
func New(apiKey string, cfg Config) (*Service, error) {
	c, err := newClient(clientConfig{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: cfg.RequestTimeout,
	})
	if err != nil {
		return nil, err
	}
	if cfg.MaxToolRounds <= 0 {
		cfg.MaxToolRounds = 8
	}
	if cfg.MaxHistoryMessages <= 0 {
		cfg.MaxHistoryMessages = 40
	}
	return &Service{client: c, cfg: cfg}, nil
}

// SetLoopbackHandler wires the in-process HTTP handler (the Gin engine) the tool
// dispatcher calls. Set after the engine is built, since the chat handler is
// itself registered on that engine.
func (s *Service) SetLoopbackHandler(h http.Handler) {
	s.dispatcher = newDispatcher(h)
}

// stream runs the agent loop for one request, writing SSE events. inbound is the
// validated client transcript; bearer is the caller's token, forwarded to tools.
func (s *Service) stream(ctx context.Context, sse *sseWriter, inbound []InboundMessage, bearer string) {
	specs := registry()
	toolDefs := anthropicToolDefs(specs)
	system := buildSystemPrompt(promptParams{
		DietaryPreferences: s.cfg.DietaryPreferences,
		Timezone:           s.cfg.Timezone,
	})

	messages := s.initialMessages(inbound)
	var full strings.Builder
	var usage Usage
	rounds := 0

	for {
		withTools := rounds < s.cfg.MaxToolRounds
		req := messagesRequest{
			Model:     s.cfg.Model,
			MaxTokens: maxTokensPerTurn,
			System:    system,
			Messages:  messages,
		}
		if withTools {
			req.Tools = toolDefs
		}

		turn, err := s.client.stream(ctx, req, func(delta string) {
			full.WriteString(delta)
			sse.text(delta)
		})
		if err != nil {
			s.emitStreamError(sse, ctx, err)
			return
		}
		usage = turn.Usage

		// Terminal: no client tools requested, or tools were withheld this turn.
		if !withTools || len(turn.ClientToolCalls) == 0 {
			stop := turn.StopReason
			if !withTools && rounds >= s.cfg.MaxToolRounds {
				stop = "max_tool_rounds"
			}
			sse.done(full.String(), stop, usage)
			return
		}

		// Echo the assistant turn (with its tool_use blocks) and dispatch tools.
		messages = append(messages, anthropicMessage{
			Role:    "assistant",
			Content: marshalBlocks(turn.AssistantContent),
		})
		resultBlocks := make([]json.RawMessage, 0, len(turn.ClientToolCalls))
		for _, call := range turn.ClientToolCalls {
			sse.tool(call.Name, "started", "running")
			res := s.dispatcher.execute(ctx, call.Name, call.Input, bearer)
			resultBlocks = append(resultBlocks, toolResultBlock(call.ID, res))
			name, status, summary := toolEventFields(call.Name, res)
			sse.tool(name, status, summary)
		}
		messages = append(messages, anthropicMessage{
			Role:    "user",
			Content: marshalBlocks(resultBlocks),
		})
		rounds++
	}
}

// initialMessages converts the inbound transcript into Anthropic messages,
// truncated to the most recent MaxHistoryMessages entries. Each content string
// is JSON-encoded into the Content raw field.
func (s *Service) initialMessages(inbound []InboundMessage) []anthropicMessage {
	if len(inbound) > s.cfg.MaxHistoryMessages {
		inbound = inbound[len(inbound)-s.cfg.MaxHistoryMessages:]
	}
	out := make([]anthropicMessage, 0, len(inbound))
	for _, m := range inbound {
		content, _ := json.Marshal(m.Content)
		out = append(out, anthropicMessage{Role: m.Role, Content: content})
	}
	return out
}

func (s *Service) emitStreamError(sse *sseWriter, ctx context.Context, err error) {
	if ctx.Err() == context.DeadlineExceeded {
		sse.error("timeout", "the request timed out")
		return
	}
	var up *ErrUpstreamUnavailable
	var proto *ErrUpstreamProtocol
	if errors.As(err, &up) || errors.As(err, &proto) {
		sse.error("upstream_unavailable", "the language model is temporarily unavailable")
		return
	}
	sse.error("upstream_unavailable", err.Error())
}

// marshalBlocks renders a slice of content-block JSON values as a JSON array.
func marshalBlocks(blocks []json.RawMessage) json.RawMessage {
	if len(blocks) == 0 {
		return json.RawMessage("[]")
	}
	raw, _ := json.Marshal(blocks)
	return raw
}

// toolResultBlock builds the tool_result content block fed back to the model.
// The REST response body becomes the tool result content; non-2xx and
// build-failures are marked is_error so the model can react.
func toolResultBlock(toolUseID string, res toolResult) json.RawMessage {
	var content string
	isError := false
	switch {
	case res.err != nil:
		content = "tool input error: " + res.err.Error()
		isError = true
	case !res.ok:
		content = string(res.body)
		isError = true
	default:
		content = string(res.body)
	}
	block := map[string]any{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"content":     content,
	}
	if isError {
		block["is_error"] = true
	}
	raw, _ := json.Marshal(block)
	return raw
}

// toolEventFields derives the SSE tool event fields from a result — name, a
// status of ok|error, and a short summary that never leaks the response body.
func toolEventFields(name string, res toolResult) (string, string, string) {
	switch {
	case res.err != nil:
		return name, "error", "invalid input"
	case !res.ok:
		return name, "error", fmt.Sprintf("failed (status %d)", res.status)
	default:
		return name, "ok", "done"
	}
}
