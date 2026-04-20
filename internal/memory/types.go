package memory

import "time"

type UserProfile struct {
	UserID                 string                 `json:"user_id"`
	BasicInfo              BasicInfo              `json:"basic_info"`
	Preferences            []Preference           `json:"preferences"`
	EmotionalStates        []EmotionState         `json:"emotional_states"`
	ImportantEvents        []ImportantEvent       `json:"important_events"`
	RelationshipPreference RelationshipPreference `json:"relationship_preference"`
	RelationshipState      RelationshipState      `json:"relationship_state"`
	MemoryHistory          []MemoryItem           `json:"memory_history"`
	Conflicts              []MemoryConflict       `json:"conflicts"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

type BasicInfo struct {
	Name       string `json:"name,omitempty"`
	Age        int    `json:"age,omitempty"`
	Occupation string `json:"occupation,omitempty"`
	City       string `json:"city,omitempty"`
	Schedule   string `json:"schedule,omitempty"`
}

type Preference struct {
	Kind       string    `json:"kind"`
	Value      string    `json:"value"`
	Confidence float64   `json:"confidence"`
	Evidence   string    `json:"evidence"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type EmotionState struct {
	Label      string    `json:"label"`
	Intensity  int       `json:"intensity"`
	Reason     string    `json:"reason,omitempty"`
	Evidence   string    `json:"evidence"`
	ObservedAt time.Time `json:"observed_at"`
}

type ImportantEvent struct {
	Name       string    `json:"name"`
	Status     string    `json:"status,omitempty"`
	Evidence   string    `json:"evidence"`
	ObservedAt time.Time `json:"observed_at"`
}

type RelationshipPreference struct {
	Tone        string    `json:"tone,omitempty"`
	Need        string    `json:"need,omitempty"`
	Humor       bool      `json:"humor,omitempty"`
	Rationality bool      `json:"rationality,omitempty"`
	Warmth      bool      `json:"warmth,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type RelationshipState struct {
	TurnCount   int     `json:"turn_count"`
	Familiarity float64 `json:"familiarity"`
	Trust       float64 `json:"trust"`
	Intimacy    float64 `json:"intimacy"`
}

type MemoryItem struct {
	Field      string    `json:"field"`
	OldValue   string    `json:"old_value,omitempty"`
	NewValue   string    `json:"new_value"`
	Evidence   string    `json:"evidence"`
	OccurredAt time.Time `json:"occurred_at"`
}

type MemoryConflict struct {
	Field      string    `json:"field"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	Resolution string    `json:"resolution"`
	Evidence   string    `json:"evidence"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ExtractedFacts struct {
	BasicInfo              BasicInfo              `json:"basic_info"`
	Preferences            []Preference           `json:"preferences"`
	EmotionalStates        []EmotionState         `json:"emotional_states"`
	ImportantEvents        []ImportantEvent       `json:"important_events"`
	RelationshipPreference RelationshipPreference `json:"relationship_preference"`
}
