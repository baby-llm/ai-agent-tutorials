package ch03

const (
	MessageTypeReasoning = "reasoning"
	MessageTypeContent   = "content"
	MessageTypeToolCall  = "tool_call"
	MessageTypeError     = "error"
)

// MessageVO 用于流式展示当前模型流式输出或者状态
type MessageVO struct {
	Type string `json:"type"`

	ReasoningContent *string `json:"reasoning_content,omitempty"`
	Content          *string `json:"content,omitempty"`

	ToolCall *ToolCallVO `json:"tool,omitempty"`
}

type ToolCallVO struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
