package agent

import (
	"errors"
	"fmt"
	"strings"

	"relationship-agent-runtime/internal/memory"
)

type AgentRuntime struct {
	Extractor ExtractionTool
	Memory    MemoryTool
	Sessions  map[string]*SessionState
}

func NewRuntime(store memory.Store) *AgentRuntime {
	return &AgentRuntime{
		Extractor: RuleBasedExtractor{},
		Memory:    PersistentMemoryTool{Store: store},
		Sessions:  map[string]*SessionState{},
	}
}

func NewRuntimeWithTools(extractor ExtractionTool, memoryTool MemoryTool) *AgentRuntime {
	return &AgentRuntime{
		Extractor: extractor,
		Memory:    memoryTool,
		Sessions:  map[string]*SessionState{},
	}
}

func (r *AgentRuntime) Chat(req ChatRequest) ChatResponse {
	resp := ChatResponse{UserID: req.UserID}
	add := func(step, status, detail string) {
		resp.Trace = append(resp.Trace, TraceStep{Step: step, Status: status, Detail: detail})
	}
	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = "default"
		resp.UserID = req.UserID
		add("Step0: Validate input", "fallback", "missing user_id; using default user id")
	}
	if strings.TrimSpace(req.Message) == "" {
		add("Step0: Validate input", "error", "message is empty")
		resp.FinalResponse = "我这边没有收到具体内容。你可以再说一句，我会继续记住我们聊到的重要信息。"
		return resp
	}
	add("Step0: Validate input", "ok", "input accepted")

	session := r.session(req.UserID)
	session.TurnIndex++
	session.RecentTurns = append(session.RecentTurns, req.Message)
	if len(session.RecentTurns) > 6 {
		session.RecentTurns = session.RecentTurns[len(session.RecentTurns)-6:]
	}

	profile, err := r.Memory.Load(req.UserID)
	if err != nil {
		add("Step1: Load memory", "fallback", fmt.Sprintf("load failed: %v; using empty in-memory profile", err))
		profile = memory.NewUserProfile(req.UserID)
	} else {
		add("Step1: Load memory", "ok", summarizeProfile(profile))
	}

	facts, err := r.Extractor.Extract(req.Message)
	if err != nil {
		add("Step2: Extract user information", "fallback", fmt.Sprintf("extraction failed: %v; continuing without new facts", err))
		facts = memory.ExtractedFacts{}
	} else {
		add("Step2: Extract user information", "ok", summarizeFacts(facts))
	}

	report := r.Memory.Update(profile, facts, req.Message)
	if len(report.UpdatedFields) == 0 {
		add("Step3: Update structured memory", "ok", "no new structured facts; relationship turn count still updated")
	} else {
		detail := "updated " + strings.Join(unique(report.UpdatedFields), ", ")
		if len(report.Conflicts) > 0 {
			detail += fmt.Sprintf("; resolved %d conflict(s) by keeping history and using the latest statement", len(report.Conflicts))
		}
		add("Step3: Update structured memory", "ok", detail)
	}

	if err := r.Memory.Save(profile); err != nil {
		add("Step4: Save memory", "fallback", fmt.Sprintf("save failed: %v; response still generated", err))
	} else {
		add("Step4: Save memory", "ok", "profile persisted")
	}

	resp.FinalResponse = GenerateReply(profile, req.Message, report)
	resp.Profile = profile
	add("Step5: Generate reply", "ok", "reply generated from message plus current relationship memory")
	return resp
}

func (r *AgentRuntime) session(userID string) *SessionState {
	if r.Sessions == nil {
		r.Sessions = map[string]*SessionState{}
	}
	if s, ok := r.Sessions[userID]; ok {
		return s
	}
	s := &SessionState{UserID: userID}
	r.Sessions[userID] = s
	return s
}

