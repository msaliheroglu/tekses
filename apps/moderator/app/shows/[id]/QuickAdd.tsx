"use client";

import { useMemo, useState } from "react";
import { imageToBitmap, padBitmapCols, textToBitmap } from "@/lib/bitmapGen";
import {
  emptyScreenStep,
  emptySequence,
  emptyTorchStep,
  type EditorScreenStep,
  type EditorSequence,
} from "@/lib/manifestEditor";
import { blockPlacement, seatsInRowOf, type EditorVenue } from "@/lib/venueEditor";

// Hazır koreografi sihirbazı (F4.5-6): "sistem komplike" geri bildirimine
// yanıt — en çok istenen üç efekt tek ekranda, tek tıkla eklenir. Sihirbaz
// hazır ayarlı bir SEKANS üretir; ekleyince akordeonda açılır ve her alanı
// oradan ince ayarlanabilir. Önizleme kartı sonucu hemen gösterir.
//
// Bayrak/slogan TÜM MEKÂNA YAYILMAZ: "Nereye?" ile bir tribün seçilir
// (varsayılan en büyük blok) ve desen o bloğun penceresine, o bloğun koltuk
// çözünürlüğünde, hücre oranı düzeltilerek oturtulur — bayrak tribünde
// bayrak gibi görünür, stadyumu şerit şerit sarmaz.

type Kind = "wave" | "slogan" | "flag";

const SPEEDS = [
  { label: "yavaş", periodMs: 6000 },
  { label: "orta", periodMs: 3000 },
  { label: "hızlı (sınır)", periodMs: 1000 },
];

