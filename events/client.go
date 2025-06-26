package events

type SessionUpdateEvent struct {
	BaseEvent
	Session SessionUpdate `json:"session"`
}

type ConversationItemCreateEvent struct {
	BaseEvent
	Item ConversationItem `json:"item"`
}

// ConversationItem is the inner “item” object.
type ConversationItem struct {
	ID      string                    `json:"id"`
	Type    string                    `json:"type"`
	Role    string                    `json:"role"`
	Content []ConversationItemContent `json:"content,omitempty"`
}

type ConversationItemContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
