// Package pg, store.Store'un Postgres (pgx) gerçeklemesidir.
//
// Kalıcı ortamların (pilot ve üretim) deposu budur; memstore yalnızca test
// ve hızlı yerel geliştirme içindir. Şema, pakete gömülü migration'larla
// açılışta uygulanır (Migrate).
package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
)

type Store struct {
	pool *pgxpool.Pool
}

var _ store.Store = (*Store)(nil)

// Open, bağlantı havuzunu kurar ve migration'ları uygular.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pg: havuz kurulamadı: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg: veritabanına erişilemedi: %w", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// ctx: Store arayüzü Faz 1'de bağlam taşımıyor (memstore ile ortak); sorgular
// arka plan bağlamıyla koşar. API zaman aşımı ihtiyacı doğarsa arayüze ctx
// eklenecek.
var bg = context.Background()

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// --- kimlik ve oturum ---

func (s *Store) CreateOrgWithUser(org model.Organization, u model.User) error {
	tx, err := s.pool.Begin(bg)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(bg) }()

	if _, err := tx.Exec(bg,
		`INSERT INTO organizations (id, name, created_at) VALUES ($1, $2, $3)`,
		org.ID, org.Name, org.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(bg,
		`INSERT INTO users (id, org_id, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.OrgID, u.Email, u.PasswordHash, u.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return store.ErrConflict
		}
		return err
	}
	return tx.Commit(bg)
}

func (s *Store) UserByEmail(email string) (model.User, error) {
	var u model.User
	err := s.pool.QueryRow(bg,
		`SELECT id, org_id, email, password_hash, created_at FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, store.ErrNotFound
	}
	return u, err
}

func (s *Store) CreateSession(sess model.Session) error {
	_, err := s.pool.Exec(bg,
		`INSERT INTO sessions (token, user_id, org_id, created_at) VALUES ($1, $2, $3, $4)`,
		sess.Token, sess.UserID, sess.OrgID, sess.CreatedAt)
	return err
}

func (s *Store) SessionByToken(token string) (model.Session, error) {
	var sess model.Session
	err := s.pool.QueryRow(bg,
		`SELECT token, user_id, org_id, created_at FROM sessions WHERE token = $1`, token).
		Scan(&sess.Token, &sess.UserID, &sess.OrgID, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Session{}, store.ErrNotFound
	}
	return sess, err
}

// --- etkinlik ve oda ---

func (s *Store) CreateEvent(e model.Event) error {
	_, err := s.pool.Exec(bg,
		`INSERT INTO events (id, org_id, name, venue, created_at) VALUES ($1, $2, $3, $4, $5)`,
		e.ID, e.OrgID, e.Name, e.Venue, e.CreatedAt)
	return err
}

