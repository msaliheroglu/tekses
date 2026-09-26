package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3, S3-uyumlu nesne deposu sürücüsüdür; hedef Cloudflare R2'dir (karar
// dokümanı §4: 10 GB + çıkış trafiği ücretsiz) ama herhangi bir S3 ucu da
// çalışır. Anahtar düzeni FS ile birebir aynıdır (düz, içerik adresli):
// paketler <sha256>.json, ses varlıkları <sha256>.<uzantı>. İçerik türü
// anahtar uzantısından yazılır ki kova bir CDN alan adıyla HERKESE AÇIK
// yayınlandığında telefonlar doğru Content-Type ile indirsin.
type S3 struct {
	cl     *minio.Client
	bucket string
}

// NewS3 bağlantıyı kurar ama ağa çıkmaz; ilk hata ilk işlemde görülür
// (control-api açılışı depoya erişime bağımlı olmasın).
func NewS3(endpoint, bucket, accessKey, secretKey string) (*S3, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("blob: s3 ucu çözülemedi %q (https://... bekleniyor)", endpoint)
	}
	cl, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: u.Scheme != "http",
		// R2 bölgesi "auto"; yol-stili adresleme R2 ve testlerdeki sahte
		// uçlarla en uyumlusu.
		Region:       "auto",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("blob: s3 istemcisi kurulamadı: %w", err)
	}
	return &S3{cl: cl, bucket: bucket}, nil
}

// contentTypeByKey, anahtar uzantısından içerik türünü verir (kova doğrudan
// CDN'den servis edildiğinde başlık buradan gelir).
func contentTypeByKey(key string) string {
	switch key[strings.LastIndexByte(key, '.')+1:] {
	case "json":
		return "application/json; charset=utf-8"
	case "mp3":
		return "audio/mpeg"
	case "m4a":
		return "audio/mp4"
	case "wav":
		return "audio/wav"
	case "ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

// Put, nesneyi yazar. Depo içerik adresli olduğundan var olan anahtara
// yeniden yazım aynı baytları yazar; ayrı bir varlık denetimi gereksizdir.
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	if err := validKey(key); err != nil {
		return err
	}
	_, err := s.cl.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentTypeByKey(key)})
	if err != nil {
		return fmt.Errorf("blob: s3 yazımı: %w", err)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validKey(key); err != nil {
		return nil, err
	}
	obj, err := s.cl.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("blob: s3 okuması: %w", err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		if isNoSuchKey(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("blob: s3 okuması: %w", err)
	}
	return data, nil
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	if err := validKey(key); err != nil {
		return false, err
	}
	_, err := s.cl.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKey(err) {
			return false, nil
		}
		return false, fmt.Errorf("blob: s3 yoklaması: %w", err)
	}
	return true, nil
}

// validKey, FS.safePath ile aynı sözleşmeyi uygular: anahtarlar düz addır,
// yol ayracı taşımaz (S3'te "klasör" oluşmasın, iki sürücü yer değiştirebilsin).
func validKey(key string) error {
	if key == "" || strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return fmt.Errorf("blob: geçersiz anahtar %q", key)
	}
	return nil
}

func isNoSuchKey(err error) bool {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp.Code == "NoSuchKey" || resp.StatusCode == 404
	}
	return false
}
