"use client";

import { useRef, useState } from "react";
import {
  blockCorners,
  defaultBlock,
  hallTemplate,
  seatsInRowOf,
  stadiumTemplate,
  totalSeats,
  uniqueBlockId,
  venueBounds,
  type EditorBlock,
  type EditorLandmark,
  type EditorVenue,
} from "@/lib/venueEditor";

// Saha/sahne işareti (üstten görünüş): izleyen, blokların neye baktığını
// görsün. SVG'de dünya y'si ters çevrilir.
function LandmarkSVG({ lm }: { lm: EditorLandmark }) {
  const left = lm.x - lm.w / 2;
  const top = -(lm.y + lm.d / 2);
  if (lm.kind === "pitch") {
    return (
      <g style={{ pointerEvents: "none" }}>
        <rect x={left} y={top} width={lm.w} height={lm.d} rx={1.5}
          fill="#12401f" stroke="#2e9e4f" strokeWidth={0.6} />
        <line x1={lm.x} y1={top} x2={lm.x} y2={top + lm.d} stroke="#2e9e4f" strokeWidth={0.35} />
        <circle cx={lm.x} cy={-lm.y} r={Math.min(lm.w, lm.d) * 0.135}
          fill="none" stroke="#2e9e4f" strokeWidth={0.35} />
      </g>
    );
  }
  return (
    <g style={{ pointerEvents: "none" }}>
      <rect x={left} y={top} width={lm.w} height={lm.d} rx={1}
        fill="#39404f" stroke="#8b93a7" strokeWidth={0.5} />
      <text x={lm.x} y={-lm.y} fontSize={Math.min(3.4, lm.d * 0.5)} fill="#cdd4e2"
        textAnchor="middle" dominantBaseline="middle" style={{ userSelect: "none" }}>
        SAHNE
      </text>
    </g>
  );
}

// Mekân planı editörü (F4.3): üstten görünüş — bloklar sürüklenerek
// yerleştirilir, seçili bloğun grid/yön/eğim alanları düzenlenir. Plan,
// manifeste venue olarak yazılır; telefon/tarayıcı koltuğu buna göre çözer
// ve uzamsal efektler (dalga/bayrak/slogan) koltuğa göre oynar.
// Dünya ekseni: +y kuzey (yukarı), SVG'de y aşağı olduğundan çizim −y'dir.

const GRID_SNAP_M = 0.5;

function fmtM(v: number): string {
  return String(Math.round(v * 10) / 10);
}

// Sayı alanı: boş/yarım girdi state'i bozmasın diye blur'da işlenir.
function NumField({
  value,
  onChange,
  step = 1,
  min,
}: {
  value: number;
  onChange: (v: number) => void;
  step?: number;
  min?: number;
}) {
  const [text, setText] = useState(String(value));
  const [editing, setEditing] = useState(false);
  return (
    <input
      value={editing ? text : String(value)}
      onFocus={() => {
        setText(String(value));
        setEditing(true);
      }}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => {
        setEditing(false);
        const v = Number(text.replace(",", "."));
        if (!Number.isFinite(v)) return;
        onChange(min !== undefined ? Math.max(min, v) : v);
      }}
      inputMode="decimal"
      step={step}
    />
  );
}

