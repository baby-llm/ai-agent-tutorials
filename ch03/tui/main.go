package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/joho/godotenv"

	"babyagent/ch03"
	"babyagent/ch03/tool"
	"babyagent/shared"
)

type streamMsg struct {
	event ch03.MessageVO
}

type streamClosedMsg struct{}

type streamDoneMsg struct {
	err error
}

type runState int

const (
	stateIdle runState = iota
	stateRunning
	stateAborting
)

type activeStream struct {
	events <-chan ch03.MessageVO
	cancel context.CancelFunc

	turnSnapshot int
	turnLogLen   int
	reasonBody   int
	contentBody  int
}

type model struct {
	modelName string
	agent     *ch03.Agent

	input string
	logs  []string
	round int

	state  runState
	active *activeStream

	notice string

	width  int
	height int

	logsViewport viewport.Model
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	reasonStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	toolStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	noticeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	borderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func newModel(agent *ch03.Agent, modelName string) *model {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.MouseWheelEnabled = false

	return &model{
		modelName:    modelName,
		agent:        agent,
		logs:         make([]string, 0),
		logsViewport: vp,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func waitStreamEvent(ch <-chan ch03.MessageVO) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return streamMsg{event: msg}
	}
}

func waitStreamDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamDoneMsg{err: err}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncLogsViewportSize()
		return m, nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollUp(3)
		case tea.MouseWheelDown:
			m.scrollDown(3)
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case streamMsg:
		return m.handleStreamMsg(msg)
	case streamClosedMsg:
		if m.active != nil {
			m.active.events = nil
		}
		return m, nil
	case streamDoneMsg:
		return m.handleStreamDone(msg)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stopActiveStream()
		return m, tea.Quit
	case "up":
		m.scrollUp(1)
		return m, nil
	case "down":
		m.scrollDown(1)
		return m, nil
	case "pgup":
		m.scrollUp(m.logsViewportHeight())
		return m, nil
	case "pgdown":
		m.scrollDown(m.logsViewportHeight())
		return m, nil
	case "home":
		m.logsViewport.GotoTop()
		return m, nil
	case "end":
		m.logsViewport.GotoBottom()
		return m, nil
	case "enter":
		return m.handleSubmit()
	case "esc":
		m.abortCurrentTurn()
		return m, nil
	case "backspace":
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
		return m, nil
	}

	if m.state != stateIdle {
		return m, nil
	}

	if key := msg.Key(); key.Text != "" {
		m.input += key.Text
	}
	return m, nil
}

func (m *model) handleSubmit() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.input)
	if query == "" {
		return m, nil
	}

	if m.state != stateIdle {
		return m, nil
	}

	m.input = ""
	if query == "/clear" {
		m.clearSession()
		return m, nil
	}

	return m.startNewTurn(query)
}

func (m *model) handleStreamEvent(event ch03.MessageVO) {
	if m.active == nil || m.state == stateAborting {
		return
	}

	switch event.Type {
	case ch03.MessageTypeReasoning:
		if event.ReasoningContent == nil {
			return
		}
		m.appendReasoning(*event.ReasoningContent)
	case ch03.MessageTypeContent:
		if event.Content == nil || *event.Content == "" {
			return
		}
		m.appendContent(*event.Content)
	case ch03.MessageTypeToolCall:
		if event.ToolCall != nil {
			m.appendLogBlock("工具调用:", fmt.Sprintf("%s(%s)", event.ToolCall.Name, event.ToolCall.Arguments))
			m.resetOutputSection()
		}
	case ch03.MessageTypeError:
		if event.Content != nil {
			m.appendLogBlock("错误:", *event.Content)
			m.resetOutputSection()
		}
	}
}

func (m *model) appendReasoning(chunk string) {
	if m.active.reasonBody == -1 {
		m.logs = append(m.logs, "推理:", chunk, "")
		m.active.reasonBody = len(m.logs) - 2
		return
	}
	m.logs[m.active.reasonBody] += chunk
}

