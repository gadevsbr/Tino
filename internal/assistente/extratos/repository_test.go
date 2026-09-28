package extratos

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestWeekRepositoryPersistsAndDeduplicatesFiles(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	period, recognized, err := ParseWeeklyCommand("extrato semana 1 setembro 2026", time.Now())
	if err != nil || !recognized {
		t.Fatalf("parse recognized=%t err=%v", recognized, err)
	}
	period, err = period.WithDates("01/09/2026", "07/09/2026")
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.CreateWeek(ctx, Week{ID: "00112233445566778899aabbccddeeff", Period: period, CreatedBy: "operator"}); err != nil {
		t.Fatal(err)
	}
	if added, err := repo.AddFile(ctx, "00112233445566778899aabbccddeeff", "dia1.pdf", "/private/dia1.pdf", "abc", "operator"); err != nil || !added {
		t.Fatalf("first add=%t err=%v", added, err)
	}
	if added, err := repo.AddFile(ctx, "00112233445566778899aabbccddeeff", "repetido.pdf", "/private/repetido.pdf", "abc", "operator"); err != nil || added {
		t.Fatalf("duplicate add=%t err=%v", added, err)
	}
	files, err := repo.Files(ctx, "00112233445566778899aabbccddeeff")
	if err != nil || len(files) != 1 || files[0].OriginalName != "dia1.pdf" {
		t.Fatalf("files=%#v err=%v", files, err)
	}
	week, found, err := repo.FindWeek(ctx, period)
	if err != nil || !found || week.Period.From.Format("02/01/2006") != "01/09/2026" {
		t.Fatalf("week=%#v found=%t err=%v", week, found, err)
	}
}
