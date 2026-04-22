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
	Replyer   ReplyTool
	Sessions  map[string]*SessionState
}

func NewRuntime(store memory.Store) *AgentRuntime {
	return &AgentRuntime{
		Extractor: RuleBasedExtractor{},
		Memory:    PersistentMemoryTool{Store: store},
		Replyer:   TemplateReplyTool{},
		Sessions:  map[string]*SessionState{},
	}
}

func NewRuntimeWithTools(extractor ExtractionTool, memoryTool MemoryTool) *AgentRuntime {
	return &AgentRuntime{
		Extractor: extractor,
		Memory:    memoryTool,
		Replyer:   TemplateReplyTool{},
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
		add("Step2: Extract user information", "fallback", fmt.Sprintf("%s failed: %v; continuing without new facts", r.Extractor.Name(), err))
		facts = memory.ExtractedFacts{}
	} else {
		add("Step2: Extract user information", "ok", fmt.Sprintf("%s: %s", r.Extractor.Name(), summarizeFacts(facts)))
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

	replyer := r.Replyer
	if replyer == nil {
		replyer = TemplateReplyTool{}
	}
	final, err := replyer.Generate(profile, req.Message, report)
	if err != nil {
		add("Step5: Generate reply", "fallback", fmt.Sprintf("%s failed: %v; using template reply", replyer.Name(), err))
		final = GenerateReply(profile, req.Message, report)
	} else {
		add("Step5: Generate reply", "ok", fmt.Sprintf("%s generated reply from message plus current relationship memory", replyer.Name()))
	}
	resp.FinalResponse = final
	resp.Profile = profile
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
	if answer := answerMemoryQuestion(profile, message); answer != "" {
		return answer
	}
	var parts []string
	if len(report.Conflicts) > 0 {
		c := report.Conflicts[len(report.Conflicts)-1]
		if c.Field == "basic_info.city" {
			parts = append(parts, fmt.Sprintf("好，我把这点更新了：你之前告诉我在%s，现在我会以%s为准。旧信息我不会硬删，会放在历史里，这样我们的关系记忆是连续的。", c.OldValue, c.NewValue))
		} else {
			parts = append(parts, fmt.Sprintf("我注意到%s从“%s”变成了“%s”。我会采用你最新的说法，也把之前的版本留在历史里。", c.Field, c.OldValue, c.NewValue))
		}
	} else if len(report.UpdatedFields) > 0 {
		if summary := describeUpdatedMemory(profile, report); summary != "" {
			parts = append(parts, fmt.Sprintf("记住了，%s。你不是每次都要从头介绍自己的人，我会把这些放进我们的长期记忆里。", summary))
		}
	}
	if len(profile.EmotionalStates) > 0 {
		e := profile.EmotionalStates[len(profile.EmotionalStates)-1]
		if e.Label == "焦虑" || e.Label == "紧张" || e.Label == "压力" || e.Label == "疲惫" || e.Label == "累" {
			if e.Reason != "" {
				parts = append(parts, fmt.Sprintf("%s，我记得你最近有点%s，和%s有关。我们可以慢一点来，我会先帮你把事情稳住，不急着催你马上变好。", name, e.Label, e.Reason))
			} else {
				parts = append(parts, fmt.Sprintf("%s，我记得你最近有点%s。先不用硬撑，我会把节奏放慢一点，陪你把事情一件件理清楚。", name, e.Label))
			}
		}
	}
	if profile.BasicInfo.Name != "" || profile.BasicInfo.City != "" || profile.BasicInfo.Occupation != "" || len(profile.Preferences) > 0 {
		parts = append(parts, fmt.Sprintf("我现在对你的认识是：%s%s%s%s。", cityClause(profile), ageClause(profile), occupationClause(profile), preferenceClause(profile)))
	}
	if profile.RelationshipPreference.Warmth {
		parts = append(parts, "我会尽量把话说得温柔一点，不用冷冰冰的方式对你。")
	}
	if profile.RelationshipPreference.Rationality {
		parts = append(parts, "如果你需要建议，我也会直接帮你抓重点和下一步。")
	}
	if profile.RelationshipPreference.Humor {
		parts = append(parts, "气氛太紧的时候，我也会留一点轻松感。")
	}
	if len(parts) == 0 {
		parts = append(parts, "我在听，真的。你可以慢慢说，和你稳定相关、反复出现、或者对你很重要的事，我会整理成长期记忆。")
	}
	parts = append(parts, nextQuestion(profile, message))
	return strings.Join(parts, " ")
}

func describeUpdatedMemory(profile *memory.UserProfile, report memory.UpdateReport) string {
	var parts []string
	if containsField(report.UpdatedFields, "basic_info.name") && profile.BasicInfo.Name != "" {
		parts = append(parts, "你叫"+profile.BasicInfo.Name)
	}
	if containsField(report.UpdatedFields, "basic_info.age") && profile.BasicInfo.Age > 0 {
		parts = append(parts, fmt.Sprintf("今年%d岁", profile.BasicInfo.Age))
	}
	if containsField(report.UpdatedFields, "basic_info.city") && profile.BasicInfo.City != "" {
		parts = append(parts, "现在在"+profile.BasicInfo.City)
	}
	if containsField(report.UpdatedFields, "basic_info.occupation") && profile.BasicInfo.Occupation != "" {
		parts = append(parts, "职业是"+profile.BasicInfo.Occupation)
	}
	if containsField(report.UpdatedFields, "basic_info.schedule") && profile.BasicInfo.Schedule != "" {
		parts = append(parts, "作息是"+profile.BasicInfo.Schedule)
	}
	if containsField(report.UpdatedFields, "preferences") {
		if prefs := describePreferences(profile); prefs != "" {
			parts = append(parts, prefs)
		}
	}
	if containsField(report.UpdatedFields, "relationship_preference") {
		parts = append(parts, "也记住你更希望我用舒服、靠近你的方式回应")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "，")
}

func containsField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func cityClause(profile *memory.UserProfile) string {
	if profile.BasicInfo.Name != "" && profile.BasicInfo.City == "" && profile.BasicInfo.Occupation == "" {
		return profile.BasicInfo.Name
	}
	if profile.BasicInfo.City == "" {
		if profile.BasicInfo.Name != "" {
			return profile.BasicInfo.Name
		}
		return ""
	}
	if profile.BasicInfo.Name != "" {
		return profile.BasicInfo.Name + "，在" + profile.BasicInfo.City
	}
	return "在" + profile.BasicInfo.City
}

func occupationClause(profile *memory.UserProfile) string {
	if profile.BasicInfo.Occupation == "" {
		return ""
	}
	if profile.BasicInfo.City == "" && profile.BasicInfo.Name == "" {
		return "是" + profile.BasicInfo.Occupation
	}
	return "，是" + profile.BasicInfo.Occupation
}

func ageClause(profile *memory.UserProfile) string {
	if profile.BasicInfo.Age <= 0 {
		return ""
	}
	if profile.BasicInfo.Name == "" && profile.BasicInfo.City == "" {
		return fmt.Sprintf("今年%d岁", profile.BasicInfo.Age)
	}
	return fmt.Sprintf("，今年%d岁", profile.BasicInfo.Age)
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
		return fmt.Sprintf("关于%s这件事，我会陪你一起看。你现在最卡的是准备、时间，还是心态？", e.Name)
	}
	if profile.BasicInfo.Name == "" {
		return "我该怎么称呼你？"
	}
	return "这一轮我先抱稳这些信息；还有什么近期重要变化，也可以直接告诉我。"
}

