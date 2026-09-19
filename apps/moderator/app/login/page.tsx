"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { control, setToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [organization, setOrganization] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const path = mode === "login" ? "/api/v1/auth/login" : "/api/v1/auth/register";
      const body =
        mode === "login" ? { email, password } : { organization, email, password };
      const resp = await control.post<{ token: string }>(path, body);
      setToken(resp.token);
      router.push("/events");
    } catch (err) {
      setError(err instanceof Error ? err.message : "beklenmeyen hata");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <h1>{mode === "login" ? "Giriş" : "Organizasyon kaydı"}</h1>
      <p className="muted">
        {mode === "login" ? "Hesabın yok mu? " : "Zaten hesabın var mı? "}
        <a
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setMode(mode === "login" ? "register" : "login");
            setError("");
          }}
        >
          {mode === "login" ? "Organizasyon kaydet" : "Giriş yap"}
        </a>
      </p>
      <form className="card" onSubmit={submit}>
        {mode === "register" && (
          <>
            <label>Organizasyon adı</label>
            <input value={organization} onChange={(e) => setOrganization(e.target.value)} required />
          </>
        )}
        <label>E-posta</label>
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label>Şifre (en az 8 karakter)</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        {error && <p className="err">{error}</p>}
        <button disabled={busy}>{mode === "login" ? "Giriş yap" : "Kaydol"}</button>
      </form>
    </>
  );
}
