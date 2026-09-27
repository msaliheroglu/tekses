"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

// Üst gezinme: bulunduğun bölüm vurgulanır — moderatör akışın neresinde
// olduğunu her an görür (iş akışı adımları ayrıca sayfa başlarındaki
// .flow şeridinde).
const LINKS = [
  { href: "/events", label: "Etkinlik & Odalar" },
  { href: "/shows", label: "Gösteriler" },
  { href: "/console", label: "Canlı Konsol" },
];

export default function TopNav() {
  const path = usePathname();
  return (
    <nav className="topbar">
      <div className="inner">
        <Link href="/" className="brand">
          TekSes
        </Link>
        {LINKS.map((l) => (
          <Link key={l.href} href={l.href} className={path.startsWith(l.href) ? "active" : ""}>
            {l.label}
          </Link>
        ))}
        <span className="spacer" />
        <Link href="/login" className={path.startsWith("/login") ? "active" : ""}>
          Hesap
        </Link>
      </div>
    </nav>
  );
}
