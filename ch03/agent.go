package ch03

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"babyagent/ch03/tool"
	"babyagent/shared"
)

type Agent struct {
	systemPrompt string
	model        string
	client       openai.Client
	messages     []openai.ChatCompletionMessageParamUnion
	tools        map[tool.AgentTool]tool.Tool
}

func NewAgent(modelConf shared.ModelConfig, systemPrompt string, tools []tool.Tool) *Agent {
	a := Agent{
		systemPrompt: systemPrompt,
		model:        modelConf.Model,
		client:       openai.NewClient(option.WithBaseURL(modelConf.BaseURL), option.WithAPIKey(modelConf.ApiKey)),
		tools:        make(map[tool.AgentTool]tool.Tool),
		messages:     []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)},
	}
	for _, t := range tools {
		a.tools[t.ToolName()] = t
	}
	return &a
}

func (a *Agent) execute(ctx context.Context, toolName string, argumentsInJSON string) (string, error) {
	t, ok := a.tools[toolName]
	if ok {
		return t.Execute(ctx, argumentsInJSON)
	}
	return "", errors.New("tool not found")
}

func (a *Agent) buildTools() []openai.ChatCompletionToolUnionParam {
	tools := make([]openai.ChatCompletionToolUnionParam, 0)
	for _, t := range a.tools {
		tools = append(tools, t.Info())
	}
	return tools
}

func (a *Agent) ResetSession() {
	a.messages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(a.systemPrompt)}
}

// RunStreaming 和 Run 基本逻辑一致，但是使用流式请求，并且通过 channel 实现流式输出
func (a *Agent) RunStreaming(ctx context.Context, query string, viewCh chan MessageVO) error {
	// 为本轮次创建新的消息链。这样如果流式过程中失败或者终止了，不会污染历史上下文。
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(a.messages))
	messages = append(messages, a.messages...)
	messages = append(messages, openai.UserMessage(query))

	for {
		params := openai.ChatCompletionNewParams{
			Model:    a.model,
			Messages: messages,
			Tools:    a.buildTools(),
		}

		log.Printf("calling llm model %s...", a.model)
		stream := a.client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}

		var reasoningBuf strings.Builder

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if len(chunk.Choices) > 0 {
				deltaRaw := chunk.Choices[0].Delta
				// 不同厂商会把推理内容放在 reasoning_content、reasoning 或 thinking 字段里。
				delta, err := parseDeltaWithReasoning(deltaRaw.RawJSON())
				if err != nil {
					log.Printf("parse delta failed, raw=%s, err=%v", deltaRaw.RawJSON(), err)
					continue
				}
				reasoningContent := delta.ReasoningText()
				reasoningBuf.WriteString(reasoningContent)
				if reasoningContent != "" {
					viewCh <- MessageVO{
						Type:             MessageTypeReasoning,
						ReasoningContent: &reasoningContent,
					}
				}
				if delta.Content != "" {
					content := delta.Content
					viewCh <- MessageVO{
						Type:    MessageTypeContent,
						Content: &content,
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			viewCh <- MessageVO{
				Type:    MessageTypeError,
				Content: shared.Ptr(err.Error()),
			}
			return err
		}

		if len(acc.Choices) == 0 {
			log.Printf("no choices returned, resp: %v", acc)
			return nil
		}
		message := acc.Choices[0].Message
		messageParam := message.ToParam()
		// 拼接 assistant message 到整体消息链中
		messages = append(messages, messageParam)

		// tool loop 结束，可以返回结果
		if len(message.ToolCalls) == 0 {
			break
		}

		// 若使用了 DeepSeek API 且发生了工具调用，则需将 reasoning_content 回传
		if reasoningBuf.Len() > 0 {
			messageParam.OfAssistant.SetExtraFields(map[string]any{
				"reasoning_content": reasoningBuf.String(),
			})
		}

		for _, toolCall := range message.ToolCalls {

			viewCh <- MessageVO{
				Type: MessageTypeToolCall,
				ToolCall: &ToolCallVO{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}

			toolResult, err := a.execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				toolResult = err.Error()

				viewCh <- MessageVO{
					Type:    MessageTypeError,
					Content: &toolResult,
				}

			}
			log.Printf("tool call %s, arguments %s, error: %v", toolCall.Function.Name, toolCall.Function.Arguments, err)
			// 返回 tool message 到整体消息链中
			messages = append(messages, openai.ToolMessage(toolResult, toolCall.ID))
		}
	}
	// 轮次正常结束，agent 保存当前最新的消息链状态
	a.messages = messages
	return nil
}

type deltaWithReasoning struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Reasoning        string `json:"reasoning"`
	Thinking         string `json:"thinking"`
}

func parseDeltaWithReasoning(rawJSON string) (deltaWithReasoning, error) {
	delta := deltaWithReasoning{}
	err := json.Unmarshal([]byte(rawJSON), &delta)
	return delta, err
}

func (d deltaWithReasoning) ReasoningText() string {
	switch {
	case d.ReasoningContent != "":
		return d.ReasoningContent
	case d.Reasoning != "":
		return d.Reasoning
	default:
		return d.Thinking
	}
}
