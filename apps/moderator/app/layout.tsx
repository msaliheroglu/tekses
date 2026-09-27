import type { Metadata } from "next";
import "./globals.css";
import TopNav from "./TopNav";

export const metadata: Metadata = {
  title: "TekSes — Moderatör Paneli",
  description: "Etkinlik, gösteri ve canlı kue yönetimi",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="tr">
      <body>
        <TopNav />
        <main>{children}</main>
      </body>
    </html>
  );
}
