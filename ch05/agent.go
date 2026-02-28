package ch05

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"babyagent/ch05/tool"
	"babyagent/shared"
)

type Agent struct {
	systemPrompt string
	model        string
	client       openai.Client
	messages     []openai.ChatCompletionMessageParamUnion
	nativeTools  map[tool.AgentTool]tool.Tool // agent 框架中原生实现的 tools
	mcpClients   map[string]*McpClient        // 集成 mcp 工具
}

func NewAgent(modelConf shared.ModelConfig, systemPrompt string, tools []tool.Tool, mcpClients []*McpClient) *Agent {
	a := Agent{
		systemPrompt: systemPrompt,
		model:        modelConf.Model,
		client:       openai.NewClient(option.WithBaseURL(modelConf.BaseURL), option.WithAPIKey(modelConf.ApiKey)),
		nativeTools:  make(map[tool.AgentTool]tool.Tool),
		mcpClients:   make(map[string]*McpClient),
		messages:     make([]openai.ChatCompletionMessageParamUnion, 0),
	}
	for _, t := range tools {
		a.nativeTools[t.ToolName()] = t
	}
	for _, mcpClient := range mcpClients {
		a.mcpClients[mcpClient.Name()] = mcpClient
	}
	a.messages = append(a.messages, openai.SystemMessage(a.buildSystemPrompt()))
	return &a
}

func (a *Agent) buildSystemPrompt() string {
	replaceMap := make(map[string]string)
	replaceMap["{runtime}"] = runtime.GOOS
	cwd, _ := os.Getwd()
	replaceMap["{workspace_path}"] = cwd

	prompt := a.systemPrompt
	for k, v := range replaceMap {
		prompt = strings.ReplaceAll(prompt, k, v)
	}

	return prompt
}

func (a *Agent) execute(ctx context.Context, toolName string, argumentsInJSON string) (string, error) {
	// 判断 native tool
	t, ok := a.nativeTools[toolName]
	if ok {
		return t.Execute(ctx, argumentsInJSON)
	}
	// 判断 MCP Tool
	for _, mcpClient := range a.mcpClients {
		for _, t := range mcpClient.GetTools() {
			if t.ToolName() != toolName {
				continue
			}
			return t.Execute(ctx, argumentsInJSON)
		}
	}
	return "", errors.New("tool not found")
}

func (a *Agent) buildTools() []openai.ChatCompletionToolUnionParam {
	tools := make([]openai.ChatCompletionToolUnionParam, 0)
	// 集成 mcp tools
	for _, t := range a.nativeTools {
		tools = append(tools, t.Info())
	}
	// 集成 mcp tools
	for _, mcpClient := range a.mcpClients {
		for _, t := range mcpClient.GetTools() {
			tools = append(tools, t.Info())
		}
	}
	return tools
}

func (a *Agent) ResetSession() {
	a.messages = make([]openai.ChatCompletionMessageParamUnion, 0)
	a.messages = append(a.messages, openai.SystemMessage(a.systemPrompt))
}

func (a *Agent) SessionSnapshot() int {
	return len(a.messages)
}

func (a *Agent) RestoreSession(snapshot int) {
	if snapshot < 1 {
		snapshot = 1
	}
	if snapshot > len(a.messages) {
		return
	}
	a.messages = append([]openai.ChatCompletionMessageParamUnion{}, a.messages[:snapshot]...)
}

// RunStreaming 和 Run 基本逻辑一致，但是使用流式请求，并且通过 channel 实现流式输出
func (a *Agent) RunStreaming(ctx context.Context, query string, viewCh chan MessageVO) error {
	a.messages = append(a.messages, openai.UserMessage(query))

	for {
		params := openai.ChatCompletionNewParams{
			Model:    a.model,
			Messages: a.messages,
			Tools:    a.buildTools(),
		}

		log.Printf("calling llm model %s...", a.model)
		stream := a.client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if len(chunk.Choices) > 0 {
				deltaRaw := chunk.Choices[0].Delta
				// 推理模型会返回 reasoning_content（有些模型使用 reasoning 字段）
				delta := deltaWithReasoning{}
				_ = json.Unmarshal([]byte(deltaRaw.RawJSON()), &delta)
				if reasoningContent := delta.ReasoningContent; reasoningContent != "" {
					viewCh <- MessageVO{
						Type:             MessageTypeReasoning,
						ReasoningContent: &reasoningContent,
					}
				}
				if delta.Content != "" {
					viewCh <- MessageVO{
						Type:    MessageTypeContent,
						Content: &chunk.Choices[0].Delta.Content,
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
		// 拼接 assistant message 到整体消息链中
		a.messages = append(a.messages, message.ToParam())

		// tool loop 结束，可以返回结果
		if len(message.ToolCalls) == 0 {
			break
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
			a.messages = append(a.messages, openai.ToolMessage(toolResult, toolCall.ID))
		}

	}
	return nil
}

type deltaWithReasoning struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}
