"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError, control, type Event } from "@/lib/api";
import { useRouter } from "next/navigation";

export default function EventsPage() {
  const router = useRouter();
  const [events, setEvents] = useState<Event[]>([]);
  const [name, setName] = useState("");
  const [venue, setVenue] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const resp = await control.get<{ events: Event[] }>("/api/v1/events");
      setEvents(resp.events);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) router.push("/login");
      else setError(err instanceof Error ? err.message : "yükleme hatası");
    }
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await control.post("/api/v1/events", { name, venue });
      setName("");
      setVenue("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "oluşturma hatası");
    }
  }

  return (
    <>
      <h1>Etkinlikler</h1>
      <form className="card" onSubmit={create}>
        <h2>Yeni etkinlik</h2>
        <div className="row">
          <div>
            <label>Ad</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div>
            <label>Mekân</label>
            <input value={venue} onChange={(e) => setVenue(e.target.value)} />
          </div>
        </div>
        <button>Oluştur</button>
      </form>
      {error && <p className="err">{error}</p>}
      <div className="card">
        <h2>Kayıtlı etkinlikler</h2>
        {events.length === 0 ? (
          <p className="muted">Henüz etkinlik yok.</p>
        ) : (
          <table>
            <thead>
              <tr><th>Ad</th><th>Mekân</th><th></th></tr>
            </thead>
            <tbody>
              {events.map((ev) => (
                <tr key={ev.id}>
                  <td>{ev.name}</td>
                  <td>{ev.venue || "—"}</td>
                  <td><Link href={`/events/${ev.id}`}>odalar →</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
