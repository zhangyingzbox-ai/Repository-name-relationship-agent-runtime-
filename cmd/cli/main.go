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
	flag.Parse()

	runtime := agent.NewRuntime(memory.NewJSONStore(*dataDir))
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Relationship Agent Runtime CLI. Type /exit to quit.")
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "/exit" || text == "/quit" {
			break
		}
		resp := runtime.Chat(agent.ChatRequest{UserID: *userID, Message: text})
		printTrace(resp.Trace)
		fmt.Println("Final Response:", resp.FinalResponse)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read input failed:", err)
	}
}

func printTrace(trace []agent.TraceStep) {
	for _, step := range trace {
		b, _ := json.Marshal(step)
		fmt.Println(string(b))
	}
}
