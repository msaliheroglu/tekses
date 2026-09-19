"use client";

import { use, useCallback, useEffect, useState } from "react";
import {
  ApiError,
  control,
  getTranscription,
  startTranscription,
  uploadAsset,
  type Event,
  type Room,
  type ShowVersion,
} from "@/lib/api";
import { parseLrc } from "@/lib/lrc";

// Söz zamanlama/dalga formu editörü sonraki yineleme; MVP'de manifest JSON
// olarak düzenlenir. Şema: packages/manifest (sunucu yayında doğrular).
//
// Şablon çok şarkılı bir set örneğidir: her sekans, Canlı Konsol'un
// "Ne çalınacak" listesinde ayrı bir seçenek olur; "program" ise hepsini
// sırayla otomatik akıtır. Şarkı sözleri telifli olduğundan şablonda yer
// tutucudur — lisansladığınız sözleri kendiniz girin (lisans sorumluluğu
// organizatördedir). Ses için: yukarıdan dosya yükleyip asset_id'yi
// "audio" şeridine yapıştırın.
const TEMPLATE = `{
  "title": "Konser Seti",
  "sequences": [
    {
      "id": "acilis",
      "title": "Açılış tezahüratı",
      "duration_ms": 30000,
      "lyric_lines": [
        {"at_ms": 0, "duration_ms": 5000, "text": "Hep beraber!"},
        {"at_ms": 5000, "duration_ms": 5000, "text": "Tek ses, tek yürek!"},
        {"at_ms": 10000, "duration_ms": 0, "text": "🔥 🔥 🔥"}
      ],
      "cue_lanes": [
        {"id": "ekran", "kind": "screen", "cues": [
          {"at_ms": 0, "duration_ms": 30000, "color": "#FF2A2A", "flash_hz": 2}
        ]},
        {"id": "fener", "kind": "torch", "cues": [
          {"at_ms": 10000, "duration_ms": 20000, "flash_hz": 2}
        ]}
      ]
    },
    {
      "id": "medcezir",
      "title": "Medcezir — Levent Yüksel",
      "duration_ms": 60000,
      "lyric_lines": [
        {"at_ms": 0, "duration_ms": 8000, "text": "(Medcezir'in sözlerini zamanlarıyla buraya girin)"},
        {"at_ms": 8000, "duration_ms": 0, "text": "(ikinci satır…)"}
      ],
      "cue_lanes": [
        {"id": "ekran", "kind": "screen", "cues": [
          {"at_ms": 0, "duration_ms": 60000, "color": "#1E5AA8"}
        ]},
        {"id": "fener", "kind": "torch", "cues": [
          {"at_ms": 0, "duration_ms": 60000, "flash_hz": 1}
        ]}
      ]
    },
    {
      "id": "dar-sokaklar",
      "title": "Biz Dar Sokaklarında",
      "duration_ms": 60000,
      "lyric_lines": [
        {"at_ms": 0, "duration_ms": 8000, "text": "(şarkının sözlerini zamanlarıyla buraya girin)"}
      ],
      "cue_lanes": [
        {"id": "ekran", "kind": "screen", "cues": [
          {"at_ms": 0, "duration_ms": 60000, "color": "#F2B705"}
        ]}
      ]
    }
  ],
  "program": [
    {"sequence_id": "acilis", "at_offset_ms": 0},
    {"sequence_id": "medcezir", "at_offset_ms": 30000},
    {"sequence_id": "dar-sokaklar", "at_offset_ms": 90000}
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
  const [assets, setAssets] = useState<{ name: string; assetId: string; bytes: number }[]>([]);
  const [uploading, setUploading] = useState(false);
  const [lrcText, setLrcText] = useState("");
  const [lyricOut, setLyricOut] = useState("");
  const [transcribing, setTranscribing] = useState("");

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

  async function onPickAudio(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setError("");
    setUploading(true);
    try {
      const up = await uploadAsset(file);
      setAssets((prev) => [{ name: file.name, assetId: up.asset_id, bytes: up.bytes }, ...prev]);
      setNotice(`"${file.name}" yüklendi; asset_id'yi manifestteki audio kuesine yapıştırın.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "yükleme hatası");
    } finally {
      setUploading(false);
    }
  }

  function importLrc() {
    setError("");
    const lines = parseLrc(lrcText);
    if (lines.length === 0) {
      setError("LRC çözülemedi: [dd:ss.xx] zaman damgalı satır bulunamadı.");
      return;
    }
    setLyricOut(JSON.stringify(lines, null, 2));
    setNotice(`${lines.length} söz satırı üretildi; aşağıdaki çıktıyı manifestin "lyric_lines" alanına yapıştırın.`);
  }

  async function transcribe(assetId: string) {
    setError("");
    setNotice("");
    setTranscribing(assetId);
    try {
      const { transcription_id } = await startTranscription(assetId);
      const deadline = Date.now() + 10 * 60 * 1000;
      for (;;) {
        await new Promise((r) => setTimeout(r, 3000));
        const res = await getTranscription(transcription_id);
        if (res.status === "done") {
          const lines = res.lyric_lines ?? [];
          setLyricOut(JSON.stringify(lines, null, 2));
          setNotice(
            `${lines.length} satırlık TASLAK üretildi — şarkılarda tanıma hatalı olabilir, ` +
              `metin ve zamanlamayı kontrol edip düzeltin; sonra "lyric_lines" alanına yapıştırın.`,
          );
          break;
        }
        if (res.status === "error") {
          setError(`Söz çıkarma başarısız: ${res.error ?? "bilinmeyen hata"}`);
          break;
        }
        if (Date.now() > deadline) {
          setError("Söz çıkarma zaman aşımına uğradı.");
          break;
        }
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 501) {
        setError("Bu sunucuda otomatik söz çıkarma yapılandırılmamış — LRC içe aktarmayı kullanın (docs/dagitim.md kurulum notu).");
      } else {
        setError(err instanceof Error ? err.message : "söz çıkarma hatası");
      }
    } finally {
      setTranscribing("");
    }
  }

  return (
    <>
      <h1>Gösteri sürümleri</h1>
      <div className="card">
        <h2>Ses varlıkları</h2>
        <p className="muted">
          Yüklenen dosya içerik adresli bir <code>asset_id</code> alır; manifestte
          <code>{'{"kind": "audio", "cues": [{"at_ms": 0, "duration_ms": …, "asset_id": "…"}]}'}</code>
          şeridiyle kullanılır. Lisans sorumluluğu organizatördedir.
        </p>
        <input type="file" accept="audio/*" onChange={onPickAudio} disabled={uploading} />
        {uploading && <p className="muted">yükleniyor…</p>}
        {assets.length > 0 && (
          <table>
            <thead><tr><th>Dosya</th><th>asset_id</th><th>Boyut</th><th></th></tr></thead>
            <tbody>
              {assets.map((a) => (
                <tr key={a.assetId}>
                  <td>{a.name}</td>
                  <td><code>{a.assetId}</code></td>
                  <td className="muted">{(a.bytes / 1024 / 1024).toFixed(2)} MB</td>
                  <td>
                    <button
                      type="button"
                      className="ghost"
                      disabled={transcribing !== ""}
                      onClick={() => transcribe(a.assetId)}
                    >
                      {transcribing === a.assetId ? "çıkarılıyor…" : "Sözleri çıkar (deneysel)"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="card">
        <h2>Sözler: LRC içe aktar</h2>
        <p className="muted">
          Senkronlu söz dosyasını (<code>[01:23.45]söz satırı</code> biçimi)
          yapıştırın; zamanlı <code>lyric_lines</code> listesine çevrilir. En
          isabetli yol budur — otomatik çıkarma yalnızca taslak üretir.
        </p>
        <textarea
          rows={6}
          value={lrcText}
          onChange={(e) => setLrcText(e.target.value)}
          placeholder={"[00:12.30]İlk satır\n[00:20.50]İkinci satır"}
        />
        <button type="button" onClick={importLrc}>Sözlere çevir</button>
      </div>

      {lyricOut && (
        <div className="card">
          <h2>lyric_lines çıktısı</h2>
          <p className="muted">
            Aşağıyı kopyalayıp manifestte ilgili sekansın <code>&quot;lyric_lines&quot;</code>
            alanına yapıştırın.
          </p>
          <textarea rows={10} readOnly value={lyricOut} onFocus={(e) => e.target.select()} />
        </div>
      )}
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
