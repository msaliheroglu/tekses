"use client";

import { useCallback, useEffect, useState } from "react";
import {
  control,
  fetchManifestSummary,
  gatewayGet,
  gatewayPost,
  listPersistedRuns,
  type ClockStatsResponse,
  type Event,
  type ManifestSummary,
  type PresenceResponse,
  type Room,
  type RoomClockStats,
  type RunRecord,
} from "@/lib/api";

type LogLine = { at: string; text: string; isErr?: boolean };

// Ayrılmış kue kimliği: gömülü otomatik programı başlatır
// (packages/manifest ProgramCueID ile aynı).
const PROGRAM_CUE_ID = "program";
// Seçici değeri: Faz 0 doğrudan yük (renk/flash/fener) modu.
const FLASH_TARGET = "__flash__";

// Canlı konsol: kue ve müdahaleler doğrudan gateway'e gider (dev GO / HOLD /
// STOP / BLACKOUT karar dokümanı §3'teki canlı konsolun MVP hali).
export default function ConsolePage() {
  const [rooms, setRooms] = useState<(Room & { eventName: string })[]>([]);
  const [roomID, setRoomID] = useState("");
  const [cueTarget, setCueTarget] = useState(FLASH_TARGET);
  const [manifest, setManifest] = useState<ManifestSummary | null>(null);
  const [runs, setRuns] = useState<RunRecord[]>([]);
  // Kayıt kaynağı: "kalıcı" (control-api, org kapsamlı, çok düğümde titremez)
  // ya da "düğüm" (gateway halkası; oturumsuz Faz 0 yedeği).
  const [runsSource, setRunsSource] = useState<"kalıcı" | "düğüm">("düğüm");
  const [presence, setPresence] = useState<PresenceResponse | null>(null);
  const [clockStats, setClockStats] = useState<ClockStatsResponse | null>(null);
  const [color, setColor] = useState("#FF2A2A");
  const [delayMs, setDelayMs] = useState(3000);
  const [durationMs, setDurationMs] = useState(4000);
  const [flashHz, setFlashHz] = useState(2);
  const [torch, setTorch] = useState(true);
  const [adminToken, setAdminToken] = useState("");
  const [lines, setLines] = useState<LogLine[]>([]);
  const [lastRunID, setLastRunID] = useState("");

  const log = useCallback((text: string, isErr?: boolean) => {
    setLines((prev) => [
      { at: new Date().toLocaleTimeString("tr-TR"), text, isErr },
      ...prev.slice(0, 49),
    ]);
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const evResp = await control.get<{ events: Event[] }>("/api/v1/events");
        const all: (Room & { eventName: string })[] = [];
        for (const ev of evResp.events) {
          const roomResp = await control.get<{ rooms: Room[] }>(`/api/v1/events/${ev.id}/rooms`);
          for (const room of roomResp.rooms) all.push({ ...room, eventName: ev.name });
        }
        setRooms(all);
        if (all.length > 0) setRoomID(all[0].id);
      } catch {
        // oturum yoksa da konsol gateway'in "faz0" odasıyla kullanılabilir
      }
    })();
  }, []);

  // Seçili odanın aktif gösterisinden sekans/program seçenekleri yüklenir.
  useEffect(() => {
    setCueTarget(FLASH_TARGET);
    const room = rooms.find((r) => r.id === roomID);
    const versionID = room?.active_show_version_id;
    if (!versionID) {
      setManifest(null);
      return;
    }
    let cancelled = false;
    fetchManifestSummary(versionID)
      .then((m) => {
        if (!cancelled) setManifest(m);
      })
      .catch(() => {
        if (!cancelled) setManifest(null);
      });
    return () => {
      cancelled = true;
    };
  }, [roomID, rooms]);

  // Telemetri: sayfa açıkken 4 sn'de bir tazelenir (tek zamanlayıcı).
  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const p = await gatewayGet<PresenceResponse>("/api/v0/presence", adminToken);
        if (!cancelled) setPresence(p);
      } catch {
        // gateway kapalı ya da eski sürüm — sessiz
      }
      try {
        const c = await gatewayGet<ClockStatsResponse>("/api/v0/clockstats", adminToken);
        if (!cancelled) setClockStats(c);
      } catch {
        // sessiz
      }
      // Çalıştırma kaydı: kalıcı liste (org kapsamlı, --scale'de titremez)
      // ile düğüm halkası BİRLEŞTİRİLİR. Yalnız kalıcıya güvenmek iki şeyi
      // kaybettirir: org'suz kayıtlar ("tüm odalar"/Faz 0 — org kapsamlı
      // listede bilinçle yok) ve TEKSES_INTERNAL_TOKEN'sız kurulumlar
      // (kalıcı liste başarılı ama hep boş döner).
      let persisted: RunRecord[] | null = null;
      try {
        const resp = await listPersistedRuns(50);
        persisted = resp.runs ?? [];
      } catch {
        persisted = null; // oturum yok ya da eski control-api
      }
      let ring: RunRecord[] = [];
      try {
        const resp = await gatewayGet<{ runs: RunRecord[] }>("/api/v0/runs", adminToken);
        ring = resp.runs ?? [];
      } catch {
        // gateway kapalıyken sessiz kal
      }
      if (cancelled) return;
      if (persisted === null) {
        setRuns(ring);
        setRunsSource("düğüm");
        return;
      }
      const seen = new Set(persisted.map((r) => r.id).filter(Boolean));
      const merged = [...persisted, ...ring.filter((r) => !r.id || !seen.has(r.id))]
        .sort((a, b) => b.issued_at_server_ms - a.issued_at_server_ms)
        .slice(0, 50);
      setRuns(merged);
      setRunsSource("kalıcı");
    };
    void tick();
    const timer = setInterval(tick, 4000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [adminToken]);

  async function sendCue() {
    try {
      // Faz 0 modunda yük (renk/flash/fener) telefonda doğrudan oynar;
      // sekans/program modunda telefon manifestten oynar, cue_id yeterlidir.
      const body: Record<string, unknown> = {
        room_id: roomID,
        delayMs,
        durationMs,
        color: color.toUpperCase(),
        torch,
        flashHz,
      };
      if (cueTarget !== FLASH_TARGET) body.cue_id = cueTarget;
      const resp = await gatewayPost<{ run_id: string; clients: number; fire_at_server_ms: number }>(
        "/api/v0/cue",
        body,
        adminToken,
      );
      setLastRunID(resp.run_id);
      const label =
        cueTarget === FLASH_TARGET
          ? "flash"
          : cueTarget === PROGRAM_CUE_ID
            ? "OTOMATİK PROGRAM"
            : `sekans ${cueTarget}`;
      log(`GO (${label}) → run ${resp.run_id}, ${resp.clients} telefon, ateşleme +${delayMs} ms`);
    } catch (err) {
      log(`kue hatası: ${err instanceof Error ? err.message : "?"}`, true);
    }
  }

  async function intervene(kind: "HOLD" | "STOP" | "SKIP" | "BLACKOUT") {
    try {
      await gatewayPost("/api/v0/intervention", { kind, room_id: roomID, run_id: lastRunID }, adminToken);
      log(`${kind} gönderildi`);
    } catch (err) {
      log(`${kind} hatası: ${err instanceof Error ? err.message : "?"}`, true);
    }
  }

  return (
    <>
      <h1>Canlı Konsol</h1>
      <div className="card">
        <div className="row">
          <div>
            <label>Oda</label>
            <select value={roomID} onChange={(e) => setRoomID(e.target.value)}>
              <option value="">tüm odalar (Faz 0)</option>
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.eventName} / {r.name} ({r.join_code})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label>Gateway yönetici token&apos;ı (gerekmez)</label>
            <input
              type="password"
              value={adminToken}
              onChange={(e) => setAdminToken(e.target.value)}
              placeholder="boş = panel oturumunuz kullanılır"
            />
          </div>
        </div>
        <div className="row">
          <div>
            <label>Ne çalınacak</label>
            <select value={cueTarget} onChange={(e) => setCueTarget(e.target.value)}>
              <option value={FLASH_TARGET}>Faz 0 flash (aşağıdaki yük)</option>
              {manifest?.hasProgram && (
                <option value={PROGRAM_CUE_ID}>OTOMATİK PROGRAM (tüm akış)</option>
              )}
              {manifest?.sequences.map((s) => (
                <option key={s.id} value={s.id}>
                  Sekans: {s.title}
                </option>
              ))}
            </select>
            {!manifest && (
              <p className="muted" style={{ margin: "4px 0 0" }}>
                Odada aktif gösteri yok; yalnızca Faz 0 flash gönderilebilir.
              </p>
            )}
          </div>
        </div>
        <div className="row">
          <div style={{ flex: "0 0 90px" }}>
            <label>Renk</label>
            <input type="color" value={color} onChange={(e) => setColor(e.target.value)} />
          </div>
          <div>
            <label>Gecikme (ms)</label>
            <input type="number" min={500} step={500} value={delayMs}
              onChange={(e) => setDelayMs(Number(e.target.value))} />
          </div>
          <div>
            <label>Süre (ms)</label>
            <input type="number" min={500} step={500} value={durationMs}
              onChange={(e) => setDurationMs(Number(e.target.value))} />
          </div>
          <div>
            <label>Yanıp sönme</label>
            <select value={flashHz} onChange={(e) => setFlashHz(Number(e.target.value))}>
              <option value={0}>sabit</option>
              <option value={1}>1 Hz</option>
              <option value={2}>2 Hz</option>
              <option value={3}>3 Hz (üst sınır)</option>
            </select>
          </div>
          <div style={{ flex: "0 0 80px" }}>
            <label>Fener</label>
            <input type="checkbox" checked={torch} onChange={(e) => setTorch(e.target.checked)} />
          </div>
        </div>
        <button onClick={sendCue} style={{ width: "100%", fontSize: 20, padding: 16 }}>
          GO — KUE GÖNDER
        </button>
        <div className="iv-grid">
          <button className="iv-hold" onClick={() => intervene("HOLD")}>HOLD</button>
          <button className="iv-stop" onClick={() => intervene("STOP")}>STOP</button>
          <button className="iv-blackout" onClick={() => intervene("BLACKOUT")}>BLACKOUT</button>
          <button className="iv-skip" onClick={() => intervene("SKIP")}>SKIP</button>
        </div>
      </div>
      <div className="card">
        <h2>Kayıt</h2>
        {lines.length === 0 ? (
          <p className="muted">Henüz komut gönderilmedi.</p>
        ) : (
          lines.map((l, i) => (
            <p key={i} className={l.isErr ? "err" : "ok"} style={{ margin: "4px 0" }}>
              {l.at} {l.text}
            </p>
          ))
        )}
      </div>
      <div className="card">
        <h2>Katılımcılar</h2>
        {presence === null ? (
          <p className="muted">
            Erişilemiyor — bu kart işletmen token'ı ister (yukarıdaki alana
            TEKSES_ADMIN_TOKEN girin; kiracılar arası veri içerdiğinden panel
            oturumu yetmez).
          </p>
        ) : (
          <>
            <p style={{ fontSize: 32, fontWeight: 800, margin: "4px 0" }}>
              {presence.total}
              <span className="muted" style={{ fontSize: 13, fontWeight: 400 }}>
                {" "}bağlı telefon · {presence.node_count} düğüm · yaklaşık (≤15 sn gecikmeli)
              </span>
            </p>
            {Object.keys(presence.rooms).length > 0 && (
              <p className="muted">
                {Object.entries(presence.rooms)
                  .map(([room, n]) => `${room}: ${n}`)
                  .join(" · ")}
              </p>
            )}
          </>
        )}
      </div>
      <div className="card">
        <h2>Saat kalitesi (RTT — senkron kalite vekili)</h2>
        {clockStats === null ? (
          <p className="muted">Erişilemiyor — bu kart da işletmen token'ı ister.</p>
        ) : Object.keys(clockStats.rooms).length === 0 ? (
          <p className="muted">
            Henüz örnek yok — ilk ölçüm bağlantıdan ~{Math.round(clockStats.ping_interval_ms / 1000)} sn
            sonra gelir (bu düğümün istemcileri).
          </p>
        ) : (
          Object.entries(clockStats.rooms).map(([room, st]) => (
            <div key={room} style={{ margin: "10px 0" }}>
              <p style={{ margin: "0 0 4px" }}>
                {room}{" "}
                <span className="muted">
                  {st.sampled > 0 ? `p50 ${st.p50_ms} ms · p95 ${st.p95_ms} ms · ` : ""}
                  {st.clients} istemci
                  {st.no_sample > 0 ? ` · ${st.no_sample} örneksiz` : ""}
                  {st.stale > 0 ? ` · ${st.stale} bayat` : ""}
                </span>
              </p>
              <HeatBar st={st} />
            </div>
          ))
        )}
      </div>
      <div className="card">
        <h2>Çalıştırma kaydı {runsSource === "kalıcı" ? "(kalıcı + bu düğümün halkası)" : "(bu düğümün halkası)"}</h2>
        {runs.length === 0 ? (
          <p className="muted">Henüz çalıştırma yok.</p>
        ) : (
          <table>
            <thead>
              <tr><th>Tür</th><th>Kue</th><th>Oda</th><th>Telefon (düğüm)</th><th>Zaman</th></tr>
            </thead>
            <tbody>
              {runs.map((r, i) => (
                <tr key={r.id ?? `${r.run_id ?? "iv"}-${r.kind}-${i}`}>
                  <td>{r.kind === "cue" ? "kue" : r.kind}</td>
                  <td>{r.cue_id || "—"}</td>
                  <td>{r.room_id || "tümü"}</td>
                  <td>{r.clients}</td>
                  <td className="muted">
                    {new Date(r.issued_at_server_ms).toLocaleTimeString("tr-TR")}
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

// Isı çubuğu: kovalar sunucuda sabit (<10 yeşil, <30 sarı — ≤30 ms ürün
// hedefiyle hizalı, <100 turuncu, ≥100 kırmızı; örneksiz/bayat gri).
function HeatBar({ st }: { st: RoomClockStats }) {
  const parts = [
    { n: st.lt10, color: "#2e9e4f", label: "<10 ms" },
    { n: st.lt30, color: "#b7a11a", label: "10-30 ms" },
    { n: st.lt100, color: "#c26a1d", label: "30-100 ms" },
    { n: st.gte100, color: "#c22222", label: "≥100 ms" },
    { n: st.no_sample + st.stale, color: "#555", label: "örnek yok/bayat" },
  ].filter((p) => p.n > 0);
  const total = parts.reduce((a, p) => a + p.n, 0) || 1;
  return (
    <div style={{ display: "flex", height: 14, borderRadius: 7, overflow: "hidden", background: "#222" }}>
      {parts.map((p, i) => (
        <div
          key={i}
          title={`${p.label}: ${p.n}`}
          style={{ width: `${(p.n / total) * 100}%`, background: p.color }}
        />
      ))}
    </div>
  );
}
