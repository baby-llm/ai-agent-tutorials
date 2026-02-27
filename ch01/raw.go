package ch01

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type RequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	FinishReason string `json:"finish_reason"`

	ReasoningContent *string `json:"reasoning_content"` // vary by different model provider
	Reasoning        *string `json:"reasoning"`         // vary by different model provider
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIChatCompletionResponse struct {
	Choices []struct {
		Message ResponseMessage `json:"message"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

type OpenAIChatCompletionStreamChunk struct {
	Choices []struct {
		Delta ResponseMessage `json:"delta"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

type OpenAIChatCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []RequestMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

func NonStreamingRequestRawHTTP(ctx context.Context, modelConf ModelConfig, query string) {
	client := http.Client{}

	requestBody := OpenAIChatCompletionRequest{
		Messages: []RequestMessage{
			{Role: "user", Content: query},
		},
		Model:  modelConf.Model,
		Stream: false,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", modelConf.BaseURL), bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+modelConf.ApiKey)

	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Fatalf("failed to send http request: %v", err)
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		log.Fatalf("failed to send http request: %v", httpResp.StatusCode)
		return
	}

	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Fatalf("failed to read http response: %v", err)
		return
	}

	resp := OpenAIChatCompletionResponse{}
	if err := json.Unmarshal(respBodyBytes, &resp); err != nil {
		log.Fatalf("failed to unmarshal http response: %v", err)
		return
	}

	if len(resp.Choices) == 0 {
		log.Printf("no choices returned, resp: %v", resp)
		return
	}
	log.Printf("resp content: %s", resp.Choices[0].Message.Content)
	log.Printf("token usage: %+v", resp.Usage)
}

func StreamingRequestRawHTTP(ctx context.Context, modelConf ModelConfig, query string) {
	client := http.Client{}

	requestBody := OpenAIChatCompletionRequest{
		Messages: []RequestMessage{
			{Role: "user", Content: query},
		},
		Model:  modelConf.Model,
		Stream: true,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", modelConf.BaseURL), bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+modelConf.ApiKey)

	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Fatalf("failed to send http request: %v", err)
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		log.Fatalf("failed to send http request: %v", httpResp.StatusCode)
		return
	}

	scanner := bufio.NewScanner(httpResp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			v := strings.TrimPrefix(line, "data:")

			if strings.TrimSpace(v) == "[DONE]" {
				break
			}

			chunk := OpenAIChatCompletionStreamChunk{}
			if err := json.Unmarshal([]byte(v), &chunk); err != nil {
				log.Fatalf("failed to unmarshal chunk: %v", err)
				return
			}
			log.Printf("stream chunk: %s", v)
			if chunk.Usage != nil {
				log.Printf("token usage: %+v", chunk.Usage)
			}
		}
	}

	if scanner.Err() != nil {
		log.Fatalf("failed to read http response: %v", err)
		return
	}
}
