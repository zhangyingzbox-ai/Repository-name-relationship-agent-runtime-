package agent

import (
	"strings"
	"testing"

	"relationship-agent-runtime/internal/memory"
)

func TestBuildRelationshipAcrossThreeTurns(t *testing.T) {
	runtime := NewRuntime(memory.NewJSONStore(t.TempDir()))
	userID := "normal-case"

	r1 := runtime.Chat(ChatRequest{UserID: userID, Message: "我叫小王，我在上海，是后端工程师。我喜欢咖啡。"})
	r2 := runtime.Chat(ChatRequest{UserID: userID, Message: "最近项目DDL让我有点焦虑，希望你温柔一点，也给我建议。"})
	r3 := runtime.Chat(ChatRequest{UserID: userID, Message: "明天还有code review，我该怎么安排？"})

	if r1.Profile.BasicInfo.Name != "小王" {
		t.Fatalf("expected name 小王, got %q", r1.Profile.BasicInfo.Name)
	}
	if r2.Profile.RelationshipPreference.Need != "advice" || !r2.Profile.RelationshipPreference.Warmth {
		t.Fatalf("expected relationship preference to be updated, got %+v", r2.Profile.RelationshipPreference)
	}
	if r3.Profile.RelationshipState.TurnCount != 3 {
		t.Fatalf("expected 3 turns, got %d", r3.Profile.RelationshipState.TurnCount)
	}
	if !strings.Contains(r3.FinalResponse, "小王") && !strings.Contains(r3.FinalResponse, "code review") {
		t.Fatalf("expected reply to use memory or event context, got %q", r3.FinalResponse)
	}
}

func TestMemoryConflictUpdatesLatestCityAndKeepsHistory(t *testing.T) {
	runtime := NewRuntime(memory.NewJSONStore(t.TempDir()))
	userID := "conflict-case"

	runtime.Chat(ChatRequest{UserID: userID, Message: "我叫阿宁，我在上海。"})
	resp := runtime.Chat(ChatRequest{UserID: userID, Message: "其实我已经搬到深圳了。"})

	if resp.Profile.BasicInfo.City != "深圳" {
		t.Fatalf("expected latest city 深圳, got %q", resp.Profile.BasicInfo.City)
	}
	if len(resp.Profile.Conflicts) == 0 {
		t.Fatal("expected conflict history")
	}
	last := resp.Profile.Conflicts[len(resp.Profile.Conflicts)-1]
	if last.OldValue != "上海" || last.NewValue != "深圳" {
		t.Fatalf("unexpected conflict: %+v", last)
	}
	if !strings.Contains(resp.FinalResponse, "旧信息") && !strings.Contains(resp.FinalResponse, "历史") {
		t.Fatalf("expected reply to explain conflict handling, got %q", resp.FinalResponse)
	}
}

func TestExtractionFailureFallsBackAndContinues(t *testing.T) {
	store := memory.NewJSONStore(t.TempDir())
	runtime := NewRuntimeWithTools(RuleBasedExtractor{FailToken: "FAIL_EXTRACT"}, PersistentMemoryTool{Store: store})

	resp := runtime.Chat(ChatRequest{UserID: "failure-case", Message: "FAIL_EXTRACT 这句会触发抽取失败，但对话不能崩。"})

	if resp.FinalResponse == "" {
		t.Fatal("expected fallback response")
	}
	if resp.Profile.RelationshipState.TurnCount != 1 {
		t.Fatalf("expected turn count to update despite extraction failure, got %d", resp.Profile.RelationshipState.TurnCount)
	}
	foundFallback := false
	for _, step := range resp.Trace {
		if step.Step == "Step2: Extract user information" && step.Status == "fallback" {
			foundFallback = true
			break
		}
	}
	if !foundFallback {
		t.Fatalf("expected fallback trace, got %+v", resp.Trace)
	}
}