func answerMemoryQuestion(profile *memory.UserProfile, message string) string {
	if !isMemoryQuestion(message) {
		return ""
	}
	name := profile.BasicInfo.Name
	if name == "" {
		name = "你"
	}
	if answer := answerCompositeMemoryQuestion(profile, message); answer != "" {
		return answer
	}
	switch {
	case (strings.Contains(message, "职业") || strings.Contains(message, "工作")) &&
		(strings.Contains(message, "偏好") || strings.Contains(message, "喜欢") || strings.Contains(message, "讨厌")):
		var parts []string
		if profile.BasicInfo.Occupation != "" {
			parts = append(parts, "你的职业是"+profile.BasicInfo.Occupation)
		}
		if prefs := describePreferences(profile); prefs != "" {
			parts = append(parts, prefs)
		}
		if len(parts) > 0 {
			return fmt.Sprintf("当然记得。你不是一串临时输入，我这里有认真把你说过的事放好：%s。%s", strings.Join(parts, "，"), relationshipStyleSuffix(profile))
		}
		return "我还没有可靠地记到你的职业和偏好。你可以直接告诉我，我会写进长期记忆。"
	case strings.Contains(message, "职业") || strings.Contains(message, "工作"):
		if profile.BasicInfo.Occupation != "" {
			return fmt.Sprintf("记得的。你跟我说过，你的职业是%s。这个信息我已经放进长期记忆里了，之后聊到压力、安排或者建议时，我会把它当成你的背景来考虑。%s", profile.BasicInfo.Occupation, relationshipStyleSuffix(profile))
		}
		return "我还没有可靠地记到你的职业。你可以直接说“我是后端工程师”或“我是一名 CEO”，我会写进长期记忆。"
	case strings.Contains(message, "名字") || strings.Contains(message, "叫什么") || strings.Contains(message, "称呼"):
		if profile.BasicInfo.Name != "" {
			return fmt.Sprintf("记得呀，你叫%s。之后我会尽量用这个名字和你说话，这样不像每次都重新认识。%s", profile.BasicInfo.Name, relationshipStyleSuffix(profile))
		}
		return "我还没有记到你的名字。你告诉我“我叫...”之后，我会把它写入结构化记忆。"
	case strings.Contains(message, "城市") || strings.Contains(message, "哪里") || strings.Contains(message, "在哪"):
		if profile.BasicInfo.City != "" {
			return fmt.Sprintf("记得，你现在在%s。要是之后你搬家或者城市变了，直接告诉我就好，我会更新当前记忆，也保留旧记录。%s", profile.BasicInfo.City, relationshipStyleSuffix(profile))
		}
		return "我还没有可靠地记到你的城市。你可以说“我在上海”或“我已经搬到深圳了”。"
	case strings.Contains(message, "喜欢") || strings.Contains(message, "讨厌") || strings.Contains(message, "偏好"):
		if len(profile.Preferences) > 0 {
			return fmt.Sprintf("记得。%s。以后我会尽量避开你不喜欢的风格，也多靠近让你舒服的方式。%s", describePreferences(profile), relationshipStyleSuffix(profile))
		}
		return "我还没有记到你的偏好。你可以告诉我你喜欢或不喜欢什么。"
	default:
		summary := summarizeKnownMemory(profile)
		if summary != "" {
			return fmt.Sprintf("我记得。慢慢拼起来，我现在对你的认识是：%s。不是很完整，但已经不是空白了。%s", summary, relationshipStyleSuffix(profile))
		}
		return fmt.Sprintf("%s，我目前还没有记到足够明确的信息。你可以告诉我姓名、职业、城市、偏好或最近的重要事件。", name)
	}
}

