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
  // "grid" = düz blok; "arc" = yay (köşe/oval tribün — sıra başına koltuk
  // yarıçapla artar: sahaya yakın az, yukarıda çok).
  kind: "grid" | "arc";
  rows: number;
  seatsPerRow: number; // grid
  x: number; // grid: 1. sıra 1. koltuk; arc: yay MERKEZİ (metre)
  y: number;
  z: number; // taban yüksekliği (metre)
  rotationDeg: number; // grid: koltuk yönünün +x'ten saat yönü tersine açısı
  seatStep: number; // koltuk aralığı (m; arc'ta yay boyunca)
  rowStep: number; // sıra aralığı (m; arc'ta yarıçap artışı)
  rake: number; // sıra başına yükselme (m; tribün eğimi)
  radius: number; // arc: 1. sıranın yarıçapı (m)
  angleStartDeg: number; // arc: koltuk 1 bu uçta
  angleEndDeg: number;
};

// Yayda o sıradaki koltuk sayısı (packages/manifest SeatsInRow aynası).
export function seatsInRowOf(b: EditorBlock, row: number): number {
  if (b.kind !== "arc") return b.seatsPerRow;
  if (b.seatStep <= 0) return 1;
  const r = b.radius + (row - 1) * b.rowStep;
  const span = ((b.angleEndDeg - b.angleStartDeg) * Math.PI) / 180;
  const n = Math.floor((r * span) / b.seatStep) + 1;
  return n < 1 ? 1 : n;
}

// Saha/sahne işareti: görsel bağlam (editör + 3B); koreografiyi etkilemez.
export type EditorLandmark = {
  kind: "pitch" | "stage";
  x: number; // merkez (metre)
  y: number;
  w: number; // genişlik (x ekseni)
  d: number; // derinlik (y ekseni)
};

export type EditorVenue = {
  name: string;
  blocks: EditorBlock[];
  landmark: EditorLandmark | null;
};

type Vec3Json = { x?: number; y?: number; z?: number };
export type VenueJson = {
  name?: string;
  blocks?: {
    id?: string;
    kind?: string;
    rows?: number;
    seats_per_row?: number;
    origin?: Vec3Json;
    row_vec?: Vec3Json;
    seat_vec?: Vec3Json;
    center?: Vec3Json;
    radius?: number;
    row_step?: number;
    rake?: number;
    angle_start_deg?: number;
    angle_end_deg?: number;
    seat_step?: number;
  }[];
  landmark?: { kind?: string; x?: number; y?: number; w?: number; d?: number };
};

const rad = (deg: number) => (deg * Math.PI) / 180;

// Kayan nokta tozunu JSON'a taşımamak için 1 mm'e yuvarla.
const mm = (v: number) => Math.round(v * 1000) / 1000;

export function defaultBlock(id: string): EditorBlock {
  return {
    id,
    kind: "grid",
    rows: 20,
    seatsPerRow: 30,
    x: 0,
    y: 0,
    z: 0,
    rotationDeg: 0,
    seatStep: 0.5,
    rowStep: 0.8,
    rake: 0.4,
    radius: 12,
    angleStartDeg: 0,
    angleEndDeg: 90,
  };
}

