// Package storetest, store.Store sözleşmesinin uygunluk test paketidir.
// Her gerçekleme (memstore, pg) aynı davranışı göstermek zorundadır; yeni
// bir gerçekleme eklendiğinde tek yapılması gereken Run'ı çağırmaktır.
package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
)

// Run, uygunluk paketini verilen taze store üzerinde koşar. Factory her
// çağrıda boş (ya da testler arası yalıtılmış) bir depo döndürmelidir.
func Run(t *testing.T, factory func(t *testing.T) store.Store) {
	t.Helper()
	t.Run("OrgUserSession", func(t *testing.T) { testOrgUserSession(t, factory(t)) })
	t.Run("EventRoom", func(t *testing.T) { testEventRoom(t, factory(t)) })
	t.Run("ShowVersions", func(t *testing.T) { testShowVersions(t, factory(t)) })
	t.Run("TenantScoping", func(t *testing.T) { testTenantScoping(t, factory(t)) })
}

func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

func mustOrg(t *testing.T, s store.Store, id, email string) {
	t.Helper()
	err := s.CreateOrgWithUser(
		model.Organization{ID: id, Name: "Org " + id, CreatedAt: now()},
		model.User{ID: "usr_" + id, OrgID: id, Email: email, PasswordHash: []byte("h"), CreatedAt: now()},
	)
	if err != nil {
		t.Fatalf("org kurulamadı: %v", err)
	}
}

func testOrgUserSession(t *testing.T, s store.Store) {
	mustOrg(t, s, "org_a", "a@ornek.com")

	// Aynı e-posta ikinci org ile çakışır ve org yaratılmamış olur.
	err := s.CreateOrgWithUser(
		model.Organization{ID: "org_b", Name: "B", CreatedAt: now()},
		model.User{ID: "usr_x", OrgID: "org_b", Email: "a@ornek.com", PasswordHash: []byte("h"), CreatedAt: now()},
	)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("çift e-posta hatası = %v, beklenen ErrConflict", err)
	}

	u, err := s.UserByEmail("a@ornek.com")
	if err != nil || u.OrgID != "org_a" {
		t.Fatalf("UserByEmail = %+v, %v", u, err)
	}
	if _, err := s.UserByEmail("yok@ornek.com"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("olmayan e-posta hatası = %v", err)
	}

	sess := model.Session{Token: "tok1", UserID: u.ID, OrgID: u.OrgID, CreatedAt: now()}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionByToken("tok1")
	if err != nil || got.OrgID != "org_a" {
		t.Fatalf("SessionByToken = %+v, %v", got, err)
	}
	if _, err := s.SessionByToken("yok"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("olmayan token hatası = %v", err)
	}
}

func testEventRoom(t *testing.T, s store.Store) {
	mustOrg(t, s, "org_a", "e@ornek.com")

	ev := model.Event{ID: "ev_1", OrgID: "org_a", Name: "Final", Venue: "Stadyum", CreatedAt: now()}
	if err := s.CreateEvent(ev); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListEvents("org_a")
	if err != nil || len(events) != 1 || events[0].Venue != "Stadyum" {
		t.Fatalf("ListEvents = %+v, %v", events, err)
	}

	room := model.Room{ID: "room_1", OrgID: "org_a", EventID: "ev_1", Name: "Tribün", JoinCode: "AAA111", CreatedAt: now()}
	if err := s.CreateRoom(room); err != nil {
		t.Fatal(err)
	}
	// joinCode çakışması.
	dup := model.Room{ID: "room_2", OrgID: "org_a", EventID: "ev_1", Name: "X", JoinCode: "AAA111", CreatedAt: now()}
	if err := s.CreateRoom(dup); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("çift joinCode hatası = %v", err)
	}

	byCode, err := s.RoomByJoinCode("AAA111")
	if err != nil || byCode.ID != "room_1" {
		t.Fatalf("RoomByJoinCode = %+v, %v", byCode, err)
	}
	rooms, err := s.ListRooms("org_a", "ev_1")
	if err != nil || len(rooms) != 1 {
		t.Fatalf("ListRooms = %+v, %v", rooms, err)
	}
	if rooms[0].ActiveShowVersionID != "" {
		t.Fatalf("yeni odada aktif sürüm olmamalı: %+v", rooms[0])
	}
}

