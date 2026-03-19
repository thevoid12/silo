package anthropic

import (
	"context"
	"iter"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultMaxTokens = 8192

type anthropicModel struct {
	client    anthropicsdk.Client
	modelName string
}

// New creates an Anthropic model.LLM.
func New(modelName, apiKey string) model.LLM {
	return &anthropicModel{
		client:    anthropicsdk.NewClient(option.WithAPIKey(apiKey)),
		modelName: modelName,
	}
}

func (m *anthropicModel) Name() string { return m.modelName }

// GenerateContent implements model.LLM for Anthropic's native API.
func (m *anthropicModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		messages := contentsToMessages(req.Contents)

		params := anthropicsdk.MessageNewParams{
			Model:     m.modelName,
			MaxTokens: defaultMaxTokens,
			Messages:  messages,
		}
		if req.Config != nil && len(req.Config.Tools) > 0 {
			params.Tools = toolsToAnthropic(req.Config.Tools)
		}

		if !stream {
			m.generateNonStreaming(ctx, params, yield)
			return
		}
		m.generateStreaming(ctx, params, yield)
	}
}

func (m *anthropicModel) generateNonStreaming(ctx context.Context, params anthropicsdk.MessageNewParams, yield func(*model.LLMResponse, error) bool) {
	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		yield(nil, err)
		return
	}
	content := messageToContent(resp)
	yield(&model.LLMResponse{Content: content, TurnComplete: true}, nil)
}

func (m *anthropicModel) generateStreaming(ctx context.Context, params anthropicsdk.MessageNewParams, yield func(*model.LLMResponse, error) bool) {
	stream := m.client.Messages.NewStreaming(ctx, params)

	var acc anthropicsdk.Message
	for stream.Next() {
		event := stream.Current()
		if err := acc.Accumulate(event); err != nil {
			yield(nil, err)
			return
		}
		if event.Type == "content_block_delta" {
			delta := event.AsContentBlockDelta()
			if textDelta := delta.Delta.AsTextDelta(); textDelta.Text != "" {
				content := &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: textDelta.Text}},
				}
				if !yield(&model.LLMResponse{Content: content, Partial: true}, nil) {
					return
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		yield(nil, err)
		return
	}
	content := messageToContent(&acc)
	yield(&model.LLMResponse{Content: content, TurnComplete: true}, nil)
}
