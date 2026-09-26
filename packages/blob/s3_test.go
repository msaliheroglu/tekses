package blob

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeS3, PUT/GET/HEAD nesne işlemlerini bellek haritasında karşılayan
// asgari bir S3 ucudur (imza doğrulamaz — sürücünün istek/yanıt akışını
// sınar; gerçek R2 doğrulaması VM dağıtımında yapılır).
func fakeS3(t *testing.T) (*httptest.Server, *sync.Map) {
	t.Helper()
	var objects sync.Map // "/bucket/key" → []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// TLS'siz uçta minio gövdeyi aws-chunked imzalı akışla yollar;
			// çerçeveyi soy ki depoya ham baytlar düşsün.
			if strings.HasPrefix(r.Header.Get("X-Amz-Content-Sha256"), "STREAMING") {
				raw = deChunk(raw)
			}
			objects.Store(r.URL.Path, raw)
			w.Header().Set("ETag", `"x"`)
		case http.MethodHead, http.MethodGet:
			v, ok := objects.Load(r.URL.Path)
			if !ok {
				// S3 hata gövdesi; HEAD'de gövde yok, durum kodu yeter.
				w.WriteHeader(http.StatusNotFound)
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`))
				}
				return
			}
			data := v.([]byte)
			w.Header().Set("Content-Length", itoa(len(data)))
			w.Header().Set("ETag", `"x"`)
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
			if r.Method == http.MethodGet {
				_, _ = w.Write(data)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &objects
}

// deChunk, aws-chunked çerçevesini söker: her parça
// "<onaltılık-boy>;chunk-signature=…\r\n<veri>\r\n" biçimindedir ve boyu 0
// olan parça akışı bitirir.
func deChunk(raw []byte) []byte {
	var out []byte
	for {
		nl := strings.Index(string(raw), "\r\n")
		if nl < 0 {
			return out
		}
		head := string(raw[:nl])
		if i := strings.IndexByte(head, ';'); i >= 0 {
			head = head[:i]
		}
		size := 0
		for _, c := range head {
			switch {
			case c >= '0' && c <= '9':
				size = size*16 + int(c-'0')
			case c >= 'a' && c <= 'f':
				size = size*16 + int(c-'a'+10)
			default:
				return out
			}
		}
		if size == 0 {
			return out
		}
		raw = raw[nl+2:]
		if len(raw) < size {
			return out
		}
		out = append(out, raw[:size]...)
		raw = raw[size+2:] // veri + kapanış \r\n
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestS3PutGetExists(t *testing.T) {
	ts, _ := fakeS3(t)
	s3, err := NewS3(ts.URL, "tekses", "test-key", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if ok, err := s3.Exists(ctx, "abc.mp3"); err != nil || ok {
		t.Fatalf("boş depoda Exists = %v, %v", ok, err)
	}
	if _, err := s3.Get(ctx, "abc.mp3"); err == nil {
		t.Fatal("olmayan anahtar hatasız okundu")
	}

	payload := []byte("merhaba tekses")
	if err := s3.Put(ctx, "abc.mp3", payload); err != nil {
		t.Fatal(err)
	}
	if ok, err := s3.Exists(ctx, "abc.mp3"); err != nil || !ok {
		t.Fatalf("yazımdan sonra Exists = %v, %v", ok, err)
	}
	got, err := s3.Get(ctx, "abc.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("okunan = %q", got)
	}

	// FS ile aynı anahtar sözleşmesi: yol ayracı reddedilir.
	if err := s3.Put(ctx, "../kacis", nil); err == nil {
		t.Fatal("yol kaçışlı anahtar kabul edildi")
	}
	if err := s3.Put(ctx, "a/b", nil); err == nil {
		t.Fatal("ayraçlı anahtar kabul edildi")
	}
}

func TestContentTypeByKey(t *testing.T) {
	for key, want := range map[string]string{
		"x.json": "application/json; charset=utf-8",
		"x.mp3":  "audio/mpeg",
		"x.m4a":  "audio/mp4",
		"x.wav":  "audio/wav",
		"x.ogg":  "audio/ogg",
		"x.bin":  "application/octet-stream",
	} {
		if got := contentTypeByKey(key); got != want {
			t.Errorf("%s → %s, beklenen %s", key, got, want)
		}
	}
	if !strings.HasPrefix(contentTypeByKey("uzantisiz"), "application/octet-stream") {
		t.Error("uzantısız anahtar octet-stream olmalı")
	}
}
