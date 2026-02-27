package main

import (
	"context"
	"flag"
	"log"

	"github.com/joho/godotenv"

	"babyagent/ch02"
	"babyagent/ch02/tool"
)

func main() {
	_ = godotenv.Load()

	query := flag.String("q", "hello", "prompt text")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	modelConf := ch02.NewModelConfig()

	agent := ch02.NewAgent(modelConf, ch02.CodingAgentSystemPrompt, []tool.Tool{
		tool.NewReadTool(),
		tool.NewEditTool(),
		tool.NewWriteTool(),
		tool.NewBashTool(),
	})
	result, err := agent.Run(ctx, *query)
	if err != nil {
		log.Printf("agent run error: %v", err)
		return
	}

	log.Printf("agent result: %s", result)
}
