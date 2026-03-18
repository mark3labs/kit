//go:build ignore

package main

import (
	"fmt"
	"kit/ext"
)

// Helper functions for the status-tools extension
// These are used by main.go but kept in a separate file
// to demonstrate the multi-file extension pattern.

// formatMemory converts bytes to human-readable format
func formatMemory(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// showMemoryStatus displays memory usage (placeholder)
func showMemoryStatus(ctx ext.Context) {
	// This is a placeholder that would show memory stats
	// In a real extension, you'd integrate with system metrics
	ctx.PrintBlock(ext.PrintBlockOpts{
		Text:        "Memory status monitoring not yet implemented",
		BorderColor: "#f9e2af",
		Subtitle:    "Memory",
	})
}
