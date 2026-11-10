// Package llm defines the streaming model seam and the DeepSeek-compatible
// provider, mirroring packages/llm.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Message is one chat message sent to a provider.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is one model call.
type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

// Chunk is one streamed piece of an assistant response.
type Chunk struct {
	Text     string `json:"text,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Finished bool   `json:"finished,omitempty"`
}

// Provider streams one completion, calling emit for every chunk.
type Provider interface {
	Stream(ctx context.Context, request Request, emit func(Chunk) error) error
}

// ErrMissingCredential reports an absent API key before any network call.
var ErrMissingCredential = errors.New("llm: no API key configured")

// DeepSeek talks to a DeepSeek-compatible OpenAI-style chat completions
// endpoint over server-sent events.
type DeepSeek struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewDeepSeek builds a provider, defaulting to the GLKJ placeholder endpoint.
func NewDeepSeek(baseURL, apiKey string) *DeepSeek {
	if baseURL == "" {
		baseURL = "https://api.glkj.example.com"
	}
	return &DeepSeek{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Client: http.DefaultClient}
}

type chatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	Stream    bool      `json:"stream"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Stream posts the request and decodes the SSE stream into chunks.
func (d *DeepSeek) Stream(ctx context.Context, request Request, emit func(Chunk) error) error {
	if d.APIKey == "" {
		return ErrMissingCredential
	}
	body, err := json.Marshal(chatRequest{Model: request.Model, Messages: request.Messages, MaxTokens: request.MaxTokens, Stream: true})
	if err != nil {
		return fmt.Errorf("llm: encode request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, d.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm: build request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+d.APIKey)

	response, err := d.Client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("llm: request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("llm: %s: %s", response.Status, strings.TrimSpace(string(message)))
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return emit(Chunk{Finished: true})
		}
		var parsed chatChunk
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			return fmt.Errorf("llm: decode chunk: %w", err)
		}
		for _, choice := range parsed.Choices {
			if choice.Delta.Content != "" {
				if err := emit(Chunk{Text: choice.Delta.Content}); err != nil {
					return err
				}
			}
			if choice.FinishReason != "" {
				if err := emit(Chunk{Reason: choice.FinishReason, Finished: true}); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("llm: read stream: %w", err)
	}
	return nil
}
