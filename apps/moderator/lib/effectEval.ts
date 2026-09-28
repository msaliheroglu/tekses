// Uzamsal efekt değerlendirmesi — panel önizlemesi için TS portu (F4.4).
//
// Referans packages/manifest/effect.go; aritmetik Go/Dart/join.html JS ile
// BİREBİRDİR ve testdata/effect_vectors.json altın vektörlerine karşı
// sınanır (scratchpad Chromium koşumu). Değiştirirken dördünü birden
// değiştir ve vektörleri yeniden üret. Ayrıca kare mantığının (aktif kue,
// flaş fazı) sade bir kopyası vardır: önizleme, telefonun o koltukta o an
// basacağı ekran rengini birebir üretir.

import type { ManifestJson } from "./manifestEditor";

export type EffectJson = {
  kind?: string;
  axis?: string;
  reverse?: boolean;
  period_ms?: number;
  width?: number;
  color2?: string;
  bitmap?: { palette?: string[]; rows?: string[] };
  blocks?: string[];
  area?: { u0: number; w0: number; u1: number; w1: number };
  colors?: string[];
};

function axisPos(eff: EffectJson, u: number, v: number, w: number): number {
  let p = u;
  if (eff.axis === "v") p = v;
  if (eff.axis === "w") p = w;
  if (eff.axis === "ring") {
    // Mekân merkezinin etrafındaki açı payı (0..1; saat yönünün tersine).
    p = Math.atan2(v - 0.5, u - 0.5) / (2 * Math.PI);
    if (p < 0) p += 1;
  }
  return eff.reverse ? 1 - p : p;
}

function lerpColor(c1: string, c2: string, t: number): string {
  const ch = (c: string, i: number) => parseInt(c.slice(i, i + 2), 16);
  const hex2 = (x: number) => x.toString(16).padStart(2, "0").toUpperCase();
  return (
    "#" +
    hex2(Math.round(ch(c1, 1) + (ch(c2, 1) - ch(c1, 1)) * t)) +
    hex2(Math.round(ch(c1, 3) + (ch(c2, 3) - ch(c1, 3)) * t)) +
    hex2(Math.round(ch(c1, 5) + (ch(c2, 5) - ch(c1, 5)) * t))
  );
}

// Efektin (u,v,w) konumuna kue başlangıcından sinceMs sonra basacağı renk;
// "" = kapalı.
export function evalEffect(
  eff: EffectJson,
  cueColor: string,
  u: number,
  v: number,
  w: number,
  block: string,
  sinceMs: number,
): string {
  if (eff.blocks && eff.blocks.length > 0 && !eff.blocks.includes(block)) {
    return ""; // blok filtresi: kapsam dışı (koltuksuz istemci dahil)
  }
  if (eff.kind === "wave") {
    if (!eff.period_ms || eff.period_ms <= 0) return "";
    const p = axisPos(eff, u, v, w);
    const front = (sinceMs % eff.period_ms) / eff.period_ms;
    let d = Math.abs(p - front);
    if (d > 0.5) d = 1 - d; // süpürme sargılıdır
    return d <= (eff.width || 0) / 2 ? cueColor : eff.color2 || "";
  }
  if (eff.kind === "gradient") {
    if (!eff.color2 || eff.color2.length !== 7 || cueColor.length !== 7) return "";
    return lerpColor(cueColor, eff.color2, axisPos(eff, u, v, w));
  }
  if (eff.kind === "cycle") {
    const cs = eff.colors ?? [];
    if (cs.length === 0) return "";
    if (!eff.period_ms || eff.period_ms <= 0) return cs[0]; // söz modu motorda
    return cs[Math.floor(sinceMs / eff.period_ms) % cs.length];
  }
  if (eff.kind === "bitmap") {
    const b = eff.bitmap;
    if (!b || !b.rows || b.rows.length === 0 || b.rows[0].length === 0) return "";
    const cols = b.rows[0].length;
    let p = axisPos(eff, u, v, w);
    let q = w; // dikey daima w
    const a = eff.area;
    if (a) {
      // Yerleştirme penceresi: dışı kapalı, içi 0..1'e haritalanır.
      if (p < a.u0 || p > a.u1 || q < a.w0 || q > a.w1) return "";
      p = (p - a.u0) / (a.u1 - a.u0);
      q = (q - a.w0) / (a.w1 - a.w0);
    }
    if (eff.period_ms && eff.period_ms > 0) {
      p += (sinceMs % eff.period_ms) / eff.period_ms;
      if (p >= 1) p -= 1;
    }
    let col = Math.floor(p * cols);
    if (col >= cols) col = cols - 1;
    if (col < 0) col = 0;
    const nrows = b.rows.length;
    let rowIdx = Math.floor((1 - q) * nrows); // ilk satır = tepe
    if (rowIdx >= nrows) rowIdx = nrows - 1;
    if (rowIdx < 0) rowIdx = 0;
    const row = b.rows[rowIdx];
    if (col >= row.length || row[col] === ".") return "";
    const idx = parseInt(row[col], 16);
    if (!Number.isInteger(idx) || idx >= (b.palette || []).length) return "";
    return b.palette![idx];
  }
  return "";
}

