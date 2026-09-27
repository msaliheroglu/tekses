"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { screenColorAt } from "@/lib/effectEval";
import { fmtTime, toManifest, type EditorShow } from "@/lib/manifestEditor";
import { seatPoints, totalSeats } from "@/lib/venueEditor";

// 3B mekân önizlemesi (F4.4): moderatör, koreografiyi tribüne göstermeden
// önce görür. Her koltuk bir nokta; rengi, telefonun o koltukta o an
// basacağı ekran rengidir (lib/effectEval — telefonla aynı aritmetik).
// Kamera ortografiktir: sürükleyince döner, tekerlek yakınlaştırır.
// Bağımlılıksız canvas çizimi — three.js'e gerek yok, nokta bulutu yeter.

const MAX_DRAWN_SEATS = 20000;
const OFF_COLOR = "#242b38"; // karanlık koltuk (mekân yine seçilsin)

export default function VenuePreview3D({ show }: { show: EditorShow }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [seqIdx, setSeqIdx] = useState(0);
  const [timeMs, setTimeMs] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [view, setView] = useState({ yawDeg: -35, pitchDeg: 55, zoom: 1 });
  const dragRef = useRef<{ x: number; y: number } | null>(null);

  // Manifest ve koltuk bulutu yalnız gösteri değişince yeniden kurulur.
  const manifest = useMemo(() => toManifest(show), [show]);
  const cloud = useMemo(
    () => (show.venue ? seatPoints(show.venue, MAX_DRAWN_SEATS) : null),
    [show.venue],
  );

  const seq = manifest.sequences[Math.min(seqIdx, manifest.sequences.length - 1)];
  const durationMs = seq?.duration_ms ?? 0;

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

  // Çizim: zaman, görünüm ya da içerik değişince kare yenilenir.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !cloud || !seq) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    const cssW = canvas.clientWidth || 640;
    const cssH = 360;
    if (canvas.width !== Math.round(cssW * dpr)) canvas.width = Math.round(cssW * dpr);
    if (canvas.height !== Math.round(cssH * dpr)) canvas.height = Math.round(cssH * dpr);

    const yaw = (view.yawDeg * Math.PI) / 180;
    const pitch = (view.pitchDeg * Math.PI) / 180;
    const cy = Math.cos(yaw), sy = Math.sin(yaw);
    const cp = Math.cos(pitch), sp = Math.sin(pitch);

    // Mekân merkezi etrafında döndür; ortografik izdüşüm:
    // ekranX = x', ekranY = −(y'·sinPitch + z·cosPitch).
    const pts = cloud.points;
    let cx0 = 0, cy0 = 0, cz0 = 0;
    for (const p of pts) { cx0 += p.x; cy0 += p.y; cz0 += p.z; }
    cx0 /= pts.length; cy0 /= pts.length; cz0 /= pts.length;

    const proj = (p: { x: number; y: number; z: number }): [number, number] => {
      const x1 = (p.x - cx0) * cy - (p.y - cy0) * sy;
      const y1 = (p.x - cx0) * sy + (p.y - cy0) * cy;
      return [x1, -(y1 * sp + (p.z - cz0) * cp)];
    };

    // Sığdırma ölçeği izdüşümden hesaplanır (görünüm değişince de doğru kalır).
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < pts.length; i += Math.max(1, Math.floor(pts.length / 500))) {
      const [x, y] = proj(pts[i]);
      minX = Math.min(minX, x); maxX = Math.max(maxX, x);
      minY = Math.min(minY, y); maxY = Math.max(maxY, y);
    }
    const scale =
      Math.min(
        (cssW - 40) / Math.max(1e-6, maxX - minX),
        (cssH - 40) / Math.max(1e-6, maxY - minY),
      ) * view.zoom;
    const ox = cssW / 2 - ((minX + maxX) / 2) * scale;
    const oy = cssH / 2 - ((minY + maxY) / 2) * scale;

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = "#0b0e13";
    ctx.fillRect(0, 0, cssW, cssH);

    const dot = Math.max(1.5, Math.min(4, scale * 0.45));
    for (const p of pts) {
      const c = screenColorAt(seq, timeMs, p.u, p.v, p.w);
      ctx.fillStyle = c || OFF_COLOR;
      const [x, y] = proj(p);
      ctx.fillRect(ox + x * scale - dot / 2, oy + y * scale - dot / 2, dot, dot);
    }
  }, [cloud, seq, timeMs, view]);

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
      <canvas
        ref={canvasRef}
        style={{ width: "100%", height: 360, borderRadius: 8, touchAction: "none", cursor: "grab" }}
        onPointerDown={(e) => {
          dragRef.current = { x: e.clientX, y: e.clientY };
          (e.target as Element).setPointerCapture?.(e.pointerId);
        }}
        onPointerMove={(e) => {
          const d = dragRef.current;
          if (!d) return;
          const dx = e.clientX - d.x;
          const dy = e.clientY - d.y;
          dragRef.current = { x: e.clientX, y: e.clientY };
          setView((vw) => ({
            ...vw,
            yawDeg: vw.yawDeg + dx * 0.4,
            pitchDeg: Math.min(89, Math.max(5, vw.pitchDeg + dy * 0.4)),
          }));
        }}
        onPointerUp={() => {
          dragRef.current = null;
        }}
        onWheel={(e) => {
          setView((vw) => ({
            ...vw,
            zoom: Math.min(6, Math.max(0.3, vw.zoom * (e.deltaY < 0 ? 1.15 : 1 / 1.15))),
          }));
        }}
      />
      <p className="muted">
        Fener şeridi ve sözler önizlemede yok; ekran renk koreografisi
        telefonla aynı aritmetikle hesaplanır.
      </p>
    </div>
  );
}
