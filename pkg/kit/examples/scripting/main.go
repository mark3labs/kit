package main

import (
	"context"
	"fmt"
	"log"
	"os"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
	ctx := context.Background()

	// Create Kit with environment variable for API key
	// Expects ANTHROPIC_API_KEY or appropriate provider key to be set
	host, err := kit.New(ctx, &kit.Options{
		Quiet: true, // Suppress debug output for scripting
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host.Close() }()

	// Process command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go \"your prompt here\"")
		os.Exit(1)
	}

	prompt := os.Args[1]

	// Send prompt and get response
	response, err := host.Prompt(ctx, prompt)
	if err != nil {
		log.Fatal(err)
	}

	// Output only the response (useful for piping)
	fmt.Println(response)
}
