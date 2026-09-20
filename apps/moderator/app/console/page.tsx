"use client";

import { useCallback, useEffect, useState } from "react";
import {
  control,
  fetchManifestSummary,
  gatewayGet,
  gatewayPost,
  type Event,
  type ManifestSummary,
  type Room,
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

  // Çalıştırma kaydı: sayfa açıkken 4 sn'de bir tazelenir.
  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const resp = await gatewayGet<{ runs: RunRecord[] }>("/api/v0/runs", adminToken);
        if (!cancelled) setRuns(resp.runs);
      } catch {
        // gateway kapalıyken sessiz kal
      }
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
        <h2>Çalıştırma kaydı (gateway)</h2>
        {runs.length === 0 ? (
          <p className="muted">Henüz çalıştırma yok.</p>
        ) : (
          <table>
            <thead>
              <tr><th>Tür</th><th>Kue</th><th>Oda</th><th>Telefon</th><th>Zaman</th></tr>
            </thead>
            <tbody>
              {runs.map((r, i) => (
                <tr key={`${r.run_id ?? "iv"}-${r.kind}-${i}`}>
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
