// LRC (senkronlu şarkı sözü) ayrıştırıcısı.
//
// Desteklenen biçim: satır başında bir ya da birden çok [dd:ss.xx] zaman
// damgası + metin. Metadata etiketleri ([ar:…], [ti:…] vb.) zaman damgası
// içermediği için kendiliğinden atlanır. Çıktı, manifest şemasındaki
// lyric_lines listesidir: her satırın süresi bir sonraki satıra kadardır,
// son satır 0 (sekans sonuna dek) kalır.

export type LrcLyricLine = { at_ms: number; duration_ms: number; text: string };

const TIMESTAMP = /\[(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\]/g;

export function parseLrc(source: string): LrcLyricLine[] {
  const entries: { at: number; text: string }[] = [];

  for (const rawLine of source.split(/\r?\n/)) {
    const stamps = [...rawLine.matchAll(TIMESTAMP)];
    if (stamps.length === 0) continue;
    // Manifest boş söz metnine izin vermez; boş (enstrümantal) satırlar atlanır.
    const text = rawLine.replace(/\[[^\]]*\]/g, "").trim();
    if (!text) continue;

    for (const stamp of stamps) {
      const minutes = Number(stamp[1]);
      const seconds = Number(stamp[2]);
      const fracRaw = stamp[3] ?? "";
      const fracMs =
        fracRaw === "" ? 0 : Math.round((Number(fracRaw) / 10 ** fracRaw.length) * 1000);
      entries.push({ at: (minutes * 60 + seconds) * 1000 + fracMs, text });
    }
  }

  entries.sort((a, b) => a.at - b.at);
  return entries.map((entry, i) => ({
    at_ms: entry.at,
    duration_ms: i + 1 < entries.length ? entries[i + 1].at - entry.at : 0,
    text: entry.text,
  }));
}
