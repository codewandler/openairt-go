package tool

type Choice string

const (
	ChoiceAuto Choice = "auto"
	ChoiceNone Choice = "none"
)

type Tool struct {
	Type        string     `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string     `json:"type"`
	Properties Properties `json:"properties"`
	Required   []string   `json:"required"`
}

type Properties map[string]Property

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Enum        []any  `json:"enum,omitempty"`
}