func answerCompositeMemoryQuestion(profile *memory.UserProfile, message string) string {
	var parts []string
	if strings.Contains(message, "名字") || strings.Contains(message, "叫什么") || strings.Contains(message, "称呼") {
		if profile.BasicInfo.Name != "" {
			parts = append(parts, "你叫"+profile.BasicInfo.Name)
		}
	}
	if strings.Contains(message, "职业") || strings.Contains(message, "工作") {
		if profile.BasicInfo.Occupation != "" {
			parts = append(parts, "职业是"+profile.BasicInfo.Occupation)
		}
	}
	if strings.Contains(message, "城市") || strings.Contains(message, "哪里") || strings.Contains(message, "在哪") {
		if profile.BasicInfo.City != "" {
			parts = append(parts, "现在在"+profile.BasicInfo.City)
		}
	}
	if strings.Contains(message, "年龄") || strings.Contains(message, "几岁") || strings.Contains(message, "多大") {
		if profile.BasicInfo.Age > 0 {
			parts = append(parts, fmt.Sprintf("今年%d岁", profile.BasicInfo.Age))
		}
	}
	if strings.Contains(message, "作息") || strings.Contains(message, "熬夜") || strings.Contains(message, "睡") {
		if profile.BasicInfo.Schedule != "" {
			parts = append(parts, "作息是"+profile.BasicInfo.Schedule)
		}
	}
	if strings.Contains(message, "偏好") || strings.Contains(message, "喜欢") || strings.Contains(message, "讨厌") {
		if prefs := describePreferences(profile); prefs != "" {
			parts = append(parts, prefs)
		}
	}
	if len(parts) >= 2 {
		return fmt.Sprintf("当然记得。你不是一串临时输入，我这里有认真把你说过的事放好：%s。%s", strings.Join(parts, "，"), relationshipStyleSuffix(profile))
	}
	return ""
}