func testShowVersions(t *testing.T, s store.Store) {
	mustOrg(t, s, "org_a", "s@ornek.com")
	if err := s.CreateShow(model.Show{ID: "show_1", OrgID: "org_a", Title: "Marş", CreatedAt: now()}); err != nil {
		t.Fatal(err)
	}

	sv1, err := s.CreateShowVersion(model.ShowVersion{
		ID: "sv_1", OrgID: "org_a", ShowID: "show_1",
		ManifestJSON: []byte(`{"a":1}`), SHA256: "aa", CreatedAt: now(),
	})
	if err != nil || sv1.Version != 1 {
		t.Fatalf("ilk sürüm = %+v, %v; beklenen version 1", sv1, err)
	}
	sv2, err := s.CreateShowVersion(model.ShowVersion{
		ID: "sv_2", OrgID: "org_a", ShowID: "show_1",
		ManifestJSON: []byte(`{"a":2}`), SHA256: "bb", CreatedAt: now(),
	})
	if err != nil || sv2.Version != 2 {
		t.Fatalf("ikinci sürüm = %+v, %v; beklenen version 2", sv2, err)
	}

	// Olmayan gösteriye sürüm yazılamaz.
	if _, err := s.CreateShowVersion(model.ShowVersion{
		ID: "sv_x", OrgID: "org_a", ShowID: "show_yok",
		ManifestJSON: []byte(`{}`), SHA256: "cc", CreatedAt: now(),
	}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("olmayan gösteri hatası = %v", err)
	}

	got, err := s.ShowVersionByID("org_a", "sv_1")
	if err != nil || string(got.ManifestJSON) != `{"a":1}` {
		t.Fatalf("manifest baytları aynen dönmedi: %q, %v", got.ManifestJSON, err)
	}
	versions, err := s.ListShowVersions("org_a", "show_1")
	if err != nil || len(versions) != 2 || versions[0].Version != 1 || versions[1].Version != 2 {
		t.Fatalf("ListShowVersions = %+v, %v", versions, err)
	}

	// Etkinleştirme akışı.
	if err := s.CreateEvent(model.Event{ID: "ev_1", OrgID: "org_a", Name: "E", CreatedAt: now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRoom(model.Room{ID: "room_1", OrgID: "org_a", EventID: "ev_1", Name: "R", JoinCode: "BBB222", CreatedAt: now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRoomActiveVersion("org_a", "room_1", "sv_2"); err != nil {
		t.Fatal(err)
	}
	room, err := s.RoomByID("org_a", "room_1")
	if err != nil || room.ActiveShowVersionID != "sv_2" {
		t.Fatalf("aktif sürüm = %+v, %v", room, err)
	}
	if err := s.SetRoomActiveVersion("org_a", "room_yok", "sv_2"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("olmayan oda hatası = %v", err)
	}
}

func testTenantScoping(t *testing.T, s store.Store) {
	mustOrg(t, s, "org_a", "ta@ornek.com")
	mustOrg(t, s, "org_b", "tb@ornek.com")

	if err := s.CreateEvent(model.Event{ID: "ev_a", OrgID: "org_a", Name: "A", CreatedAt: now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateShow(model.Show{ID: "show_a", OrgID: "org_a", Title: "A", CreatedAt: now()}); err != nil {
		t.Fatal(err)
	}
	sv, err := s.CreateShowVersion(model.ShowVersion{
		ID: "sv_a", OrgID: "org_a", ShowID: "show_a",
		ManifestJSON: []byte(`{}`), SHA256: "aa", CreatedAt: now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// B kiracısı A'nın kaynaklarını göremez.
	if _, err := s.EventByID("org_b", "ev_a"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("çapraz etkinlik erişimi = %v", err)
	}
	if _, err := s.ShowByID("org_b", "show_a"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("çapraz gösteri erişimi = %v", err)
	}
	if _, err := s.ShowVersionByID("org_b", sv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("çapraz sürüm erişimi = %v", err)
	}
	events, err := s.ListEvents("org_b")
	if err != nil || len(events) != 0 {
		t.Fatalf("B'nin etkinlik listesi = %+v, %v", events, err)
	}
	// B, A'nın gösterisine sürüm yazamaz.
	if _, err := s.CreateShowVersion(model.ShowVersion{
		ID: "sv_hack", OrgID: "org_b", ShowID: "show_a",
		ManifestJSON: []byte(`{}`), SHA256: "hh", CreatedAt: now(),
	}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("çapraz sürüm yazımı = %v", err)
	}
}
