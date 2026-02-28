# 第四章：让 Agent 接入 MCP 生态

欢迎来到第四章！在第三章的基础上，本章继续完善一个接入 MCP 生态的 Agent：

- **接入 MCP（Model Context Protocol）工具生态**

---

## 🎯 你将学到什么

1. **MCP 原理与接入**：理解 MCP 的基本角色与工具生命周期，并在 Agent 中同时管理 MCP 工具与本地工具。

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

### 1. MCP（Model Context Protocol）与工具生态

#### 1.1 MCP 的基本原理

MCP 是一种开放协议，用于让 Agent/LLM 以统一方式发现并调用外部工具。它主要定义了三类角色：

- **Client/Host**：嵌入在 Agent 或应用中，负责发现工具并调用
- **Server**：对外暴露工具的服务端
- **Tool**：可被调用的功能接口（输入 JSON，输出结构化结果）

MCP 的意义在于：**用标准协议解决“模型 × 工具”爆炸式组合问题**，让工具接入更可复用、更可组合。

#### 1.2 本章的 MCP 接入方式

在 `ch04/mcp.go` 中实现 MCP 客户端封装，核心流程：

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

相关代码：`ch04/mcp.go`、`shared/mcp.go`

#### 1.3 Agent 如何管理 MCP 工具与本地工具

`ch04/agent.go` 中，Agent 维护了两类工具：

- `nativeTools`：本地实现的工具（read / write / edit / bash）
- `mcpClients`：通过 MCP 动态加载的工具集合

在 `buildTools()` 时统一注册给模型；在 `execute()` 时先查本地工具，再查 MCP 工具，确保两类工具可以无缝共存。

这使得 Agent 的工具能力具备“本地 + 远程”双模式：

- 本地工具：低延迟、可控、适合文件与命令
- MCP 工具：扩展性强、生态丰富、可跨应用复用

相关代码：`ch04/agent.go`

---

## 💻 代码结构速览

- `ch04/agent.go`：增强后的 Agent Loop（MCP 支持）
- `ch04/mcp.go`：MCP 客户端与 MCP Tool 封装
- `shared/mcp.go`：MCP 服务器配置解析
- `mcp-server.json`：MCP 服务器配置（默认文件系统工具）

---

## 🚀 动手运行

进入项目根目录，执行：

```bash
go run ./ch04/tui
```

示例：

- “请读取 README.md 并总结项目目标”
- “使用 MCP 工具列出当前目录下的文件”

如果 MCP 文件系统服务正常启动，你会看到工具调用日志出现在 TUI 中。

---

## 📚 扩展阅读与参考资料

以下资料可帮助你进一步理解 MCP 相关内容：

1. MCP 官方文档（概览）：`https://modelcontextprotocol.io/`
2. MCP 规范（Spec）：`https://github.com/modelcontextprotocol/spec`
3. MCP Go SDK：`https://github.com/modelcontextprotocol/go-sdk`