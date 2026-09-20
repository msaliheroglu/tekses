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
import {
  defaultShow,
  emptySequence,
  fmtTime,
  fromManifest,
  parseTime,
  toManifestJson,
  type EditorLyric,
  type EditorScreenStep,
  type EditorSequence,
  type EditorShow,
  type EditorTorchStep,
} from "@/lib/manifestEditor";

// Görsel gösteri editörü: sekans/ses/söz/ekran/fener form ve seçicilerle
// düzenlenir; manifest JSON'u yayında lib/manifestEditor üretir. JSON'u elle
// yazmak isteyen (ya da editörün taşımadığı gelişmiş yapıları kullanan) için
// "Gelişmiş (JSON)" görünümü açılır. Şema ve kurallar: packages/manifest —
// sunucu yayında yine de doğrular.

// Süre alanı: "d:ss.o" biçimi (ör. 1:23.5); geçersiz girdi kırmızı kalır ve
// değer değişmez.
function TimeField({
  ms,
  onChange,
  placeholder,
}: {
  ms: number;
  onChange: (ms: number) => void;
  placeholder?: string;
}) {
  const [text, setText] = useState(fmtTime(ms));
  const [bad, setBad] = useState(false);
  useEffect(() => {
    setText(fmtTime(ms));
    setBad(false);
  }, [ms]);
  return (
    <input
      value={text}
      placeholder={placeholder}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => {
        const v = parseTime(text);
        if (v === null) {
          setBad(true);
          return;
        }
        setBad(false);
        onChange(v);
      }}
      style={bad ? { borderColor: "var(--err)" } : undefined}
    />
  );
}

function FlashSelect({ hz, onChange }: { hz: number; onChange: (hz: number) => void }) {
  return (
    <select value={hz} onChange={(e) => onChange(Number(e.target.value))}>
      <option value={0}>sabit yanar</option>
      <option value={1}>yanıp söner — 1 Hz</option>
      <option value={2}>yanıp söner — 2 Hz</option>
      <option value={3}>yanıp söner — 3 Hz</option>
    </select>
  );
}

type AssetInfo = { name: string; assetId: string; bytes: number; durationMs: number | null };

// Tarayıcı, yüklenen dosyanın süresini metadata'dan okuyabilir; sekans süresi
// buna göre otomatik doldurulur (okunamazsa null — elle girilir).
function audioDurationMs(file: File): Promise<number | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const a = new Audio();
    a.preload = "metadata";
    a.onloadedmetadata = () => {
      URL.revokeObjectURL(url);
      resolve(Number.isFinite(a.duration) ? Math.round(a.duration * 1000) : null);
    };
    a.onerror = () => {
      URL.revokeObjectURL(url);
      resolve(null);
    };
    a.src = url;
  });
}

