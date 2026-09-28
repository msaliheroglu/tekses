"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  imageToBitmap,
  MAX_BITMAP_COLS,
  MAX_BITMAP_ROWS,
  MIN_SCROLL_MS_PER_COL,
  textToBitmap,
} from "@/lib/bitmapGen";
import type { EditorArea, EditorBitmap, EditorScreenStep } from "@/lib/manifestEditor";
import { blockPlacement, seatPoints, venueBounds, type EditorVenue } from "@/lib/venueEditor";

// Bayrak/slogan paneli (F4.3b + F4.5-3.tur): desen üretimi, tribün
// çözünürlüğüne otomatik uydurma ve fareyle YERLEŞTİRME. Yerleştirme
// tuvali tribün duvarını düzleştirir (yatay = efekt ekseni, dikey = w):
// gri noktalar koltuklar, renkli dikdörtgen desenin oynayacağı penceredir —
// içinden sürükleyerek taşınır, sağ alt köşesinden boyutlanır.

function BitmapPreview({ bitmap }: { bitmap: EditorBitmap }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const rows = bitmap.rows.length;
    const cols = rows > 0 ? bitmap.rows[0].length : 0;
    if (!cols) return;
    canvas.width = cols;
    canvas.height = rows;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, cols, rows);
    for (let y = 0; y < rows; y++) {
      for (let x = 0; x < cols; x++) {
        const ch = bitmap.rows[y][x];
        if (ch === "." || ch === undefined) continue;
        const idx = parseInt(ch, 16);
        ctx.fillStyle = bitmap.palette[idx] ?? "#000000";
        ctx.fillRect(x, y, 1, 1);
      }
    }
  }, [bitmap]);
  return (
    <canvas
      ref={ref}
      style={{
        width: "100%",
        maxWidth: 520,
        imageRendering: "pixelated",
        background:
          "repeating-conic-gradient(#dbe2ec 0% 25%, #eef2f8 0% 50%) 0 0 / 12px 12px",
        borderRadius: 6,
        display: "block",
      }}
    />
  );
}

