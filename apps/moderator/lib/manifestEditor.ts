// Görsel gösteri editörü ↔ manifest JSON dönüşümleri.
//
// Editör, manifest şemasının (packages/manifest) kullanıcı dostu bir alt
// kümesini taşır: sekans başına en çok BİR ekran şeridi, BİR fener şeridi ve
// BİR ses kuesi (0. ms'de). Otomatik program, sekans sırasından ve "öncesinde
// boşluk" alanından türetilir. Bu alt kümenin dışındaki manifestler (ör. çok
// ekran şeritli) editöre yüklenemez; sayfa o zaman JSON görünümüne düşer.
// Tüm fonksiyonlar saftır (DOM/ağ yok).

export type EditorLyric = { atMs: number; durationMs: number; text: string };

export type EditorScreenStep = {
  atMs: number;
  durationMs: number; // 0 = sekans sonuna dek
  color: string; // #rrggbb
  flashHz: number; // 0 = sabit yanar
};

export type EditorTorchStep = {
  atMs: number;
  durationMs: number;
  flashHz: number;
};

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
    screen: [{ atMs: 0, durationMs: 0, color: "#d92b2b", flashHz: 0 }],
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
  seq.screen = [{ atMs: 0, durationMs: 0, color: "#d92b2b", flashHz: 2 }];
  return { title: "Yeni Gösteri", sequences: [seq] };
}

// --- editör → manifest ---

type ManifestCue = {
  at_ms: number;
  duration_ms: number;
  color?: string;
  flash_hz?: number;
  asset_id?: string;
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
            if (st.flashHz > 0) cue.flash_hz = st.flashHz;
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
            if (st.flashHz > 0) cue.flash_hz = st.flashHz;
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
        sq.screen = (lane.cues ?? []).map((c) => ({
          atMs: c.at_ms || 0,
          durationMs: c.duration_ms || 0,
          color: c.color || "#ffffff",
          flashHz: c.flash_hz || 0,
        }));
      } else if (lane.kind === "torch") {
        if (torchSeen) return { reason: `"${seq.id}" sekansında birden çok fener şeridi var` };
        torchSeen = true;
        sq.torch = (lane.cues ?? []).map((c) => ({
          atMs: c.at_ms || 0,
          durationMs: c.duration_ms || 0,
          flashHz: c.flash_hz || 0,
        }));
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

  return { show: { title: mm.title || "Gösteri", sequences } };
}
