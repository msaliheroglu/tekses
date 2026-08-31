// Package blob, gösteri paketlerinin ikili depolama sözleşmesidir.
//
// Paketler içerik adreslidir (anahtar SHA-256'dan türetilir) ve değişmezdir:
// aynı anahtara ikinci yazım aynı içeriği yazar, üzerine yazmak zararsızdır.
// Geliştirme ve pilot için dosya sistemi sürücüsü vardır; üretimde aynı
// arayüzün arkasına R2/S3 sürücüsü gelecek ve indirme CDN'den yapılacak.
package blob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound: anahtar depoda yok.
var ErrNotFound = errors.New("blob: kayıt bulunamadı")

type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// FS, kök dizin altında düz dosyalarla çalışan sürücüdür.
type FS struct {
	root string
}

func NewFS(root string) (*FS, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("blob: kök dizin açılamadı: %w", err)
	}
	return &FS{root: root}, nil
}

// safePath, anahtarı kök altına hapseder; yol kaçışı (.. , /) reddedilir.
func (f *FS) safePath(key string) (string, error) {
	if key == "" || strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return "", fmt.Errorf("blob: geçersiz anahtar %q", key)
	}
	return filepath.Join(f.root, key), nil
}

// Put, atomik yazar (geçici dosya + yeniden adlandırma): yarıda kesilen
// yazım asla yarım paket bırakmaz. Var olan anahtara yazım sessizce başarılı
// sayılır (içerik adresli depo idempotenttir).
func (f *FS) Put(_ context.Context, key string, data []byte) error {
	path, err := f.safePath(key)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	tmp, err := os.CreateTemp(f.root, ".tmp-*")
	if err != nil {
		return fmt.Errorf("blob: geçici dosya açılamadı: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("blob: yazılamadı: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (f *FS) Get(_ context.Context, key string) ([]byte, error) {
	path, err := f.safePath(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}
