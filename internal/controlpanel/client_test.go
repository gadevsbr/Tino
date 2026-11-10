package controlpanel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncAndAckUseAuthenticatedContract(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("autorização ausente: %q", r.Header.Get("Authorization"))
		}
		calls++
		if r.URL.Path == "/api/bridge/sync" {
			var snapshot Snapshot
			if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil || snapshot.InstanceID != "desk-1" || len(snapshot.Contacts) != 1 {
				t.Fatalf("snapshot inválido: %+v err=%v", snapshot, err)
			}
			_, _ = w.Write([]byte(`{"commands":[{"ID":"c1","Account":"a","Contact":"b","Action":"pause"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Client{Endpoint: server.URL, Token: "secret", InstanceID: "desk-1", HTTP: server.Client()}
	commands, err := client.Sync(context.Background(), Snapshot{Contacts: []Contact{{Account: "a", Contact: "b", Stage: "new"}}})
	if err != nil || len(commands) != 1 || commands[0].Action != "pause" {
		t.Fatalf("comandos inválidos: %+v err=%v", commands, err)
	}
	if err := client.Ack(context.Background(), "c1", nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("chamadas=%d", calls)
	}
}