func GenerateReply(profile *memory.UserProfile, message string, report memory.UpdateReport) string {
	name := profile.BasicInfo.Name
	if name == "" {
		name = "你"
	}
	var parts []string
	if len(report.Conflicts) > 0 {
		c := report.Conflicts[len(report.Conflicts)-1]
		if c.Field == "basic_info.city" {
			parts = append(parts, fmt.Sprintf("我更新一下记忆：你之前在%s，现在以%s为准；旧信息我会放进历史里。", c.OldValue, c.NewValue))
		} else {
			parts = append(parts, fmt.Sprintf("我注意到%s从“%s”变成了“%s”，我会采用最新说法，同时保留历史。", c.Field, c.OldValue, c.NewValue))
		}
	}
	if len(profile.EmotionalStates) > 0 {
		e := profile.EmotionalStates[len(profile.EmotionalStates)-1]
		if e.Label == "焦虑" || e.Label == "紧张" || e.Label == "压力" || e.Label == "疲惫" || e.Label == "累" {
			parts = append(parts, fmt.Sprintf("%s，我记得你现在有点%s，我会先稳住节奏，不催你。", name, e.Label))
		}
	}
	if profile.BasicInfo.City != "" || profile.BasicInfo.Occupation != "" {
		parts = append(parts, fmt.Sprintf("目前我对你的认识是：%s%s%s。", cityClause(profile), occupationClause(profile), preferenceClause(profile)))
	}
	if profile.RelationshipPreference.Warmth {
		parts = append(parts, "我会用更温柔一点的方式陪你聊。")
	}
	if profile.RelationshipPreference.Rationality {
		parts = append(parts, "需要建议时，我会尽量讲清楚取舍和下一步。")
	}
	if profile.RelationshipPreference.Humor {
		parts = append(parts, "也会适当保留一点轻松感。")
	}
	if len(parts) == 0 {
		parts = append(parts, "我收到了。你可以继续说，我会把稳定、重要、和你有关的信息整理成长期记忆。")
	}
	parts = append(parts, nextQuestion(profile, message))
	return strings.Join(parts, " ")
}

func cityClause(profile *memory.UserProfile) string {
	if profile.BasicInfo.City == "" {
		return ""
	}
	return "在" + profile.BasicInfo.City
}

func occupationClause(profile *memory.UserProfile) string {
	if profile.BasicInfo.Occupation == "" {
		return ""
	}
	if profile.BasicInfo.City == "" {
		return "是" + profile.BasicInfo.Occupation
	}
	return "，是" + profile.BasicInfo.Occupation
}

func preferenceClause(profile *memory.UserProfile) string {
	if len(profile.Preferences) == 0 {
		return ""
	}
	p := profile.Preferences[len(profile.Preferences)-1]
	if p.Kind == "like" {
		return "，喜欢" + p.Value
	}
	return "，不喜欢" + p.Value
}

func nextQuestion(profile *memory.UserProfile, message string) string {
	if strings.Contains(message, "怎么办") || strings.Contains(message, "建议") {
		return "你想让我先帮你拆一个最小下一步，还是先陪你把情绪缓一下？"
	}
	if len(profile.ImportantEvents) > 0 {
		e := profile.ImportantEvents[len(profile.ImportantEvents)-1]
		return fmt.Sprintf("关于%s这件事，你现在最卡的是准备、时间，还是心态？", e.Name)
	}
	if profile.BasicInfo.Name == "" {
		return "我该怎么称呼你？"
	}
	return "这一轮我先记到这里；还有什么近期重要变化，也可以直接告诉我。"
}

func summarizeProfile(profile *memory.UserProfile) string {
	if profile == nil {
		return "empty profile"
	}
	fields := []string{fmt.Sprintf("turns=%d", profile.RelationshipState.TurnCount)}
	if profile.BasicInfo.Name != "" {
		fields = append(fields, "name="+profile.BasicInfo.Name)
	}
	if profile.BasicInfo.City != "" {
		fields = append(fields, "city="+profile.BasicInfo.City)
	}
	return strings.Join(fields, ", ")
}

func summarizeFacts(f memory.ExtractedFacts) string {
	var parts []string
	if f.BasicInfo.Name != "" {
		parts = append(parts, "name")
	}
	if f.BasicInfo.City != "" {
		parts = append(parts, "city")
	}
	if f.BasicInfo.Occupation != "" {
		parts = append(parts, "occupation")
	}
	if len(f.Preferences) > 0 {
		parts = append(parts, "preferences")
	}
	if len(f.EmotionalStates) > 0 {
		parts = append(parts, "emotion")
	}
	if len(f.ImportantEvents) > 0 {
		parts = append(parts, "event")
	}
	if f.RelationshipPreference.Tone != "" || f.RelationshipPreference.Need != "" || f.RelationshipPreference.Humor {
		parts = append(parts, "relationship_preference")
	}
	if len(parts) == 0 {
		return "no structured facts detected"
	}
	return strings.Join(parts, ", ")
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

var ErrNoMemoryTool = errors.New("memory tool is required")
