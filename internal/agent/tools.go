package agent

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"relationship-agent-runtime/internal/memory"
)

type RuleBasedExtractor struct {
	FailToken string
}

func (RuleBasedExtractor) Name() string {
	return "rule_based_information_extractor"
}

func (RuleBasedExtractor) Description() string {
	return "Extracts structured relationship facts from user text with deterministic rules."
}

func (e RuleBasedExtractor) Extract(message string) (memory.ExtractedFacts, error) {
	if e.FailToken != "" && strings.Contains(message, e.FailToken) {
		return memory.ExtractedFacts{}, errors.New("simulated extraction failure")
	}
	now := time.Now()
	facts := memory.ExtractedFacts{}
	msg := strings.TrimSpace(message)

	if v := firstGroup(msg, `(?:我叫|我的名字叫)([\p{Han}A-Za-z0-9_ -]{1,20})`); v != "" {
		facts.BasicInfo.Name = cleanValue(v)
	}
	if v := firstGroup(msg, `(?:我今年|年龄是)(\d{1,3})`); v != "" {
		if age, err := strconv.Atoi(v); err == nil && age > 0 && age < 130 {
			facts.BasicInfo.Age = age
		}
	}
	if v := firstGroup(msg, `(?:我在|我住在|现在在|搬到|搬去了|已经搬到)([\p{Han}A-Za-z]{2,20})`); v != "" {
		facts.BasicInfo.City = cleanValue(v)
	}
	if v := firstGroup(msg, `(?:我是|职业是|工作是|，是|,是)([^，。,.!！]{1,24}(?:工程师|程序员|学生|老师|产品经理|设计师|医生|律师|运营|研究员))`); v != "" {
		facts.BasicInfo.Occupation = cleanValue(v)
	}
	if v := firstGroup(msg, `(?:作息是|通常|一般)([^，。,.!！]*(?:早睡|晚睡|熬夜|早起|夜班|九点睡|十二点睡)[^，。,.!！]*)`); v != "" {
		facts.BasicInfo.Schedule = cleanValue(v)
	}

	if v := firstGroup(msg, `喜欢([^，。,.!！]{1,30})`); v != "" {
		facts.Preferences = append(facts.Preferences, memory.Preference{Kind: "like", Value: cleanValue(v), Confidence: 0.82, Evidence: msg, UpdatedAt: now})
	}
	if v := firstGroup(msg, `(?:讨厌|不喜欢)([^，。,.!！]{1,30})`); v != "" {
		facts.Preferences = append(facts.Preferences, memory.Preference{Kind: "dislike", Value: cleanValue(v), Confidence: 0.82, Evidence: msg, UpdatedAt: now})
	}

	emotions := map[string]int{
		"焦虑": 4, "紧张": 4, "压力": 4, "疲惫": 3, "累": 3,
		"开心": 3, "高兴": 3, "难过": 4, "失落": 3, "烦": 3,
	}
	for label, intensity := range emotions {
		if strings.Contains(msg, label) {
			facts.EmotionalStates = append(facts.EmotionalStates, memory.EmotionState{Label: label, Intensity: intensity, Evidence: msg, ObservedAt: now})
			break
		}
	}

	eventKeywords := []string{"考试", "面试", "搬家", "分手", "项目", "DDL", "答辩", "code review"}
	for _, kw := range eventKeywords {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(kw)) {
			facts.ImportantEvents = append(facts.ImportantEvents, memory.ImportantEvent{Name: kw, Status: inferEventStatus(msg), Evidence: msg, ObservedAt: now})
			break
		}
	}

	if strings.Contains(msg, "温柔") {
		facts.RelationshipPreference.Warmth = true
		facts.RelationshipPreference.Tone = "warm"
	}
	if strings.Contains(msg, "理性") || strings.Contains(msg, "直接") {
		facts.RelationshipPreference.Rationality = true
		facts.RelationshipPreference.Tone = "rational"
	}
	if strings.Contains(msg, "幽默") || strings.Contains(msg, "开玩笑") {
		facts.RelationshipPreference.Humor = true
	}
	if strings.Contains(msg, "陪伴") {
		facts.RelationshipPreference.Need = "companionship"
	}
	if strings.Contains(msg, "安慰") {
		facts.RelationshipPreference.Need = "comfort"
	}
	if strings.Contains(msg, "建议") {
		facts.RelationshipPreference.Need = "advice"
	}
	return facts, nil
}

type PersistentMemoryTool struct {
	Store memory.Store
}

func (PersistentMemoryTool) Name() string {
	return "persistent_memory_tool"
}

func (PersistentMemoryTool) Description() string {
	return "Loads, updates, and saves structured user relationship memory."
}

func (t PersistentMemoryTool) Load(userID string) (*memory.UserProfile, error) {
	return t.Store.Load(userID)
}

func (t PersistentMemoryTool) Save(profile *memory.UserProfile) error {
	return t.Store.Save(profile)
}

func (PersistentMemoryTool) Update(profile *memory.UserProfile, facts memory.ExtractedFacts, evidence string) memory.UpdateReport {
	return memory.ApplyFacts(profile, facts, evidence, time.Now())
}

func firstGroup(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(s)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func cleanValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "，。,.!！?？；; ")
	stopWords := []string{"了", "呀", "啊", "吧", "呢"}
	for _, sw := range stopWords {
		v = strings.TrimSuffix(v, sw)
	}
	return strings.TrimSpace(v)
}

func inferEventStatus(msg string) string {
	switch {
	case strings.Contains(msg, "明天") || strings.Contains(msg, "下周") || strings.Contains(msg, "快要"):
		return "upcoming"
	case strings.Contains(msg, "刚") || strings.Contains(msg, "已经") || strings.Contains(msg, "结束"):
		return "finished"
	default:
		return "mentioned"
	}
}