func (m *model) appendContent(chunk string) {
	if m.active.contentBody == -1 {
		m.logs = append(m.logs, "回答:", chunk, "")
		m.active.contentBody = len(m.logs) - 2
		return
	}
	m.logs[m.active.contentBody] += chunk
}

func (m *model) appendLogBlock(label, content string) {
	m.logs = append(m.logs, label, content, "")
}

func (m *model) resetOutputSection() {
	if m.active == nil {
		return
	}
	m.active.reasonBody = -1
	m.active.contentBody = -1
}

func (m *model) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	if m.active == nil || m.active.events == nil {
		return m, nil
	}
	m.handleStreamEvent(msg.event)
	m.refreshLogsViewportContent()
	return m, waitStreamEvent(m.active.events)
}

func (m *model) handleStreamDone(msg streamDoneMsg) (tea.Model, tea.Cmd) {
	if m.active == nil {
		m.state = stateIdle
		return m, nil
	}

	m.stopActiveStream()
	if m.state == stateAborting {
		m.rollbackTurn()
		m.notice = "已取消本轮输入。"
		m.state = stateIdle
		return m, nil
	}

	if msg.err != nil {
		m.appendLogBlock("错误:", msg.err.Error())
	}
	m.ensureTrailingBlank()
	m.logs = append(m.logs, strings.Repeat("─", 48))
	m.state = stateIdle
	m.refreshLogsViewportContent()
	return m, nil
}

func (m *model) startNewTurn(query string) (tea.Model, tea.Cmd) {
	m.notice = ""
	m.round++
	turnStart := len(m.logs)
	m.logs = append(m.logs, fmt.Sprintf("第 %d 轮", m.round), "")
	m.appendLogBlock("你:", query)

	streamC := make(chan ch03.MessageVO, 256)
	doneC := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	m.active = &activeStream{
		events:       streamC,
		cancel:       cancel,
		turnSnapshot: m.agent.SessionSnapshot(),
		turnLogLen:   turnStart,
		reasonBody:   -1,
		contentBody:  -1,
	}
	m.state = stateRunning
	m.refreshLogsViewportContent()

	go func() {
		err := m.agent.RunStreaming(ctx, query, streamC)
		close(streamC)
		doneC <- err
		close(doneC)
	}()

	return m, tea.Batch(waitStreamEvent(streamC), waitStreamDone(doneC))
}

func (m *model) clearSession() {
	m.agent.ResetSession()
	m.logs = m.logs[:0]
	m.notice = "会话已清空（仅保留 system prompt）。"
	m.round = 0
	m.refreshLogsViewportContent()
}

func (m *model) abortCurrentTurn() {
	if m.state != stateRunning || m.active == nil || m.active.cancel == nil {
		return
	}
	m.state = stateAborting
	m.notice = "正在终止流式输出..."
	m.active.cancel()
}

func (m *model) rollbackTurn() {
	if m.active == nil {
		return
	}
	m.agent.RestoreSession(m.active.turnSnapshot)
	if m.active.turnLogLen >= 0 && m.active.turnLogLen <= len(m.logs) {
		m.logs = m.logs[:m.active.turnLogLen]
	}
	m.refreshLogsViewportContent()
}

func (m *model) stopActiveStream() {
	if m.active == nil {
		return
	}
	if m.active.cancel != nil {
		m.active.cancel()
	}
	m.active = nil
}

func (m *model) ensureTrailingBlank() {
	if len(m.logs) == 0 {
		return
	}
	if m.logs[len(m.logs)-1] == "" {
		return
	}
	m.logs = append(m.logs, "")
}

func (m *model) scrollUp(n int) {
	if n <= 0 {
		return
	}
	m.logsViewport.ScrollUp(n)
}

func (m *model) scrollDown(n int) {
	if n <= 0 {
		return
	}
	m.logsViewport.ScrollDown(n)
}

