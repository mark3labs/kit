package kit_test

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/kit/pkg/kit"
)

func ExampleLoginProvider() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// OpenAI uses the existing ChatGPT/Codex subscription login, not general
	// OpenAI API OAuth. The host decides how to display or open the URL.
	err := kit.LoginProvider(ctx, "openai", kit.LoginOptions{
		OnAuthorization: func(ctx context.Context, login kit.LoginInstructions) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			fmt.Println("Open this URL in a browser:", login.URL)
			return nil
		},
	})
	if err != nil {
		fmt.Println("Login failed:", err)
		return
	}
	// Credentials are saved to $XDG_CONFIG_HOME/.kit/credentials.json (0600),
	// or ~/.config/.kit/credentials.json. Keep this store writable for refresh.
	// Call SetModel on an existing Kit, or create a new Kit, to use them.
	// A remote browser needs port forwarding to the host's loopback port 1455.
	// There is no manual code-entry fallback.
}