export function toManifestVenue(v: EditorVenue): VenueJson {
  return {
    ...(v.name ? { name: v.name } : {}),
    ...(v.landmark
      ? {
          landmark: {
            kind: v.landmark.kind,
            x: mm(v.landmark.x),
            y: mm(v.landmark.y),
            w: mm(v.landmark.w),
            d: mm(v.landmark.d),
          },
        }
      : {}),
    blocks: v.blocks.map((b) => {
      if (b.kind === "arc") {
        return {
          id: b.id,
          kind: "arc",
          rows: b.rows,
          center: { x: mm(b.x), y: mm(b.y), z: mm(b.z) },
          radius: mm(b.radius),
          row_step: mm(b.rowStep),
          rake: mm(b.rake),
          angle_start_deg: b.angleStartDeg,
          angle_end_deg: b.angleEndDeg,
          seat_step: mm(b.seatStep),
        };
      }
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
    if (b.kind === "arc") {
      blocks.push({
        ...defaultBlock(id),
        kind: "arc",
        rows: b.rows ?? 1,
        x: b.center?.x ?? 0,
        y: b.center?.y ?? 0,
        z: b.center?.z ?? 0,
        radius: b.radius ?? 10,
        rowStep: b.row_step ?? 0.8,
        rake: b.rake ?? 0,
        angleStartDeg: b.angle_start_deg ?? 0,
        angleEndDeg: b.angle_end_deg ?? 90,
        seatStep: b.seat_step ?? 0.5,
      });
      continue;
    }
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
      ...defaultBlock(id),
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
  let landmark: EditorLandmark | null = null;
  if (vv.landmark && (vv.landmark.kind === "pitch" || vv.landmark.kind === "stage")) {
    landmark = {
      kind: vv.landmark.kind,
      x: vv.landmark.x ?? 0,
      y: vv.landmark.y ?? 0,
      w: vv.landmark.w ?? 10,
      d: vv.landmark.d ?? 5,
    };
  }
  return { venue: { name: vv.name || "", blocks, landmark } };
}

// --- şablonlar ---

// Salon: sahneye bakan tek büyük blok + sahne işareti. Blok bölünebilir,
// sayılar değiştirilebilir; şablon başlangıç noktasıdır.
export function hallTemplate(): EditorVenue {
  return {
    name: "Salon",
    blocks: [{ ...defaultBlock("SALON"), x: -12.25, y: 4, rows: 30, seatsPerRow: 50 }],
    landmark: { kind: "stage", x: 0, y: -1, w: 16, d: 6 },
  };
}

// Stadyum: sahayı (105×68) çevreleyen 4 kenar + 4 köşe tribünü — tam tur.
// Büyük stadyuma göre boyutlanmıştır (tek katman ~30k; sıra/koltuk
// sayılarını artırarak büyütülür). İstenmeyen blok "Bloğu sil" ile
// çıkarılır (ör. köşesiz stadyum için köşeler silinir).
export function stadiumTemplate(): EditorVenue {
  const mk = (
    id: string,
    x: number,
    y: number,
    rotationDeg: number,
    rows: number,
    seats: number,
  ): EditorBlock => ({
    ...defaultBlock(id),
    x,
    y,
    rotationDeg,
    rows,
    seatsPerRow: seats,
  });
  const arc = (
    id: string,
    x: number,
    y: number,
    a0: number,
    a1: number,
  ): EditorBlock => ({
    ...defaultBlock(id),
    kind: "arc",
    x,
    y,
    rows: 45,
    radius: 20,
    angleStartDeg: a0,
    angleEndDeg: a1,
  });
  return {
    name: "Stadyum",
    blocks: [
      // Kenarlar: ilk sıra sahadan ~8 m; sıralar sahadan dışarı yükselir.
      mk("KUZEY", -35, 42, 0, 45, 140),
      mk("GUNEY", 35, -42, 180, 45, 140),
      mk("DOGU", 60.5, 25, 270, 45, 100),
      mk("BATI", -60.5, -25, 90, 45, 100),
      // Köşeler: YAY bloklar (gerçek stadyum gibi oval) — kenarların uçlarını
      // kavisle bağlar; sahaya yakın sırada az, üst sıralarda çok koltuk.
      arc("KUZEYDOGU", 37, 24, 0, 90),
      arc("KUZEYBATI", -37, 24, 90, 180),
      arc("GUNEYBATI", -37, -24, 180, 270),
      arc("GUNEYDOGU", 37, -24, 270, 360),
    ],
    landmark: { kind: "pitch", x: 0, y: 0, w: 105, d: 68 },
  };
}

// --- çizim yardımcıları (SVG üstten görünüş) ---

// Blok kontürü (dünya xy, metre): grid'de dört köşe; yayda iç yay → dış yay
// (geri) örneklenmiş çokgen — SVG/3B çizimi ve sınır kutusu bunu kullanır.
export function blockCorners(b: EditorBlock): [number, number][] {
  if (b.kind === "arc") {
    const span = b.angleEndDeg - b.angleStartDeg;
    const steps = Math.max(4, Math.ceil(Math.abs(span) / 7.5));
    // 90° katları da eklenir: sınır kutusu uçları eksen katlarındadır ve
    // telefonun normalize u/v/w'suyla birebir kalmalıdır (Go boundsSamples).
    const angles: number[] = [];
    for (let i = 0; i <= steps; i++) angles.push(b.angleStartDeg + (span * i) / steps);
    for (let k = Math.ceil(b.angleStartDeg / 90); k * 90 < b.angleEndDeg; k++) {
      if (k * 90 > b.angleStartDeg) angles.push(k * 90);
    }
    angles.sort((a, bb) => a - bb);
    const rIn = b.radius;
    const rOut = b.radius + (b.rows - 1) * b.rowStep;
    const pts: [number, number][] = [];
    for (const deg of angles) {
      const th = rad(deg);
      pts.push([b.x + rIn * Math.cos(th), b.y + rIn * Math.sin(th)]);
    }
    for (let i = angles.length - 1; i >= 0; i--) {
      const th = rad(angles[i]);
      pts.push([b.x + rOut * Math.cos(th), b.y + rOut * Math.sin(th)]);
    }
    return pts;
  }
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
  // Saha/sahne de görünür alanda kalmalı (salonun sahnesi blokların dışında).
  if (v.landmark) {
    const lm = v.landmark;
    minX = Math.min(minX, lm.x - lm.w / 2); maxX = Math.max(maxX, lm.x + lm.w / 2);
    minY = Math.min(minY, lm.y - lm.d / 2); maxY = Math.max(maxY, lm.y + lm.d / 2);
  }
  if (!Number.isFinite(minX)) return { minX: -10, minY: -10, maxX: 10, maxY: 10 };
  return { minX, minY, maxX, maxY };
}

export function totalSeats(v: EditorVenue): number {
  return v.blocks.reduce((sum, b) => {
    if (b.kind !== "arc") return sum + b.rows * b.seatsPerRow;
    let n = 0;
    for (let row = 1; row <= b.rows; row++) n += seatsInRowOf(b, row);
    return sum + n;
  }, 0);
}

// --- 3B önizleme için koltuk noktaları (F4.4) ---

// Dünya konumu (metre) + normalize u/v/w. Normalizasyon packages/manifest
// Bounds/normAxis aynasıdır: sınır kutusu blok köşelerinden, sıfır genişlikli
// eksen 0.5. Efekt önizlemesi telefonla aynı u/v/w'yu görmek zorundadır.
export type SeatPoint = {
  x: number;
  y: number;
  z: number;
  u: number;
  v: number;
  w: number;
  block: string; // blok filtreli efektler için koltuğun bloğu
};

function seatWorld3(b: EditorBlock, row: number, seat: number): [number, number, number] {
  if (b.kind === "arc") {
    const r = b.radius + (row - 1) * b.rowStep;
    const th = rad(b.angleStartDeg) + ((seat - 1) * b.seatStep) / r;
    return [b.x + r * Math.cos(th), b.y + r * Math.sin(th), b.z + b.rake * (row - 1)];
  }
  const c = Math.cos(rad(b.rotationDeg));
  const s = Math.sin(rad(b.rotationDeg));
  return [
    b.x + (-s * b.rowStep) * (row - 1) + c * b.seatStep * (seat - 1),
    b.y + (c * b.rowStep) * (row - 1) + s * b.seatStep * (seat - 1),
    b.z + b.rake * (row - 1),
  ];
}

// maxSeats üstünde her stride'ıncı koltuk örneklenir (çizim bütçesi).
// Sınır kutusu blok kontürlerinden (yay dahil) — normalize u/v/w telefonla
// aynı kalsın diye z uçları da uç sıralardan alınır.
export function seatPoints(v: EditorVenue, maxSeats: number): { points: SeatPoint[]; stride: number } {
  let minX = Infinity, minY = Infinity, minZ = Infinity;
  let maxX = -Infinity, maxY = -Infinity, maxZ = -Infinity;
  for (const b of v.blocks) {
    for (const [x, y] of blockCorners(b)) {
      minX = Math.min(minX, x); maxX = Math.max(maxX, x);
      minY = Math.min(minY, y); maxY = Math.max(maxY, y);
    }
    for (const row of [1, b.rows]) {
      const z = b.z + b.rake * (row - 1);
      minZ = Math.min(minZ, z); maxZ = Math.max(maxZ, z);
    }
  }
  const norm = (p: number, lo: number, hi: number) => (hi - lo < 0.001 ? 0.5 : (p - lo) / (hi - lo));
  const total = totalSeats(v);
  const stride = Math.max(1, Math.ceil(total / maxSeats));
  const points: SeatPoint[] = [];
  let counter = 0;
  for (const b of v.blocks) {
    for (let row = 1; row <= b.rows; row++) {
      const rowSeats = seatsInRowOf(b, row);
      for (let seat = 1; seat <= rowSeats; seat++) {
        if (counter++ % stride !== 0) continue;
        const [x, y, z] = seatWorld3(b, row, seat);
        points.push({
          x, y, z,
          u: norm(x, minX, maxX),
          v: norm(y, minY, maxY),
          w: norm(z, minZ, maxZ),
          block: b.id,
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
