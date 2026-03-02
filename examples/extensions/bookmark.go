//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kit/ext"
)

// Init adds bookmark commands for marking and recalling important points in
// a conversation. Bookmarks are persisted in the session tree and survive
// restarts.
//
// Commands:
//
//	/bookmark <label>  — bookmark the current point with a label
//	/bookmarks         — list all bookmarks in this session
//
// Usage: kit -e examples/extensions/bookmark.go
func Init(api ext.API) {
	api.RegisterCommand(ext.CommandDef{
		Name:        "bookmark",
		Description: "Bookmark the current point in the conversation",
		Execute: func(args string, ctx ext.Context) (string, error) {
			label := strings.TrimSpace(args)
			if label == "" {
				label = time.Now().Format("15:04:05")
			}

			// Count existing messages to record position.
			msgs := ctx.GetMessages()

			data, _ := json.Marshal(map[string]any{
				"label":    label,
				"messages": len(msgs),
			})

			_, err := ctx.AppendEntry("bookmark", string(data))
			if err != nil {
				ctx.PrintError("Failed to save bookmark: " + err.Error())
				return "", nil
			}

			ctx.PrintInfo(fmt.Sprintf("Bookmarked: %s (at message %d)", label, len(msgs)))
			return "", nil
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "bookmarks",
		Description: "List all bookmarks in this session",
		Execute: func(args string, ctx ext.Context) (string, error) {
			entries := ctx.GetEntries("bookmark")
			if len(entries) == 0 {
				ctx.PrintInfo("No bookmarks yet. Use /bookmark <label> to create one.")
				return "", nil
			}

			var lines []string
			for i, e := range entries {
				var data map[string]any
				if err := json.Unmarshal([]byte(e.Data), &data); err != nil {
					continue
				}
				label, _ := data["label"].(string)
				msgCount, _ := data["messages"].(float64)
				lines = append(lines, fmt.Sprintf("  %d. %s (msg %d, %s)",
					i+1, label, int(msgCount), e.Timestamp[:19]))
			}

			ctx.PrintInfo("Bookmarks:\n" + strings.Join(lines, "\n"))
			return "", nil
		},
	})
}
