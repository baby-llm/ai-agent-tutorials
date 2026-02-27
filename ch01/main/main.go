package main

import (
	"context"
	"flag"

	"github.com/joho/godotenv"

	"babyagent/ch01"
)

func main() {
	_ = godotenv.Load()

	useRaw := flag.Bool("raw", false, "use raw http implementation")
	useStream := flag.Bool("stream", false, "use streaming response")
	query := flag.String("q", "hello", "prompt text")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	modelConf := ch01.NewModelConfig()

	switch {
	case *useRaw && *useStream:
		ch01.StreamingRequestRawHTTP(ctx, modelConf, *query)
	case *useRaw:
		ch01.NonStreamingRequestRawHTTP(ctx, modelConf, *query)
	case *useStream:
		ch01.StreamingRequestSDK(ctx, modelConf, *query)
	default:
		ch01.NonStreamingRequestSDK(ctx, modelConf, *query)
	}
}