export default function VenueEditor({
  venue,
  onChange,
}: {
  venue: EditorVenue | null;
  onChange: (v: EditorVenue | null) => void;
}) {
  const [selected, setSelected] = useState(0);
  const svgRef = useRef<SVGSVGElement>(null);
  // Sürükleme: bloğun origin'i ile tutulan noktanın farkı korunur.
  const drag = useRef<{ index: number; dx: number; dy: number } | null>(null);

  if (!venue) {
    return (
      <div className="card">
        <h2>3 · Mekân planı (koltuk bazlı koreografi)</h2>
        <p className="muted">
          Plan eklerseniz seyirciler katılırken koltuklarını girer (bilet
          QR&apos;ı <code>?seat=BLOK-SIRA-KOLTUK</code> da taşıyabilir) ve
          dalga/bayrak/slogan gibi uzamsal efektler koltuğa göre oynar.
          Plansız gösteri bugünkü gibi herkese aynı oynar.
        </p>
        <button type="button" className="secondary" onClick={() => onChange(hallTemplate())}>
          Salon şablonuyla başla
        </button>{" "}
        <button type="button" className="secondary" onClick={() => onChange(stadiumTemplate())}>
          Stadyum şablonuyla başla
        </button>
      </div>
    );
  }

  const sel: EditorBlock | undefined = venue.blocks[selected];

  function patchBlock(i: number, patch: Partial<EditorBlock>) {
    if (!venue) return;
    onChange({
      ...venue,
      blocks: venue.blocks.map((b, j) => (j === i ? { ...b, ...patch } : b)),
    });
  }

  // Ekran noktası → dünya koordinatı (metre). SVG y'si ters çevrilir.
  function toWorld(e: React.PointerEvent): { x: number; y: number } | null {
    const svg = svgRef.current;
    if (!svg) return null;
    const pt = svg.createSVGPoint();
    pt.x = e.clientX;
    pt.y = e.clientY;
    const m = svg.getScreenCTM();
    if (!m) return null;
    const p = pt.matrixTransform(m.inverse());
    return { x: p.x, y: -p.y };
  }

  const b = venueBounds(venue);
  const pad = 8;
  const vb = `${b.minX - pad} ${-(b.maxY + pad)} ${b.maxX - b.minX + 2 * pad} ${b.maxY - b.minY + 2 * pad}`;

  return (
    <div className="card">
      <h2>3 · Mekân planı (koltuk bazlı koreografi)</h2>
      <p className="muted">
        Üstten görünüş; bloğu sürükleyerek taşıyın, tıklayıp alanlardan
        düzenleyin. Ok, koltuk numaralandırma yönünü (1. koltuk → son)
        gösterir; sıralar okun 90° solunda ilerler. Toplam{" "}
        {totalSeats(venue).toLocaleString("tr-TR")} koltuk.
      </p>

      <div className="row">
        <div>
          <label>Mekân adı</label>
          <input value={venue.name} onChange={(e) => onChange({ ...venue, name: e.target.value })} />
        </div>
        <div>
          <label>Saha / sahne işareti</label>
          <select
            value={venue.landmark?.kind ?? ""}
            onChange={(e) => {
              const kind = e.target.value as "" | "pitch" | "stage";
              onChange({
                ...venue,
                landmark:
                  kind === ""
                    ? null
                    : kind === "pitch"
                      ? { kind, x: 0, y: 0, w: 105, d: 68 }
                      : { kind, x: 0, y: -1, w: 16, d: 6 },
              });
            }}
          >
            <option value="">yok</option>
            <option value="pitch">saha (yeşil)</option>
            <option value="stage">sahne</option>
          </select>
        </div>
        {venue.landmark && (
          <>
            <div style={{ flex: "0 1 90px" }}>
              <label>Genişlik (m)</label>
              <NumField
                value={venue.landmark.w}
                min={1}
                onChange={(w) => onChange({ ...venue, landmark: { ...venue.landmark!, w } })}
              />
            </div>
            <div style={{ flex: "0 1 90px" }}>
              <label>Derinlik (m)</label>
              <NumField
                value={venue.landmark.d}
                min={1}
                onChange={(d) => onChange({ ...venue, landmark: { ...venue.landmark!, d } })}
              />
            </div>
          </>
        )}
        <div style={{ flex: "0 0 auto", alignSelf: "flex-end" }}>
          <button
            type="button"
            className="secondary"
            onClick={() => {
              const id = uniqueBlockId("BLOK", venue.blocks);
              onChange({ ...venue, blocks: [...venue.blocks, { ...defaultBlock(id), x: b.maxX + 5, y: b.minY }] });
              setSelected(venue.blocks.length);
            }}
          >
            + Blok ekle
          </button>{" "}
          <button type="button" className="ghost" onClick={() => onChange(null)}>
            Planı kaldır
          </button>
        </div>
      </div>

      <svg
        ref={svgRef}
        viewBox={vb}
        style={{
          width: "100%",
          height: 340,
          background: "#0b0e13",
          borderRadius: 8,
          touchAction: "none",
          cursor: drag.current ? "grabbing" : "default",
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (!d) return;
          const w = toWorld(e);
          if (!w) return;
          const snap = (v: number) => Math.round((v + 0) / GRID_SNAP_M) * GRID_SNAP_M;
          patchBlock(d.index, { x: snap(w.x - d.dx), y: snap(w.y - d.dy) });
        }}
        onPointerUp={() => {
          drag.current = null;
        }}
      >
        {venue.landmark && <LandmarkSVG lm={venue.landmark} />}
        {venue.blocks.map((blk, i) => {
          const corners = blockCorners(blk);
          const pts = corners.map(([x, y]) => `${x},${-y}`).join(" ");
          const isSel = i === selected;
          // Koltuk yönü oku: origin'den ilk sıranın ucuna.
          const [o, seatEnd] = corners;
          const cx = corners.reduce((s, c) => s + c[0], 0) / 4;
          const cy = corners.reduce((s, c) => s + c[1], 0) / 4;
          return (
            <g
              key={i}
              onPointerDown={(e) => {
                setSelected(i);
                const w = toWorld(e);
                if (!w) return;
                drag.current = { index: i, dx: w.x - blk.x, dy: w.y - blk.y };
                (e.target as Element).setPointerCapture?.(e.pointerId);
              }}
              style={{ cursor: "grab" }}
            >
              <polygon
                points={pts}
                fill={isSel ? "rgba(217,43,43,0.35)" : "rgba(120,140,180,0.25)"}
                stroke={isSel ? "#d92b2b" : "#5a6b8a"}
                strokeWidth="0.6"
              />
              <line
                x1={o[0]}
                y1={-o[1]}
                x2={o[0] + (seatEnd[0] - o[0]) * 0.35}
                y2={-(o[1] + (seatEnd[1] - o[1]) * 0.35)}
                stroke={isSel ? "#ffb3b3" : "#8fa3c8"}
                strokeWidth="0.8"
                markerEnd="url(#seatArrow)"
              />
              <text
                x={cx}
                y={-cy}
                fill="#dfe6f3"
                fontSize="4"
                textAnchor="middle"
                dominantBaseline="middle"
                style={{ pointerEvents: "none", userSelect: "none" }}
              >
                {blk.id}
              </text>
            </g>
          );
        })}
        <defs>
          <marker id="seatArrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#ffb3b3" />
          </marker>
        </defs>
      </svg>

      {sel && (
        <>
          <div className="row">
            <div>
              <label>Blok kimliği (bilette yazan)</label>
              <input
                value={sel.id}
                onChange={(e) => patchBlock(selected, { id: e.target.value.replace(/\s+/g, "") })}
              />
            </div>
            <div>
              <label>Blok biçimi</label>
              <select
                value={sel.kind}
                onChange={(e) => patchBlock(selected, { kind: e.target.value as "grid" | "arc" })}
              >
                <option value="grid">düz (ızgara)</option>
                <option value="arc">yay / oval (köşe tribünü)</option>
              </select>
            </div>
            <div>
              <label>Sıra sayısı</label>
              <NumField value={sel.rows} min={1} onChange={(v) => patchBlock(selected, { rows: Math.round(v) })} />
            </div>
            {sel.kind === "grid" ? (
              <>
                <div>
                  <label>Sıradaki koltuk</label>
                  <NumField
                    value={sel.seatsPerRow}
                    min={1}
                    onChange={(v) => patchBlock(selected, { seatsPerRow: Math.round(v) })}
                  />
                </div>
                <div>
                  <label>Dönüş (°)</label>
                  <NumField value={sel.rotationDeg} step={15} onChange={(v) => patchBlock(selected, { rotationDeg: v })} />
                </div>
              </>
            ) : (
              <>
                <div>
                  <label>1. sıra yarıçapı (m)</label>
                  <NumField value={sel.radius} min={1} step={0.5} onChange={(v) => patchBlock(selected, { radius: v })} />
                </div>
                <div>
                  <label>Açı başı (°)</label>
                  <NumField value={sel.angleStartDeg} step={5} onChange={(v) => patchBlock(selected, { angleStartDeg: v })} />
                </div>
                <div>
                  <label>Açı sonu (°)</label>
                  <NumField value={sel.angleEndDeg} step={5} onChange={(v) => patchBlock(selected, { angleEndDeg: v })} />
                </div>
              </>
            )}
            <div style={{ flex: "0 0 auto", alignSelf: "flex-end" }}>
              <button
                type="button"
                className="ghost"
                onClick={() => {
                  if (!venue) return;
                  onChange({ ...venue, blocks: venue.blocks.filter((_, j) => j !== selected) });
                  setSelected(0);
                }}
              >
                Bloğu sil
              </button>
            </div>
          </div>
          <div className="row">
            <div>
              <label>{sel.kind === "arc" ? "Yay merkezi x (m)" : "Konum x (m)"}</label>
              <NumField value={sel.x} step={0.5} onChange={(v) => patchBlock(selected, { x: v })} />
            </div>
            <div>
              <label>{sel.kind === "arc" ? "Yay merkezi y (m)" : "Konum y (m)"}</label>
              <NumField value={sel.y} step={0.5} onChange={(v) => patchBlock(selected, { y: v })} />
            </div>
            <div>
              <label>Taban yüksekliği z (m)</label>
              <NumField value={sel.z} step={0.5} onChange={(v) => patchBlock(selected, { z: v })} />
            </div>
            <div>
              <label>Koltuk aralığı (m)</label>
              <NumField value={sel.seatStep} step={0.05} min={0.05} onChange={(v) => patchBlock(selected, { seatStep: v })} />
            </div>
            <div>
              <label>{sel.kind === "arc" ? "Sıra başına yarıçap artışı (m)" : "Sıra aralığı (m)"}</label>
              <NumField value={sel.rowStep} step={0.05} min={0.05} onChange={(v) => patchBlock(selected, { rowStep: v })} />
            </div>
            <div>
              <label>Sıra başına yükselme (m)</label>
              <NumField value={sel.rake} step={0.05} min={0} onChange={(v) => patchBlock(selected, { rake: v })} />
            </div>
          </div>
          <p className="muted">
            Bu bloğun koltuk dizgileri: <code>{sel.id || "?"}-1-1</code> …{" "}
            <code>
              {sel.id || "?"}-{sel.rows}-{seatsInRowOf(sel, sel.rows)}
            </code>
            {sel.kind === "arc" &&
              ` — yay blokta sıradaki koltuk sayısı değişir: 1. sırada ${seatsInRowOf(sel, 1)}, son sırada ${seatsInRowOf(sel, sel.rows)}.`}
          </p>
        </>
      )}
    </div>
  );
}
