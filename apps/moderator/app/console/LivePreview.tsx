"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  manifestCueDurationMs,
  manifestLyricAt,
  manifestScreenColorAt,
} from "@/lib/effectEval";
import { fmtTime, type ManifestJson } from "@/lib/manifestEditor";
import { blockCorners, fromManifestVenue, seatPoints, type SeatPoint } from "@/lib/venueEditor";
import SeatCloudCanvas, { type CloudScene } from "../components/SeatCloudCanvas";

// Canlı konsol önizlemesi (F4.5): GO'dan sonra moderatör koşuyu buradan
// izler — telefon maketi ekran rengini + söz satırını, mekân planı varsa
// 3B bulut tribünün o anki görüntüsünü verir. Zamanlama, GO isteğinin
// yanıtlandığı ana gecikme eklenerek yereldir: sunucu ateşlemesiyle ufak
// (≤ birkaç yüz ms) fark olabilir; bu bir TAKİP görünümüdür, senkron
// kaynağı değildir.

export type PreviewRun = {
  // Ne oynuyor: manifest cue_id ("program" dahil) ya da Faz 0 flash.
  cueId: string; // FLASH ise "__flash__"
  fireAtLocal: number; // Date.now() tabanı
  flash?: { color: string; flashHz: number; durationMs: number };
  runId: string;
};

const FLASH = "__flash__";
const MAX_DRAWN_SEATS = 12000; // konsol saniyede ~30 kare boyar; bütçe daha sıkı

export default function LivePreview({
  manifest,
  run,
  onDismiss,
}: {
  manifest: ManifestJson | null;
  run: PreviewRun;
  onDismiss: () => void;
}) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    let raf = 0;
    const tick = () => {
      setNow(Date.now());
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, []);

  const elapsed = now - run.fireAtLocal;
  const durationMs = run.flash
    ? run.flash.durationMs
    : manifest
      ? manifestCueDurationMs(manifest, run.cueId)
      : 0;
  const done = durationMs > 0 && elapsed >= durationMs;

  // Telefon maketinin rengi/sözü (mekân ortası koltuğu: u=v=w=0.5).
  let color = "";
  let lyric = "";
  if (elapsed >= 0 && !done) {
    if (run.flash) {
      const lit =
        run.flash.flashHz === 0 ||
        Math.floor((elapsed * run.flash.flashHz) / 500) % 2 === 0;
      color = lit ? run.flash.color : "";
    } else if (manifest) {
      color = manifestScreenColorAt(manifest, run.cueId, elapsed, 0.5, 0.5, 0.5);
      lyric = manifestLyricAt(manifest, run.cueId, elapsed);
    }
  }

  // Mekân planı varsa 3B bulut + sahne bağlamı (plan editör kalıbına
  // uymuyorsa maketle yetinilir).
  const venueView = useMemo(() => {
    if (!manifest?.venue || run.cueId === FLASH) return null;
    const r = fromManifestVenue(manifest.venue);
    if (!r.venue) return null;
    const scene: CloudScene = {
      landmark: r.venue.landmark,
      blocks: r.venue.blocks.map((b) => ({ label: b.id, corners: blockCorners(b), z: b.z })),
    };
    return { cloud: seatPoints(r.venue, MAX_DRAWN_SEATS), scene };
  }, [manifest, run.cueId]);

  const colorFor = useCallback(
    (p: SeatPoint, t: number) =>
      manifest ? manifestScreenColorAt(manifest, run.cueId, t, p.u, p.v, p.w) : "",
    [manifest, run.cueId],
  );

  return (
    <div className="card">
      <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
        <h2 style={{ margin: 0 }}>Önizleme — şu an oynayan</h2>
        <span className="muted">run {run.runId.slice(0, 8)}</span>
        <span style={{ flex: 1 }} />
        <button type="button" className="ghost" style={{ margin: 0, padding: "6px 12px" }} onClick={onDismiss}>
          kapat
        </button>
      </div>

      {elapsed < 0 ? (
        <p style={{ fontSize: 20, fontWeight: 700, margin: "12px 0" }}>
          Ateşlemeye {Math.ceil(-elapsed / 1000)} sn…
        </p>
      ) : done ? (
        <p className="ok" style={{ margin: "12px 0" }}>Koşu tamamlandı.</p>
      ) : (
        <p className="muted" style={{ margin: "8px 0 0" }}>
          {fmtTime(Math.max(0, elapsed))}
          {durationMs > 0 && <> / {fmtTime(durationMs)}</>} — takip görünümü;
          telefonlar kendi senkron saatinden oynar.
        </p>
      )}

      {durationMs > 0 && elapsed >= 0 && !done && (
        <div style={{ height: 6, borderRadius: 3, background: "#e2e8f1", overflow: "hidden", margin: "8px 0 12px" }}>
          <div
            style={{
              width: `${Math.min(100, (elapsed / durationMs) * 100)}%`,
              height: "100%",
              background: "var(--accent)",
            }}
          />
        </div>
      )}

      <div style={{ display: "flex", gap: 14, flexWrap: "wrap", alignItems: "stretch" }}>
        {/* Telefon maketi: mekânın ortasındaki koltuğun ekranı. */}
        <div style={{ flex: "0 0 130px" }}>
          <div
            style={{
              width: 130,
              height: 240,
              borderRadius: 18,
              border: "4px solid #1f2733",
              background: color || "#000",
              transition: "background 80ms linear",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              padding: 10,
            }}
          >
            {lyric && (
              <span
                style={{
                  color: "#fff",
                  fontWeight: 800,
                  fontSize: 13,
                  textAlign: "center",
                  textShadow: "0 0 6px rgba(0,0,0,.9)",
                }}
              >
                {lyric}
              </span>
            )}
          </div>
          <p className="muted" style={{ textAlign: "center", margin: "6px 0 0" }}>
            telefon (orta koltuk)
          </p>
        </div>
        {venueView && (
          <div style={{ flex: "1 1 320px", minWidth: 0 }}>
            <SeatCloudCanvas
              points={venueView.cloud.points}
              timeMs={Math.max(0, elapsed)}
              colorFor={colorFor}
              scene={venueView.scene}
              height={252}
            />
            <p className="muted" style={{ margin: "6px 0 0" }}>
              tribün görünümü — sürükleyip döndürün
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
