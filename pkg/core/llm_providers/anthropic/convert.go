package anthropic

import (
	"encoding/base64"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"google.golang.org/genai"
)

// contentsToMessages converts ADK genai.Content slice to Anthropic message params.
func contentsToMessages(contents []*genai.Content) []anthropic.MessageParam {
	var msgs []anthropic.MessageParam
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
			var blocks []anthropic.ContentBlockParamUnion
			for _, p := range c.Parts {
				if p != nil && p.FunctionResponse != nil {
					out, _ := json.Marshal(p.FunctionResponse.Response)
					blocks = append(blocks, anthropic.NewToolResultBlock(
						p.FunctionResponse.ID, string(out), false,
					))
				}
			}
			if len(blocks) > 0 {
				msgs = append(msgs, anthropic.MessageParam{
					Role:    anthropic.MessageParamRoleUser,
					Content: blocks,
				})
			}
		}
	}
	return msgs
}

func buildUserMessage(parts []*genai.Part) anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(p.Text))
		}
		if p.InlineData != nil {
			encoded := base64.StdEncoding.EncodeToString(p.InlineData.Data)
			blocks = append(blocks, anthropic.NewImageBlockBase64(p.InlineData.MIMEType, encoded))
		}
	}
	return anthropic.NewUserMessage(blocks...)
}

func buildAssistantMessage(parts []*genai.Part) anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(p.Text))
		}
		if p.FunctionCall != nil {
			blocks = append(blocks, anthropic.NewToolUseBlock(
				p.FunctionCall.ID, p.FunctionCall.Args, p.FunctionCall.Name,
			))
		}
	}
	return anthropic.NewAssistantMessage(blocks...)
}

// toolsToAnthropic converts ADK genai.Tool declarations to Anthropic tool params.
func toolsToAnthropic(tools []*genai.Tool) []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam
	for _, t := range tools {
		for _, fd := range t.FunctionDeclarations {
			schema := schemaToInputSchema(fd.Parameters)
			tp := anthropic.ToolParam{
				Name:        fd.Name,
				InputSchema: schema,
			}
			if fd.Description != "" {
				tp.Description = param.NewOpt(fd.Description)
			}
			out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
		}
	}
	return out
}

func schemaToInputSchema(s *genai.Schema) anthropic.ToolInputSchemaParam {
	if s == nil {
		return anthropic.ToolInputSchemaParam{
			Properties: map[string]any{},
		}
	}
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	props, _ := m["properties"]
	required, _ := m["required"].([]string)
	return anthropic.ToolInputSchemaParam{
		Properties: props,
		Required:   required,
	}
}

// messageToContent converts a completed Anthropic Message to a genai.Content.
func messageToContent(msg *anthropic.Message) *genai.Content {
	var parts []*genai.Part
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			if tb := block.AsText(); tb.Text != "" {
				parts = append(parts, &genai.Part{Text: tb.Text})
			}
		case "tool_use":
			tu := block.AsToolUse()
			var args map[string]any
			_ = json.Unmarshal(tu.Input, &args)
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   tu.ID,
					Name: tu.Name,
					Args: args,
				},
			})
		}
	}
	if len(parts) == 0 {
		parts = append(parts, &genai.Part{Text: ""})
	}
	return &genai.Content{Role: "model", Parts: parts}
}
