// Package runsink, gateway'in kue/müdahale izlerini control-api'nin iç
// ucuna (POST /internal/runs) kalıcılaştıran istemcidir.
//
// İlke: kayıt yolu kue yolunu ASLA engellemez. Persist, yayın yapıldıktan
// sonra ayrı bir goroutine'de çağrılır; hata yalnızca loglanır (kayıt o
// zaman yalnız gateway'in 50'lik halkasında yaşar). Kimlik: paylaşımlı
// TEKSES_INTERNAL_TOKEN — yol gizliliği yeterli değildir, çünkü Caddy'nin
// /control/* önek soyması /internal/… yolunu internete taşıyabilir.
package runsink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const persistTimeout = 3 * time.Second

// Record, kalıcılaştırılan izdir (alan adları control-api iç ucuyla hizalı).
type Record struct {
	ID               string `json:"id"`
	RoomID           string `json:"room_id,omitempty"`
	Kind             string `json:"kind"`
	RunID            string `json:"run_id,omitempty"`
	CueID            string `json:"cue_id,omitempty"`
	FireAtServerMs   int64  `json:"fire_at_server_ms,omitempty"`
	IssuedAtServerMs int64  `json:"issued_at_server_ms"`
	Clients          int    `json:"clients"`
	Node             string `json:"node,omitempty"`
}

type Client struct {
	url    string
	token  string
	client *http.Client
}

func New(controlBaseURL, internalToken string) *Client {
	return &Client{
		url:    strings.TrimRight(controlBaseURL, "/") + "/internal/runs",
		token:  internalToken,
		client: &http.Client{Timeout: persistTimeout},
	}
}

// Persist, kaydı control-api'ye yazar. Çağıran, handler bağlamını DEĞİL
// arka plan bağlamını kullanmalı (handler dönünce r.Context iptal olur).
func (c *Client) Persist(ctx context.Context, rec Record) error {
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		// Durum kodu mesajda: 404 "eski control-api ya da token ayarsız",
		// 401 "token uyuşmuyor" teşhisleri loglardan ayrışabilsin.
		return fmt.Errorf("iç uç durum %d", resp.StatusCode)
	}
	return nil
}
