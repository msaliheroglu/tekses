-- TekSes kontrol düzlemi ilk şeması.
-- Kiracılık zinciri: organizations → events → rooms; shows → show_versions.
-- Kimlikler uygulama tarafında üretilir (org_…, ev_… önekli).

CREATE TABLE organizations (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE users (
    id            text PRIMARY KEY,
    org_id        text NOT NULL REFERENCES organizations (id),
    email         text NOT NULL UNIQUE, -- uygulama küçük harfe indirger
    password_hash bytea NOT NULL,
    created_at    timestamptz NOT NULL
);

CREATE TABLE sessions (
    token      text PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users (id),
    org_id     text NOT NULL REFERENCES organizations (id),
    created_at timestamptz NOT NULL
);

CREATE TABLE events (
    id         text PRIMARY KEY,
    org_id     text NOT NULL REFERENCES organizations (id),
    name       text NOT NULL,
    venue      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);
CREATE INDEX events_org_idx ON events (org_id, created_at);

CREATE TABLE shows (
    id         text PRIMARY KEY,
    org_id     text NOT NULL REFERENCES organizations (id),
    title      text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX shows_org_idx ON shows (org_id, created_at);

CREATE TABLE show_versions (
    id            text PRIMARY KEY,
    org_id        text NOT NULL REFERENCES organizations (id),
    show_id       text NOT NULL REFERENCES shows (id),
    version       int  NOT NULL,
    -- Kanonik manifest baytları aynen saklanır (SHA-256 bunlar üzerinden);
    -- jsonb kullanılmaz çünkü jsonb baytları yeniden biçimler.
    manifest_json bytea NOT NULL,
    sha256        text NOT NULL,
    created_at    timestamptz NOT NULL,
    UNIQUE (show_id, version)
);
CREATE INDEX show_versions_show_idx ON show_versions (org_id, show_id, version);

CREATE TABLE rooms (
    id                     text PRIMARY KEY,
    org_id                 text NOT NULL REFERENCES organizations (id),
    event_id               text NOT NULL REFERENCES events (id),
    name                   text NOT NULL,
    join_code              text NOT NULL UNIQUE,
    active_show_version_id text REFERENCES show_versions (id),
    created_at             timestamptz NOT NULL
);
CREATE INDEX rooms_event_idx ON rooms (org_id, event_id, created_at);
