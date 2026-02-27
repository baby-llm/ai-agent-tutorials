package ch02

import (
	"context"
	"errors"
	"log"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"babyagent/ch02/tool"
)

type Agent struct {
	systemPrompt string
	model        string
	client       openai.Client
	tools        map[tool.AgentTool]tool.Tool
}

func NewAgent(modelConf ModelConfig, systemPrompt string, tools []tool.Tool) *Agent {
	a := Agent{
		systemPrompt: systemPrompt,
		model:        modelConf.Model,
		client:       openai.NewClient(option.WithBaseURL(modelConf.BaseURL), option.WithAPIKey(modelConf.ApiKey)),
		tools:        make(map[tool.AgentTool]tool.Tool),
	}
	for _, t := range tools {
		a.tools[t.ToolName()] = t
	}
	return &a
}

func (a *Agent) execute(ctx context.Context, toolName string, argumentsInJSON string) (string, error) {
	t, ok := a.tools[tool.AgentTool(toolName)]
	if !ok {
		return "", errors.New("tool not found")
	}
	return t.Execute(ctx, argumentsInJSON)
}

func (a *Agent) Run(ctx context.Context, query string) (string, error) {

	messages := make([]openai.ChatCompletionMessageParamUnion, 0)
	messages = append(messages, openai.SystemMessage(a.systemPrompt), openai.UserMessage(query))

	var result string
	for {
		params := openai.ChatCompletionNewParams{
			Model:    a.model,
			Messages: messages,
			Tools:    make([]openai.ChatCompletionToolUnionParam, 0),
		}

		for _, t := range a.tools {
			params.Tools = append(params.Tools, t.Info())
		}

		log.Printf("calling llm model %s...", a.model)
		resp, err := a.client.Chat.Completions.New(ctx, params)
		if err != nil {
			log.Fatalf("failed to send a new completion request: %v", err)
			return "", err
		}
		if len(resp.Choices) == 0 {
			log.Printf("no choices returned, resp: %v", resp)
			return "", nil
		}
		message := resp.Choices[0].Message

		// tool loop 结束，可以返回结果
		if len(message.ToolCalls) == 0 {
			result = message.Content
			break
		}

		// 拼接 assistant message 到整体消息链中
		messages = append(messages, message.ToParam())

		for _, toolCall := range message.ToolCalls {
			toolResult, err := a.execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				toolResult = err.Error()
			}
			log.Printf("tool call %s, arguments %s, error: %v", toolCall.Function.Name, toolCall.Function.Arguments, err)
			// 返回 tool message 到整体消息链中
			messages = append(messages, openai.ToolMessage(toolResult, toolCall.ID))
		}

	}
	return result, nil
}
