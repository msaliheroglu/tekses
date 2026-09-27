import {
  fromManifestVenue,
  toManifestVenue,
  type EditorVenue,
  type VenueJson,
} from "./venueEditor";

// Görsel gösteri editörü ↔ manifest JSON dönüşümleri.
//
// Editör, manifest şemasının (packages/manifest) kullanıcı dostu bir alt
// kümesini taşır: sekans başına en çok BİR ekran şeridi, BİR fener şeridi ve
// BİR ses kuesi (0. ms'de). Otomatik program, sekans sırasından ve "öncesinde
// boşluk" alanından türetilir. Bu alt kümenin dışındaki manifestler (ör. çok
// ekran şeritli) editöre yüklenemez; sayfa o zaman JSON görünümüne düşer.
// Tüm fonksiyonlar saftır (DOM/ağ yok).

export type EditorLyric = { atMs: number; durationMs: number; text: string };

// packages/manifest Bitmap ile aynı biçim: palet ≤16 renk, karakter = palet
// indeksi, '.' = kapalı.
export type EditorBitmap = { palette: string[]; rows: string[] };

// Bitmap yerleştirme penceresi (normalize yatay×dikey; null = tüm mekân).
export type EditorArea = { u0: number; w0: number; u1: number; w1: number };

export type EditorScreenStep = {
  atMs: number;
  durationMs: number; // 0 = sekans sonuna dek
  color: string; // #rrggbb
  flashHz: number; // 0 = sabit yanar (efekt seçiliyken yok sayılır)
  // Uzamsal efekt (F4.2): "" = düz renk.
  effectKind: "" | "wave" | "gradient" | "bitmap" | "cycle";
  effectAxis: "u" | "v" | "w" | "ring"; // ring = stadyum turu
  effectReverse: boolean;
  effectPeriodMs: number; // dalga süpürme / cycle adım süresi (≥334 ms)
  effectWidth: number; // dalga bandı, 0..1
  effectColor2: string; // dalga arka planı ("" = kapalı) / gradyan bitişi
  effectScrollMs: number; // bitmap: kaydırma dönemi (0 = sabit; ≥ sütun×334)
  effectBitmap: EditorBitmap | null; // bitmap: üreticiden (F4.3b) ya da JSON'dan
  effectBlocks: string[]; // yalnız bu bloklar oynar (boş = herkes)
  effectArea: EditorArea | null; // bitmap yerleştirme penceresi
  effectColors: string[]; // cycle renkleri
  effectLyricSync: boolean; // cycle: renk söz satırını izler (period 0)
};

export function emptyScreenStep(atMs: number): EditorScreenStep {
  return {
    atMs,
    durationMs: 0,
    color: "#d92b2b",
    flashHz: 0,
    effectKind: "",
    effectAxis: "u",
    effectReverse: false,
    effectPeriodMs: 2000,
    effectWidth: 0.2,
    effectColor2: "",
    effectScrollMs: 0,
    effectBitmap: null,
    effectBlocks: [],
    effectArea: null,
    effectColors: ["#d92b2b", "#ffffff"],
    effectLyricSync: true,
  };
}

export type EditorTorchStep = {
  atMs: number;
  durationMs: number;
  flashHz: number;
  // Fener dalgası (F4.5-3.tur): fenerler de meksika dalgasına katılır.
  effectKind: "" | "wave";
  effectAxis: "u" | "v" | "w" | "ring";
  effectReverse: boolean;
  effectPeriodMs: number;
  effectWidth: number;
  effectBlocks: string[];
};

export function emptyTorchStep(atMs: number): EditorTorchStep {
  return {
    atMs,
    durationMs: 0,
    flashHz: 1,
    effectKind: "",
    effectAxis: "ring",
    effectReverse: false,
    effectPeriodMs: 4000,
    effectWidth: 0.2,
    effectBlocks: [],
  };
}

export type EditorSequence = {
  id: string;
  title: string;
  durationMs: number;
  inProgram: boolean;
  gapBeforeMs: number; // programda bir önceki sekansın bitişiyle arasındaki boşluk
  audioAssetId: string; // "" = ses yok
  lyrics: EditorLyric[];
  screen: EditorScreenStep[];
  torch: EditorTorchStep[];
};

export type EditorShow = {
  title: string;
  sequences: EditorSequence[];
  // Mekân planı (F4.3): null = plansız gösteri (konumdan bağımsız oynar).
  venue: EditorVenue | null;
};

// --- süre biçimi: "d:ss.o" (ör. 1:23.5) ↔ ms ---

export function fmtTime(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) ms = 0;
  const tenths = Math.round(ms / 100);
  const m = Math.floor(tenths / 600);
  const s = Math.floor((tenths % 600) / 10);
  const t = tenths % 10;
  const sec = String(s).padStart(2, "0");
  return t === 0 ? `${m}:${sec}` : `${m}:${sec}.${t}`;
}