// Yerleştirme tuvali: koltuklar (yatayda efekt ekseni payı, dikeyde w) +
// sürüklenebilir pencere. Fare desenin tribünde tam nereye düşeceğini seçer.
function PlacementCanvas({
  venue,
  axis,
  reverse,
  area,
  bitmap,
  onChange,
}: {
  venue: EditorVenue;
  axis: string;
  reverse: boolean;
  area: EditorArea | null;
  bitmap: EditorBitmap | null;
  onChange: (area: EditorArea) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const drag = useRef<{ mode: "move" | "resize"; dx: number; dy: number } | null>(null);
  const H = 200;

  // Koltukların (yatay, dikey) izdüşümü — efekt ekseniyle aynı aritmetik.
  const dots = useMemo(() => {
    const pts = seatPoints(venue, 4000).points;
    return pts.map((p) => {
      let h = p.u;
      if (axis === "v") h = p.v;
      if (axis === "w") h = p.w;
      if (axis === "ring") {
        h = Math.atan2(p.v - 0.5, p.u - 0.5) / (2 * Math.PI);
        if (h < 0) h += 1;
      }
      if (reverse) h = 1 - h;
      return { h, w: p.w };
    });
  }, [venue, axis, reverse]);

  const a: EditorArea = area ?? { u0: 0, w0: 0, u1: 1, w1: 1 };

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const dpr = window.devicePixelRatio || 1;
    const W = canvas.clientWidth || 520;
    if (canvas.width !== Math.round(W * dpr)) canvas.width = Math.round(W * dpr);
    if (canvas.height !== Math.round(H * dpr)) canvas.height = Math.round(H * dpr);
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = "#0b0e13";
    ctx.fillRect(0, 0, W, H);
    // Koltuklar.
    ctx.fillStyle = "#3a4557";
    for (const d of dots) {
      ctx.fillRect(d.h * W - 1, (1 - d.w) * H - 1, 2, 2);
    }
    // Pencere + desen hayaleti.
    const rx = a.u0 * W, ry = (1 - a.w1) * H;
    const rw = (a.u1 - a.u0) * W, rh = (a.w1 - a.w0) * H;
    if (bitmap && bitmap.rows.length > 0 && bitmap.rows[0].length > 0) {
      const cols = bitmap.rows[0].length, rows = bitmap.rows.length;
      for (let y = 0; y < rows; y++) {
        for (let x = 0; x < cols; x++) {
          const ch = bitmap.rows[y][x];
          if (ch === ".") continue;
          const idx = parseInt(ch, 16);
          ctx.fillStyle = (bitmap.palette[idx] ?? "#000") + "b0"; // yarı saydam
          ctx.fillRect(rx + (x / cols) * rw, ry + (y / rows) * rh, rw / cols + 0.5, rh / rows + 0.5);
        }
      }
    }
    ctx.strokeStyle = "#ffb74d";
    ctx.lineWidth = 2;
    ctx.strokeRect(rx, ry, rw, rh);
    // Boyutlandırma tutamacı (sağ alt).
    ctx.fillStyle = "#ffb74d";
    ctx.fillRect(rx + rw - 6, ry + rh - 6, 12, 12);
  }, [dots, a, bitmap, H]);

  function toNorm(e: React.PointerEvent): { x: number; y: number } {
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    return {
      x: Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width)),
      y: Math.min(1, Math.max(0, (e.clientY - rect.top) / rect.height)),
    };
  }

  return (
    <canvas
      ref={canvasRef}
      style={{ width: "100%", maxWidth: 520, height: H, borderRadius: 6, touchAction: "none", cursor: "move", display: "block" }}
      onPointerDown={(e) => {
        const { x, y } = toNorm(e);
        const nearCorner =
          Math.abs(x - a.u1) < 0.05 && Math.abs(y - (1 - a.w0)) < 0.08;
        drag.current = nearCorner
          ? { mode: "resize", dx: 0, dy: 0 }
          : { mode: "move", dx: x - a.u0, dy: y - (1 - a.w1) };
        (e.target as Element).setPointerCapture?.(e.pointerId);
      }}
      onPointerMove={(e) => {
        const d = drag.current;
        if (!d) return;
        const { x, y } = toNorm(e);
        const r2 = (v: number) => Math.round(v * 100) / 100;
        if (d.mode === "resize") {
          onChange({
            u0: a.u0,
            w1: a.w1,
            u1: r2(Math.max(a.u0 + 0.05, x)),
            w0: r2(Math.min(a.w1 - 0.05, 1 - y)),
          });
        } else {
          const uw = a.u1 - a.u0, wh = a.w1 - a.w0;
          const u0 = r2(Math.min(1 - uw, Math.max(0, x - d.dx)));
          const w1 = r2(Math.max(wh, Math.min(1, 1 - (y - d.dy))));
          onChange({ u0, u1: r2(u0 + uw), w1, w0: r2(w1 - wh) });
        }
      }}
      onPointerUp={() => {
        drag.current = null;
      }}
    />
  );
}

