package bitz

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestJobClaimIsIdempotentAndApprovalRequiresWaitingState(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	jobs, err := NewJobs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	job, claimed, err := jobs.Claim(ctx, "quote-1", "account", "contact", "5511@s.whatsapp.net")
	if err != nil || !claimed {
		t.Fatal(job, claimed, err)
	}
	if _, claimed, err = jobs.Claim(ctx, "quote-1", "account", "contact", "5511@s.whatsapp.net"); err != nil || claimed {
		t.Fatal(claimed, err)
	}
	if _, err = jobs.Approve(ctx, job.Code); err == nil {
		t.Fatal("running job approved")
	}
	if err = jobs.AwaitingApproval(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	approved, err := jobs.Approve(ctx, job.Code)
	if err != nil || approved.Status != JobApproved {
		t.Fatal(approved, err)
	}
}
