package main

import (
	"testing"
	"time"
)

func TestActivitySignalUsesLatestOutcome(t *testing.T) {
	success := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	failure := success.Add(time.Minute)
	got := activitySignal("OmniBees", success, failure)
	if got.State != "error" || got.CheckedAt != failure.Format(time.RFC3339) {
		t.Fatalf("falha mais recente=%+v", got)
	}
	got = activitySignal("OmniBees", failure.Add(time.Minute), failure)
	if got.State != "ok" {
		t.Fatalf("sucesso mais recente=%+v", got)
	}
	if got = activitySignal("OmniBees", time.Time{}, time.Time{}); got.State != "unknown" {
		t.Fatalf("sem atividade=%+v", got)
	}
}
