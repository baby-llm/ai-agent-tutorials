# 第三章：让 Agent “更能看见”（Reasoning 展示、TUI 与 MCP）

欢迎来到第三章！在第二章的基础上，本章继续完善一个更“可观察”的 Agent：

- **可视化展示推理与工具调用过程**（便于调试）
- **兼容推理模型的 `reasoning_content` 字段**
- **接入 MCP（Model Context Protocol）工具生态**

本章依旧保留最小 Agent Loop 的结构，但在“输出可观察性”和“工具生态扩展性”上向前迈了一步。

---

## 🎯 你将学到什么

1. **推理字段兼容处理**：如何从流式响应中解析 `reasoning_content`（以及不同厂商字段差异的处理思路）。
2. **TUI 可视化层**：用 Bubble Tea 实现一个轻量的可视化输出（这不是本课重点，仅为后续调试服务）。
3. **MCP 原理与接入**：理解 MCP 的基本角色与工具生命周期，并在 Agent 中同时管理 MCP 工具与本地工具。

---

## 🛠 准备工作

复用根目录的 `.env` 配置（见项目根目录 `README.md`）。

```env
OPENAI_API_KEY=sk-your-api-key-here
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-5.2
```

此外，本章默认在 `mcp-server.json` 中配置了一个 MCP 文件系统服务器（基于 `@modelcontextprotocol/server-filesystem`）。这需要本地能够运行 `npx`（通常意味着安装 Node.js）。

---

## 📖 核心原理解析

### 1. 推理模型的 `reasoning_content` 兼容处理


**推理模型 vs 非推理模型**

- 推理模型：强调“步骤化思考”，可能输出额外的推理字段，适合复杂任务、工具编排与多步规划，但输出更慢、成本更高。
- 非推理模型：更偏“直接生成”，通常只有最终内容字段，适合简单问答与低延迟场景。

部分“推理模型”在流式响应里会返回额外的推理字段，用于展示中间思考过程。**但不同厂商字段命名不一致**，常见情况包括：

- `reasoning_content`（本章直接处理）
- `reasoning` / `thinking` 等（需按供应商实际字段做兼容）

`ch03/agent.go` 中的 `RunStreaming` 通过 `RawJSON()` 解析增量消息，尝试抽取 `reasoning_content`，并将其单独作为 `MessageTypeReasoning` 发送给 TUI。这样就能把“推理过程”和“最终内容”分开显示，利于调试与对齐。

> 注意：本章只演示了 `reasoning_content` 的处理方式。如果你接入的是其他厂商或自建模型，需要根据其文档调整字段名和解析逻辑。

相关代码：`ch03/agent.go`

---

### 2. TUI 只是“可视化外壳”

在 `ch03/tui/main.go` 中，使用 Bubble Tea 搭建了一个轻量的 TUI：

- 以流式方式展示推理、工具调用、错误和最终内容
- 便于观察 Agent Loop 的执行轨迹

**流式输出与 UI 协程**

本章让 Agent 以流式方式输出，并通过 Go 的 `channel` 将增量消息传递给 UI 协程。`RunStreaming` 会持续向 `viewCh` 写入 `MessageVO`，而 TUI 侧以事件循环消费这些消息并渲染。这种“流式 + 通道”的模式可以在响应尚未完成时就持续显示过程输出。

这部分**不是本课程的核心内容**，也不需要深入理解 Bubble Tea 的内部机制。你只需要知道：TUI 的存在是为了让调试更直观，后续章节会频繁用到“可视化输出”。

相关代码：`ch03/tui/main.go`

---

### 3. MCP（Model Context Protocol）与工具生态

#### 3.1 MCP 的基本原理

MCP 是一种开放协议，用于让 Agent/LLM 以统一方式发现并调用外部工具。它主要定义了三类角色：

- **Client/Host**：嵌入在 Agent 或应用中，负责发现工具并调用
- **Server**：对外暴露工具的服务端
- **Tool**：可被调用的功能接口（输入 JSON，输出结构化结果）

MCP 的意义在于：**用标准协议解决“模型 × 工具”爆炸式组合问题**，让工具接入更可复用、更可组合。

#### 3.2 本章的 MCP 接入方式

在 `ch03/mcp.go` 中实现 MCP 客户端封装，核心流程：

1. 从 `mcp-server.json` 加载 MCP 服务器配置。
2. 连接 MCP Server（支持 stdio 或 HTTP 方式）。
3. 调用 `ListTools` 拉取工具列表，并封装为本项目统一的 `tool.Tool` 接口。
4. 在 Agent 中将 MCP 工具合并到 tools 列表中。

**工具命名策略**

为了避免冲突，本章将 MCP 工具名包装成：

```
babyagent_mcp__{serverName}__{toolName}
```

这样模型侧看到的是“命名空间化工具”，而 MCP 服务器端实际执行的是原始工具名。

相关代码：`ch03/mcp.go`、`shared/mcp.go`

#### 3.3 Agent 如何管理 MCP 工具与本地工具

`ch03/agent.go` 中，Agent 维护了两类工具：

- `nativeTools`：本地实现的工具（read / write / edit / bash）
- `mcpClients`：通过 MCP 动态加载的工具集合

在 `buildTools()` 时统一注册给模型；在 `execute()` 时先查本地工具，再查 MCP 工具，确保两类工具可以无缝共存。

这使得 Agent 的工具能力具备“本地 + 远程”双模式：

- 本地工具：低延迟、可控、适合文件与命令
- MCP 工具：扩展性强、生态丰富、可跨应用复用

相关代码：`ch03/agent.go`

---

## 💻 代码结构速览

- `ch03/agent.go`：增强后的 Agent Loop（流式 + reasoning 解析 + MCP 支持）
- `ch03/mcp.go`：MCP 客户端与 MCP Tool 封装
- `shared/mcp.go`：MCP 服务器配置解析
- `ch03/tui/main.go`：Bubble Tea TUI 可视化界面
- `mcp-server.json`：MCP 服务器配置（默认文件系统工具）

---

## 🚀 动手运行

进入项目根目录，执行：

```bash
go run ./ch03/tui
```

示例：

- “请读取 README.md 并总结项目目标”
- “列出当前目录下的文件”

如果 MCP 文件系统服务正常启动，你会看到工具调用日志出现在 TUI 中。

---

## 📚 扩展阅读与参考资料

以下资料可帮助你进一步理解 MCP、Function Calling 以及 TUI 相关内容：

1. MCP 官方文档（概览）：`https://modelcontextprotocol.io/`
2. MCP 规范（Spec）：`https://github.com/modelcontextprotocol/spec`
3. MCP Go SDK：`https://github.com/modelcontextprotocol/go-sdk`
4. Bubble Tea（TUI 框架）：`https://github.com/charmbracelet/bubbletea`
