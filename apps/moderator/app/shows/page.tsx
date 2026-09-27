"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { control, type Show } from "@/lib/api";
import Flow from "../Flow";

export default function ShowsPage() {
  const [shows, setShows] = useState<Show[]>([]);
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const resp = await control.get<{ shows: Show[] }>("/api/v1/shows");
      setShows(resp.shows);
    } catch (err) {
      setError(err instanceof Error ? err.message : "yükleme hatası");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await control.post("/api/v1/shows", { title });
      setTitle("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "oluşturma hatası");
    }
  }

  return (
    <>
      <Flow step={2} />
      <h1>Gösteriler</h1>
      <p className="muted">
        Gösteri = koreografinin tamamı: sekanslar (şarkılar/bölümler), sözler,
        ışık adımları, mekân planı ve efektler. Odadan bağımsızdır — aynı
        gösteri birden çok odada etkinleştirilebilir. Düzenlemek için gösteriye
        tıklayın.
      </p>
      <form className="card" onSubmit={create}>
        <h2>Yeni gösteri</h2>
        <label>Başlık</label>
        <input value={title} onChange={(e) => setTitle(e.target.value)} required />
        <button>Oluştur</button>
      </form>
      {error && <p className="err">{error}</p>}
      <div className="card">
        <h2>Kayıtlı gösteriler</h2>
        {shows.length === 0 ? (
          <p className="muted">Henüz gösteri yok.</p>
        ) : (
          <table>
            <thead><tr><th>Başlık</th><th></th></tr></thead>
            <tbody>
              {shows.map((s) => (
                <tr key={s.id}>
                  <td>{s.title}</td>
                  <td><Link href={`/shows/${s.id}`}>düzenle & yayınla →</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
