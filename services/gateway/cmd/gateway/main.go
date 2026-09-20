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
//	TEKSES_CONTROL_URL  boş değilse hello'daki join_code bu control-api
//	                    üzerinden odaya çözülür; boşsa herkes "faz0" odasına
//	                    düşer (Faz 0 yerel denemesi)
//	TEKSES_NATS_URL     boş değilse yayınlar NATS üzerinden TÜM gateway
//	                    düğümlerine dağıtılır (çok düğümlü kurulum, F2.6);
//	                    boşsa tek düğüm — yayınlar yerel hub'a gider
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

	"github.com/msaliheroglu/tekses/services/gateway/internal/fanout"
	"github.com/msaliheroglu/tekses/services/gateway/internal/rooms"
	"github.com/msaliheroglu/tekses/services/gateway/internal/runsink"
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
	var resolver rooms.Resolver
	var sessions rooms.SessionValidator
	if controlURL := os.Getenv("TEKSES_CONTROL_URL"); controlURL != "" {
		cr := rooms.NewControlResolver(controlURL)
		resolver = cr
		// Aynı control-api, konsol isteklerindeki panel oturumlarını da
		// doğrular: moderatörün TEKSES_ADMIN_TOKEN bilmesi gerekmez.
		sessions = cr
		log.Info("katılım kodları ve panel oturumları control-api'den doğrulanacak", "url", controlURL)
	}
	srv := server.New(log, os.Getenv("TEKSES_ADMIN_TOKEN"), resolver, sessions)

	// Kalıcı Run kaydı: control-api adresi ve iç uç sırrı birlikte
	// ayarlıysa açılır; yoksa izler yalnız yerel halkada yaşar.
	if controlURL, internalToken := os.Getenv("TEKSES_CONTROL_URL"), os.Getenv("TEKSES_INTERNAL_TOKEN"); controlURL != "" && internalToken != "" {
		srv.SetRunSink(runsink.New(controlURL, internalToken))
		log.Info("run kayıtları control-api'ye kalıcılaştırılacak")
	}

	if natsURL := os.Getenv("TEKSES_NATS_URL"); natsURL != "" {
		bus, err := fanout.NewNATS(natsURL, srv.BroadcastSink())
		if err != nil {
			log.Error("nats dağıtımı kurulamadı", "hata", err)
			os.Exit(1)
		}
		defer bus.Close()
		srv.SetBus(bus)
		log.Info("yayın dağıtımı NATS üzerinden (çok düğüm)", "url", natsURL)
	}

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