// Kabul edilen girdiler: "83", "83.5", "1:23", "1:23.5", "1:02:03".
// Geçersiz girdi null döner (alan kırmızıyla işaretlenir, değer korunur).
export function parseTime(text: string): number | null {
  const s = text.trim().replace(",", ".");
  if (s === "") return null;
  const parts = s.split(":");
  if (parts.length > 3) return null;
  let totalSec = 0;
  for (const part of parts) {
    if (!/^\d+(\.\d+)?$/.test(part)) return null;
    totalSec = totalSec * 60 + Number(part);
  }
  return Math.round(totalSec * 1000);
}

// --- sekans kimliği: başlıktan URL/JSON dostu kimlik ---

const trMap: Record<string, string> = {
  ç: "c", ğ: "g", ı: "i", ö: "o", ş: "s", ü: "u",
  Ç: "c", Ğ: "g", İ: "i", I: "i", Ö: "o", Ş: "s", Ü: "u",
};

export function slugify(title: string): string {
  const ascii = title
    .split("")
    .map((ch) => trMap[ch] ?? ch)
    .join("")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return ascii || "sekans";
}

export function uniqueId(base: string, taken: Set<string>): string {
  let id = base;
  for (let i = 2; taken.has(id); i++) id = `${base}-${i}`;
  taken.add(id);
  return id;
}

// --- boş/örnek durumlar ---

export function emptySequence(title: string): EditorSequence {
  return {
    id: "",
    title,
    durationMs: 60000,
    inProgram: true,
    gapBeforeMs: 0,
    audioAssetId: "",
    lyrics: [],
    screen: [emptyScreenStep(0)],
    torch: [],
  };
}

// Yeni gösteri şablonu: yapı örneklensin diye tek sekanslık iskelet.
// Sözler telifli olabileceğinden yer tutucudur (lisans organizatörde).
export function defaultShow(): EditorShow {
  const seq = emptySequence("Açılış");
  seq.durationMs = 30000;
  seq.lyrics = [
    { atMs: 0, durationMs: 5000, text: "Hep beraber!" },
    { atMs: 5000, durationMs: 5000, text: "Tek ses, tek yürek!" },
  ];
  seq.screen = [{ ...emptyScreenStep(0), flashHz: 2 }];
  return { title: "Yeni Gösteri", sequences: [seq], venue: null };
}

// --- editör → manifest ---

type ManifestEffect = {
  kind?: string;
  axis?: string;
  reverse?: boolean;
  period_ms?: number;
  width?: number;
  color2?: string;
  bitmap?: { palette?: string[]; rows?: string[] };
  blocks?: string[];
  area?: EditorArea;
  colors?: string[];
};
type ManifestCue = {
  at_ms: number;
  duration_ms: number;
  color?: string;
  flash_hz?: number;
  asset_id?: string;
  effect?: ManifestEffect;
};
type ManifestLane = { id: string; kind: string; cues: ManifestCue[] };
type ManifestSeq = {
  id: string;
  title: string;
  duration_ms: number;
  lyric_lines?: { at_ms: number; duration_ms: number; text: string }[];
  cue_lanes?: ManifestLane[];
};
export type ManifestJson = {
  title: string;
  sequences: ManifestSeq[];
  program?: { sequence_id: string; at_offset_ms: number }[];
  venue?: VenueJson;
};

