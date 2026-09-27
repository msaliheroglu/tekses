import Link from "next/link";
import { Fragment } from "react";

// İş akışı adım şeridi: sistemin dört adımı her sayfanın başında görünür,
// bulunduğun adım vurgulanır — "gösteri/oda/sekans nerede?" karışıklığını
// tek bakışta çözer. Adım tıklanabilir; sıra dayatılmaz.
const STEPS = [
  { href: "/events", label: "Etkinlik & Oda kur" },
  { href: "/shows", label: "Gösteriyi tasarla" },
  { href: "/shows", label: "Yayınla & odada etkinleştir" },
  { href: "/console", label: "Canlı yönet" },
];

export default function Flow({ step }: { step: 1 | 2 | 3 | 4 }) {
  return (
    <div className="flow">
      {STEPS.map((s, i) => (
        <Fragment key={i}>
          {i > 0 && <span className="sep">›</span>}
          {i + 1 === step ? (
            <span className="step on">
              <span className="num">{i + 1}</span>
              {s.label}
            </span>
          ) : (
            <Link href={s.href}>
              <span className="num">{i + 1}</span>
              {s.label}
            </Link>
          )}
        </Fragment>
      ))}
    </div>
  );
}
