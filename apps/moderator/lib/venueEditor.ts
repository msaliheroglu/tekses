// Mekân editörü ↔ manifest venue dönüşümleri (F4.3).
//
// Şemanın gerçeği packages/manifest/venue.go: blok = sıra×koltuk grid'i +
// origin/row_vec/seat_vec. Editör, moderatörün düşündüğü dille çalışır:
// konum + dönüş açısı + koltuk/sıra aralığı + eğim. Dönüşüm kuralı:
//   seat_vec = (cosθ, sinθ, 0)·seatStep
//   row_vec  = (−sinθ, cosθ, 0)·rowStep + (0, 0, rake)
// yani sıralar koltuk yönünün 90° solundadır (dikdörtgen blok). Elle yazılmış
// bir manifest bu kalıba uymuyorsa (eğik seat_vec, dik olmayan row_vec)
// editöre yüklenemez; sayfa nedeniyle birlikte JSON görünümüne düşer.
// Tüm fonksiyonlar saftır (DOM/ağ yok).

export type EditorBlock = {
  id: string;
  rows: number;
  seatsPerRow: number;
  x: number; // 1. sıra 1. koltuğun konumu (metre)
  y: number;
  z: number; // taban yüksekliği (metre)
  rotationDeg: number; // koltuk yönünün +x'ten saat yönü tersine açısı
  seatStep: number; // koltuk aralığı (m)
  rowStep: number; // sıra aralığı (m)
  rake: number; // sıra başına yükselme (m; tribün eğimi)
};

export type EditorVenue = { name: string; blocks: EditorBlock[] };

type Vec3Json = { x?: number; y?: number; z?: number };
export type VenueJson = {
  name?: string;
  blocks?: {
    id?: string;
    rows?: number;
    seats_per_row?: number;
    origin?: Vec3Json;
    row_vec?: Vec3Json;
    seat_vec?: Vec3Json;
  }[];
};

const rad = (deg: number) => (deg * Math.PI) / 180;

// Kayan nokta tozunu JSON'a taşımamak için 1 mm'e yuvarla.
const mm = (v: number) => Math.round(v * 1000) / 1000;

export function defaultBlock(id: string): EditorBlock {
  return {
    id,
    rows: 20,
    seatsPerRow: 30,
    x: 0,
    y: 0,
    z: 0,
    rotationDeg: 0,
    seatStep: 0.5,
    rowStep: 0.8,
    rake: 0.4,
  };
}

export function toManifestVenue(v: EditorVenue): VenueJson {
  return {
    ...(v.name ? { name: v.name } : {}),
    blocks: v.blocks.map((b) => {
      const c = Math.cos(rad(b.rotationDeg));
      const s = Math.sin(rad(b.rotationDeg));
      return {
        id: b.id,
        rows: b.rows,
        seats_per_row: b.seatsPerRow,
        origin: { x: mm(b.x), y: mm(b.y), z: mm(b.z) },
        seat_vec: { x: mm(c * b.seatStep), y: mm(s * b.seatStep), z: 0 },
        row_vec: { x: mm(-s * b.rowStep), y: mm(c * b.rowStep), z: mm(b.rake) },
      };
    }),
  };
}

