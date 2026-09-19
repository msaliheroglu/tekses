// Package rooms, katılım kodunu oda kimliğine çözer.
//
// Gerçek çözücü control-api'nin herkese açık /api/v1/join/{code} ucudur.
// Gateway'e control-api adresi verilmemişse (Faz 0 yerel denemesi) çözücü
// yoktur ve herkes varsayılan odaya düşer.
package rooms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrUnknownCode: kod control-api'de kayıtlı değil.
var ErrUnknownCode = errors.New("rooms: katılım kodu geçersiz")

type Resolver interface {
	ResolveJoinCode(ctx context.Context, code string) (roomID string, err error)
}

// ControlResolver, control-api üzerinden çözer.
type ControlResolver struct {
	baseURL string
	client  *http.Client
}

func NewControlResolver(baseURL string) *ControlResolver {
	return &ControlResolver{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 3 * time.Second},
	}
}

func (r *ControlResolver) ResolveJoinCode(ctx context.Context, code string) (string, error) {
	u := r.baseURL + "/api/v1/join/" + url.PathEscape(code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("rooms: control-api erişilemedi: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", ErrUnknownCode
	default:
		return "", fmt.Errorf("rooms: control-api beklenmeyen durum %d", resp.StatusCode)
	}
	var body struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.RoomID == "" {
		return "", fmt.Errorf("rooms: katılım yanıtı çözülemedi")
	}
	return body.RoomID, nil
}
