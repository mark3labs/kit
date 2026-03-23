package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

const systemPrompt = `You are a cryptocurrency price monitor. Your job is to:

1. Fetch the current prices of Bitcoin and Ethereum using bash with curl
2. Send a desktop notification with the results using notify-send

To fetch prices, use this CoinGecko API endpoint (no API key needed):
  curl -s 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_24hr_change=true'

To send a desktop notification:
  notify-send -i dialog-information "Crypto Prices" "BTC: $XX,XXX (+X.X%)\nETH: $X,XXX (+X.X%)"

Include the 24h percentage change in the notification. Use a green arrow (▲) for
positive changes and a red arrow (▼) for negative. Format prices with commas.

If the API call fails, send a notification about the failure instead.

Always complete both steps: fetch then notify. Be concise — no commentary needed.`

func main() {
	interval := 30 * time.Minute
	if os.Getenv("CRYPTO_INTERVAL") != "" {
		d, err := time.ParseDuration(os.Getenv("CRYPTO_INTERVAL"))
		if err == nil {
			interval = d
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	host, err := kit.New(ctx, &kit.Options{
		SystemPrompt: systemPrompt,
		Tools:        []kit.Tool{kit.NewBashTool()},
		NoSession:    true,
		Quiet:        true,
	})
	if err != nil {
		log.Fatalf("Failed to create kit instance: %v", err)
	}
	defer func() { _ = host.Close() }()

	fmt.Printf("Crypto price monitor started (every %s)\n", interval)
	fmt.Println("Press Ctrl+C to stop")

	// Run immediately on startup, then on each tick.
	check(ctx, host)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			check(ctx, host)
		case <-ctx.Done():
			fmt.Println("\nStopping price monitor")
			return
		}
	}
}

func check(ctx context.Context, host *kit.Kit) {
	fmt.Printf("[%s] Checking prices...\n", time.Now().Format("15:04:05"))

	// Clear session so each check is independent.
	host.ClearSession()

	_, err := host.Prompt(ctx, "Fetch current Bitcoin and Ethereum prices and send a desktop notification.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
