package pg

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/storetest"
)

// TestConformance, TEKSES_TEST_DATABASE_URL ayarlıysa gerçek Postgres'e
// karşı koşar (CI'da servis konteyneri; yerelde elle başlatılan sunucu).
// Her alt test, yalıtım için kendi taze şemasında çalışır: search_path o
// şemaya bakar, migration'lar tabloları oraya kurar.
func TestConformance(t *testing.T) {
	baseURL := os.Getenv("TEKSES_TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEKSES_TEST_DATABASE_URL ayarlı değil; Postgres uygunluk testi atlandı")
	}
	ctx := context.Background()

	boot, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer boot.Close()

	seq := 0
	storetest.Run(t, func(t *testing.T) store.Store {
		seq++
		schema := fmt.Sprintf("tekses_test_%d", seq)
		if _, err := boot.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Fatal(err)
		}
		if _, err := boot.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}

		sep := "?"
		if strings.Contains(baseURL, "?") {
			sep = "&"
		}
		url := baseURL + sep + "options=" + neturl.QueryEscape("-csearch_path="+schema)
		s, err := Open(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Close)
		return s
	})
}