// Manifest → editör; kalıba uymayan yapı neden ile reddedilir (JSON görünümü).
export function fromManifestVenue(v: unknown): { venue?: EditorVenue; reason?: string } {
  if (!v || typeof v !== "object") return { reason: "venue bir nesne değil" };
  const vv = v as VenueJson;
  if (!Array.isArray(vv.blocks) || vv.blocks.length === 0) {
    return { reason: "venue.blocks boş" };
  }
  const blocks: EditorBlock[] = [];
  for (const b of vv.blocks) {
    const id = b.id || "";
    const sv = { x: b.seat_vec?.x ?? 0, y: b.seat_vec?.y ?? 0, z: b.seat_vec?.z ?? 0 };
    const rv = { x: b.row_vec?.x ?? 0, y: b.row_vec?.y ?? 0, z: b.row_vec?.z ?? 0 };
    if (Math.abs(sv.z) > 1e-6) {
      return { reason: `"${id}" bloğunda seat_vec z bileşeni taşıyor (editör desteklemez)` };
    }
    const seatStep = Math.hypot(sv.x, sv.y);
    const rowStep = Math.hypot(rv.x, rv.y);
    const rows = b.rows ?? 0;
    const seats = b.seats_per_row ?? 0;
    // Tek koltuk/sıralı blokta yön vektörü sıfır olabilir; varsayılanla doldur.
    let rotationDeg: number;
    if (seatStep < 1e-6) {
      if (seats > 1) return { reason: `"${id}" bloğunda seat_vec sıfır` };
      rotationDeg = 0;
    } else {
      rotationDeg = Math.round((Math.atan2(sv.y, sv.x) * 180) / Math.PI * 10) / 10;
    }
    const c = Math.cos(rad(rotationDeg));
    const s = Math.sin(rad(rotationDeg));
    let outRowStep = rowStep;
    if (rowStep < 1e-6) {
      if (rows > 1) return { reason: `"${id}" bloğunda row_vec sıfır` };
      outRowStep = 0.8;
    } else if (seatStep >= 1e-6) {
      // Kalıp: row_vec.xy, koltuk yönünün +90°'si olmalı.
      const ex = -s * rowStep;
      const ey = c * rowStep;
      if (Math.abs(rv.x - ex) > 0.005 || Math.abs(rv.y - ey) > 0.005) {
        return { reason: `"${id}" bloğunda sıralar koltuk yönüne dik değil (editör desteklemez)` };
      }
    }
    blocks.push({
      id,
      rows,
      seatsPerRow: seats,
      x: b.origin?.x ?? 0,
      y: b.origin?.y ?? 0,
      z: b.origin?.z ?? 0,
      rotationDeg,
      seatStep: seatStep < 1e-6 ? 0.5 : Math.round(seatStep * 1000) / 1000,
      rowStep: Math.round(outRowStep * 1000) / 1000,
      rake: Math.round(rv.z * 1000) / 1000,
    });
  }
  return { venue: { name: vv.name || "", blocks } };
}

// --- şablonlar ---

// Salon: sahneye bakan tek dikdörtgen blok.
export function hallTemplate(): EditorVenue {
  return {
    name: "Salon",
    blocks: [{ ...defaultBlock("SALON"), x: -7.5, y: 3, rows: 20, seatsPerRow: 30 }],
  };
}

// Stadyum: sahayı (105×68) çevreleyen dört tribün; sıralar sahadan dışarı
// doğru yükselir. Başlangıç noktasıdır — moderatör sürükleyip uyarlar.
export function stadiumTemplate(): EditorVenue {
  const mk = (
    id: string,
    x: number,
    y: number,
    rotationDeg: number,
    seats: number,
  ): EditorBlock => ({
    ...defaultBlock(id),
    x,
    y,
    rotationDeg,
    rows: 20,
    seatsPerRow: seats,
  });
  return {
    name: "Stadyum",
    blocks: [
      // Kuzey: koltuklar +x yönünde, sıralar +y (sahadan uzağa).
      mk("KUZEY", -25, 38, 0, 100),
      // Güney: koltuklar −x, sıralar −y.
      mk("GUNEY", 25, -38, 180, 100),
      // Doğu: koltuklar −y, sıralar +x.
      mk("DOGU", 57, 20, 270, 80),
      // Batı: koltuklar +y, sıralar −x.
      mk("BATI", -57, -20, 90, 80),
    ],
  };
}

// --- çizim yardımcıları (SVG üstten görünüş) ---

