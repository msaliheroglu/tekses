"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getToken } from "@/lib/api";

// Ana sayfa: sistemin nasıl işlediğini dört adımda anlatan pano.
// Oturum yoksa girişe yönlendirir.
const STEPS = [
  {
    href: "/events",
    title: "1 · Etkinlik & Oda kur",
    desc:
      "Etkinliği (konser, maç) ve içindeki odaları (tribün, salon) oluştur. " +
      "Her oda bir katılım kodu ve QR alır — seyirci telefonuyla bu koda katılır.",
    action: "Etkinliklere git",
  },
  {
    href: "/shows",
    title: "2 · Gösteriyi tasarla",
    desc:
      "Gösteri = koreografinin tamamı. İçinde sekanslar (şarkılar/bölümler), " +
      "sözler, ekran/fener adımları, dilersen mekân planı ve koltuğa göre " +
      "dalga/bayrak/slogan efektleri var. 3B önizlemeyle tribünde nasıl " +
      "duracağını görürsün.",
    action: "Gösterilere git",
  },
  {
    href: "/shows",
    title: "3 · Yayınla & odada etkinleştir",
    desc:
      "\"Doğrula ve yayınla\" değişmez bir sürüm üretir; sürümü bir odada " +
      "etkinleştirince o odaya katılan her telefon paketi indirir. Düzeltme " +
      "gerekirse yeni sürüm yayınlar, onu etkinleştirirsin.",
    action: "Sürümleri yönet",
  },
  {
    href: "/console",
    title: "4 · Canlı yönet",
    desc:
      "Gösteri anında Canlı Konsol'dan GO ile koreografiyi başlatır, " +
      "önizlemeden takip eder, gerekirse HOLD/STOP/BLACKOUT ile müdahale " +
      "edersin. Katılımcı sayısı ve senkron kalitesi de burada.",
    action: "Canlı Konsol'a git",
  },
];

export default function Home() {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  useEffect(() => {
    if (!getToken()) router.replace("/login");
    else setReady(true);
  }, [router]);
  if (!ready) return <p className="muted">yönlendiriliyor…</p>;
  return (
    <>
      <h1>Hoş geldin</h1>
      <p className="muted">
        TekSes dört adımda işler — her adım üstteki menüden de erişilebilir.
      </p>
      {STEPS.map((s) => (
        <div className="card" key={s.title}>
          <h2>{s.title}</h2>
          <p className="muted" style={{ fontSize: 14 }}>{s.desc}</p>
          <Link href={s.href}>{s.action} →</Link>
        </div>
      ))}
    </>
  );
}
