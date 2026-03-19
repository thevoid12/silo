package openaicompat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"google.golang.org/genai"
)

// contentsToMessages converts ADK genai.Content slice to OpenAI message params.
func contentsToMessages(contents []*genai.Content) []openai.ChatCompletionMessageParamUnion {
	var msgs []openai.ChatCompletionMessageParamUnion
	for _, c := range contents {
		if c == nil {
			continue
		}
		switch c.Role {
		case "user":
			msgs = append(msgs, buildUserMessage(c.Parts))
		case "model":
			msgs = append(msgs, buildAssistantMessage(c.Parts))
		case "tool", "function":
			for _, p := range c.Parts {
				if p != nil && p.FunctionResponse != nil {
					out, _ := json.Marshal(p.FunctionResponse.Response)
					msgs = append(msgs, openai.ToolMessage(string(out), p.FunctionResponse.ID))
				}
			}
		}
	}
	return msgs
}

func buildUserMessage(parts []*genai.Part) openai.ChatCompletionMessageParamUnion {
	var contentParts []openai.ChatCompletionContentPartUnionParam
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			contentParts = append(contentParts, openai.TextContentPart(p.Text))
		}
		if p.InlineData != nil {
			encoded := base64.StdEncoding.EncodeToString(p.InlineData.Data)
			dataURL := fmt.Sprintf("data:%s;base64,%s", p.InlineData.MIMEType, encoded)
			contentParts = append(contentParts, openai.ImageContentPart(
				openai.ChatCompletionContentPartImageImageURLParam{URL: dataURL},
			))
		}
	}
	if len(contentParts) == 1 {
		if text := contentParts[0].GetText(); text != nil {
			return openai.UserMessage(*text)
		}
	}
	return openai.UserMessage(contentParts)
}

func buildAssistantMessage(parts []*genai.Part) openai.ChatCompletionMessageParamUnion {
	var text string
	var toolCalls []openai.ChatCompletionMessageToolCallParam
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			text += p.Text
		}
		if p.FunctionCall != nil {
			args, _ := json.Marshal(p.FunctionCall.Args)
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
				ID: p.FunctionCall.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      p.FunctionCall.Name,
					Arguments: string(args),
				},
			})
		}
	}
	msg := openai.ChatCompletionAssistantMessageParam{}
	if text != "" {
		msg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: param.NewOpt(text),
		}
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return openai.ChatCompletionMessageParamUnion{OfAssistant: &msg}
}

// toolsToOpenAI converts ADK genai.Tool declarations to OpenAI tool params.
func toolsToOpenAI(tools []*genai.Tool) []openai.ChatCompletionToolParam {
	var out []openai.ChatCompletionToolParam
	for _, t := range tools {
		for _, fd := range t.FunctionDeclarations {
			out = append(out, openai.ChatCompletionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        fd.Name,
					Description: param.NewOpt(fd.Description),
					Parameters:  shared.FunctionParameters(schemaToMap(fd.Parameters)),
				},
			})
		}
	}
	return out
}

// schemaToMap converts a genai.Schema to a plain map for OpenAI function parameters.
func schemaToMap(s *genai.Schema) map[string]any {
	if s == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// assembleContent builds a genai.Content from accumulated stream values.
func assembleContent(text string, toolCalls []bufferedToolCall) *genai.Content {
	var parts []*genai.Part
	if text != "" {
		parts = append(parts, &genai.Part{Text: text})
	}
	for _, tc := range toolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.arguments), &args)
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.id,
				Name: tc.name,
				Args: args,
			},
		})
	}
	if len(parts) == 0 {
		parts = append(parts, &genai.Part{Text: ""})
	}
	return &genai.Content{Role: "model", Parts: parts}
}

type bufferedToolCall struct {
	id        string
	name      string
	arguments string
}
