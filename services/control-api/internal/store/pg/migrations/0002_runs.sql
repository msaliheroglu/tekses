-- Kalıcı Run izleri (F2.6 telemetri): kue yayınları ve müdahaleler.
-- Kaydı, olayı işleyen gateway düğümü üretir (POST /internal/runs).
--
-- Bilinçli kararlar:
--   * id gateway'de üretilir (rec_…): yeniden denemede ON CONFLICT DO
--     NOTHING ile idempotent yazım.
--   * run_id benzersiz DEĞİL (müdahale kuenin run_id'sini taşır ya da boş).
--   * room_id'ye FK YOK: "tüm odalara" (boş) ve Faz 0 varsayılan odası
--     ("faz0") rooms tablosunda yoktur; kue izi yine saklanmalı.
--   * org_id NULL olabilir ve FK taşımaz: org'suz kayıt org kapsamlı
--     listelerde görünmez ama kaybolmaz.
--   * clients, üreten DÜĞÜMÜN yerel istemci sayısıdır (küme toplamı değil).
CREATE TABLE runs (
    id                  text PRIMARY KEY,
    org_id              text,
    room_id             text NOT NULL DEFAULT '',
    kind                text NOT NULL,
    run_id              text NOT NULL DEFAULT '',
    cue_id              text NOT NULL DEFAULT '',
    fire_at_server_ms   bigint NOT NULL DEFAULT 0,
    issued_at_server_ms bigint NOT NULL,
    clients             integer NOT NULL,
    node                text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL
);

-- Konsol listesi: org kapsamlı, en yenisi başta (eşit zaman damgasında id
-- azalan — memstore ile aynı kararlı sıra).
CREATE INDEX runs_org_created_idx ON runs (org_id, created_at DESC, id DESC);
