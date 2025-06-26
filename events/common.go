package events

import "encoding/json"
import nanoid "github.com/matoous/go-nanoid/v2"

type BaseEvent struct {
	EventID        string  `json:"event_id"`
	Type           string  `json:"type"`
	PreviousItemID *string `json:"previous_item_id,omitempty"`
}

func NewBaseEvent(eventType string) BaseEvent {
	id, err := nanoid.New()
	if err != nil {
		panic(err)
	}
	return BaseEvent{
		EventID: id,
		Type:    eventType,
	}
}

func Parse[T any](data []byte) (*T, error) {
	var x T
	if err := json.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	return &x, nil
}
