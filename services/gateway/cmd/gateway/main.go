// TekSes Faz 0 gateway'i: saat senkronu yanıtlar, kue ve müdahale yayınlar.
//
// Kullanım:
//
//	go run ./services/gateway/cmd/gateway [-addr :8080]
//
// Ortam değişkenleri:
//
//	TEKSES_ADDR         dinlenecek adres (bayrak öncelikli, varsayılan :8080)
//	TEKSES_ADMIN_TOKEN  boş değilse /api/* uçları Bearer token ister
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

	"github.com/msaliheroglu/tekses/services/gateway/internal/server"
)

func main() {
	defaultAddr := os.Getenv("TEKSES_ADDR")
	if defaultAddr == "" {
		defaultAddr = ":8080"
	}
	addr := flag.String("addr", defaultAddr, "dinlenecek adres")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.New(log, os.Getenv("TEKSES_ADMIN_TOKEN"))

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("gateway dinliyor", "addr", *addr)
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
