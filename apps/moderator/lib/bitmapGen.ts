// Bitmap efekt üretimi (F4.3b): metinden kayan slogan, görüntüden bayrak.
//
// Hedef biçim packages/manifest/effect.go Bitmap'idir: palet (≤16 #RRGGBB)
// + satır dizgileri (karakter = palet indeksi 0-9a-f, '.' = kapalı). Koltuk
// çözünürlüğü kaba olduğundan (blok grid'i) küçük ve yüksek kontrastlı
// desenler en iyi sonucu verir. Metin, tarayıcının kendi fontuyla canvas'a
// çizilip pikselleştirilir — Türkçe karakterler font tablosu olmadan gelir.

export type GenBitmap = { palette: string[]; rows: string[] };

export const MAX_BITMAP_COLS = 256; // packages/manifest maxBitmapCols
export const MAX_BITMAP_ROWS = 64;
const MAX_PALETTE = 16;

// Kaydırma ışık güvenliği: period_ms ≥ sütun × 334 (manifest doğrular).
export const MIN_SCROLL_MS_PER_COL = 334;

const hex2 = (v: number) => v.toString(16).padStart(2, "0").toUpperCase();
const toHex = (r: number, g: number, b: number) => `#${hex2(r)}${hex2(g)}${hex2(b)}`;

// --- median-cut nicemleme (saf; canvas'tan bağımsız test edilebilir) ---

// Girdi: [r,g,b] üçlüleri. Çıktı: en çok maxColors renk. Kova, en geniş
// kanalından medyanla ikiye bölünür; kova rengi ortalamadır.
export function medianCut(pixels: [number, number, number][], maxColors: number): [number, number, number][] {
  if (pixels.length === 0) return [];
  type Bucket = [number, number, number][];
  let buckets: Bucket[] = [pixels];
  while (buckets.length < maxColors) {
    // En geniş renk aralıklı kovayı bul.
    let bestIdx = -1, bestRange = 1, bestChan = 0;
    buckets.forEach((b, i) => {
      if (b.length < 2) return;
      for (let ch = 0; ch < 3; ch++) {
        let lo = 255, hi = 0;
        for (const p of b) {
          if (p[ch] < lo) lo = p[ch];
          if (p[ch] > hi) hi = p[ch];
        }
        if (hi - lo > bestRange) {
          bestRange = hi - lo;
          bestIdx = i;
          bestChan = ch;
        }
      }
    });
    if (bestIdx < 0) break; // her kova tek renk — bölünecek şey kalmadı
    const b = buckets[bestIdx];
    const sorted = [...b].sort((p, q) => p[bestChan] - q[bestChan]);
    const mid = Math.floor(sorted.length / 2);
    buckets.splice(bestIdx, 1, sorted.slice(0, mid), sorted.slice(mid));
  }
  return buckets.map((b) => {
    let r = 0, g = 0, bl = 0;
    for (const p of b) { r += p[0]; g += p[1]; bl += p[2]; }
    const n = b.length;
    return [Math.round(r / n), Math.round(g / n), Math.round(bl / n)];
  });
}

function nearestIdx(palette: [number, number, number][], r: number, g: number, b: number): number {
  let best = 0, bestD = Infinity;
  for (let i = 0; i < palette.length; i++) {
    const [pr, pg, pb] = palette[i];
    const d = (pr - r) ** 2 + (pg - g) ** 2 + (pb - b) ** 2;
    if (d < bestD) { bestD = d; best = i; }
  }
  return best;
}