export default function ShowDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [versions, setVersions] = useState<ShowVersion[]>([]);
  const [rooms, setRooms] = useState<(Room & { eventName: string })[]>([]);
  const [selectedRoom, setSelectedRoom] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [assets, setAssets] = useState<AssetInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [lrcText, setLrcText] = useState("");
  const [transcribing, setTranscribing] = useState("");

  const [show, setShow] = useState<EditorShow>(defaultShow);
  const [advanced, setAdvanced] = useState(false);
  const [jsonText, setJsonText] = useState("");
  // LRC/otomatik çıkarmadan gelen sözler: kullanıcı hangi sekansa koyacağını
  // sekans kartındaki düğmeyle seçer.
  const [pendingLyrics, setPendingLyrics] = useState<EditorLyric[] | null>(null);
  const [pendingSource, setPendingSource] = useState("");

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

  // --- sekans durumu yardımcıları ---

  function patchSeq(i: number, patch: Partial<EditorSequence>) {
    setShow((s) => ({
      ...s,
      sequences: s.sequences.map((sq, j) => (j === i ? { ...sq, ...patch } : sq)),
    }));
  }

  function moveSeq(i: number, dir: -1 | 1) {
    setShow((s) => {
      const seqs = [...s.sequences];
      const j = i + dir;
      if (j < 0 || j >= seqs.length) return s;
      [seqs[i], seqs[j]] = [seqs[j], seqs[i]];
      return { ...s, sequences: seqs };
    });
  }

  function removeSeq(i: number) {
    setShow((s) => ({ ...s, sequences: s.sequences.filter((_, j) => j !== i) }));
  }

  // --- yayınlama / sürümler ---

  async function publish(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setNotice("");
    try {
      const raw = advanced ? jsonText : toManifestJson(show);
      const sv = await control.postRaw<ShowVersion>(`/api/v1/shows/${id}/versions`, raw);
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
      setNotice(
        `Sürüm, "${room?.name ?? selectedRoom}" odasında etkinleştirildi. ` +
          `Katılmış telefonların yeni paketi alması için odadan çıkıp yeniden katılmaları gerekir.`,
      );
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "etkinleştirme hatası");
    }
  }

  async function loadVersionIntoEditor(versionID: string, versionNo: number) {
    setError("");
    setNotice("");
    try {
      const resp = await control.get<{ manifest: unknown }>(`/api/v1/show-versions/${versionID}`);
      const r = fromManifest(resp.manifest);
      if (r.show) {
        setShow(r.show);
        setAdvanced(false);
        setNotice(`v${versionNo} editöre yüklendi — değişiklikler YENİ sürüm olarak yayınlanır.`);
      } else {
        setJsonText(JSON.stringify(resp.manifest, null, 2));
        setAdvanced(true);
        setNotice(`v${versionNo} görsel editörün taşımadığı bir yapı içeriyor (${r.reason}); JSON görünümünde açıldı.`);
      }
      window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "sürüm yüklenemedi");
    }
  }

  function toggleAdvanced() {
    setError("");
    if (!advanced) {
      setJsonText(toManifestJson(show));
      setAdvanced(true);
      return;
    }
    try {
      const r = fromManifest(JSON.parse(jsonText));
      if (!r.show) {
        setError(`JSON, görsel editöre taşınamıyor: ${r.reason}. JSON görünümünde kalındı.`);
        return;
      }
      setShow(r.show);
      setAdvanced(false);
    } catch {
      setError("JSON çözülemedi; düzeltin ya da JSON görünümünde yayınlayın.");
    }
  }

  // --- ses varlıkları ve sözler ---

  async function onPickAudio(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setError("");
    setUploading(true);
    try {
      const [up, durationMs] = await Promise.all([uploadAsset(file), audioDurationMs(file)]);
      setAssets((prev) => [{ name: file.name, assetId: up.asset_id, bytes: up.bytes, durationMs }, ...prev]);
      setNotice(`"${file.name}" yüklendi — sekans kartındaki "Müzik" listesinden seçin.`);
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
    setPendingLyrics(lines.map((l) => ({ atMs: l.at_ms, durationMs: l.duration_ms, text: l.text })));
    setPendingSource("LRC");
    setNotice(`${lines.length} söz satırı hazır — istediğiniz sekansta "Hazır sözleri koy" düğmesine basın.`);
  }

  async function transcribe(assetId: string) {
    setError("");
    setNotice("");
    setTranscribing(assetId);
    try {
      const { transcription_id } = await startTranscription(assetId);
      // Sunucu tarafı zaman aşımı ayarlanabilir (TEKSES_TRANSCRIBE_TIMEOUT,
      // whisper'lı VM'de 60 dk — Demucs CPU'da yavaştır); panel ondan önce
      // pes etmesin.
      const deadline = Date.now() + 65 * 60 * 1000;
      for (;;) {
        await new Promise((r) => setTimeout(r, 3000));
        const res = await getTranscription(transcription_id);
        if (res.status === "done") {
          const lines = res.lyric_lines ?? [];
          if (lines.length === 0) {
            // Boş sonuçtan "0 satır hazır" düğmesi üretmek yanıltıcı olur.
            setError(
              "Söz bulunamadı: Whisper bu kayıtta vokal seçemedi. Sunucuda Demucs yoksa " +
                "(ya da kapalıysa) mikslenmiş şarkılarda bu beklenir — LRC içe aktarmayı deneyin.",
            );
            break;
          }
          setPendingLyrics(lines.map((l) => ({ atMs: l.at_ms, durationMs: l.duration_ms, text: l.text })));
          setPendingSource("otomatik çıkarma (TASLAK)");
          setNotice(
            `${lines.length} satırlık TASLAK hazır — şarkılarda tanıma hatalı olabilir; ` +
              `sekansa koyduktan sonra metni ve zamanlamayı kontrol edin.`,
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
        setError(
          "Bu sunucuda otomatik söz çıkarma yapılandırılmamış — LRC içe aktarmayı kullanın (docs/dagitim.md kurulum notu).",
        );
      } else {
        setError(err instanceof Error ? err.message : "söz çıkarma hatası");
      }
    } finally {
      setTranscribing("");
    }
  }

  // Program özeti: dahil sekansların hesaplanan başlama anları.
  const programSummary: string[] = [];
  {
    let cursor = 0;
    for (const sq of show.sequences) {
      if (!sq.inProgram) continue;
      const offset = cursor + Math.max(0, sq.gapBeforeMs);
      programSummary.push(`${fmtTime(offset)} → ${sq.title || "(adsız)"}`);
      cursor = offset + sq.durationMs;
    }
  }

  return (
    <>
      <h1>Gösteri düzenle</h1>

      <div className="card">
        <h2>Ses varlıkları</h2>
        <p className="muted">
          Şarkı dosyasını yükleyin, sonra ilgili sekans kartında &quot;Müzik&quot;
          listesinden seçin. Süre otomatik okunur. Lisans sorumluluğu
          organizatördedir.
        </p>
        <input type="file" accept="audio/*" onChange={onPickAudio} disabled={uploading} />
        {uploading && <p className="muted">yükleniyor…</p>}
        {assets.length > 0 && (
          <table>
            <thead><tr><th>Dosya</th><th>Süre</th><th>Boyut</th><th></th></tr></thead>
            <tbody>
              {assets.map((a) => (
                <tr key={a.assetId}>
                  <td>{a.name}</td>
                  <td className="muted">{a.durationMs !== null ? fmtTime(a.durationMs) : "?"}</td>
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
          yapıştırın. En isabetli yol budur — otomatik çıkarma yalnızca taslak üretir.
        </p>
        <textarea
          rows={5}
          value={lrcText}
          onChange={(e) => setLrcText(e.target.value)}
          placeholder={"[00:12.30]İlk satır\n[00:20.50]İkinci satır"}
        />
        <button type="button" onClick={importLrc}>Sözlere çevir</button>
        {pendingLyrics && (
          <p className="ok">
            {pendingLyrics.length} satır hazır ({pendingSource}) — aşağıda bir sekansta
            &quot;Hazır sözleri koy&quot; düğmesine basın.
          </p>
        )}
      </div>

      <form onSubmit={publish}>
        <div className="card">
          <div className="row">
            <div>
              <label>Gösteri başlığı</label>
              <input value={show.title} onChange={(e) => setShow((s) => ({ ...s, title: e.target.value }))} />
            </div>
            <div style={{ flex: "0 0 auto" }}>
              <button type="button" className="ghost" onClick={toggleAdvanced}>
                {advanced ? "← Görsel editöre dön" : "Gelişmiş (JSON)"}
              </button>
            </div>
          </div>
        </div>

        {advanced ? (
          <div className="card">
            <h2>Manifest JSON</h2>
            <p className="muted">
              Kurallar: flash_hz ≤ 3, screen kuelerinde #RRGGBB renk, audio kuelerinde
              asset_id. Sunucu yayında doğrular.
            </p>
            <textarea rows={22} value={jsonText} onChange={(e) => setJsonText(e.target.value)} />
          </div>
        ) : (
          <>
            {show.sequences.map((sq, i) => (
              <div className="card" key={i}>
                <div className="row">
                  <div style={{ flex: "2 1 220px" }}>
                    <label>Sekans başlığı (konsolda bu adla görünür)</label>
                    <input value={sq.title} onChange={(e) => patchSeq(i, { title: e.target.value })} />
                  </div>
                  <div>
                    <label>Süre (dk:sn)</label>
                    <TimeField ms={sq.durationMs} onChange={(ms) => patchSeq(i, { durationMs: ms })} />
                  </div>
                  <div style={{ flex: "0 0 auto" }}>
                    <button type="button" className="ghost" onClick={() => moveSeq(i, -1)} disabled={i === 0}>↑</button>{" "}
                    <button type="button" className="ghost" onClick={() => moveSeq(i, 1)} disabled={i === show.sequences.length - 1}>↓</button>{" "}
                    <button type="button" className="ghost" onClick={() => removeSeq(i)}>Sil</button>
                  </div>
                </div>

                <div className="row">
                  <div>
                    <label>Müzik (isteğe bağlı)</label>
                    <select
                      value={sq.audioAssetId}
                      onChange={(e) => {
                        const assetId = e.target.value;
                        const known = assets.find((a) => a.assetId === assetId);
                        // Şarkı seçilince sekans süresi şarkının süresine çekilir.
                        patchSeq(i, {
                          audioAssetId: assetId,
                          ...(known?.durationMs ? { durationMs: known.durationMs } : {}),
                        });
                      }}
                    >
                      <option value="">— müzik yok —</option>
                      {sq.audioAssetId && !assets.some((a) => a.assetId === sq.audioAssetId) && (
                        <option value={sq.audioAssetId}>(mevcut ses: {sq.audioAssetId.slice(0, 12)}…)</option>
                      )}
                      {assets.map((a) => (
                        <option key={a.assetId} value={a.assetId}>{a.name}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label>Otomatik programda</label>
                    <select
                      value={sq.inProgram ? "1" : "0"}
                      onChange={(e) => patchSeq(i, { inProgram: e.target.value === "1" })}
                    >
                      <option value="1">dahil</option>
                      <option value="0">hariç (yalnız elle başlatılır)</option>
                    </select>
                  </div>
                  {sq.inProgram && (
                    <div>
                      <label>Öncesinde boşluk (dk:sn)</label>
                      <TimeField ms={sq.gapBeforeMs} onChange={(ms) => patchSeq(i, { gapBeforeMs: ms })} />
                    </div>
                  )}
                </div>

                <label>Ekran adımları (telefon ekranının rengi)</label>
                {sq.screen.length === 0 && <p className="muted">Adım yok — ekran karanlık kalır.</p>}
                {sq.screen.map((st, j) => (
                  <div className="row" key={j}>
                    <div>
                      <label>Başlangıç</label>
                      <TimeField
                        ms={st.atMs}
                        onChange={(ms) =>
                          patchSeq(i, { screen: sq.screen.map((x, k) => (k === j ? { ...x, atMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div>
                      <label>Süre (0:00 = sona dek)</label>
                      <TimeField
                        ms={st.durationMs}
                        onChange={(ms) =>
                          patchSeq(i, { screen: sq.screen.map((x, k) => (k === j ? { ...x, durationMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "0 0 70px" }}>
                      <label>Renk</label>
                      <input
                        type="color"
                        value={st.color}
                        style={{ padding: 2, height: 42 }}
                        onChange={(e) =>
                          patchSeq(i, { screen: sq.screen.map((x, k) => (k === j ? { ...x, color: e.target.value } : x)) })
                        }
                      />
                    </div>
                    <div>
                      <label>Flaş</label>
                      <FlashSelect
                        hz={st.flashHz}
                        onChange={(hz) =>
                          patchSeq(i, { screen: sq.screen.map((x, k) => (k === j ? { ...x, flashHz: hz } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "0 0 auto" }}>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => patchSeq(i, { screen: sq.screen.filter((_, k) => k !== j) })}
                      >
                        Sil
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    const last = sq.screen[sq.screen.length - 1];
                    const at = last ? (last.durationMs > 0 ? last.atMs + last.durationMs : last.atMs) : 0;
                    const step: EditorScreenStep = { atMs: at, durationMs: 0, color: "#d92b2b", flashHz: 0 };
                    patchSeq(i, { screen: [...sq.screen, step] });
                  }}
                >
                  + Ekran adımı
                </button>

                <label>Fener adımları (isteğe bağlı; yalnız telefon uygulamasında)</label>
                {sq.torch.map((st, j) => (
                  <div className="row" key={j}>
                    <div>
                      <label>Başlangıç</label>
                      <TimeField
                        ms={st.atMs}
                        onChange={(ms) =>
                          patchSeq(i, { torch: sq.torch.map((x, k) => (k === j ? { ...x, atMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div>
                      <label>Süre (0:00 = sona dek)</label>
                      <TimeField
                        ms={st.durationMs}
                        onChange={(ms) =>
                          patchSeq(i, { torch: sq.torch.map((x, k) => (k === j ? { ...x, durationMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div>
                      <label>Flaş</label>
                      <FlashSelect
                        hz={st.flashHz}
                        onChange={(hz) =>
                          patchSeq(i, { torch: sq.torch.map((x, k) => (k === j ? { ...x, flashHz: hz } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "0 0 auto" }}>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => patchSeq(i, { torch: sq.torch.filter((_, k) => k !== j) })}
                      >
                        Sil
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    const step: EditorTorchStep = { atMs: 0, durationMs: 0, flashHz: 1 };
                    patchSeq(i, { torch: [...sq.torch, step] });
                  }}
                >
                  + Fener adımı
                </button>

                <label>Sözler (karaoke)</label>
                {pendingLyrics && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => {
                      patchSeq(i, { lyrics: pendingLyrics });
                      setPendingLyrics(null);
                      setNotice(`Sözler "${sq.title}" sekansına kondu (${pendingSource}).`);
                    }}
                  >
                    Hazır sözleri koy ({pendingLyrics.length} satır — {pendingSource})
                  </button>
                )}
                {sq.lyrics.map((ln, j) => (
                  <div className="row" key={j}>
                    <div style={{ flex: "0 1 110px" }}>
                      <label>Başlangıç</label>
                      <TimeField
                        ms={ln.atMs}
                        onChange={(ms) =>
                          patchSeq(i, { lyrics: sq.lyrics.map((x, k) => (k === j ? { ...x, atMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "0 1 110px" }}>
                      <label>Süre</label>
                      <TimeField
                        ms={ln.durationMs}
                        onChange={(ms) =>
                          patchSeq(i, { lyrics: sq.lyrics.map((x, k) => (k === j ? { ...x, durationMs: ms } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "3 1 240px" }}>
                      <label>Metin</label>
                      <input
                        value={ln.text}
                        onChange={(e) =>
                          patchSeq(i, { lyrics: sq.lyrics.map((x, k) => (k === j ? { ...x, text: e.target.value } : x)) })
                        }
                      />
                    </div>
                    <div style={{ flex: "0 0 auto" }}>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => patchSeq(i, { lyrics: sq.lyrics.filter((_, k) => k !== j) })}
                      >
                        Sil
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    const last = sq.lyrics[sq.lyrics.length - 1];
                    const at = last ? last.atMs + (last.durationMs || 4000) : 0;
                    patchSeq(i, { lyrics: [...sq.lyrics, { atMs: at, durationMs: 4000, text: "" }] });
                  }}
                >
                  + Söz satırı
                </button>
              </div>
            ))}

            <button
              type="button"
              className="secondary"
              onClick={() =>
                setShow((s) => ({ ...s, sequences: [...s.sequences, emptySequence(`Şarkı ${s.sequences.length + 1}`)] }))
              }
            >
              + Sekans ekle
            </button>

            {programSummary.length > 0 && (
              <div className="card">
                <h2>Otomatik program akışı</h2>
                <p className="muted">
                  Konsolda &quot;OTOMATİK PROGRAM&quot; başlatıldığında bu sırayla çalar:
                </p>
                {programSummary.map((line, i) => (
                  <p key={i} style={{ margin: "4px 0" }}>{line}</p>
                ))}
              </div>
            )}
          </>
        )}

        <div className="card">
          <p className="muted">
            Yayınlanan sürüm değişmezdir; telefonlar içeriği SHA-256 ile doğrular.
            Düzeltme gerektiğinde yeni sürüm yayınlayıp odada onu etkinleştirin.
          </p>
          <button>Doğrula ve yayınla</button>
        </div>
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
                    <button type="button" className="ghost" onClick={() => loadVersionIntoEditor(v.id, v.version)}>
                      Editörde aç
                    </button>{" "}
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