export function toManifest(show: EditorShow): ManifestJson {
  const taken = new Set<string>();
  const sequences: ManifestSeq[] = show.sequences.map((sq) => {
    const id = uniqueId(sq.id || slugify(sq.title), taken);
    const lanes: ManifestLane[] = [];
    if (sq.screen.length > 0) {
      lanes.push({
        id: "ekran",
        kind: "screen",
        cues: [...sq.screen]
          .sort((a, b) => a.atMs - b.atMs)
          .map((st) => {
            const cue: ManifestCue = { at_ms: st.atMs, duration_ms: st.durationMs, color: st.color };
            // Bitmap seçilmiş ama henüz üretilmemişse düz renge düşülür
            // (yayın sunucuda reddedilmesin); arayüz "önce üretin" uyarır.
            const kind = st.effectKind === "bitmap" && !st.effectBitmap ? "" : st.effectKind;
            if (kind) {
              // Efekt ve flash_hz birlikte yasak (ışık güvenliği) — efekt kazanır.
              const eff: ManifestEffect = { kind };
              if (kind !== "cycle") {
                // cycle uzamsal değildir; sunucu axis taşımasını reddeder.
                if (st.effectAxis !== "u") eff.axis = st.effectAxis;
                if (st.effectReverse) eff.reverse = true;
              }
              if (kind === "wave") {
                eff.period_ms = st.effectPeriodMs;
                eff.width = st.effectWidth;
                if (st.effectColor2) eff.color2 = st.effectColor2;
              } else if (kind === "gradient") {
                eff.color2 = st.effectColor2 || "#000000";
              } else if (kind === "bitmap" && st.effectBitmap) {
                if (st.effectScrollMs > 0) eff.period_ms = st.effectScrollMs;
                eff.bitmap = { palette: st.effectBitmap.palette, rows: st.effectBitmap.rows };
                if (st.effectArea) eff.area = { ...st.effectArea };
              } else if (kind === "cycle") {
                eff.colors = [...st.effectColors];
                // Söz izleme = period 0 (motor kuralı); değilse adım süresi.
                if (!st.effectLyricSync) eff.period_ms = st.effectPeriodMs;
              }
              if (st.effectBlocks.length > 0) eff.blocks = [...st.effectBlocks];
              cue.effect = eff;
            } else if (st.flashHz > 0) {
              cue.flash_hz = st.flashHz;
            }
            return cue;
          }),
      });
    }
    if (sq.torch.length > 0) {
      lanes.push({
        id: "fener",
        kind: "torch",
        cues: [...sq.torch]
          .sort((a, b) => a.atMs - b.atMs)
          .map((st) => {
            const cue: ManifestCue = { at_ms: st.atMs, duration_ms: st.durationMs };
            if (st.effectKind === "wave") {
              // Fener dalgası: fenerler bantta yanar, dışında söner.
              const eff: ManifestEffect = {
                kind: "wave",
                period_ms: st.effectPeriodMs,
                width: st.effectWidth,
              };
              if (st.effectAxis !== "u") eff.axis = st.effectAxis;
              if (st.effectReverse) eff.reverse = true;
              if (st.effectBlocks.length > 0) eff.blocks = [...st.effectBlocks];
              cue.effect = eff;
            } else if (st.flashHz > 0) {
              cue.flash_hz = st.flashHz;
            }
            return cue;
          }),
      });
    }
    if (sq.audioAssetId) {
      lanes.push({
        id: "muzik",
        kind: "audio",
        cues: [{ at_ms: 0, duration_ms: 0, asset_id: sq.audioAssetId }],
      });
    }
    const seq: ManifestSeq = { id, title: sq.title || id, duration_ms: sq.durationMs };
    if (sq.lyrics.length > 0) {
      seq.lyric_lines = [...sq.lyrics]
        .sort((a, b) => a.atMs - b.atMs)
        .map((l) => ({ at_ms: l.atMs, duration_ms: l.durationMs, text: l.text }));
    }
    if (lanes.length > 0) seq.cue_lanes = lanes;
    return seq;
  });

  // Program: dahil edilen sekanslar, sıra + boşluklarla arka arkaya.
  const program: { sequence_id: string; at_offset_ms: number }[] = [];
  let cursor = 0;
  show.sequences.forEach((sq, i) => {
    if (!sq.inProgram) return;
    const offset = cursor + Math.max(0, sq.gapBeforeMs);
    program.push({ sequence_id: sequences[i].id, at_offset_ms: offset });
    cursor = offset + sq.durationMs;
  });

  const m: ManifestJson = { title: show.title || "Gösteri", sequences };
  if (program.length > 0) m.program = program;
  if (show.venue && show.venue.blocks.length > 0) m.venue = toManifestVenue(show.venue);
  return m;
}

export function toManifestJson(show: EditorShow): string {
  return JSON.stringify(toManifest(show), null, 2);
}

// --- manifest → editör ---

