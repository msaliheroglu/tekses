"use client";

import { useCallback, useEffect, useState } from "react";
import { control, gatewayPost, type Event, type Room } from "@/lib/api";

type LogLine = { at: string; text: string; isErr?: boolean };

// Canlı konsol: kue ve müdahaleler doğrudan gateway'e gider (dev GO / HOLD /
// STOP / BLACKOUT karar dokümanı §3'teki canlı konsolun MVP hali).
export default function ConsolePage() {
  const [rooms, setRooms] = useState<(Room & { eventName: string })[]>([]);
  const [roomID, setRoomID] = useState("");
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

  async function sendCue() {
    try {
      const resp = await gatewayPost<{ run_id: string; clients: number; fire_at_server_ms: number }>(
        "/api/v0/cue",
        { room_id: roomID, delayMs, durationMs, color: color.toUpperCase(), torch, flashHz },
        adminToken,
      );
      setLastRunID(resp.run_id);
      log(`GO → run ${resp.run_id}, ${resp.clients} telefon, ateşleme +${delayMs} ms`);
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
            <label>Gateway yönetici token&apos;ı (ayarlıysa)</label>
            <input
              type="password"
              value={adminToken}
              onChange={(e) => setAdminToken(e.target.value)}
              placeholder="boş bırakılabilir"
            />
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
    </>
  );
}
