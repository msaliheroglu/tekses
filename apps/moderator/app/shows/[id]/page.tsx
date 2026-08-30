"use client";

import { use, useCallback, useEffect, useState } from "react";
import { control, type Event, type Room, type ShowVersion } from "@/lib/api";

// Söz zamanlama/dalga formu editörü sonraki yineleme; MVP'de manifest JSON
// olarak düzenlenir. Şema: packages/manifest (sunucu yayında doğrular).
const TEMPLATE = `{
  "title": "Marş Seti",
  "sequences": [
    {
      "id": "seq-1",
      "title": "Açılış",
      "duration_ms": 60000,
      "lyric_lines": [
        {"at_ms": 0, "duration_ms": 4000, "text": "Hep beraber!"}
      ],
      "cue_lanes": [
        {"id": "ekran", "kind": "screen", "cues": [
          {"at_ms": 0, "duration_ms": 8000, "color": "#FF2A2A", "flash_hz": 2}
        ]},
        {"id": "fener", "kind": "torch", "cues": [
          {"at_ms": 0, "duration_ms": 8000, "flash_hz": 2}
        ]}
      ]
    }
  ]
}`;

export default function ShowDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [versions, setVersions] = useState<ShowVersion[]>([]);
  const [manifest, setManifest] = useState(TEMPLATE);
  const [rooms, setRooms] = useState<(Room & { eventName: string })[]>([]);
  const [selectedRoom, setSelectedRoom] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    try {
      const resp = await control.get<{ versions: ShowVersion[] }>(`/api/v1/shows/${id}/versions`);
      setVersions(resp.versions);
      // Etkinleştirme seçimi için tüm odalar (etkinlik adıyla) toplanır.
      const evResp = await control.get<{ events: Event[] }>("/api/v1/events");
      const all: (Room & { eventName: string })[] = [];
      for (const ev of evResp.events) {
        const roomResp = await control.get<{ rooms: Room[] }>(`/api/v1/events/${ev.id}/rooms`);
        for (const room of roomResp.rooms) all.push({ ...room, eventName: ev.name });
      }
      setRooms(all);
      if (all.length > 0 && !selectedRoom) setSelectedRoom(all[0].id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "yükleme hatası");
    }
    // selectedRoom bilinçli olarak bağımlılık dışı: ilk seçim korunmalı.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  async function publish(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setNotice("");
    try {
      const sv = await control.postRaw<ShowVersion>(`/api/v1/shows/${id}/versions`, manifest);
      setNotice(`Sürüm ${sv.version} yayınlandı (sha256 ${sv.sha256.slice(0, 12)}…)`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "yayınlama hatası");
    }
  }

  async function activate(versionID: string) {
    setError("");
    setNotice("");
    if (!selectedRoom) {
      setError("Önce bir oda seçin (Etkinlikler sayfasından oda oluşturun).");
      return;
    }
    try {
      await control.post(`/api/v1/rooms/${selectedRoom}/activate`, { show_version_id: versionID });
      const room = rooms.find((r) => r.id === selectedRoom);
      setNotice(`Sürüm, "${room?.name ?? selectedRoom}" odasında etkinleştirildi.`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "etkinleştirme hatası");
    }
  }

  return (
    <>
      <h1>Gösteri sürümleri</h1>
      <form className="card" onSubmit={publish}>
        <h2>Manifest yayınla</h2>
        <p className="muted">
          Yayınlanan sürüm değişmezdir; telefonlar içeriği SHA-256 ile doğrular.
          Kurallar: flash_hz ≤ 3, screen kuelerinde #RRGGBB renk, audio
          kuelerinde asset_id.
        </p>
        <textarea rows={18} value={manifest} onChange={(e) => setManifest(e.target.value)} />
        <button>Doğrula ve yayınla</button>
      </form>
      {error && <p className="err">{error}</p>}
      {notice && <p className="ok">{notice}</p>}
      <div className="card">
        <h2>Sürümler</h2>
        {rooms.length > 0 && (
          <>
            <label>Etkinleştirilecek oda</label>
            <select value={selectedRoom} onChange={(e) => setSelectedRoom(e.target.value)}>
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.eventName} / {r.name} ({r.join_code})
                </option>
              ))}
            </select>
          </>
        )}
        {versions.length === 0 ? (
          <p className="muted">Henüz sürüm yok.</p>
        ) : (
          <table>
            <thead>
              <tr><th>Sürüm</th><th>SHA-256</th><th>Tarih</th><th></th></tr>
            </thead>
            <tbody>
              {versions.map((v) => (
                <tr key={v.id}>
                  <td>v{v.version}</td>
                  <td><code>{v.sha256.slice(0, 16)}…</code></td>
                  <td className="muted">{new Date(v.created_at).toLocaleString("tr-TR")}</td>
                  <td>
                    <button type="button" className="secondary" onClick={() => activate(v.id)}>
                      Odada etkinleştir
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