func (s *Store) ListEvents(orgID string) ([]model.Event, error) {
	rows, err := s.pool.Query(bg,
		`SELECT id, org_id, name, venue, created_at FROM events WHERE org_id = $1 ORDER BY created_at, id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Name, &e.Venue, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) EventByID(orgID, id string) (model.Event, error) {
	var e model.Event
	err := s.pool.QueryRow(bg,
		`SELECT id, org_id, name, venue, created_at FROM events WHERE id = $1 AND org_id = $2`, id, orgID).
		Scan(&e.ID, &e.OrgID, &e.Name, &e.Venue, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Event{}, store.ErrNotFound
	}
	return e, err
}

func (s *Store) CreateRoom(r model.Room) error {
	_, err := s.pool.Exec(bg,
		`INSERT INTO rooms (id, org_id, event_id, name, join_code, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ID, r.OrgID, r.EventID, r.Name, r.JoinCode, r.CreatedAt)
	if isUniqueViolation(err) {
		return store.ErrConflict
	}
	return err
}

func (s *Store) ListRooms(orgID, eventID string) ([]model.Room, error) {
	rows, err := s.pool.Query(bg,
		`SELECT id, org_id, event_id, name, join_code, COALESCE(active_show_version_id, ''), created_at
		 FROM rooms WHERE org_id = $1 AND event_id = $2 ORDER BY created_at, id`, orgID, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Room
	for rows.Next() {
		var r model.Room
		if err := rows.Scan(&r.ID, &r.OrgID, &r.EventID, &r.Name, &r.JoinCode, &r.ActiveShowVersionID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) roomBy(where string, args ...any) (model.Room, error) {
	var r model.Room
	err := s.pool.QueryRow(bg,
		`SELECT id, org_id, event_id, name, join_code, COALESCE(active_show_version_id, ''), created_at
		 FROM rooms WHERE `+where, args...).
		Scan(&r.ID, &r.OrgID, &r.EventID, &r.Name, &r.JoinCode, &r.ActiveShowVersionID, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Room{}, store.ErrNotFound
	}
	return r, err
}

func (s *Store) RoomByID(orgID, id string) (model.Room, error) {
	return s.roomBy(`id = $1 AND org_id = $2`, id, orgID)
}

func (s *Store) RoomByJoinCode(code string) (model.Room, error) {
	return s.roomBy(`join_code = $1`, code)
}

func (s *Store) SetRoomActiveVersion(orgID, roomID, showVersionID string) error {
	tag, err := s.pool.Exec(bg,
		`UPDATE rooms SET active_show_version_id = $1 WHERE id = $2 AND org_id = $3`,
		showVersionID, roomID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

// --- gösteri ---

func (s *Store) CreateShow(sh model.Show) error {
	_, err := s.pool.Exec(bg,
		`INSERT INTO shows (id, org_id, title, created_at) VALUES ($1, $2, $3, $4)`,
		sh.ID, sh.OrgID, sh.Title, sh.CreatedAt)
	return err
}

func (s *Store) ListShows(orgID string) ([]model.Show, error) {
	rows, err := s.pool.Query(bg,
		`SELECT id, org_id, title, created_at FROM shows WHERE org_id = $1 ORDER BY created_at, id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Show
	for rows.Next() {
		var sh model.Show
		if err := rows.Scan(&sh.ID, &sh.OrgID, &sh.Title, &sh.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

func (s *Store) ShowByID(orgID, id string) (model.Show, error) {
	var sh model.Show
	err := s.pool.QueryRow(bg,
		`SELECT id, org_id, title, created_at FROM shows WHERE id = $1 AND org_id = $2`, id, orgID).
		Scan(&sh.ID, &sh.OrgID, &sh.Title, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Show{}, store.ErrNotFound
	}
	return sh, err
}

func (s *Store) CreateShowVersion(sv model.ShowVersion) (model.ShowVersion, error) {
	tx, err := s.pool.Begin(bg)
	if err != nil {
		return model.ShowVersion{}, err
	}
	defer func() { _ = tx.Rollback(bg) }()

	// Gösteri satırı kilitlenir: sürüm numarası eşzamanlı yayınlarda da
	// çakışmadan artar (UNIQUE(show_id, version) son emniyettir).
	var lockedID string
	err = tx.QueryRow(bg,
		`SELECT id FROM shows WHERE id = $1 AND org_id = $2 FOR UPDATE`,
		sv.ShowID, sv.OrgID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ShowVersion{}, store.ErrNotFound
	}
	if err != nil {
		return model.ShowVersion{}, err
	}
	if err := tx.QueryRow(bg,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM show_versions WHERE show_id = $1`, sv.ShowID).
		Scan(&sv.Version); err != nil {
		return model.ShowVersion{}, err
	}
	if _, err := tx.Exec(bg,
		`INSERT INTO show_versions (id, org_id, show_id, version, manifest_json, sha256, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sv.ID, sv.OrgID, sv.ShowID, sv.Version, sv.ManifestJSON, sv.SHA256, sv.CreatedAt); err != nil {
		return model.ShowVersion{}, err
	}
	if err := tx.Commit(bg); err != nil {
		return model.ShowVersion{}, err
	}
	return sv, nil
}

func (s *Store) ListShowVersions(orgID, showID string) ([]model.ShowVersion, error) {
	rows, err := s.pool.Query(bg,
		`SELECT id, org_id, show_id, version, manifest_json, sha256, created_at
		 FROM show_versions WHERE org_id = $1 AND show_id = $2 ORDER BY version`, orgID, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ShowVersion
	for rows.Next() {
		var sv model.ShowVersion
		if err := rows.Scan(&sv.ID, &sv.OrgID, &sv.ShowID, &sv.Version, &sv.ManifestJSON, &sv.SHA256, &sv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) ShowVersionByID(orgID, id string) (model.ShowVersion, error) {
	var sv model.ShowVersion
	err := s.pool.QueryRow(bg,
		`SELECT id, org_id, show_id, version, manifest_json, sha256, created_at
		 FROM show_versions WHERE id = $1 AND org_id = $2`, id, orgID).
		Scan(&sv.ID, &sv.OrgID, &sv.ShowID, &sv.Version, &sv.ManifestJSON, &sv.SHA256, &sv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ShowVersion{}, store.ErrNotFound
	}
	return sv, err
}
