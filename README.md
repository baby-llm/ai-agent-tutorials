# 后端工程师的零基础 AI Agent 开发教程 (Go 语言版)

🚀 **从后端视角出发，用 Go 语言构建工业级 AI 智能体。**

本项目专为没有 LLM 背景但具备基础 Golang 经验的后端工程师设计。我们将跳过复杂的数学模型，直接进入工程实践，带你从零构建能够感知、决策并执行任务的 AI Agent。

---

## 🏗 核心技术栈

*   **Language:** Go 1.24+
*   **Concepts:** LLM, Function Calling, ReAct, RAG, MCP, Observability
*   **Key Skills:** 上下文管理、流式传输、工具调用、外部系统集成

---

## 🗺 学习路径图 (按知识循序渐进)

我们将按照以下阶段逐步深入，每一阶段都包含可运行的代码示例。

### 第一阶段：初识 LLM

*   **Hello LLM:** 发起你的第一次 Chat Completion 请求，理解基础参数。
*   **结构化 Prompt 设计 (The Go Way):** 后端开发不应该手写字符串拼接。我们要像管理模板一样管理 Prompt。

### 第二阶段：交互与体验

*   **流式输出 (SSE):** 像 ChatGPT 一样通过 Server-Sent Events 实现后端流式响应，优化用户感知的响应延迟 (TTFT)。

### 第三阶段：赋予 AI “手脚”

*   **Tool Use (Function Calling):** 让 LLM 学会调用你写的 Go 函数。
*   **ReAct 范式:** 深入理解“推理-行动”循环，让 Agent 具备自主拆解任务并使用工具的能力。

### 第四阶段：知识增强

*   **基础 RAG:** 理解向量数据库，实现简单的“文档检索 + 生成”。
*   **Agentic RAG:** 当检索本身变成一个工具，让 Agent 自主决定何时去查文档、查什么文档。

### 第五阶段：上下文工程

*   **上下文工程:** 如何用有限的 Token 装载最多的有效信息？降本增效的秘籍——如何优化 Token 使用，实现高效的上下文缓存。

### 第六阶段：生态与标准化

*   **MCP (Model Context Protocol):** 开发符合 Anthropic 标准的 MCP Server，让你的后端能力可以被任意 AI 客户端（如 Claude Desktop）直接调用。

### 第七阶段：生产环境保障

*   **可观测性 (Observability):** 接入 Trace 和 Log，监控 Agent 的推理链路，定位“幻觉”发生的节点。

---

## 🛠 开发环境准备

1.  **安装 Go:** 确保本地已安装 Go 1.24 或更高版本。
2.  **获取 API Key:** 你需要一个 LLM 供应商的 API Key（如 OpenAI, 或国内的 DeepSeek/GLM）。
3.  **配置文件:**
    ```bash
    cp .env.example .env
    # 编辑 .env 文件，填入你的 API_KEY
    ```

---

## 🎯 为什么选择 Go 开发 Agent？

*   **类型安全:** 在处理复杂的 Tool 定义和 JSON 解析时，Go 的强类型能减少 80% 的运行时错误。
*   **并发优势:** Agent 往往需要并行调用多个工具或检索源，Go 的 Goroutine 是天然的利器。
*   **部署简单:** 无需复杂的 Python 依赖环境，单个二进制文件即可上线。

---

## 📂 项目结构说明

(待代码编写后补充具体目录树)

---

## 🤝 参与贡献

我们非常欢迎社区贡献！如果你有更好的 Agent 设计模式或有趣的工具实现，请随时提交 PR。

## 📄 开源协议

本项目采用 [Apache License 2.0](LICENSE) 协议。