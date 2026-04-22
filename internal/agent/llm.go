package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"relationship-agent-runtime/internal/memory"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-4o-mini"
)

type LLMConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	Timeout     time.Duration
}

func LLMConfigFromEnv() LLMConfig {
	cfg := LLMConfig{
		APIKey:      strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/"),
		Model:       strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		Temperature: 0.7,
		Timeout:     25 * time.Second,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultOpenAIModel
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_TEMPERATURE")); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = parsed
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_TIMEOUT_SECONDS")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cfg.Timeout = time.Duration(parsed) * time.Second
		}
	}
	return cfg
}

func (c LLMConfig) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

func NewRuntimeFromEnv(store memory.Store) *AgentRuntime {
	rt := NewRuntime(store)
	cfg := LLMConfigFromEnv()
	if !cfg.Enabled() {
		return rt
	}
	client := NewOpenAICompatibleClient(cfg)
	rt.Extractor = FallbackExtractionTool{
		Primary:  LLMExtractionTool{Client: client},
		Fallback: RuleBasedExtractor{},
	}
	rt.Replyer = LLMReplyTool{Client: client}
	return rt
}

type OpenAICompatibleClient struct {
	cfg    LLMConfig
	client *http.Client
}

func NewOpenAICompatibleClient(cfg LLMConfig) *OpenAICompatibleClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultOpenAIModel
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 25 * time.Second
	}
	return &OpenAICompatibleClient{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
	} `json:"error,omitempty"`
}

func (c *OpenAICompatibleClient) Complete(systemPrompt, userPrompt string, temperature float64) (string, error) {
	if c == nil {
		return "", errors.New("llm client is nil")
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return "", errors.New("OPENAI_API_KEY is empty")
	}
	reqBody := chatCompletionRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("llm returned empty completion")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

type FallbackExtractionTool struct {
	Primary  ExtractionTool
	Fallback ExtractionTool
}

func (t FallbackExtractionTool) Name() string {
	return t.Primary.Name() + "_with_" + t.Fallback.Name() + "_fallback"
}

func (t FallbackExtractionTool) Description() string {
	return "Extracts with a primary tool and falls back to a deterministic extractor when needed."
}

func (t FallbackExtractionTool) Extract(message string) (memory.ExtractedFacts, error) {
	facts, err := t.Primary.Extract(message)
	if err == nil {
		return facts, nil
	}
	fallbackFacts, fallbackErr := t.Fallback.Extract(message)
	if fallbackErr != nil {
		return memory.ExtractedFacts{}, fmt.Errorf("primary failed: %v; fallback failed: %w", err, fallbackErr)
	}
	return fallbackFacts, nil
}

type LLMExtractionTool struct {
	Client *OpenAICompatibleClient
}

func (LLMExtractionTool) Name() string {
	return "llm_information_extractor"
}

func (LLMExtractionTool) Description() string {
	return "Extracts structured relationship facts with an OpenAI-compatible chat completion model."
}

func (t LLMExtractionTool) Extract(message string) (memory.ExtractedFacts, error) {
	system := `你是关系型 Agent Runtime 的信息抽取工具。只输出 JSON，不要解释。
字段结构：
{
  "basic_info": {"name":"","age":0,"occupation":"","city":"","schedule":""},
  "preferences": [{"kind":"like|dislike","value":"","confidence":0.0}],
  "emotional_states": [{"label":"","intensity":1,"reason":""}],
  "important_events": [{"name":"","status":"mentioned|upcoming|finished"}],
  "relationship_preference": {"tone":"","need":"","humor":false,"rationality":false,"warmth":false}
}
只抽取用户明确表达的信息；不确定就留空或空数组。`
	raw, err := t.Client.Complete(system, message, 0.1)
	if err != nil {
		return memory.ExtractedFacts{}, err
	}
	var facts memory.ExtractedFacts
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &facts); err != nil {
		return memory.ExtractedFacts{}, fmt.Errorf("parse extracted facts: %w", err)
	}
	now := time.Now()
	for i := range facts.Preferences {
		facts.Preferences[i].Evidence = message
		facts.Preferences[i].UpdatedAt = now
		if facts.Preferences[i].Confidence == 0 {
			facts.Preferences[i].Confidence = 0.78
		}
	}
	for i := range facts.EmotionalStates {
		facts.EmotionalStates[i].Evidence = message
		facts.EmotionalStates[i].ObservedAt = now
	}
	for i := range facts.ImportantEvents {
		facts.ImportantEvents[i].Evidence = message
		facts.ImportantEvents[i].ObservedAt = now
	}
	return facts, nil
}

type LLMReplyTool struct {
	Client *OpenAICompatibleClient
}

func (LLMReplyTool) Name() string {
	return "llm_relationship_reply_tool"
}

func (LLMReplyTool) Description() string {
	return "Generates warm relationship-aware replies with an OpenAI-compatible chat completion model."
}

func (t LLMReplyTool) Generate(profile *memory.UserProfile, message string, report memory.UpdateReport) (string, error) {
	profileJSON, _ := json.Marshal(profile)
	reportJSON, _ := json.Marshal(report)
	system := `你是一个温柔、稳定、有边界感的人机关系 Agent。你要像持续认识用户的人一样说话。
规则：
1. 必须基于结构化记忆回复，不要假装知道记忆里没有的信息。
2. 如果有冲突，说明采用最新说法，并温柔提到旧信息已保留为历史。
3. 语气亲近、有人的味道，但不要声称自己是真人、恋人或拥有真实情感。
4. 输出中文，80 到 180 字，最多问一个自然的后续问题。
5. 不要输出执行轨迹，执行轨迹由 Runtime 单独展示。`
	user := fmt.Sprintf("用户消息：%s\n\n当前结构化记忆：%s\n\n本轮记忆更新报告：%s", message, profileJSON, reportJSON)
	return t.Client.Complete(system, user, t.Client.cfg.Temperature)
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end >= start {
		return raw[start : end+1]
	}
	return raw
}