func (m *model) logsHeaderHeight() int {
	return 4
}

func (m *model) logsFooterHeight() int {
	h := 4
	if m.state != stateIdle {
		h++
	}
	if m.notice != "" {
		h++
	}
	return h
}

func (m *model) logsViewportHeight() int {
	if m.height <= 0 {
		return 1
	}
	h := m.height - m.logsHeaderHeight() - m.logsFooterHeight()
	if h < 1 {
		return 1
	}
	return h
}

func (m *model) syncLogsViewportSize() {
	w := m.width
	if w < 1 {
		w = 1
	}
	m.logsViewport.SetWidth(w)
	m.logsViewport.SetHeight(m.logsViewportHeight())
}

func (m *model) refreshLogsViewportContent() {
	atBottom := m.logsViewport.AtBottom()
	offset := m.logsViewport.YOffset()
	lines := make([]string, len(m.logs))
	for i, line := range m.logs {
		lines[i] = m.renderLogLine(line)
	}
	m.logsViewport.SetContent(strings.Join(lines, "\n"))
	if !atBottom {
		m.logsViewport.GotoBottom()
		return
	}
	m.logsViewport.SetYOffset(offset)
}

func (m *model) renderLogLine(line string) string {
	switch {
	case strings.HasPrefix(line, "第 "):
		return labelStyle.Render(line)
	case strings.HasPrefix(line, "你:"), strings.HasPrefix(line, "回答:"):
		return contentStyle.Render(line)
	case strings.HasPrefix(line, "推理:"):
		return reasonStyle.Render(line)
	case strings.HasPrefix(line, "工具调用:"):
		return toolStyle.Render(line)
	case strings.HasPrefix(line, "错误:"):
		return errorStyle.Render(line)
	case strings.Trim(line, "─") == "":
		return borderStyle.Render(line)
	default:
		return line
	}
}

func (m *model) View() tea.View {
	var b strings.Builder

	m.syncLogsViewportSize()

	b.WriteString(titleStyle.Render("BabyAgent TUI (Bubble Tea)"))
	b.WriteString("\n")
	b.WriteString(borderStyle.Render(strings.Repeat("─", 48)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("当前模型: "))
	b.WriteString(contentStyle.Render(m.modelName))
	b.WriteString("\n")
	b.WriteString(contentStyle.Render("欢迎使用，输入问题后回车。"))
	b.WriteString("\n")
	b.WriteString(m.logsViewport.View())

	b.WriteString("\n")
	if m.state != stateIdle {
		b.WriteString(footerStyle.Render("模型响应中，输入暂不可用。"))
		b.WriteString("\n")
	}
	b.WriteString(contentStyle.Render(">>> " + m.input))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("快捷键: Ctrl+C 退出，Esc 取消当前流式"))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("命令: /clear 清空会话"))
	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(noticeStyle.Render(m.notice))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	modelConf := shared.NewModelConfig()

	mcpServerMap, err := shared.LoadMcpServerConfig("mcp-server.json")
	if err != nil {
		log.Printf("Failed to load MCP server configuration: %v", err)
	}
	mcpClients := make([]*ch03.McpClient, 0)
	for k, v := range mcpServerMap {
		mcpClient := ch03.NewMcpToolProvider(k, v)
		if err := mcpClient.RefreshTools(ctx); err != nil {
			log.Printf("Failed to refresh tools for MCP server %s: %v", k, err)
			continue
		}
		mcpClients = append(mcpClients, mcpClient)
	}

	agent := ch03.NewAgent(
		modelConf,
		ch03.CodingAgentSystemPrompt,
		[]tool.Tool{
			tool.NewReadTool(),
			tool.NewEditTool(),
			tool.NewWriteTool(),
			tool.NewBashTool(),
		},
		mcpClients,
	)

	log.SetOutput(io.Discard)
	p := tea.NewProgram(newModel(agent, modelConf.Model))
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}
