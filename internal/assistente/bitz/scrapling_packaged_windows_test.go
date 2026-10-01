//go:build windows && bitzpackaged

package bitz

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestScraplingRuntimeStartsAndUsesJSONProtocol(t *testing.T) {
	executable, args, err := scraplingCommand()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, args...)
	cmd.Stdin = strings.NewReader(`{"mode":"bad"}`)
	var output bytes.Buffer
	cmd.Stdout = &output
	_ = cmd.Run()
	var result ScraplingResult
	if err = json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid runtime output: %v", err)
	}
	if result.Stage != "input" || result.Code != "invalid_mode" {
		t.Fatalf("unexpected runtime result: %+v", result)
	}
}
