// TekSes kontrol API'si: organizasyon/etkinlik/oda/gösteri yönetimi.
//
// Kullanım:
//
//	go run ./services/control-api/cmd/control-api [-addr :8090]
//
// Ortam değişkenleri:
//
//	TEKSES_CONTROL_ADDR   dinlenecek adres (bayrak öncelikli, varsayılan :8090)
//	TEKSES_DATABASE_URL   ayarlıysa Postgres kalıcılığı (migration'lar açılışta
//	                      uygulanır); ayarsızsa bellek içi depo — süreç ölünce
//	                      veri gider, yalnızca yerel geliştirme için
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/msaliheroglu/tekses/packages/blob"
	"github.com/msaliheroglu/tekses/services/control-api/internal/api"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/memstore"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/pg"
)

func main() {
	defaultAddr := os.Getenv("TEKSES_CONTROL_ADDR")
	if defaultAddr == "" {
		defaultAddr = ":8090"
	}
	addr := flag.String("addr", defaultAddr, "dinlenecek adres")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var st store.Store
	if dbURL := os.Getenv("TEKSES_DATABASE_URL"); dbURL != "" {
		pgStore, err := pg.Open(context.Background(), dbURL)
		if err != nil {
			log.Error("postgres açılamadı", "hata", err)
			os.Exit(1)
		}
		defer pgStore.Close()
		st = pgStore
		log.Info("depolama: postgres")
	} else {
		st = memstore.New()
		log.Warn("depolama: bellek içi — veriler süreçle birlikte silinir (TEKSES_DATABASE_URL ayarlayın)")
	}

	packagesDir := os.Getenv("TEKSES_PACKAGES_DIR")
	if packagesDir == "" {
		packagesDir = "data/packages"
	}
	packages, err := blob.NewFS(packagesDir)
	if err != nil {
		log.Error("paket deposu açılamadı", "hata", err)
		os.Exit(1)
	}
	log.Info("paket deposu", "dizin", packagesDir)

	srv := api.New(log, st, packages)

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("control-api dinliyor", "addr", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("sunucu durdu", "hata", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("kapanılıyor")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
