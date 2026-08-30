// TekSes kontrol API'si: organizasyon/etkinlik/oda/gösteri yönetimi.
//
// Kullanım:
//
//	go run ./services/control-api/cmd/control-api [-addr :8090]
//
// Şimdilik depolama bellek içidir (süreç ölünce veri gider); Postgres
// kalıcılığı Faz 1 Adım 5'te aynı arayüzün arkasına gelecek.
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

	"github.com/msaliheroglu/tekses/services/control-api/internal/api"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/memstore"
)

func main() {
	defaultAddr := os.Getenv("TEKSES_CONTROL_ADDR")
	if defaultAddr == "" {
		defaultAddr = ":8090"
	}
	addr := flag.String("addr", defaultAddr, "dinlenecek adres")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := api.New(log, memstore.New())

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
