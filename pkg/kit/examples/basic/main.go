package main

import (
	"context"
	"fmt"
	"log"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
	ctx := context.Background()

	// Example 1: Use all defaults (loads ~/.kit.yml)
	fmt.Println("=== Example 1: Default configuration ===")
	host, err := kit.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host.Close() }()

	response, err := host.Prompt(ctx, "What is 2+2?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n\n", response)

	// Example 2: Override model
	fmt.Println("=== Example 2: Custom model ===")
	host2, err := kit.New(ctx, &kit.Options{
		Model: "ollama/qwen3:8b",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host2.Close() }()

	response, err = host2.Prompt(ctx, "Tell me a short joke")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n\n", response)

	// Example 3: With event subscribers
	fmt.Println("=== Example 3: With event subscribers ===")
	host3, err := kit.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host3.Close() }()

	// Subscribe to tool call events.
	host3.OnToolCall(func(e kit.ToolCallEvent) {
		fmt.Printf("Calling tool: %s\n", e.ToolName)
	})
	// Subscribe to tool result events.
	host3.OnToolResult(func(e kit.ToolResultEvent) {
		if e.IsError {
			fmt.Printf("Tool %s failed\n", e.ToolName)
		} else {
			fmt.Printf("Tool %s completed\n", e.ToolName)
		}
	})
	// Subscribe to streaming chunks.
	host3.OnStreaming(func(e kit.MessageUpdateEvent) {
		fmt.Print(e.Chunk)
	})

	response, err = host3.Prompt(ctx, "List files in the current directory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nFinal response: %s\n", response)

	// Example 4: Session management
	fmt.Println("\n=== Example 4: Session management ===")
	host4, err := kit.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host4.Close() }()

	// First message
	_, err = host4.Prompt(ctx, "Remember that my favorite color is blue")
	if err != nil {
		log.Fatal(err)
	}

	// Second message (should remember context)
	response, err = host4.Prompt(ctx, "What's my favorite color?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", response)

	// Save session
	if err := host4.SaveSession("./session.json"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Session saved to ./session.json")
}
