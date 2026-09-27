"use client";

import { useEffect, useRef, useState } from "react";
import {
  imageToBitmap,
  MAX_BITMAP_COLS,
  MAX_BITMAP_ROWS,
  MIN_SCROLL_MS_PER_COL,
  textToBitmap,
} from "@/lib/bitmapGen";
import type { EditorBitmap, EditorScreenStep } from "@/lib/manifestEditor";

// Bayrak/slogan üretici paneli (F4.3b): bitmap efektli ekran adımının
// desen kaynağı. Metin tarayıcı fontuyla rasterleştirilir (kayan slogan),
// görüntü küçültülüp ≤16 renge nicemlenir (bayrak). Desen tribüne koltuk
// çözünürlüğünde çizilir — küçük ve yüksek kontrastlı desen en iyisidir.

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

export default function BitmapEffectPanel({
  step,
  onPatch,
}: {
  step: EditorScreenStep;
  onPatch: (patch: Partial<EditorScreenStep>) => void;
}) {
  const [text, setText] = useState("");
  const [textColor, setTextColor] = useState("#ffffff");
  const [imgCols, setImgCols] = useState(48);
  const [imgRows, setImgRows] = useState(16);
  const [err, setErr] = useState("");

  const bm = step.effectBitmap;
  const cols = bm && bm.rows.length > 0 ? bm.rows[0].length : 0;
  const minScroll = cols * MIN_SCROLL_MS_PER_COL;

  function makeText() {
    setErr("");
    const r = textToBitmap(text, textColor);
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
      const r = imageToBitmap(img, imgCols, imgRows);
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
          <label>Görüntüden (bayrak/logo)</label>
          <input type="file" accept="image/*" onChange={onPickImage} />
        </div>
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
            {" — "}desen tribüne koltuk çözünürlüğünde çizilir; blok grid&apos;inden
            kabaysa sadeleştirin.
          </p>
        </>
      ) : (
        <p className="muted">Henüz desen yok — slogan üretin ya da görüntü seçin; üretmeden yayınlarsanız bu adım düz renk oynar.</p>
      )}
    </div>
  );
}
