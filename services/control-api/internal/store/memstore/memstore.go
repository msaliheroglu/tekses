// Package memstore, store.Store'un bellek içi gerçeklemesidir.
//
// Faz 1 geliştirme ve testler içindir; süreç ölünce veri gider. Kalıcılık
// plan Adım 5'te Postgres'e geçer. Tüm erişimler tek RWMutex ile korunur;
// pilot ölçeğindeki kontrol düzlemi trafiği için fazlasıyla yeterli.
package memstore

import (
	"sort"
	"strings"
	"sync"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
)

type Memstore struct {
	mu sync.RWMutex

	orgs         map[string]model.Organization
	users        map[string]model.User // anahtar: küçük harfli e-posta
	sessions     map[string]model.Session
	events       map[string]model.Event
	rooms        map[string]model.Room
	roomsByCode  map[string]string // joinCode → roomID
	shows        map[string]model.Show
	showVersions map[string]model.ShowVersion
	nextVersion  map[string]int // showID → sıradaki sürüm numarası
}

func New() *Memstore {
	return &Memstore{
		orgs:         map[string]model.Organization{},
		users:        map[string]model.User{},
		sessions:     map[string]model.Session{},
		events:       map[string]model.Event{},
		rooms:        map[string]model.Room{},
		roomsByCode:  map[string]string{},
		shows:        map[string]model.Show{},
		showVersions: map[string]model.ShowVersion{},
		nextVersion:  map[string]int{},
	}
}

var _ store.Store = (*Memstore)(nil)

// --- kimlik ve oturum ---

func (m *Memstore) CreateOrganization(org model.Organization) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orgs[org.ID] = org
	return nil
}

func (m *Memstore) CreateUser(u model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(u.Email)
	if _, exists := m.users[key]; exists {
		return store.ErrConflict
	}
	m.users[key] = u
	return nil
}

func (m *Memstore) UserByEmail(email string) (model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[strings.ToLower(email)]
	if !ok {
		return model.User{}, store.ErrNotFound
	}
	return u, nil
}

func (m *Memstore) CreateSession(s model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.Token] = s
	return nil
}

func (m *Memstore) SessionByToken(token string) (model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[token]
	if !ok {
		return model.Session{}, store.ErrNotFound
	}
	return s, nil
}

// --- etkinlik ve oda ---

func (m *Memstore) CreateEvent(e model.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[e.ID] = e
	return nil
}

func (m *Memstore) ListEvents(orgID string) ([]model.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Event
	for _, e := range m.events {
		if e.OrgID == orgID {
			out = append(out, e)
		}
	}
	sortByCreated(out, func(e model.Event) (string, int64) { return e.ID, e.CreatedAt.UnixNano() })
	return out, nil
}

func (m *Memstore) EventByID(orgID, id string) (model.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.events[id]
	if !ok || e.OrgID != orgID {
		return model.Event{}, store.ErrNotFound
	}
	return e, nil
}

func (m *Memstore) CreateRoom(r model.Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.roomsByCode[r.JoinCode]; exists {
		return store.ErrConflict
	}
	m.rooms[r.ID] = r
	m.roomsByCode[r.JoinCode] = r.ID
	return nil
}

func (m *Memstore) ListRooms(orgID, eventID string) ([]model.Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Room
	for _, r := range m.rooms {
		if r.OrgID == orgID && r.EventID == eventID {
			out = append(out, r)
		}
	}
	sortByCreated(out, func(r model.Room) (string, int64) { return r.ID, r.CreatedAt.UnixNano() })
	return out, nil
}

func (m *Memstore) RoomByID(orgID, id string) (model.Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[id]
	if !ok || r.OrgID != orgID {
		return model.Room{}, store.ErrNotFound
	}
	return r, nil
}

func (m *Memstore) RoomByJoinCode(code string) (model.Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.roomsByCode[code]
	if !ok {
		return model.Room{}, store.ErrNotFound
	}
	return m.rooms[id], nil
}

func (m *Memstore) SetRoomActiveVersion(orgID, roomID, showVersionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok || r.OrgID != orgID {
		return store.ErrNotFound
	}
	r.ActiveShowVersionID = showVersionID
	m.rooms[roomID] = r
	return nil
}

// --- gösteri ---

func (m *Memstore) CreateShow(s model.Show) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shows[s.ID] = s
	return nil
}

func (m *Memstore) ListShows(orgID string) ([]model.Show, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Show
	for _, s := range m.shows {
		if s.OrgID == orgID {
			out = append(out, s)
		}
	}
	sortByCreated(out, func(s model.Show) (string, int64) { return s.ID, s.CreatedAt.UnixNano() })
	return out, nil
}

func (m *Memstore) ShowByID(orgID, id string) (model.Show, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.shows[id]
	if !ok || s.OrgID != orgID {
		return model.Show{}, store.ErrNotFound
	}
	return s, nil
}

func (m *Memstore) CreateShowVersion(sv model.ShowVersion) (model.ShowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.shows[sv.ShowID]; !ok || s.OrgID != sv.OrgID {
		return model.ShowVersion{}, store.ErrNotFound
	}
	next := m.nextVersion[sv.ShowID]
	if next == 0 {
		next = 1
	}
	sv.Version = next
	m.nextVersion[sv.ShowID] = next + 1
	m.showVersions[sv.ID] = sv
	return sv, nil
}

func (m *Memstore) ListShowVersions(orgID, showID string) ([]model.ShowVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.ShowVersion
	for _, sv := range m.showVersions {
		if sv.OrgID == orgID && sv.ShowID == showID {
			out = append(out, sv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (m *Memstore) ShowVersionByID(orgID, id string) (model.ShowVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sv, ok := m.showVersions[id]
	if !ok || sv.OrgID != orgID {
		return model.ShowVersion{}, store.ErrNotFound
	}
	return sv, nil
}

// sortByCreated, oluşturulma zamanına (eşitse kimliğe) göre kararlı sıralar;
// map gezinme sırasının rastgeleliği API yanıtlarına sızmasın.
func sortByCreated[T any](items []T, key func(T) (string, int64)) {
	sort.Slice(items, func(i, j int) bool {
		idI, tI := key(items[i])
		idJ, tJ := key(items[j])
		if tI != tJ {
			return tI < tJ
		}
		return idI < idJ
	})
}
