"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { getToken } from "@/lib/api";

export default function Home() {
  const router = useRouter();
  useEffect(() => {
    router.replace(getToken() ? "/events" : "/login");
  }, [router]);
  return <p className="muted">yönlendiriliyor…</p>;
}
