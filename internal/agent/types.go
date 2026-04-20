package agent

import "relationship-agent-runtime/internal/memory"

type ChatRequest struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type ChatResponse struct {
	UserID        string              `json:"user_id"`
	Trace         []TraceStep         `json:"trace"`
	FinalResponse string              `json:"final_response"`
	Profile       *memory.UserProfile `json:"profile,omitempty"`
}

type TraceStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type SessionState struct {
	UserID      string   `json:"user_id"`
	TurnIndex   int      `json:"turn_index"`
	RecentTurns []string `json:"recent_turns"`
}

type Tool interface {
	Name() string
	Description() string
}

type ExtractionTool interface {
	Tool
	Extract(message string) (memory.ExtractedFacts, error)
}

type MemoryTool interface {
	Tool
	Load(userID string) (*memory.UserProfile, error)
	Save(profile *memory.UserProfile) error
	Update(profile *memory.UserProfile, facts memory.ExtractedFacts, evidence string) memory.UpdateReport
}