// Blok köşeleri (dünya koordinatında, metre): origin → koltuk ucu → karşı
// köşe → sıra ucu. Konum doğrusal olduğundan dikdörtgeni bu dördü tanımlar.
export function blockCorners(b: EditorBlock): [number, number][] {
  const c = Math.cos(rad(b.rotationDeg));
  const s = Math.sin(rad(b.rotationDeg));
  const seatLen = (b.seatsPerRow - 1) * b.seatStep;
  const rowLen = (b.rows - 1) * b.rowStep;
  const sx = c * seatLen, sy = s * seatLen;
  const rx = -s * rowLen, ry = c * rowLen;
  return [
    [b.x, b.y],
    [b.x + sx, b.y + sy],
    [b.x + sx + rx, b.y + sy + ry],
    [b.x + rx, b.y + ry],
  ];
}

export function venueBounds(v: EditorVenue): { minX: number; minY: number; maxX: number; maxY: number } {
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const b of v.blocks) {
    for (const [x, y] of blockCorners(b)) {
      minX = Math.min(minX, x); maxX = Math.max(maxX, x);
      minY = Math.min(minY, y); maxY = Math.max(maxY, y);
    }
  }
  if (!Number.isFinite(minX)) return { minX: -10, minY: -10, maxX: 10, maxY: 10 };
  return { minX, minY, maxX, maxY };
}

export function totalSeats(v: EditorVenue): number {
  return v.blocks.reduce((sum, b) => sum + b.rows * b.seatsPerRow, 0);
}

// --- 3B önizleme için koltuk noktaları (F4.4) ---

// Dünya konumu (metre) + normalize u/v/w. Normalizasyon packages/manifest
// Bounds/normAxis aynasıdır: sınır kutusu blok köşelerinden, sıfır genişlikli
// eksen 0.5. Efekt önizlemesi telefonla aynı u/v/w'yu görmek zorundadır.
export type SeatPoint = { x: number; y: number; z: number; u: number; v: number; w: number };

function seatWorld3(b: EditorBlock, row: number, seat: number): [number, number, number] {
  const c = Math.cos(rad(b.rotationDeg));
  const s = Math.sin(rad(b.rotationDeg));
  return [
    b.x + (-s * b.rowStep) * (row - 1) + c * b.seatStep * (seat - 1),
    b.y + (c * b.rowStep) * (row - 1) + s * b.seatStep * (seat - 1),
    b.z + b.rake * (row - 1),
  ];
}

// maxSeats üstünde her stride'ıncı koltuk örneklenir (çizim bütçesi).
export function seatPoints(v: EditorVenue, maxSeats: number): { points: SeatPoint[]; stride: number } {
  let minX = Infinity, minY = Infinity, minZ = Infinity;
  let maxX = -Infinity, maxY = -Infinity, maxZ = -Infinity;
  for (const b of v.blocks) {
    for (const row of [1, b.rows]) {
      for (const seat of [1, b.seatsPerRow]) {
        const [x, y, z] = seatWorld3(b, row, seat);
        minX = Math.min(minX, x); maxX = Math.max(maxX, x);
        minY = Math.min(minY, y); maxY = Math.max(maxY, y);
        minZ = Math.min(minZ, z); maxZ = Math.max(maxZ, z);
      }
    }
  }
  const norm = (p: number, lo: number, hi: number) => (hi - lo < 0.001 ? 0.5 : (p - lo) / (hi - lo));
  const total = totalSeats(v);
  const stride = Math.max(1, Math.ceil(total / maxSeats));
  const points: SeatPoint[] = [];
  let counter = 0;
  for (const b of v.blocks) {
    for (let row = 1; row <= b.rows; row++) {
      for (let seat = 1; seat <= b.seatsPerRow; seat++) {
        if (counter++ % stride !== 0) continue;
        const [x, y, z] = seatWorld3(b, row, seat);
        points.push({
          x, y, z,
          u: norm(x, minX, maxX),
          v: norm(y, minY, maxY),
          w: norm(z, minZ, maxZ),
        });
      }
    }
  }
  return { points, stride };
}

export function uniqueBlockId(base: string, blocks: EditorBlock[]): string {
  const taken = new Set(blocks.map((b) => b.id));
  if (!taken.has(base)) return base;
  for (let i = 2; ; i++) {
    if (!taken.has(`${base}${i}`)) return `${base}${i}`;
  }
}