// RGBA piksel verisinden bitmap: yarı saydam hücreler '.' (kapalı) olur.
export function rgbaToBitmap(data: Uint8ClampedArray, cols: number, rows: number): GenBitmap {
  const opaque: [number, number, number][] = [];
  for (let i = 0; i < cols * rows; i++) {
    if (data[i * 4 + 3] >= 128) opaque.push([data[i * 4], data[i * 4 + 1], data[i * 4 + 2]]);
  }
  const palette = medianCut(opaque, MAX_PALETTE);
  // Aynı #RRGGBB'ye yuvarlanan kovalar birleştirilir — palet yeri değerli
  // (≤16) ve tekrar eden renk desene hiçbir şey katmaz.
  const hexes: string[] = [];
  const remap = palette.map(([r, g, b]) => {
    const h = toHex(r, g, b);
    const at = hexes.indexOf(h);
    if (at >= 0) return at;
    hexes.push(h);
    return hexes.length - 1;
  });
  const out: string[] = [];
  for (let y = 0; y < rows; y++) {
    let row = "";
    for (let x = 0; x < cols; x++) {
      const i = (y * cols + x) * 4;
      if (data[i + 3] < 128 || palette.length === 0) {
        row += ".";
      } else {
        row += remap[nearestIdx(palette, data[i], data[i + 1], data[i + 2])].toString(16);
      }
    }
    out.push(row);
  }
  return { palette: hexes, rows: out };
}

// --- canvas tabanlı üreticiler (yalnız tarayıcıda) ---

// Metin → tek renkli bitmap. Tarayıcı fontu küçük boyda çizilir ve alfa
// eşiğiyle pikselleştirilir; kayan slogan için genişlik sütun sınırını
// aşarsa hata metni döner.
export function textToBitmap(
  text: string,
  color: string,
  rows = 9,
): { bitmap?: GenBitmap; error?: string } {
  const t = text.trim();
  if (!t) return { error: "metin boş" };
  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d");
  if (!ctx) return { error: "canvas desteklenmiyor" };
  const font = `bold ${rows}px "Arial Black", Arial, sans-serif`;
  ctx.font = font;
  const cols = Math.ceil(ctx.measureText(t).width) + 2;
  if (cols > MAX_BITMAP_COLS) {
    return { error: `metin çok uzun (${cols} sütun; sınır ${MAX_BITMAP_COLS}) — kısaltın` };
  }
  canvas.width = cols;
  canvas.height = rows;
  ctx.font = font; // boyut değişince font sıfırlanır
  ctx.textBaseline = "middle";
  ctx.fillStyle = "#ffffff";
  ctx.fillText(t, 1, rows / 2 + 1);
  const data = ctx.getImageData(0, 0, cols, rows).data;
  const out: string[] = [];
  let any = false;
  for (let y = 0; y < rows; y++) {
    let row = "";
    for (let x = 0; x < cols; x++) {
      const on = data[(y * cols + x) * 4 + 3] >= 128;
      if (on) any = true;
      row += on ? "0" : ".";
    }
    out.push(row);
  }
  if (!any) return { error: "metin çizilemedi (boş çıktı)" };
  return { bitmap: { palette: [color.toUpperCase()], rows: out } };
}

// Görüntü → cols×rows bitmap (küçültme + nicemleme).
export function imageToBitmap(
  img: HTMLImageElement,
  cols: number,
  rows: number,
): { bitmap?: GenBitmap; error?: string } {
  if (cols < 1 || cols > MAX_BITMAP_COLS || rows < 1 || rows > MAX_BITMAP_ROWS) {
    return { error: `boyut 1..${MAX_BITMAP_COLS} × 1..${MAX_BITMAP_ROWS} olmalı` };
  }
  const canvas = document.createElement("canvas");
  canvas.width = cols;
  canvas.height = rows;
  const ctx = canvas.getContext("2d");
  if (!ctx) return { error: "canvas desteklenmiyor" };
  ctx.imageSmoothingEnabled = true;
  // En-boy oranı KORUNUR (contain): görüntü alana ortalanarak sığdırılır,
  // artan kenarlar saydam kalır → '.' (kapalı koltuk). Bayrak yamulmaz.
  const iw = img.naturalWidth || img.width;
  const ih = img.naturalHeight || img.height;
  if (!iw || !ih) return { error: "görüntü boyutu okunamadı" };
  const scale = Math.min(cols / iw, rows / ih);
  const w = iw * scale;
  const h = ih * scale;
  ctx.drawImage(img, (cols - w) / 2, (rows - h) / 2, w, h);
  const data = ctx.getImageData(0, 0, cols, rows).data;
  return { bitmap: rgbaToBitmap(data, cols, rows) };
}
