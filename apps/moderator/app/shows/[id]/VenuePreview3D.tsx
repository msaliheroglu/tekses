"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { screenColorAt } from "@/lib/effectEval";
import { fmtTime, toManifest, type EditorShow } from "@/lib/manifestEditor";
import { seatPoints, totalSeats, type SeatPoint } from "@/lib/venueEditor";
import SeatCloudCanvas from "../../components/SeatCloudCanvas";

// 3B mekân önizlemesi (F4.4): moderatör, koreografiyi tribüne göstermeden
// önce görür. Çizim ortak SeatCloudCanvas'ta; nokta rengi telefonun o
// koltukta o an basacağı ekran rengidir (lib/effectEval — aynı aritmetik).

const MAX_DRAWN_SEATS = 20000;

export default function VenuePreview3D({ show }: { show: EditorShow }) {
  const [seqIdx, setSeqIdx] = useState(0);
  const [timeMs, setTimeMs] = useState(0);
  const [playing, setPlaying] = useState(false);

  // Manifest ve koltuk bulutu yalnız gösteri değişince yeniden kurulur.
  const manifest = useMemo(() => toManifest(show), [show]);
  const cloud = useMemo(
    () => (show.venue ? seatPoints(show.venue, MAX_DRAWN_SEATS) : null),
    [show.venue],
  );

  const seq = manifest.sequences[Math.min(seqIdx, manifest.sequences.length - 1)];
  const durationMs = seq?.duration_ms ?? 0;

  const colorFor = useCallback(
    (p: SeatPoint, t: number) => (seq ? screenColorAt(seq, t, p.u, p.v, p.w) : ""),
    [seq],
  );

  // Oynatma: rAF ile gerçek zaman akar, sekans sonunda başa sarar.
  useEffect(() => {
    if (!playing) return;
    let raf = 0;
    let last = performance.now();
    const tick = (now: number) => {
      const dt = now - last;
      last = now;
      setTimeMs((t) => (durationMs > 0 ? (t + dt) % durationMs : 0));
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [playing, durationMs]);

  if (!show.venue || !cloud || manifest.sequences.length === 0) return null;

  return (
    <div className="card">
      <h2>3B önizleme — tribün böyle görecek</h2>
      <p className="muted">
        Her nokta bir koltuk; rengi, telefonun o koltukta o an basacağı ekran
        rengi. Sürükleyerek döndürün, tekerlekle yakınlaştırın.
        {cloud.stride > 1 &&
          ` ${totalSeats(show.venue).toLocaleString("tr-TR")} koltuktan her ${cloud.stride}. çizildi.`}
      </p>
      <div className="row">
        <div>
          <label>Sekans</label>
          <select
            value={seqIdx}
            onChange={(e) => {
              setSeqIdx(Number(e.target.value));
              setTimeMs(0);
            }}
          >
            {manifest.sequences.map((s, i) => (
              <option key={s.id} value={i}>
                {s.title || s.id}
              </option>
            ))}
          </select>
        </div>
        <div style={{ flex: "0 0 auto", alignSelf: "flex-end" }}>
          <button type="button" className="secondary" onClick={() => setPlaying((p) => !p)}>
            {playing ? "Duraklat" : "Oynat"}
          </button>
        </div>
        <div style={{ flex: "2 1 200px" }}>
          <label>
            Zaman: {fmtTime(timeMs)} / {fmtTime(durationMs)}
          </label>
          <input
            type="range"
            min={0}
            max={Math.max(0, durationMs - 1)}
            value={Math.min(timeMs, Math.max(0, durationMs - 1))}
            onChange={(e) => {
              setPlaying(false);
              setTimeMs(Number(e.target.value));
            }}
            style={{ width: "100%" }}
          />
        </div>
      </div>
      <SeatCloudCanvas points={cloud.points} timeMs={timeMs} colorFor={colorFor} />
      <p className="muted">
        Fener şeridi ve sözler önizlemede yok; ekran renk koreografisi
        telefonla aynı aritmetikle hesaplanır.
      </p>
    </div>
  );
}