// Söz izleyen renk döngüsü (cycle, period_ms=0): başlamış söz satırı sayısı
// mod renk sayısı — kural telefon motoru ve /join JS'iyle birebirdir.
export function cycleLyricColor(eff: EffectJson, block: string, lyricsStarted: number): string {
  if (eff.blocks && eff.blocks.length > 0 && !eff.blocks.includes(block)) return "";
  const cs = eff.colors ?? [];
  if (cs.length === 0) return "";
  return cs[lyricsStarted % cs.length];
}

// --- kare mantığı (telefonun screen şeridi kuralları) ---

export type SeqJson = ManifestJson["sequences"][number];

function seqById(m: ManifestJson, id: string): SeqJson | undefined {
  return m.sequences.find((s) => s.id === id);
}

// cue_id → o anki hedef sekans + sekans içi süre. "program" gömülü otomatik
// akışı açar (öğeler artan sıralı; telefonun ProgramEngine'i gibi öğeler
// arası boşlukta karanlık beklenir → null).
export function resolveTarget(
  m: ManifestJson,
  cueId: string,
  elapsedMs: number,
): { seq: SeqJson; seqElapsedMs: number } | null {
  if (cueId === "program") {
    let cur: { sequence_id: string; at_offset_ms: number } | null = null;
    for (const it of m.program ?? []) {
      if ((it.at_offset_ms || 0) <= elapsedMs) cur = it;
    }
    if (!cur) return null;
    const seq = seqById(m, cur.sequence_id);
    if (!seq) return null;
    const e = elapsedMs - (cur.at_offset_ms || 0);
    return e < seq.duration_ms ? { seq, seqElapsedMs: e } : null;
  }
  const seq = seqById(m, cueId);
  return seq ? { seq, seqElapsedMs: elapsedMs } : null;
}

export function manifestScreenColorAt(
  m: ManifestJson,
  cueId: string,
  elapsedMs: number,
  u: number,
  v: number,
  w: number,
  block = "",
): string {
  const t = resolveTarget(m, cueId, elapsedMs);
  return t ? screenColorAt(t.seq, t.seqElapsedMs, u, v, w, block) : "";
}

export function seqLyricAt(seq: SeqJson, elapsedMs: number): string {
  if (elapsedMs < 0 || elapsedMs >= seq.duration_ms) return "";
  let lyric = "";
  for (const l of seq.lyric_lines ?? []) {
    const end = l.duration_ms ? l.at_ms + l.duration_ms : seq.duration_ms;
    if (elapsedMs >= l.at_ms && elapsedMs < end) lyric = l.text;
  }
  return lyric;
}

export function manifestLyricAt(m: ManifestJson, cueId: string, elapsedMs: number): string {
  const t = resolveTarget(m, cueId, elapsedMs);
  return t ? seqLyricAt(t.seq, t.seqElapsedMs) : "";
}

// Koşunun toplam süresi (önizleme ilerleme çubuğu): program → son öğenin
// bitişi, sekans → kendi süresi, tanınmayan kimlik → 0.
export function manifestCueDurationMs(m: ManifestJson, cueId: string): number {
  if (cueId === "program") {
    let end = 0;
    for (const it of m.program ?? []) {
      const seq = seqById(m, it.sequence_id);
      if (seq) end = Math.max(end, (it.at_offset_ms || 0) + seq.duration_ms);
    }
    return end;
  }
  return seqById(m, cueId)?.duration_ms ?? 0;
}

// O koltuğun o anki ekran rengi ('' = karanlık). Çakışmada son kue kazanır,
// flaş fazı floor((e−at)·hz/500) — telefon/tarayıcıyla aynı. block, koltuğun
// blok kimliğidir (blok filtreli efektler + söz izleyen döngü için).
export function screenColorAt(
  seq: SeqJson,
  elapsedMs: number,
  u: number,
  v: number,
  w: number,
  block = "",
): string {
  if (elapsedMs < 0 || elapsedMs >= seq.duration_ms) return "";
  let color = "";
  for (const lane of seq.cue_lanes ?? []) {
    if (lane.kind !== "screen") continue;
    let active: (typeof lane.cues)[number] | null = null;
    for (const c of lane.cues) {
      const end = c.duration_ms ? c.at_ms + c.duration_ms : Infinity;
      if (elapsedMs >= c.at_ms && elapsedMs < end) active = c;
    }
    if (!active) {
      color = "";
      continue;
    }
    const eff = (active as { effect?: EffectJson }).effect;
    if (eff) {
      if (eff.kind === "cycle" && (!eff.period_ms || eff.period_ms <= 0)) {
        // Söz izleyen döngü: başlamış söz satırı sayısı rengi seçer.
        let started = 0;
        for (const l of seq.lyric_lines ?? []) {
          if ((l.at_ms || 0) <= elapsedMs) started++;
        }
        color = cycleLyricColor(eff, block, started);
      } else {
        color = evalEffect(eff, active.color || "", u, v, w, block, elapsedMs - active.at_ms);
      }
    } else {
      const hz = active.flash_hz || 0;
      const lit = hz === 0 || Math.floor(((elapsedMs - active.at_ms) * hz) / 500) % 2 === 0;
      color = lit ? active.color || "" : "";
    }
  }
  return color;
}
