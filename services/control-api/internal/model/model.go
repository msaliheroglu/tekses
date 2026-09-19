// Package model, kontrol düzleminin alan modelini tanımlar.
//
// Kiracılık zinciri karar dokümanı §3'teki gibidir:
// Organization → Event → Room; Show → ShowVersion (değişmez manifest).
// Her kaynak OrgID taşır; API katmanı her erişimi oturumun org'una daraltır.
package model

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Email        string    `json:"email"`
	PasswordHash []byte    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session, basit bearer token oturumudur (pilot kararı: e-posta + şifre).
type Session struct {
	Token     string    `json:"-"`
	UserID    string    `json:"user_id"`
	OrgID     string    `json:"org_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Venue     string    `json:"venue,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Room, katılımcıların joinCode ile girdiği fiziksel/mantıksal odadır.
// ActiveShowVersionID boşsa odada henüz yayınlanmış gösteri yoktur.
type Room struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	EventID             string    `json:"event_id"`
	Name                string    `json:"name"`
	JoinCode            string    `json:"join_code"`
	ActiveShowVersionID string    `json:"active_show_version_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type Show struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// ShowVersion, bir gösterinin DEĞİŞMEZ anlık görüntüsüdür: kanonik manifest
// baytları ve SHA-256 özeti oluşturulurken sabitlenir, sonrasında hiçbir uç
// bunları değiştiremez. Telefonlar paketi bu özetle doğrular.
type ShowVersion struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	ShowID       string    `json:"show_id"`
	Version      int       `json:"version"`
	ManifestJSON []byte    `json:"-"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}
