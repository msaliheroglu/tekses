// Package store, kontrol API'sinin depolama sözleşmesini tanımlar.
//
// Faz 1 başında tek gerçekleme bellek içidir (memstore); Postgres/pgx
// gerçeklemesi plan Adım 5'te aynı arayüzün arkasına gelecek. Bu yüzden
// arayüz baştan "veritabanı gibi" düşünülür: kimlikler çağıran tarafça
// üretilir, org daraltması sorgu parametresidir, hatalar sınıflıdır.
package store

import (
	"errors"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
)

var (
	// ErrNotFound: kayıt yok ya da başka kiracıya ait (ikisi aynı görünür;
	// kiracılar birbirinin kaynak kimliklerinin varlığını dahi öğrenemez).
	ErrNotFound = errors.New("store: kayıt bulunamadı")
	// ErrConflict: benzersizlik ihlali (e-posta, joinCode).
	ErrConflict = errors.New("store: kayıt çakışması")
)

type Store interface {
	// Kimlik ve oturum.
	// CreateOrgWithUser atomiktir: e-posta çakışırsa (ErrConflict) ortada
	// sahipsiz organizasyon kalmaz.
	CreateOrgWithUser(org model.Organization, u model.User) error
	UserByEmail(email string) (model.User, error)
	CreateSession(s model.Session) error
	SessionByToken(token string) (model.Session, error)

	// Etkinlik ve oda
	CreateEvent(e model.Event) error
	ListEvents(orgID string) ([]model.Event, error)
	EventByID(orgID, id string) (model.Event, error)
	CreateRoom(r model.Room) error // joinCode benzersiz → ErrConflict
	ListRooms(orgID, eventID string) ([]model.Room, error)
	RoomByID(orgID, id string) (model.Room, error)
	RoomByJoinCode(code string) (model.Room, error)
	SetRoomActiveVersion(orgID, roomID, showVersionID string) error

	// Gösteri
	CreateShow(s model.Show) error
	ListShows(orgID string) ([]model.Show, error)
	ShowByID(orgID, id string) (model.Show, error)
	// CreateShowVersion sürüm numarasını atomik olarak atar (gösteri başına
	// 1'den artan) ve atanan sürümü döndürür.
	CreateShowVersion(sv model.ShowVersion) (model.ShowVersion, error)
	ListShowVersions(orgID, showID string) ([]model.ShowVersion, error)
	ShowVersionByID(orgID, id string) (model.ShowVersion, error)
}
