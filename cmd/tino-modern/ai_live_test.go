package main

import (
	"context"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/flow"
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

func TestLiveCommercialUnderstanding(t *testing.T) {
	if os.Getenv("TINO_AI_LIVE_TEST") != "1" {
		t.Skip("explicit live test required")
	}
	base, err := filepath.Abs(filepath.Join("..", "..", "dist", "data"))
	if err != nil {
		t.Fatal(err)
	}
	ai, err := flow.ConfiguredAI(capability.NewStore(filepath.Join(base, "capabilities.json")))
	if err != nil || ai == nil {
		t.Fatal("AI unavailable", err)
	}
	for _, tc := range []struct{ step, message, value string }{{"PUBLIC_QUOTE_ROOMS", "para 01", "1"}, {"PUBLIC_QUOTE_ROOM_ADULTS", "somos um casal", "2"}, {"PUBLIC_QUOTE_ROOM_CHILDREN", "não temos crianças", "0"}, {"PUBLIC_QUOTE_ROOM_AGES", "minha filha tem três anos", "3"}} {
		r, err := ai.Converse(context.Background(), commercial.DialogueRequest{Task: "interpret", Step: tc.step, Message: tc.message, Prompt: tc.step, Today: commercial.HotelToday()})
		if err != nil || !r.Understood || r.Value != tc.value {
			t.Fatalf("%s: %#v, %v", tc.step, r, err)
		}
		t.Logf("%s understood as %s", tc.message, r.Value)
	}
	app := &App{capabilities: capability.NewStore(filepath.Join(base, "capabilities.json"))}
	preview, err := app.PreviewCommercialAI("para 01")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(preview)
}
