package rooms

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestControlResolver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/join/ABC234":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"room_id":"room_x","room_name":"Tribün"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	r := NewControlResolver(ts.URL + "/") // sondaki / kırpılmalı

	room, err := r.ResolveJoinCode(context.Background(), "ABC234")
	if err != nil || room != "room_x" {
		t.Fatalf("çözüm = %q, %v; beklenen room_x", room, err)
	}

	if _, err := r.ResolveJoinCode(context.Background(), "YOK"); !errors.Is(err, ErrUnknownCode) {
		t.Fatalf("bilinmeyen kod hatası = %v, beklenen ErrUnknownCode", err)
	}
}
