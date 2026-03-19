package openaicompat

import (
	"context"
	"iter"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// builtinBaseURLs maps known provider names to their OpenAI-compat endpoints.
var builtinBaseURLs = map[string]string{
	"openai":      "https://api.openai.com/v1",
	"gemini":      "https://generativelanguage.googleapis.com/v1beta/openai/",
	"openrouter":  "https://openrouter.ai/api/v1",
}

// BuiltinBaseURL returns the built-in base URL for the provider name, or "".
func BuiltinBaseURL(provider string) string {
	return builtinBaseURLs[provider]
}

type openAICompatModel struct {
	client    openai.Client
	modelName string
}

// New creates an OpenAI-compat model.LLM. baseURL overrides the provider default.
func New(modelName, apiKey, baseURL string) model.LLM {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &openAICompatModel{
		client:    openai.NewClient(opts...),
		modelName: modelName,
	}
}

func (m *openAICompatModel) Name() string { return m.modelName }

// GenerateContent implements model.LLM for OpenAI-compat providers.
func (m *openAICompatModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		messages := contentsToMessages(req.Contents)

		var tools []openai.ChatCompletionToolParam
		if req.Config != nil && len(req.Config.Tools) > 0 {
			tools = toolsToOpenAI(req.Config.Tools)
		}

		params := openai.ChatCompletionNewParams{
			Model:    m.modelName,
			Messages: messages,
		}
		if len(tools) > 0 {
			params.Tools = tools
		}

		if !stream {
			m.generateNonStreaming(ctx, params, yield)
			return
		}
		m.generateStreaming(ctx, params, yield)
	}
}

func (m *openAICompatModel) generateNonStreaming(ctx context.Context, params openai.ChatCompletionNewParams, yield func(*model.LLMResponse, error) bool) {
	resp, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		yield(nil, err)
		return
	}
	if len(resp.Choices) == 0 {
		yield(&model.LLMResponse{TurnComplete: true}, nil)
		return
	}
	choice := resp.Choices[0]
	var toolCalls []bufferedToolCall
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, bufferedToolCall{
			id:        tc.ID,
			name:      tc.Function.Name,
			arguments: tc.Function.Arguments,
		})
	}
	content := assembleContent(choice.Message.Content, toolCalls)
	yield(&model.LLMResponse{Content: content, TurnComplete: true}, nil)
}

func (m *openAICompatModel) generateStreaming(ctx context.Context, params openai.ChatCompletionNewParams, yield func(*model.LLMResponse, error) bool) {
	stream := m.client.Chat.Completions.NewStreaming(ctx, params)

	var accText string
	// index → bufferedToolCall (accumulate arguments incrementally)
	tcMap := map[int]*bufferedToolCall{}
	var tcOrder []int

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		// Accumulate text and emit partial token
		if delta.Content != "" {
			accText += delta.Content
			content := &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: delta.Content}},
			}
			if !yield(&model.LLMResponse{Content: content, Partial: true}, nil) {
				return
			}
		}

		// Accumulate tool call deltas (must not forward until complete)
		for _, tc := range delta.ToolCalls {
			idx := int(tc.Index)
			if _, ok := tcMap[idx]; !ok {
				tcMap[idx] = &bufferedToolCall{id: tc.ID, name: tc.Function.Name}
				tcOrder = append(tcOrder, idx)
			} else {
				// ID and name only come on first chunk for this index
				if tc.ID != "" {
					tcMap[idx].id = tc.ID
				}
				if tc.Function.Name != "" {
					tcMap[idx].name = tc.Function.Name
				}
			}
			tcMap[idx].arguments += tc.Function.Arguments
		}
	}

	if err := stream.Err(); err != nil {
		yield(nil, err)
		return
	}

	// Build ordered tool calls
	var toolCalls []bufferedToolCall
	for _, idx := range tcOrder {
		toolCalls = append(toolCalls, *tcMap[idx])
	}

	content := assembleContent(accText, toolCalls)
	yield(&model.LLMResponse{Content: content, TurnComplete: true}, nil)
}
