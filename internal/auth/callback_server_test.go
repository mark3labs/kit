package auth

import (
	"net"
	"net/http"
	"testing"
	"time"
)

// TestCallbackLoopbackFamilies checks state rejection, valid callbacks, and cleanup on each loopback family.
func TestCallbackLoopbackFamilies(t *testing.T) {
	for _, address := range []string{"127.0.0.1:1455", "[::1]:1455"} {
		t.Run(address, func(t *testing.T) {
			probe, err := net.Listen("tcp", address)
			if err != nil {
				t.Skipf("loopback address unavailable: %v", err)
			}
			_ = probe.Close()
			server, err := StartOpenAICallbackServer("expected")
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			client := &http.Client{Timeout: time.Second}
			response, err := client.Get("http://" + address + "/auth/callback?state=wrong&error=access_denied")
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatal("wrong state accepted")
			}
			select {
			case err := <-server.ErrorChan:
				t.Fatalf("wrong state stopped login: %v", err)
			default:
			}
			response, err = client.Get("http://" + address + "/auth/callback?state=expected&code=code")
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			select {
			case code := <-server.CodeChan:
				if code != "code" {
					t.Fatal("wrong code")
				}
			case <-time.After(time.Second):
				t.Fatal("callback not received")
			}
			server.Close()
			listener, err := net.Listen("tcp", address)
			if err != nil {
				t.Fatalf("listener leaked: %v", err)
			}
			_ = listener.Close()
		})
	}
}

// TestCallbackSingleFamilyFallback checks startup and cleanup when only one loopback family is available.
func TestCallbackSingleFamilyFallback(t *testing.T) {
	for _, occupied := range []string{"127.0.0.1:1455", "[::1]:1455"} {
		t.Run(occupied, func(t *testing.T) {
			blocker, err := net.Listen("tcp", occupied)
			if err != nil {
				t.Skipf("address unavailable: %v", err)
			}
			defer func() { _ = blocker.Close() }()
			other := "[::1]:1455"
			if occupied == other {
				other = "127.0.0.1:1455"
			}
			probe, err := net.Listen("tcp", other)
			if err != nil {
				t.Skipf("other family unavailable: %v", err)
			}
			_ = probe.Close()
			server, err := StartOpenAICallbackServer("expected")
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			response, err := (&http.Client{Timeout: time.Second}).Get("http://" + other + "/auth/callback?state=expected&error=access_denied")
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			select {
			case <-server.ErrorChan:
			case <-time.After(time.Second):
				t.Fatal("matching-state denial did not stop login")
			}
			server.Close()
			listener, err := net.Listen("tcp", other)
			if err != nil {
				t.Fatalf("fallback listener leaked: %v", err)
			}
			_ = listener.Close()
		})
	}
}
