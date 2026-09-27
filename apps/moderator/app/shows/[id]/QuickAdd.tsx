"use client";

import { useState } from "react";
import { imageToBitmap, textToBitmap } from "@/lib/bitmapGen";
import {
  emptyScreenStep,
  emptySequence,
  type EditorSequence,
} from "@/lib/manifestEditor";

// Hazır koreografi sihirbazı (F4.5-6): "sistem komplike" geri bildirimine
// yanıt — en çok istenen üç efekt tek ekranda, tek tıkla eklenir. Sihirbaz
// hazır ayarlı bir SEKANS üretir; ekleyince akordeonda açılır ve her alanı
// oradan ince ayarlanabilir. Önizleme kartı sonucu hemen gösterir.

type Kind = "wave" | "slogan" | "flag";

const SPEEDS = [
  { label: "yavaş", periodMs: 6000 },
  { label: "orta", periodMs: 3000 },
  { label: "hızlı (sınır)", periodMs: 1000 },
];

export default function QuickAdd({ onAdd }: { onAdd: (seq: EditorSequence) => void }) {
  const [kind, setKind] = useState<Kind>("wave");
  const [color, setColor] = useState("#d92b2b");
  const [speed, setSpeed] = useState(1);
  const [text, setText] = useState("");
  const [durationSec, setDurationSec] = useState(30);
  const [err, setErr] = useState("");

  function baseSequence(title: string): EditorSequence {
    const seq = emptySequence(title);
    seq.durationMs = Math.max(5, durationSec) * 1000;
    seq.screen = [];
    return seq;
  }

  function add() {
    setErr("");
    if (kind === "wave") {
      const seq = baseSequence("Meksika dalgası");
      seq.screen = [
        {
          ...emptyScreenStep(0),
          color,
          effectKind: "wave",
          effectPeriodMs: SPEEDS[speed].periodMs,
          effectWidth: 0.2,
        },
      ];
      onAdd(seq);
      return;
    }
    if (kind === "slogan") {
      const r = textToBitmap(text, color);
      if (!r.bitmap) {
        setErr(r.error ?? "slogan üretilemedi — metni kontrol edin");
        return;
      }
      const seq = baseSequence(`Slogan: ${text.trim()}`);
      seq.screen = [
        {
          ...emptyScreenStep(0),
          color,
          effectKind: "bitmap",
          effectBitmap: r.bitmap,
          effectScrollMs: r.bitmap.rows[0].length * 400,
        },
      ];
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
      const r = imageToBitmap(img, 48, 16);
      if (!r.bitmap) {
        setErr(r.error ?? "görüntü işlenemedi");
        return;
      }
      const seq = baseSequence(`Bayrak: ${file.name.replace(/\.[^.]+$/, "")}`);
      seq.screen = [
        {
          ...emptyScreenStep(0),
          color,
          effectKind: "bitmap",
          effectBitmap: r.bitmap,
          effectScrollMs: 0,
        },
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
          <div>
            <label>Hız</label>
            <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
              {SPEEDS.map((s, i) => (
                <option key={i} value={i}>{s.label}</option>
              ))}
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
      {err && <p className="err">{err}</p>}
    </div>
  );
}
