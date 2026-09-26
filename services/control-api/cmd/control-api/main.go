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

	// Paket/varlık deposu: S3 ucu ayarlıysa R2/S3, değilse yerel dosya
	// sistemi. İki sürücü aynı düz, içerik adresli anahtarları kullanır;
	// aralarında geçiş dosyaları kovaya kopyalamaktan ibarettir.
	var packages blob.Store
	if endpoint := os.Getenv("TEKSES_BLOB_S3_ENDPOINT"); endpoint != "" {
		bucket := os.Getenv("TEKSES_BLOB_S3_BUCKET")
		if bucket == "" {
			log.Error("TEKSES_BLOB_S3_ENDPOINT ayarlı ama TEKSES_BLOB_S3_BUCKET boş")
			os.Exit(1)
		}
		s3, err := blob.NewS3(endpoint, bucket,
			os.Getenv("TEKSES_BLOB_S3_ACCESS_KEY_ID"),
			os.Getenv("TEKSES_BLOB_S3_SECRET_KEY"))
		if err != nil {
			log.Error("s3 deposu kurulamadı", "hata", err)
			os.Exit(1)
		}
		packages = s3
		log.Info("paket deposu: s3/r2", "uç", endpoint, "kova", bucket)
	} else {
		packagesDir := os.Getenv("TEKSES_PACKAGES_DIR")
		if packagesDir == "" {
			packagesDir = "data/packages"
		}
		fsStore, err := blob.NewFS(packagesDir)
		if err != nil {
			log.Error("paket deposu açılamadı", "hata", err)
			os.Exit(1)
		}
		packages = fsStore
		log.Info("paket deposu: dosya sistemi", "dizin", packagesDir)
	}

	// Deneysel söz çıkarma: sesten zamanlı taslak üreten dış komut
	// (ör. deploy/transcribe-whisper.sh). Ayarsızsa uç 501 döner.
	transcriber := os.Getenv("TEKSES_TRANSCRIBER")
	if transcriber != "" {
		log.Info("söz çıkarma etkin", "komut", transcriber)
	}

	// Servisler arası iç uçların (gateway → /internal/runs) paylaşımlı
	// sırrı; boşsa iç uçlar kapalı (kalıcı Run kaydı devre dışı).
	internalToken := os.Getenv("TEKSES_INTERNAL_TOKEN")
	if internalToken == "" {
		log.Warn("TEKSES_INTERNAL_TOKEN ayarsız — kalıcı Run kaydı (iç uç) kapalı")
	}

	srv := api.New(log, st, packages, transcriber, internalToken)
	// Kovanın herkese açık tabanı (R2 özel alan adı / CDN): ayarlıysa
	// telefonlara mutlak paket/varlık URL'leri döner ve indirme VM'ye
	// uğramaz. S3 deposu olmadan da çalışır (ör. FS + ayrı yansıtma) ama
	// olağan eşleşme S3 + taban birlikte.
	if base := os.Getenv("TEKSES_ASSET_PUBLIC_BASE"); base != "" {
		srv.SetAssetPublicBase(base)
		log.Info("varlık indirmeleri herkese açık tabandan", "taban", base)
	}

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