export default function BitmapEffectPanel({
  step,
  venue,
  onPatch,
}: {
  step: EditorScreenStep;
  venue: EditorVenue | null;
  onPatch: (patch: Partial<EditorScreenStep>) => void;
}) {
  const [text, setText] = useState("");
  const [textColor, setTextColor] = useState("#ffffff");
  const [autoRes, setAutoRes] = useState(true);
  const [imgCols, setImgCols] = useState(48);
  const [imgRows, setImgRows] = useState(16);
  const [err, setErr] = useState("");

  const bm = step.effectBitmap;
  const cols = bm && bm.rows.length > 0 ? bm.rows[0].length : 0;
  const minScroll = cols * MIN_SCROLL_MS_PER_COL;

  // Adım tek bloğa hedefliyse yerleşim/çözünürlük/oran O bloğun grid'inden
  // gelir — bayrak tribüne bire bir oturur.
  const singlePl = useMemo(
    () =>
      venue && step.effectBlocks.length === 1
        ? blockPlacement(venue, step.effectBlocks[0])
        : null,
    [venue, step.effectBlocks],
  );

  // Hücre en-boy oranı (koltuk aralığı / sıra adımı): oran korumalı sığdırma
  // ve harflerin ön-germesi için. Blok yoksa mekân ortalaması.
  const cellAspect = useMemo(() => {
    if (singlePl) return singlePl.cellAspect;
    if (!venue || venue.blocks.length === 0) return 1;
    const n = venue.blocks.length;
    const sStep = venue.blocks.reduce((s, x) => s + (x.seatStep || 0.5), 0) / n;
    const rowH =
      venue.blocks.reduce((s, x) => s + Math.hypot(x.rowStep || 0.8, x.rake || 0), 0) / n;
    return rowH > 0 ? sStep / rowH : 1;
  }, [venue, singlePl]);

  // Tribün çözünürlüğü: pencere kaç koltuk kaplıyorsa o kadar piksel —
  // "bir koltuk bir piksel" hedefi (item 4: resim alana OTOMATİK uyar).
  const suggested = useMemo(() => {
    if (!venue || venue.blocks.length === 0) return null;
    const uSpan = step.effectArea ? step.effectArea.u1 - step.effectArea.u0 : 1;
    const wSpan = step.effectArea ? step.effectArea.w1 - step.effectArea.w0 : 1;
    if (singlePl) {
      // Pencere blok penceresinin ne kadarını kaplıyorsa piksel de o oranda.
      const puSpan = Math.max(0.01, singlePl.area.u1 - singlePl.area.u0);
      const pwSpan = Math.max(0.01, singlePl.area.w1 - singlePl.area.w0);
      return {
        cols: Math.min(MAX_BITMAP_COLS, Math.max(4, Math.round((singlePl.cols * uSpan) / puSpan))),
        rows: Math.min(MAX_BITMAP_ROWS, Math.max(2, Math.round((singlePl.rows * wSpan) / pwSpan))),
      };
    }
    const b = venueBounds(venue);
    const widthM = Math.max(1, b.maxX - b.minX);
    const maxRows = Math.max(...venue.blocks.map((x) => x.rows));
    const meanStep =
      venue.blocks.reduce((s, x) => s + (x.seatStep || 0.5), 0) / venue.blocks.length || 0.5;
    return {
      cols: Math.min(MAX_BITMAP_COLS, Math.max(4, Math.round((widthM * uSpan) / meanStep))),
      rows: Math.min(MAX_BITMAP_ROWS, Math.max(2, Math.round(maxRows * wSpan))),
    };
  }, [venue, step.effectArea, singlePl]);

  const effCols = venue && autoRes && suggested ? suggested.cols : imgCols;
  const effRows = venue && autoRes && suggested ? suggested.rows : imgRows;

  // Tek tık tribün yerleşimi: eksen/yön/blok filtresi/pencere bloğa kurulur.
  function placeOnBlock(blockId: string) {
    if (!venue) return;
    const pl = blockPlacement(venue, blockId);
    if (!pl) return;
    onPatch({
      effectAxis: pl.axis,
      effectReverse: pl.reverse,
      effectBlocks: [blockId],
      effectArea: { ...pl.area },
    });
  }

  function makeText() {
    setErr("");
    // Harf oranı tribün hücresinde korunur (yatay ön-germe).
    const r = textToBitmap(text, textColor, 9, cellAspect > 0 ? 1 / cellAspect : 1);
    if (!r.bitmap) {
      setErr(r.error ?? "üretilemedi");
      return;
    }
    const c = r.bitmap.rows[0].length;
    // Slogan varsayılan kayar: tur süresi güvenlik sınırının biraz üstünde.
    onPatch({ effectBitmap: r.bitmap, effectScrollMs: c * 400 });
  }

  function onPickImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setErr("");
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      const r = imageToBitmap(img, effCols, effRows, cellAspect);
      if (!r.bitmap) {
        setErr(r.error ?? "üretilemedi");
        return;
      }
      onPatch({ effectBitmap: r.bitmap, effectScrollMs: 0 }); // bayrak sabit durur
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      setErr("görüntü açılamadı");
    };
    img.src = url;
  }

  return (
    <div style={{ margin: "4px 0 12px", padding: "8px 12px", borderLeft: "3px solid var(--border)" }}>
      {venue && venue.blocks.length > 0 && (
        <div style={{ marginBottom: 8 }}>
          <label>
            Nereye? — tıklanan tribüne yerleşir (yön, pencere ve çözünürlük
            otomatik kurulur; desen ötekilerde oynamaz)
          </label>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {venue.blocks.map((b) => {
              const active = step.effectBlocks.length === 1 && step.effectBlocks[0] === b.id;
              return (
                <button
                  key={b.id}
                  type="button"
                  className="secondary"
                  style={{
                    padding: "4px 10px",
                    ...(active ? { outline: "2px solid var(--accent)" } : {}),
                  }}
                  onClick={() => placeOnBlock(b.id)}
                >
                  {b.id}
                </button>
              );
            })}
            <button
              type="button"
              className="secondary"
              style={{
                padding: "4px 10px",
                ...(step.effectBlocks.length === 0 && !step.effectArea
                  ? { outline: "2px solid var(--accent)" }
                  : {}),
              }}
              onClick={() => onPatch({ effectBlocks: [], effectArea: null })}
            >
              tüm mekân
            </button>
          </div>
        </div>
      )}
      <div className="row">
        <div style={{ flex: "2 1 220px" }}>
          <label>Slogan metni</label>
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="ör. ŞAMPİYON"
          />
        </div>
        <div style={{ flex: "0 0 70px" }}>
          <label>Yazı rengi</label>
          <input
            type="color"
            value={textColor}
            style={{ padding: 2, height: 42 }}
            onChange={(e) => setTextColor(e.target.value)}
          />
        </div>
        <div style={{ flex: "0 0 auto", alignSelf: "flex-end" }}>
          <button type="button" className="secondary" onClick={makeText}>
            Slogan üret
          </button>
        </div>
      </div>
      <div className="row">
        <div>
          <label>Görüntüden (bayrak/logo; oran korunur)</label>
          <input type="file" accept="image/*" onChange={onPickImage} />
        </div>
        {venue && (
          <div>
            <label>Çözünürlük</label>
            <select value={autoRes ? "auto" : "manual"} onChange={(e) => setAutoRes(e.target.value === "auto")}>
              <option value="auto">
                otomatik — tribüne uydur{suggested ? ` (${suggested.cols}×${suggested.rows})` : ""}
              </option>
              <option value="manual">elle</option>
            </select>
          </div>
        )}
        {(!venue || !autoRes) && (
          <>
            <div style={{ flex: "0 1 90px" }}>
              <label>Sütun</label>
              <input
                type="number"
                min={1}
                max={MAX_BITMAP_COLS}
                value={imgCols}
                onChange={(e) =>
                  setImgCols(Math.min(MAX_BITMAP_COLS, Math.max(1, Number(e.target.value) || 1)))
                }
              />
            </div>
            <div style={{ flex: "0 1 90px" }}>
              <label>Satır</label>
              <input
                type="number"
                min={1}
                max={MAX_BITMAP_ROWS}
                value={imgRows}
                onChange={(e) =>
                  setImgRows(Math.min(MAX_BITMAP_ROWS, Math.max(1, Number(e.target.value) || 1)))
                }
              />
            </div>
          </>
        )}
        <div style={{ flex: "0 1 150px" }}>
          <label>Kaydırma (sn; 0 = sabit)</label>
          <input
            type="number"
            min={0}
            step={1}
            value={Math.round(step.effectScrollMs / 1000)}
            disabled={!bm}
            onChange={(e) => {
              const sec = Math.max(0, Number(e.target.value) || 0);
              // Işık güvenliği: tur süresi sütun×334 ms'in altına inemez.
              const ms = sec === 0 ? 0 : Math.max(minScroll, sec * 1000);
              onPatch({ effectScrollMs: ms });
            }}
          />
        </div>
      </div>
      {err && <p className="err">{err}</p>}
      {bm ? (
        <>
          <BitmapPreview bitmap={bm} />
          <p className="muted">
            {cols}×{bm.rows.length} hücre · {bm.palette.length} renk
            {step.effectScrollMs > 0
              ? ` · tam tur ${Math.round(step.effectScrollMs / 1000)} sn (alt sınır ${Math.ceil(minScroll / 1000)} sn)`
              : " · sabit"}
          </p>
          {venue && venue.blocks.length > 0 && (
            <>
              <label>
                Yerleştirme — deseni tribünde fareyle konumlandırın (içinden
                sürükleyin, sağ alt köşeden boyutlandırın)
              </label>
              <PlacementCanvas
                venue={venue}
                axis={step.effectAxis}
                reverse={step.effectReverse}
                area={step.effectArea}
                bitmap={bm}
                onChange={(area) => onPatch({ effectArea: area })}
              />
              <p className="muted" style={{ margin: "4px 0 0" }}>
                {step.effectArea
                  ? `pencere: yatay %${Math.round(step.effectArea.u0 * 100)}–${Math.round(step.effectArea.u1 * 100)}, dikey %${Math.round(step.effectArea.w0 * 100)}–${Math.round(step.effectArea.w1 * 100)} · `
                  : "pencere: tüm mekân · "}
                <a
                  href="#"
                  onClick={(e) => {
                    e.preventDefault();
                    onPatch({ effectArea: null });
                  }}
                >
                  tüm mekâna yay
                </a>
              </p>
            </>
          )}
        </>
      ) : (
        <p className="muted">Henüz desen yok — slogan üretin ya da görüntü seçin; üretmeden yayınlarsanız bu adım düz renk oynar.</p>
      )}
    </div>
  );
}