func isMemoryQuestion(message string) bool {
	return strings.Contains(message, "记得") ||
		strings.Contains(message, "记住") ||
		strings.Contains(message, "我的") ||
		strings.Contains(message, "我是什么") ||
		strings.Contains(message, "你知道我") ||
		strings.Contains(message, "你了解我")
}

func summarizeKnownMemory(profile *memory.UserProfile) string {
	var parts []string
	if profile.BasicInfo.Name != "" {
		parts = append(parts, "你叫"+profile.BasicInfo.Name)
	}
	if profile.BasicInfo.Occupation != "" {
		parts = append(parts, "职业是"+profile.BasicInfo.Occupation)
	}
	if profile.BasicInfo.City != "" {
		parts = append(parts, "现在在"+profile.BasicInfo.City)
	}
	if profile.BasicInfo.Age > 0 {
		parts = append(parts, fmt.Sprintf("今年%d岁", profile.BasicInfo.Age))
	}
	if profile.BasicInfo.Schedule != "" {
		parts = append(parts, "作息是"+profile.BasicInfo.Schedule)
	}
	if len(profile.Preferences) > 0 {
		parts = append(parts, describePreferences(profile))
	}
	if len(profile.ImportantEvents) > 0 {
		parts = append(parts, "最近提到过"+profile.ImportantEvents[len(profile.ImportantEvents)-1].Name)
	}
	return strings.Join(parts, "，")
}

func describePreferences(profile *memory.UserProfile) string {
	var likes []string
	var dislikes []string
	for _, pref := range profile.Preferences {
		if pref.Kind == "like" {
			likes = append(likes, pref.Value)
		}
		if pref.Kind == "dislike" {
			dislikes = append(dislikes, pref.Value)
		}
	}
	var parts []string
	if len(likes) > 0 {
		parts = append(parts, "你喜欢"+strings.Join(likes, "、"))
	}
	if len(dislikes) > 0 {
		parts = append(parts, "不喜欢"+strings.Join(dislikes, "、"))
	}
	return strings.Join(parts, "，")
}

func relationshipStyleSuffix(profile *memory.UserProfile) string {
	var parts []string
	if profile.RelationshipPreference.Warmth {
		parts = append(parts, "我会把语气放软一点")
	}
	if profile.RelationshipPreference.Rationality {
		parts = append(parts, "需要建议时也会直接帮你抓重点")
	}
	if len(parts) == 0 {
		return "如果之后有变化，你直接告诉我，我会跟着更新。"
	}
	return strings.Join(parts, "，") + "，尽量既有人味，也不绕远。"
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

type TemplateReplyTool struct{}

func (TemplateReplyTool) Name() string {
	return "template_relationship_reply_tool"
}

func (TemplateReplyTool) Description() string {
	return "Generates deterministic warm replies from structured relationship memory."
}

func (TemplateReplyTool) Generate(profile *memory.UserProfile, message string, report memory.UpdateReport) (string, error) {
	return GenerateReply(profile, message, report), nil
}
