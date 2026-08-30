import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "TekSes — Moderatör Paneli",
  description: "Etkinlik, gösteri ve canlı kue yönetimi",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="tr">
      <body>
        <nav className="topbar">
          <Link href="/" className="brand">TekSes</Link>
          <Link href="/events">Etkinlikler</Link>
          <Link href="/shows">Gösteriler</Link>
          <Link href="/console">Canlı Konsol</Link>
          <span className="spacer" />
          <Link href="/login">Oturum</Link>
        </nav>
        <main>{children}</main>
      </body>
    </html>
  );
}
