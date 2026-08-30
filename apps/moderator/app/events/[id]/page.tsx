"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import QRCode from "qrcode";
import { control, type Event, type Room } from "@/lib/api";

// Katılımcı sayfasının telefonlardan erişilen adresi; QR bunu kodlar.
const GATEWAY_PUBLIC_URL =
  process.env.NEXT_PUBLIC_GATEWAY_PUBLIC_URL ?? "http://localhost:8080";

function joinURL(code: string): string {
  return `${GATEWAY_PUBLIC_URL}/join?code=${code}`;
}

function RoomQR({ code }: { code: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    if (ref.current) {
      void QRCode.toCanvas(ref.current, joinURL(code), { width: 140, margin: 1 });
    }
  }, [code]);
  return <canvas ref={ref} />;
}

export default function EventDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [event, setEvent] = useState<Event | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setEvent(await control.get<Event>(`/api/v1/events/${id}`));
      const resp = await control.get<{ rooms: Room[] }>(`/api/v1/events/${id}/rooms`);
      setRooms(resp.rooms);
    } catch (err) {
      setError(err instanceof Error ? err.message : "yükleme hatası");
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  async function createRoom(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await control.post(`/api/v1/events/${id}/rooms`, { name });
      setName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "oda oluşturma hatası");
    }
  }

  return (
    <>
      <h1>{event ? event.name : "…"}</h1>
      {event?.venue && <p className="muted">{event.venue}</p>}
      <form className="card" onSubmit={createRoom}>
        <h2>Yeni oda</h2>
        <label>Oda adı (ör. Kuzey Tribünü)</label>
        <input value={name} onChange={(e) => setName(e.target.value)} required />
        <button>Oda oluştur</button>
      </form>
      {error && <p className="err">{error}</p>}
      {rooms.map((room) => (
        <div className="card" key={room.id}>
          <h2>{room.name}</h2>
          <p>
            Katılım kodu: <code>{room.join_code}</code>
            <br />
            <span className="muted">Katılım adresi: {joinURL(room.join_code)}</span>
            <br />
            <span className="muted">
              Aktif gösteri sürümü: {room.active_show_version_id || "yok — Gösteriler sayfasından etkinleştirin"}
            </span>
          </p>
          <RoomQR code={room.join_code} />
        </div>
      ))}
      {rooms.length === 0 && (
        <p className="muted">Henüz oda yok; seyircilerin katılacağı ilk odayı oluşturun.</p>
      )}
    </>
  );
}
