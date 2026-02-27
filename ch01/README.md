# 第一章：初识 LLM（Raw HTTP 与 OpenAI SDK）

欢迎来到第一章！作为后端工程师，我们每天都在和各种 API 打交道。AI 大模型的调用本质上也是一次 HTTP 请求。

本章的目标是带你**拨开 SDK 的迷雾，直视大模型调用的本质**。我们将分别使用 Go 标准库 `net/http`（原生手写请求）和官方 `openai-go` SDK 来完成同一个任务：向 LLM 发起对话。这不仅能帮助你深刻理解 OpenAI 协议与 SSE（流式输出）机制，更为后续开发健壮的 AI Agent 奠定坚实的基础。

---

## 🎯 你将学到什么

1. **协议本质**：掌握 `chat/completions` 接口的最小化请求和响应结构。
2. **流式输出解析**：深入了解如何解析 Server-Sent Events (SSE) 协议，实现“打字机”效果。
3. **工程实践**：对比 `Raw HTTP` 和 `OpenAI Go SDK` 的使用方式。

---

## 🛠 准备工作

在开始之前，请确保你已经准备好了环境配置。我们在项目根目录使用了 `.env` 来管理敏感信息（`main.go` 中通过 `godotenv.Load()` 自动读取）。

在项目根目录创建 `.env` 文件，并填入以下内容：

```env
# 如果你使用的是国内模型（如 DeepSeek/GLM），请修改 Base URL 和 Model
OPENAI_API_KEY=sk-your-api-key-here
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini
```

> **💡 小贴士**：得益于 OpenAI 协议的非正式标准化，只需更改 `BASE_URL` 和 `MODEL`，同一套代码就能无缝切换到绝大多数主流大模型厂商。

---

## 📖 核心原理解析

### 1. 最小请求结构 (Request)

发起一次对话的核心路径是：`POST {OPENAI_BASE_URL}/chat/completions`

我们来看 `raw.go` 中定义的结构体，这就是发送给大模型的核心数据：

```go
type OpenAIChatCompletionRequest struct {
	Model    string           `json:"model"`    // 例如："gpt-4o-mini"
	Messages []RequestMessage `json:"messages"` // 对话上下文历史
	Stream   bool             `json:"stream"`   // 是否开启流式增量返回
}
```
*   `messages` 是一个数组，大模型本身是**无记忆**的，你需要把之前的聊天记录一并传给它，这就是所谓的“上下文”。
*   `stream` 是控制体验的关键，开启后将大幅降低首字响应时间 (TTFT)。

### 2. 标准与流式响应的区别 (Response vs SSE)

**非流式模式 (`Stream: false`)**
模型会思考完毕后，一次性返回一个巨大的 JSON。在 `raw.go` 中我们解析 `OpenAIChatCompletionResponse`：
你需要提取的内容在 `choices[0].message.content` 中。同时，`usage` 字段会告诉你这次调用花了多少 Token。

**流式模式 (`Stream: true`)**
这是主流 AI 产品的标准体验。模型会通过 **SSE (Server-Sent Events)** 协议，像水流一样逐字吐出数据。

如果你使用原生 HTTP 请求（见 `raw.go` 中的 `StreamingRequestRawHTTP`），你会看到服务端返回的是这样逐行的纯文本：
```text
data: {"choices":[{"delta":{"content":"Hi"}}]}
data: {"choices":[{"delta":{"content":"!"}}]}
data: [DONE]
```
在 Go 中，我们通常使用 `bufio.NewScanner(httpResp.Body)` 来逐行读取，遇到 `data: [DONE]` 标志着生成结束。你需要将每个 chunk 中 `delta.content` 的内容拼接起来。

---

## 💻 代码实现对比

本项目提供了两套完整的实现，通过命令行参数进行切换。

### 方式一：使用官方 SDK (推荐在生产使用)
在 `sdk.go` 中，我们使用了官方的 `github.com/openai/openai-go`。代码非常简洁：
```go
// 流式调用的核心逻辑
stream := client.Chat.Completions.NewStreaming(ctx, req)
for stream.Next() {
    chunk := stream.Current()
    log.Printf("stream chunk: %v", chunk)
}
```
SDK 帮我们封装了底层的 HTTP 请求、JSON 序列化、SSE 解析以及错误重试逻辑。在后续章节中，我们将统一采用这种方式。

### 方式二：Raw HTTP 手写调用 (推荐学习原理)
在 `raw.go` 中，我们使用 Go 原生的 `net/http` 发起调用。这对于排查网络问题、理解底层通信机制、或者在不方便引入庞大 SDK 的极简项目中非常有帮助。

---

## 🚀 动手运行

你可以通过命令行标志（Flags）随意组合调用方式。进入项目根目录后，执行以下命令：

**1. 基础调用（SDK + 非流式，默认）**
```bash
go run ./ch01 -q "讲一个关于程序员的冷笑话"
```

**2. 体验流式输出（SDK + 流式）**
注意观察控制台日志，体会 `chunk` 是一块块返回的：
```bash
go run ./ch01 --stream -q "用 Go 语言写一个 Hello World"
```

**3. 探索底层原理（Raw HTTP + 非流式）**
```bash
go run ./ch01 --raw -q "什么是大语言模型？用一句话解释"
```

**4. 终极挑战：手写 SSE 解析（Raw HTTP + 流式）**
```bash
go run ./ch01 --raw --stream -q "从 1 数到 5"
```

---

## 📚 扩展阅读与参考资料

为了更深入地理解本章涉及的内容，强烈推荐阅读以下官方文档：

1.  **[OpenAI API Reference - Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)**
   *   了解所有可用的请求参数（如 `temperature`, `top_p`, `presence_penalty` 等），以及完整的响应数据结构。这是构建任何基于 OpenAI 协议的 Agent 的基石。
2.  **[MDN Web Docs - Server-sent events (SSE)](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)**
   *   虽然这里展示的是 Web 前端的文档，但 SSE 协议的本质是一样的。了解 SSE 协议的标准格式（如 `data:`, `event:`, `id:` 等），有助于你理解大模型流式输出的底层机制。
3.  **[openai-go SDK GitHub Repository](https://github.com/openai/openai-go)**
   *   官方 Go SDK 的源码仓库。如果你想了解 SDK 是如何封装重试逻辑、处理错误回调的，阅读其源码是非常好的学习途径。
4.  **[DeepSeek API Docs](https://api-docs.deepseek.com//)** (或者你常用的国内模型文档)
   *   看看其他厂商是如何实现兼容 OpenAI 格式的 API 接口的，通常只需要替换 Base URL 和模型名即可。