// Desteklenmeyen yapı: null yerine neden döner ki sayfa JSON görünümüne
// düşerken kullanıcıya söyleyebilsin.
export function fromManifest(m: unknown): { show?: EditorShow; reason?: string } {
  if (!m || typeof m !== "object") return { reason: "manifest bir nesne değil" };
  const mm = m as ManifestJson & { format_version?: number };
  if (!Array.isArray(mm.sequences)) return { reason: "sequences listesi yok" };

  const sequences: EditorSequence[] = [];
  for (const seq of mm.sequences) {
    const sq = emptySequence(seq.title || seq.id || "Sekans");
    sq.id = seq.id || "";
    sq.durationMs = seq.duration_ms || 0;
    sq.inProgram = false;
    sq.screen = [];
    sq.lyrics = (seq.lyric_lines ?? []).map((l) => ({
      atMs: l.at_ms || 0,
      durationMs: l.duration_ms || 0,
      text: l.text || "",
    }));
    let screenSeen = false, torchSeen = false, audioSeen = false;
    for (const lane of seq.cue_lanes ?? []) {
      if (lane.kind === "screen") {
        if (screenSeen) return { reason: `"${seq.id}" sekansında birden çok ekran şeridi var` };
        screenSeen = true;
        sq.screen = [];
        for (const c of lane.cues ?? []) {
          const eff = c.effect;
          if (
            eff &&
            eff.kind !== "wave" &&
            eff.kind !== "gradient" &&
            eff.kind !== "bitmap" &&
            eff.kind !== "cycle"
          ) {
            return { reason: `"${seq.id}" sekansında editörün taşımadığı efekt türü (${eff.kind})` };
          }
          const isBitmap = eff?.kind === "bitmap";
          const isCycle = eff?.kind === "cycle";
          sq.screen.push({
            atMs: c.at_ms || 0,
            durationMs: c.duration_ms || 0,
            color: c.color || "#ffffff",
            flashHz: c.flash_hz || 0,
            effectKind: (eff?.kind as EditorScreenStep["effectKind"] | undefined) ?? "",
            effectAxis:
              eff?.axis === "v" || eff?.axis === "w" || eff?.axis === "ring" ? eff.axis : "u",
            effectReverse: !!eff?.reverse,
            effectPeriodMs: (!isBitmap && eff?.period_ms) || 2000,
            effectWidth: eff?.width || 0.2,
            effectColor2: eff?.color2 || "",
            effectScrollMs: isBitmap ? eff?.period_ms || 0 : 0,
            effectBitmap:
              isBitmap && eff?.bitmap
                ? { palette: eff.bitmap.palette ?? [], rows: eff.bitmap.rows ?? [] }
                : null,
            effectBlocks: eff?.blocks ? [...eff.blocks] : [],
            effectArea: eff?.area ? { ...eff.area } : null,
            effectColors: eff?.colors?.length ? [...eff.colors] : ["#d92b2b", "#ffffff"],
            effectLyricSync: isCycle ? !eff?.period_ms : true,
          });
        }
      } else if (lane.kind === "torch") {
        if (torchSeen) return { reason: `"${seq.id}" sekansında birden çok fener şeridi var` };
        torchSeen = true;
        if ((lane.cues ?? []).some((c) => c.effect && c.effect.kind !== "wave")) {
          return { reason: `"${seq.id}" sekansında editörün taşımadığı fener efekti var` };
        }
        sq.torch = (lane.cues ?? []).map((c) => {
          const eff = c.effect;
          return {
            ...emptyTorchStep(c.at_ms || 0),
            durationMs: c.duration_ms || 0,
            flashHz: c.flash_hz || 0,
            effectKind: (eff?.kind === "wave" ? "wave" : "") as "" | "wave",
            effectAxis:
              eff?.axis === "v" || eff?.axis === "w" || eff?.axis === "ring" ? eff.axis : "u",
            effectReverse: !!eff?.reverse,
            effectPeriodMs: eff?.period_ms || 4000,
            effectWidth: eff?.width || 0.2,
            effectBlocks: eff?.blocks ? [...eff.blocks] : [],
          };
        });
      } else if (lane.kind === "audio") {
        const cues = lane.cues ?? [];
        if (audioSeen || cues.length > 1) return { reason: `"${seq.id}" sekansında birden çok ses kuesi var` };
        audioSeen = true;
        if (cues.length === 1) {
          if ((cues[0].at_ms || 0) !== 0) {
            return { reason: `"${seq.id}" sekansında ses 0. saniyede başlamıyor` };
          }
          sq.audioAssetId = cues[0].asset_id || "";
        }
      } else {
        return { reason: `bilinmeyen şerit türü: ${lane.kind}` };
      }
    }
    sequences.push(sq);
  }

  // Program → dahil bayrağı + boşluklar. Program sırası, manifestteki sekans
  // sırasıyla uyumlu olmalı (editör programı sekans sırasından türetir).
  const idx = new Map(mm.sequences.map((s, i) => [s.id, i]));
  let prevIdx = -1, prevEnd = 0;
  for (const item of mm.program ?? []) {
    const i = idx.get(item.sequence_id);
    if (i === undefined) return { reason: `programda bilinmeyen sekans: ${item.sequence_id}` };
    if (i <= prevIdx) return { reason: "program sırası sekans sırasından farklı" };
    sequences[i].inProgram = true;
    sequences[i].gapBeforeMs = Math.max(0, (item.at_offset_ms || 0) - prevEnd);
    prevEnd = (item.at_offset_ms || 0) + sequences[i].durationMs;
    prevIdx = i;
  }

  // Mekân planı: editör kalıbına uymayan plan (eğik/dik olmayan vektörler)
  // JSON görünümüne düşürür ki hiçbir bilgi sessizce kaybolmasın.
  let venue: EditorVenue | null = null;
  if (mm.venue !== undefined) {
    const r = fromManifestVenue(mm.venue);
    if (!r.venue) return { reason: `mekân planı: ${r.reason}` };
    venue = r.venue;
  }

  return { show: { title: mm.title || "Gösteri", sequences, venue } };
}
