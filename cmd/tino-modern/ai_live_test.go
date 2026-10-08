package main

import (
	"github.com/gadevsbr/tino/internal/capability"
	"os"
	"path/filepath"
	"testing"
)

func TestAILiveDesktopBridge(t *testing.T) {
	if os.Getenv("TINO_AI_LIVE_TEST") != "1" {
		t.Skip("explicit live AI test required")
	}
	base, err := filepath.Abs(filepath.Join("..", "..", "dist", "data"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{capabilities: capability.NewStore(filepath.Join(base, "capabilities.json"))}
	status, err := app.GetAIStatus()
	if err != nil || !status.Ready {
		t.Fatalf("AI connection unavailable: %v", err)
	}
	reply, err := app.TestAI("Responda em portugues: como falar com a equipe humana?")
	if err != nil || reply == "" {
		t.Fatalf("real AI request failed: %v", err)
	}
	t.Log("Cloudflare returned a nonempty response through the desktop bridge")
}
