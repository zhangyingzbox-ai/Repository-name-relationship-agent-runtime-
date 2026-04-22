package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"relationship-agent-runtime/internal/agent"
	"relationship-agent-runtime/internal/memory"
)

func main() {
	userID := flag.String("user", "default", "user id")
	dataDir := flag.String("memory", "data/memory", "memory directory")
	showTrace := flag.Bool("trace", true, "show runtime trace")
	flag.Parse()

	runtime := agent.NewRuntime(memory.NewJSONStore(*dataDir))
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Relationship Agent Runtime CLI")
	fmt.Println("直接输入中文聊天。输入 /exit 退出。输入 /trace 可切换执行轨迹显示。")
	fmt.Println()
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "/exit" || text == "/quit" {
			break
		}
		if text == "/trace" {
			*showTrace = !*showTrace
			fmt.Printf("Trace display: %v\n", *showTrace)
			continue
		}
		if text == "" {
			continue
		}
		resp := runtime.Chat(agent.ChatRequest{UserID: *userID, Message: text})
		if *showTrace {
			printTrace(resp.Trace)
		}
		fmt.Println("Agent:", resp.FinalResponse)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read input failed:", err)
	}
}

func printTrace(trace []agent.TraceStep) {
	fmt.Println("Runtime Trace:")
	for _, step := range trace {
		b, _ := json.Marshal(step)
		fmt.Println(" ", string(b))
	}
}
