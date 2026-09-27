"use client";

import { useEffect, useRef, useState } from "react";
import type { SeatPoint } from "@/lib/venueEditor";

// Ortak 3B koltuk bulutu tuvali: gösteri editöründeki önizleme (F4.4) ve
// canlı konsol takibi (F4.5) aynı çizimi kullanır. Her koltuk bir nokta;
// rengi colorFor verir (telefonla aynı aritmetik — lib/effectEval).
// Kamera ortografiktir: sürükleyince döner, tekerlek yakınlaştırır.
// Zemin bilinçli koyu: koreografi renkleri karanlıkta okunur.

const OFF_COLOR = "#242b38"; // karanlık koltuk (mekân yine seçilsin)

export default function SeatCloudCanvas({
  points,
  timeMs,
  colorFor,
  height = 360,
}: {
  points: SeatPoint[];
  timeMs: number;
  colorFor: (p: SeatPoint, timeMs: number) => string;
  height?: number;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [view, setView] = useState({ yawDeg: -35, pitchDeg: 55, zoom: 1 });
  const dragRef = useRef<{ x: number; y: number } | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || points.length === 0) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    const cssW = canvas.clientWidth || 640;
    const cssH = height;
    if (canvas.width !== Math.round(cssW * dpr)) canvas.width = Math.round(cssW * dpr);
    if (canvas.height !== Math.round(cssH * dpr)) canvas.height = Math.round(cssH * dpr);

    const yaw = (view.yawDeg * Math.PI) / 180;
    const pitch = (view.pitchDeg * Math.PI) / 180;
    const cy = Math.cos(yaw), sy = Math.sin(yaw);
    const cp = Math.cos(pitch), sp = Math.sin(pitch);

    // Mekân merkezi etrafında döndür; ortografik izdüşüm:
    // ekranX = x', ekranY = −(y'·sinPitch + z·cosPitch).
    let cx0 = 0, cy0 = 0, cz0 = 0;
    for (const p of points) { cx0 += p.x; cy0 += p.y; cz0 += p.z; }
    cx0 /= points.length; cy0 /= points.length; cz0 /= points.length;

    const proj = (p: { x: number; y: number; z: number }): [number, number] => {
      const x1 = (p.x - cx0) * cy - (p.y - cy0) * sy;
      const y1 = (p.x - cx0) * sy + (p.y - cy0) * cy;
      return [x1, -(y1 * sp + (p.z - cz0) * cp)];
    };

    // Sığdırma ölçeği izdüşümden hesaplanır (görünüm değişince de doğru kalır).
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < points.length; i += Math.max(1, Math.floor(points.length / 500))) {
      const [x, y] = proj(points[i]);
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
    for (const p of points) {
      ctx.fillStyle = colorFor(p, timeMs) || OFF_COLOR;
      const [x, y] = proj(p);
      ctx.fillRect(ox + x * scale - dot / 2, oy + y * scale - dot / 2, dot, dot);
    }
  }, [points, timeMs, view, colorFor, height]);

  return (
    <canvas
      ref={canvasRef}
      style={{ width: "100%", height, borderRadius: 8, touchAction: "none", cursor: "grab" }}
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
  );
}
