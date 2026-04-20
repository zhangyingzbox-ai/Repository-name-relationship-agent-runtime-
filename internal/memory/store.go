package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Load(userID string) (*UserProfile, error)
	Save(profile *UserProfile) error
}

type JSONStore struct {
	dir string
	mu  sync.Mutex
}

func NewJSONStore(dir string) *JSONStore {
	return &JSONStore{dir: dir}
}

func (s *JSONStore) Load(userID string) (*UserProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == "" {
		return nil, errors.New("user id is required")
	}
	path := s.path(userID)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewUserProfile(userID), nil
	}
	if err != nil {
		return nil, err
	}
	var p UserProfile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.UserID == "" {
		p.UserID = userID
	}
	return &p, nil
}

func (s *JSONStore) Save(profile *UserProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if profile == nil {
		return errors.New("profile is nil")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	profile.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(profile.UserID), b, 0o644)
}

func (s *JSONStore) path(userID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_").Replace(userID)
	return filepath.Join(s.dir, safe+".json")
}

func NewUserProfile(userID string) *UserProfile {
	return &UserProfile{
		UserID:    userID,
		UpdatedAt: time.Now(),
		RelationshipState: RelationshipState{
			Familiarity: 0.05,
			Trust:       0.05,
			Intimacy:    0.05,
		},
	}
}

type UpdateReport struct {
	UpdatedFields []string         `json:"updated_fields"`
	Conflicts     []MemoryConflict `json:"conflicts"`
}

func ApplyFacts(profile *UserProfile, facts ExtractedFacts, evidence string, now time.Time) UpdateReport {
	var report UpdateReport
	if profile == nil {
		return report
	}
	updateString := func(field string, current *string, next string) {
		next = strings.TrimSpace(next)
		if next == "" {
			return
		}
		old := strings.TrimSpace(*current)
		if strings.EqualFold(old, next) {
			return
		}
		*current = next
		item := MemoryItem{Field: field, OldValue: old, NewValue: next, Evidence: evidence, OccurredAt: now}
		profile.MemoryHistory = append(profile.MemoryHistory, item)
		report.UpdatedFields = append(report.UpdatedFields, field)
		if old != "" {
			conflict := MemoryConflict{
				Field:      field,
				OldValue:   old,
				NewValue:   next,
				Resolution: "latest_user_statement_overwrites_current_value_and_old_value_is_kept_in_history",
				Evidence:   evidence,
				OccurredAt: now,
			}
			profile.Conflicts = append(profile.Conflicts, conflict)
			report.Conflicts = append(report.Conflicts, conflict)
		}
	}

	updateString("basic_info.name", &profile.BasicInfo.Name, facts.BasicInfo.Name)
	updateString("basic_info.occupation", &profile.BasicInfo.Occupation, facts.BasicInfo.Occupation)
	updateString("basic_info.city", &profile.BasicInfo.City, facts.BasicInfo.City)
	updateString("basic_info.schedule", &profile.BasicInfo.Schedule, facts.BasicInfo.Schedule)
	if facts.BasicInfo.Age > 0 && facts.BasicInfo.Age != profile.BasicInfo.Age {
		old := ""
		if profile.BasicInfo.Age > 0 {
			old = fmt.Sprintf("%d", profile.BasicInfo.Age)
		}
		profile.BasicInfo.Age = facts.BasicInfo.Age
		profile.MemoryHistory = append(profile.MemoryHistory, MemoryItem{Field: "basic_info.age", OldValue: old, NewValue: fmt.Sprintf("%d", facts.BasicInfo.Age), Evidence: evidence, OccurredAt: now})
		report.UpdatedFields = append(report.UpdatedFields, "basic_info.age")
		if old != "" {
			conflict := MemoryConflict{Field: "basic_info.age", OldValue: old, NewValue: fmt.Sprintf("%d", facts.BasicInfo.Age), Resolution: "latest_user_statement_overwrites_current_value_and_old_value_is_kept_in_history", Evidence: evidence, OccurredAt: now}
			profile.Conflicts = append(profile.Conflicts, conflict)
			report.Conflicts = append(report.Conflicts, conflict)
		}
	}

	for _, pref := range facts.Preferences {
		upsertPreference(profile, pref, now, evidence, &report)
	}
	for _, e := range facts.EmotionalStates {
		e.ObservedAt = now
		profile.EmotionalStates = append(profile.EmotionalStates, e)
		report.UpdatedFields = append(report.UpdatedFields, "emotional_states")
	}
	for _, e := range facts.ImportantEvents {
		e.ObservedAt = now
		profile.ImportantEvents = append(profile.ImportantEvents, e)
		report.UpdatedFields = append(report.UpdatedFields, "important_events")
	}

	rel := facts.RelationshipPreference
	if rel.Tone != "" || rel.Need != "" || rel.Humor || rel.Rationality || rel.Warmth {
		if rel.Tone != "" {
			profile.RelationshipPreference.Tone = rel.Tone
		}
		if rel.Need != "" {
			profile.RelationshipPreference.Need = rel.Need
		}
		if rel.Humor {
			profile.RelationshipPreference.Humor = true
		}
		if rel.Rationality {
			profile.RelationshipPreference.Rationality = true
		}
		if rel.Warmth {
			profile.RelationshipPreference.Warmth = true
		}
		profile.RelationshipPreference.UpdatedAt = now
		report.UpdatedFields = append(report.UpdatedFields, "relationship_preference")
	}

	profile.RelationshipState.TurnCount++
	profile.RelationshipState.Familiarity = clamp(profile.RelationshipState.Familiarity+0.07, 0, 1)
	if len(report.UpdatedFields) > 0 {
		profile.RelationshipState.Trust = clamp(profile.RelationshipState.Trust+0.04, 0, 1)
		profile.RelationshipState.Intimacy = clamp(profile.RelationshipState.Intimacy+0.03, 0, 1)
	}
	profile.UpdatedAt = now
	return report
}

func upsertPreference(profile *UserProfile, pref Preference, now time.Time, evidence string, report *UpdateReport) {
	pref.Value = strings.TrimSpace(pref.Value)
	if pref.Value == "" || pref.Kind == "" {
		return
	}
	pref.UpdatedAt = now
	pref.Evidence = evidence
	if pref.Confidence == 0 {
		pref.Confidence = 0.75
	}
	for i, old := range profile.Preferences {
		if old.Kind == pref.Kind && strings.EqualFold(old.Value, pref.Value) {
			profile.Preferences[i] = pref
			report.UpdatedFields = append(report.UpdatedFields, "preferences")
			return
		}
	}
	profile.Preferences = append(profile.Preferences, pref)
	profile.MemoryHistory = append(profile.MemoryHistory, MemoryItem{Field: "preferences." + pref.Kind, NewValue: pref.Value, Evidence: evidence, OccurredAt: now})
	report.UpdatedFields = append(report.UpdatedFields, "preferences")
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
