"use client";

import { useEffect, useRef, useState } from "react";
import type { EditorLandmark, SeatPoint } from "@/lib/venueEditor";

// Ortak 3B koltuk bulutu tuvali: gösteri editöründeki önizleme (F4.4) ve
// canlı konsol takibi (F4.5) aynı çizimi kullanır. Her koltuk bir nokta;
// rengi colorFor verir (telefonla aynı aritmetik — lib/effectEval).
// scene, izleyenin yön bulmasını sağlar: saha/sahne zemini, blok kontürleri
// ve adları, kuzey oku. Kamera ortografiktir: sürükleyince döner, tekerlek
// yakınlaştırır. Zemin bilinçli koyu: koreografi renkleri karanlıkta okunur.

const OFF_COLOR = "#242b38"; // karanlık koltuk (mekân yine seçilsin)

export type CloudScene = {
  landmark: EditorLandmark | null;
  blocks: { label: string; corners: [number, number][]; z: number }[];
};

export default function SeatCloudCanvas({
  points,
  timeMs,
  colorFor,
  scene,
  height = 360,
}: {
  points: SeatPoint[];
  timeMs: number;
  colorFor: (p: SeatPoint, timeMs: number) => string;
  scene?: CloudScene;
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

    const proj = (x: number, y: number, z: number): [number, number] => {
      const x1 = (x - cx0) * cy - (y - cy0) * sy;
      const y1 = (x - cx0) * sy + (y - cy0) * cy;
      return [x1, -(y1 * sp + (z - cz0) * cp)];
    };

    // Sığdırma ölçeği izdüşümden; sahne/saha köşeleri de hesaba katılır
    // (salonda sahne koltukların dışındadır, kadraj dışına düşmesin).
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    const fit = (x: number, y: number) => {
      minX = Math.min(minX, x); maxX = Math.max(maxX, x);
      minY = Math.min(minY, y); maxY = Math.max(maxY, y);
    };
    for (let i = 0; i < points.length; i += Math.max(1, Math.floor(points.length / 500))) {
      const p = points[i];
      const [x, y] = proj(p.x, p.y, p.z);
      fit(x, y);
    }
    if (scene?.landmark) {
      const lm = scene.landmark;
      for (const [dx, dy] of [[-1, -1], [1, -1], [1, 1], [-1, 1]] as const) {
        const [x, y] = proj(lm.x + (dx * lm.w) / 2, lm.y + (dy * lm.d) / 2, 0);
        fit(x, y);
      }
    }
    const scale =
      Math.min(
        (cssW - 40) / Math.max(1e-6, maxX - minX),
        (cssH - 40) / Math.max(1e-6, maxY - minY),
      ) * view.zoom;
    const ox = cssW / 2 - ((minX + maxX) / 2) * scale;
    const oy = cssH / 2 - ((minY + maxY) / 2) * scale;
    const P = (x: number, y: number, z: number): [number, number] => {
      const [px, py] = proj(x, y, z);
      return [ox + px * scale, oy + py * scale];
    };

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = "#0b0e13";
    ctx.fillRect(0, 0, cssW, cssH);

    // --- zemin bağlamı: saha/sahne + blok kontürleri ---
    if (scene?.landmark) {
      const lm = scene.landmark;
      const cs: [number, number][] = [
        P(lm.x - lm.w / 2, lm.y - lm.d / 2, 0),
        P(lm.x + lm.w / 2, lm.y - lm.d / 2, 0),
        P(lm.x + lm.w / 2, lm.y + lm.d / 2, 0),
        P(lm.x - lm.w / 2, lm.y + lm.d / 2, 0),
      ];
      ctx.beginPath();
      ctx.moveTo(cs[0][0], cs[0][1]);
      for (const [x, y] of cs.slice(1)) ctx.lineTo(x, y);
      ctx.closePath();
      ctx.fillStyle = lm.kind === "pitch" ? "#123c1f" : "#333a48";
      ctx.strokeStyle = lm.kind === "pitch" ? "#2e9e4f" : "#8b93a7";
      ctx.lineWidth = 1;
      ctx.fill();
      ctx.stroke();
      if (lm.kind === "pitch") {
        // Orta çizgi + orta yuvarlak (24-gen; izdüşümde elips olur).
        const [t0x, t0y] = P(lm.x, lm.y - lm.d / 2, 0);
        const [t1x, t1y] = P(lm.x, lm.y + lm.d / 2, 0);
        ctx.beginPath();
        ctx.moveTo(t0x, t0y);
        ctx.lineTo(t1x, t1y);
        ctx.stroke();
        const r = Math.min(lm.w, lm.d) * 0.135;
        ctx.beginPath();
        for (let i = 0; i <= 24; i++) {
          const a = (i / 24) * Math.PI * 2;
          const [x, y] = P(lm.x + Math.cos(a) * r, lm.y + Math.sin(a) * r, 0);
          if (i === 0) ctx.moveTo(x, y);
          else ctx.lineTo(x, y);
        }
        ctx.stroke();
      } else {
        const [tx, ty] = P(lm.x, lm.y, 0);
        ctx.fillStyle = "#cdd4e2";
        ctx.font = "600 11px system-ui";
        ctx.textAlign = "center";
        ctx.fillText("SAHNE", tx, ty + 4);
      }
    }
    for (const b of scene?.blocks ?? []) {
      if (b.corners.length < 3) continue;
      ctx.beginPath();
      const [sx, syy] = P(b.corners[0][0], b.corners[0][1], b.z);
      ctx.moveTo(sx, syy);
      for (const [x, y] of b.corners.slice(1)) {
        const [px, py] = P(x, y, b.z);
        ctx.lineTo(px, py);
      }
      ctx.closePath();
      ctx.strokeStyle = "rgba(120,140,180,0.45)";
      ctx.lineWidth = 1;
      ctx.stroke();
      let lx = 0, ly = 0;
      for (const [x, y] of b.corners) { lx += x; ly += y; }
      const [tx, ty] = P(lx / b.corners.length, ly / b.corners.length, b.z);
      ctx.fillStyle = "rgba(160,175,205,0.8)";
      ctx.font = "600 10px system-ui";
      ctx.textAlign = "center";
      ctx.fillText(b.label, tx, ty);
    }

    // --- koltuklar ---
    const dot = Math.max(1.5, Math.min(4, scale * 0.45));
    for (const p of points) {
      ctx.fillStyle = colorFor(p, timeMs) || OFF_COLOR;
      const [x, y] = P(p.x, p.y, p.z);
      ctx.fillRect(x - dot / 2, y - dot / 2, dot, dot);
    }

    // --- kuzey oku (dünya +y yönü) ---
    {
      const [c0x, c0y] = proj(cx0, cy0, cz0);
      const [n1x, n1y] = proj(cx0, cy0 + 1, cz0);
      let dx = n1x - c0x, dy = n1y - c0y;
      const len = Math.hypot(dx, dy) || 1;
      dx /= len; dy /= len;
      const bx = cssW - 30, by = 32;
      ctx.strokeStyle = "#8fa3c8";
      ctx.fillStyle = "#8fa3c8";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(bx - dx * 12, by - dy * 12);
      ctx.lineTo(bx + dx * 12, by + dy * 12);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(bx + dx * 16, by + dy * 16);
      ctx.lineTo(bx + dx * 8 - dy * 4, by + dy * 8 + dx * 4);
      ctx.lineTo(bx + dx * 8 + dy * 4, by + dy * 8 - dx * 4);
      ctx.closePath();
      ctx.fill();
      ctx.font = "700 10px system-ui";
      ctx.textAlign = "center";
      ctx.fillText("K", bx + dx * 24, by + dy * 24 + 3);
    }
  }, [points, timeMs, view, colorFor, height, scene]);

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
