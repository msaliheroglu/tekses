"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { screenColorAt, seqLyricAt } from "@/lib/effectEval";
import { fmtTime, toManifest, type EditorShow } from "@/lib/manifestEditor";
import { blockCorners, seatPoints, totalSeats, type SeatPoint } from "@/lib/venueEditor";
import SeatCloudCanvas, { type CloudScene } from "../../components/SeatCloudCanvas";

// Gösteri önizlemesi (F4.4/F4.5): moderatör koreografiyi yayınlamadan görür.
// Solda telefon maketi (mekânın ortasındaki koltuk: renk + söz), mekân planı
// varsa sağda 3B tribün (ortak SeatCloudCanvas; saha/sahne + blok adlarıyla).
// Renkler telefonla aynı aritmetikten (lib/effectEval).

const MAX_DRAWN_SEATS = 20000;

export default function VenuePreview3D({ show }: { show: EditorShow }) {
  const [seqIdx, setSeqIdx] = useState(0);
  const [timeMs, setTimeMs] = useState(0);
  const [playing, setPlaying] = useState(false);

  // Manifest ve koltuk bulutu yalnız gösteri değişince yeniden kurulur.
  const manifest = useMemo(() => toManifest(show), [show]);
  const cloud = useMemo(
    () => (show.venue && show.venue.blocks.length > 0 ? seatPoints(show.venue, MAX_DRAWN_SEATS) : null),
    [show.venue],
  );
  const scene = useMemo<CloudScene | undefined>(
    () =>
      show.venue
        ? {
            landmark: show.venue.landmark,
            blocks: show.venue.blocks.map((b) => ({
              label: b.id,
              corners: blockCorners(b),
              z: b.z,
            })),
          }
        : undefined,
    [show.venue],
  );

  const seq = manifest.sequences[Math.min(seqIdx, manifest.sequences.length - 1)];
  const durationMs = seq?.duration_ms ?? 0;

  const colorFor = useCallback(
    (p: SeatPoint, t: number) =>
      seq ? screenColorAt(seq, t, p.u, p.v, p.w, p.block) : "",
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

  if (!seq) return null;

  const phoneColor = screenColorAt(seq, timeMs, 0.5, 0.5, 0.5);
  const phoneLyric = seqLyricAt(seq, timeMs);

  return (
    <div className="card">
      <h2>Önizleme — seyirci böyle görecek</h2>
      <p className="muted">
        Solda bir seyirci telefonu (mekânın ortasındaki koltuk)
        {cloud ? ", sağda tribünün tamamı — sürükleyerek döndürün, tekerlekle yakınlaştırın" : ""}.
        {cloud && cloud.stride > 1 && show.venue &&
          ` ${totalSeats(show.venue).toLocaleString("tr-TR")} koltuktan her ${cloud.stride}. çizildi.`}
        {!cloud && " Mekân planı eklerseniz tribünün 3B görünümü de burada oynar."}
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
      <div style={{ display: "flex", gap: 14, flexWrap: "wrap", alignItems: "stretch" }}>
        <div style={{ flex: "0 0 120px" }}>
          <div
            style={{
              width: 120,
              height: 224,
              borderRadius: 18,
              border: "4px solid #1f2733",
              background: phoneColor || "#000",
              transition: "background 80ms linear",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              padding: 10,
            }}
          >
            {phoneLyric && (
              <span
                style={{
                  color: "#fff",
                  fontWeight: 800,
                  fontSize: 12,
                  textAlign: "center",
                  textShadow: "0 0 6px rgba(0,0,0,.9)",
                }}
              >
                {phoneLyric}
              </span>
            )}
          </div>
          <p className="muted" style={{ textAlign: "center", margin: "6px 0 0" }}>telefon</p>
        </div>
        {cloud && (
          <div style={{ flex: "1 1 320px", minWidth: 0 }}>
            <SeatCloudCanvas
              points={cloud.points}
              timeMs={timeMs}
              colorFor={colorFor}
              scene={scene}
              height={300}
            />
          </div>
        )}
      </div>
      <p className="muted">
        Fener şeridi ve zamanlanmış ses önizlemede yok; ekran renk
        koreografisi telefonla aynı aritmetikle hesaplanır.
      </p>
    </div>
  );
}