export default function QuickAdd({
  venue,
  onAdd,
}: {
  venue: EditorVenue | null;
  onAdd: (seq: EditorSequence) => void;
}) {
  const [kind, setKind] = useState<Kind>("wave");
  const [color, setColor] = useState("#d92b2b");
  const [speed, setSpeed] = useState(1);
  const [dir, setDir] = useState<"ring+" | "ring-" | "u+">("ring+");
  const [withTorch, setWithTorch] = useState(true);
  const [target, setTarget] = useState("auto"); // "auto" | blok id | "all"
  const [text, setText] = useState("");
  const [durationSec, setDurationSec] = useState(30);
  const [err, setErr] = useState("");

  // Varsayılan hedef: en çok koltuklu blok (stadyumda kale arkası/uzun kenar,
  // salonda tek blok) — moderatör hiçbir şey seçmese de doğru yere düşer.
  const autoBlockId = useMemo(() => {
    if (!venue || venue.blocks.length === 0) return "";
    let best = "", bestN = -1;
    for (const b of venue.blocks) {
      let n = 0;
      for (let row = 1; row <= b.rows; row++) n += seatsInRowOf(b, row);
      if (n > bestN) { bestN = n; best = b.id; }
    }
    return best;
  }, [venue]);

  const targetBlockId = target === "auto" ? autoBlockId : target === "all" ? "" : target;
  const placement =
    venue && targetBlockId ? blockPlacement(venue, targetBlockId) : null;

  function baseSequence(title: string): EditorSequence {
    const seq = emptySequence(title);
    seq.durationMs = Math.max(5, durationSec) * 1000;
    seq.screen = [];
    return seq;
  }

  // Yerleşimi ekran adımına uygular (eksen/yön/blok filtresi/pencere).
  function placeStep(st: EditorScreenStep): EditorScreenStep {
    if (!placement || !targetBlockId) return st;
    return {
      ...st,
      effectAxis: placement.axis,
      effectReverse: placement.reverse,
      effectBlocks: [targetBlockId],
      effectArea: { ...placement.area },
    };
  }

  function add() {
    setErr("");
    if (kind === "wave") {
      const axis = dir.startsWith("ring") ? ("ring" as const) : ("u" as const);
      const reverse = dir === "ring-";
      const seq = baseSequence("Meksika dalgası");
      seq.screen = [
        {
          ...emptyScreenStep(0),
          color,
          effectKind: "wave",
          effectAxis: axis,
          effectReverse: reverse,
          effectPeriodMs: SPEEDS[speed].periodMs,
          effectWidth: 0.2,
        },
      ];
      if (withTorch) {
        // Fenerler de dalgaya katılır: aynı yön/hızla açılıp söner.
        seq.torch = [
          {
            ...emptyTorchStep(0),
            effectKind: "wave",
            effectAxis: axis,
            effectReverse: reverse,
            effectPeriodMs: SPEEDS[speed].periodMs,
            effectWidth: 0.2,
          },
        ];
      }
      onAdd(seq);
      return;
    }
    if (kind === "slogan") {
      // Harf oranı hücrede korunur (yatay ön-germe = 1/hücre oranı).
      const r = textToBitmap(text, color, 9, placement ? 1 / placement.cellAspect : 1);
      if (!r.bitmap) {
        setErr(r.error ?? "slogan üretilemedi — metni kontrol edin");
        return;
      }
      const seq = baseSequence(`Slogan: ${text.trim()}`);
      let st: EditorScreenStep = { ...emptyScreenStep(0), color, effectKind: "bitmap" };
      if (placement) {
        // Kayarken 1 koltuk ≈ 1 sütun kalsın diye blok genişliğine yastıkla;
        // 9 satırlık yazı bloğun ortasında dar bir bant kaplar (dikeyde
        // pencereye gerilirse harfler kule gibi uzar).
        const bm = padBitmapCols(r.bitmap, placement.cols);
        const a = placement.area;
        const band = Math.min(1, 9 / placement.rows) * (a.w1 - a.w0);
        const mid = (a.w0 + a.w1) / 2;
        const r3 = (x: number) => Math.round(x * 1000) / 1000;
        st = placeStep(st);
        st.effectBitmap = bm;
        st.effectScrollMs = bm.rows[0].length * 400;
        st.effectArea = { u0: a.u0, u1: a.u1, w0: r3(mid - band / 2), w1: r3(mid + band / 2) };
      } else {
        // Tüm mekân: stadyum turu — yazı ring ekseninde çepeçevre kayar.
        st.effectAxis = "ring";
        st.effectBitmap = r.bitmap;
        st.effectScrollMs = r.bitmap.rows[0].length * 400;
      }
      seq.screen = [st];
      onAdd(seq);
      setText("");
    }
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
      // Tribün seçiliyse çözünürlük o bloğun koltuk grid'idir ve en-boy
      // oranı hücre boyutlarıyla (metre) korunur.
      const r = placement
        ? imageToBitmap(img, placement.cols, placement.rows, placement.cellAspect)
        : imageToBitmap(img, 48, 16);
      if (!r.bitmap) {
        setErr(r.error ?? "görüntü işlenemedi");
        return;
      }
      const seq = baseSequence(`Bayrak: ${file.name.replace(/\.[^.]+$/, "")}`);
      seq.screen = [
        placeStep({
          ...emptyScreenStep(0),
          color,
          effectKind: "bitmap",
          effectBitmap: r.bitmap,
          effectScrollMs: 0,
        }),
      ];
      onAdd(seq);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      setErr("görüntü açılamadı");
    };
    img.src = url;
  }

  return (
    <div className="card">
      <h2>Hazır koreografi ekle — tek tıkla</h2>
      <p className="muted">
        En çok kullanılan üç efekt hazır ayarlarla sekans olarak eklenir;
        eklendikten sonra aşağıdaki kartından her ayrıntısı değiştirilebilir.
        Sonucu üstteki <b>Önizleme</b>'de hemen izleyin.
      </p>
      <div className="row">
        <div>
          <label>Ne eklensin?</label>
          <select value={kind} onChange={(e) => setKind(e.target.value as Kind)}>
            <option value="wave">🌊 Meksika dalgası</option>
            <option value="slogan">🔤 Kayan slogan (yazı)</option>
            <option value="flag">🚩 Bayrak / logo (görüntüden)</option>
          </select>
        </div>
        {kind !== "flag" && (
          <div style={{ flex: "0 0 70px" }}>
            <label>{kind === "wave" ? "Dalga rengi" : "Yazı rengi"}</label>
            <input
              type="color"
              value={color}
              style={{ padding: 2, height: 42 }}
              onChange={(e) => setColor(e.target.value)}
            />
          </div>
        )}
        {kind === "wave" && (
          <>
            <div>
              <label>Yön</label>
              <select value={dir} onChange={(e) => setDir(e.target.value as typeof dir)}>
                <option value="ring+">stadyum turu ↺</option>
                <option value="ring-">stadyum turu ↻</option>
                <option value="u+">düz — soldan sağa</option>
              </select>
            </div>
            <div>
              <label>Hız</label>
              <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
                {SPEEDS.map((s, i) => (
                  <option key={i} value={i}>{s.label}</option>
                ))}
              </select>
            </div>
            <div style={{ flex: "0 0 auto", alignSelf: "flex-end", paddingBottom: 10 }}>
              <label style={{ display: "inline-flex", gap: 6, alignItems: "center", margin: 0 }}>
                <input
                  type="checkbox"
                  checked={withTorch}
                  onChange={(e) => setWithTorch(e.target.checked)}
                  style={{ width: "auto" }}
                />
                fenerler de katılsın
              </label>
            </div>
          </>
        )}
        {kind !== "wave" && venue && venue.blocks.length > 0 && (
          <div>
            <label>Nereye?</label>
            <select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="auto">
                otomatik — en büyük tribün{autoBlockId ? ` (${autoBlockId})` : ""}
              </option>
              {venue.blocks.map((b) => (
                <option key={b.id} value={b.id}>{b.id}</option>
              ))}
              <option value="all">
                {kind === "slogan" ? "çepeçevre — stadyum turu" : "tüm mekâna yay"}
              </option>
            </select>
          </div>
        )}
        {kind === "slogan" && (
          <div style={{ flex: "2 1 200px" }}>
            <label>Slogan metni</label>
            <input value={text} onChange={(e) => setText(e.target.value)} placeholder="ör. ŞAMPİYON" />
          </div>
        )}
        {kind === "flag" && (
          <div style={{ flex: "2 1 200px" }}>
            <label>Görüntü seç (eklenince otomatik işlenir)</label>
            <input type="file" accept="image/*" onChange={onPickImage} />
          </div>
        )}
        <div style={{ flex: "0 1 110px" }}>
          <label>Süre (sn)</label>
          <input
            type="number"
            min={5}
            value={durationSec}
            onChange={(e) => setDurationSec(Math.max(5, Number(e.target.value) || 5))}
          />
        </div>
        {kind !== "flag" && (
          <div style={{ flex: "0 0 auto", alignSelf: "flex-end" }}>
            <button type="button" className="secondary" onClick={add}>
              + Ekle
            </button>
          </div>
        )}
      </div>
      {kind !== "wave" && placement && (
        <p className="muted" style={{ margin: "4px 0 0" }}>
          Desen <b>{targetBlockId}</b> bloğuna {placement.cols}×{placement.rows} koltuk
          çözünürlüğünde, en-boy oranı korunarak oturtulur; ince ayar için
          eklenen sekans kartındaki yerleştirme tuvalini kullanın.
        </p>
      )}
      {err && <p className="err">{err}</p>}
    </div>
  );
}
