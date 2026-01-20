package llm

import (
	"strings"
)

// Conversation manages a sequence of messages.
type Conversation struct {
	messages []Message
	system   string
}

// NewConversation creates a new conversation.
func NewConversation() *Conversation {
	return &Conversation{
		messages: make([]Message, 0),
	}
}

// SetSystem sets the system prompt.
func (c *Conversation) SetSystem(system string) *Conversation {
	c.system = system
	return c
}

// AddMessage adds a message to the conversation.
func (c *Conversation) AddMessage(msg Message) *Conversation {
	c.messages = append(c.messages, msg)
	return c
}

// AddUser adds a user message.
func (c *Conversation) AddUser(content string) *Conversation {
	return c.AddMessage(UserMessage(content))
}

// AddAssistant adds an assistant message.
func (c *Conversation) AddAssistant(content string) *Conversation {
	return c.AddMessage(AssistantMessage(content))
}

// AddToolResult adds a tool result message.
func (c *Conversation) AddToolResult(callID, result string, isError bool) *Conversation {
	return c.AddMessage(ToolResultMessage(callID, result, isError))
}

// Messages returns all messages.
func (c *Conversation) Messages() []Message {
	return c.messages
}

// System returns the system prompt.
func (c *Conversation) System() string {
	return c.system
}

// Len returns the number of messages.
func (c *Conversation) Len() int {
	return len(c.messages)
}

// Clear removes all messages but keeps the system prompt.
func (c *Conversation) Clear() *Conversation {
	c.messages = make([]Message, 0)
	return c
}

// Last returns the last message or nil if empty.
func (c *Conversation) Last() *Message {
	if len(c.messages) == 0 {
		return nil
	}
	return &c.messages[len(c.messages)-1]
}

// ToRequest converts the conversation to a completion request.
func (c *Conversation) ToRequest(model string, maxTokens int) *CompletionRequest {
	return &CompletionRequest{
		Model:     model,
		Messages:  c.messages,
		System:    c.system,
		MaxTokens: maxTokens,
	}
}

// Clone creates a copy of the conversation.
func (c *Conversation) Clone() *Conversation {
	clone := &Conversation{
		messages: make([]Message, len(c.messages)),
		system:   c.system,
	}
	copy(clone.messages, c.messages)
	return clone
}

// Truncate removes messages from the beginning, keeping the last n messages.
func (c *Conversation) Truncate(n int) *Conversation {
	if n >= len(c.messages) {
		return c
	}
	c.messages = c.messages[len(c.messages)-n:]
	return c
}

// MessageBuilder helps construct complex messages.
type MessageBuilder struct {
	role       string
	parts      []string
	toolCalls  []ToolCall
	toolCallID string
}

// NewMessageBuilder creates a new message builder.
func NewMessageBuilder(role string) *MessageBuilder {
	return &MessageBuilder{
		role:  role,
		parts: make([]string, 0),
	}
}

// AddText adds text content.
func (b *MessageBuilder) AddText(text string) *MessageBuilder {
	b.parts = append(b.parts, text)
	return b
}

// AddTextf adds formatted text content.
func (b *MessageBuilder) AddTextf(format string, args ...any) *MessageBuilder {
	text := sprintf(format, args...)
	b.parts = append(b.parts, text)
	return b
}

// AddToolCall adds a tool call.
func (b *MessageBuilder) AddToolCall(call ToolCall) *MessageBuilder {
	b.toolCalls = append(b.toolCalls, call)
	return b
}

// SetToolCallID sets the tool call ID (for tool result messages).
func (b *MessageBuilder) SetToolCallID(id string) *MessageBuilder {
	b.toolCallID = id
	return b
}

// Build creates the message.
func (b *MessageBuilder) Build() Message {
	return Message{
		Role:       b.role,
		Content:    strings.Join(b.parts, "\n"),
		ToolCalls:  b.toolCalls,
		ToolCallID: b.toolCallID,
	}
}

// sprintf is a simple format function to avoid importing fmt.
func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	// Simple implementation - in production use fmt.Sprintf
	result := format
	for _, arg := range args {
		idx := strings.Index(result, "%")
		if idx == -1 {
			break
		}
		// Find the end of the format specifier
		end := idx + 1
		for end < len(result) && !isFormatEnd(result[end]) {
			end++
		}
		if end < len(result) {
			end++
		}
		// Replace with string representation
		result = result[:idx] + toString(arg) + result[end:]
	}
	return result
}

func isFormatEnd(c byte) bool {
	return c == 's' || c == 'd' || c == 'v' || c == 'f' || c == 'q'
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return intToString(x)
	case int64:
		return int64ToString(x)
	case float64:
		return floatToString(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return "<value>"
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func int64ToString(n int64) string {
	return intToString(int(n))
}

func floatToString(f float64) string {
	// Simple implementation
	whole := int(f)
	frac := int((f - float64(whole)) * 100)
	if frac < 0 {
		frac = -frac
	}
	if frac == 0 {
		return intToString(whole)
	}
	return intToString(whole) + "." + intToString(frac)
}

// EstimateTokens provides a rough token estimate for text.
// This is approximately 4 characters per token for English text.
func EstimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// TruncateToTokens truncates text to approximately the given token limit.
func TruncateToTokens(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	// Try to truncate at word boundary
	truncated := text[:maxChars]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars*3/4 {
		return truncated[:lastSpace] + "..."
	}
	return truncated + "..."
}
