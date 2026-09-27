"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { control, getToken, setToken } from "@/lib/api";

type Whoami = {
  user_id: string;
  org_id: string;
  email?: string;
  organization?: string;
  member_since?: string;
};

// Hesap sayfası: oturum AÇIKKEN üyelik bilgilerini gösterir (kullanıcı
// isteği — menüden gelince yeniden giriş formu değil, hesap durumu
// görünmeli); oturum yokken giriş/kayıt formu.
export default function AccountPage() {
  const router = useRouter();
  const [me, setMe] = useState<Whoami | null>(null);
  const [checking, setChecking] = useState(true);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [organization, setOrganization] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!getToken()) {
      setChecking(false);
      return;
    }
    control
      .get<Whoami>("/api/v1/auth/whoami")
      .then(setMe)
      .catch(() => setToken("")) // süresi dolmuş/bozuk oturum → forma düş
      .finally(() => setChecking(false));
  }, []);

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

  if (checking) return <p className="muted">hesap denetleniyor…</p>;

  if (me) {
    return (
      <>
        <h1>Hesap</h1>
        <div className="card">
          <h2>{me.organization || "Organizasyon"}</h2>
          <table>
            <tbody>
              <tr>
                <th style={{ width: 160 }}>E-posta</th>
                <td>{me.email || "—"}</td>
              </tr>
              <tr>
                <th>Organizasyon</th>
                <td>{me.organization || me.org_id}</td>
              </tr>
              <tr>
                <th>Üyelik başlangıcı</th>
                <td>
                  {me.member_since
                    ? new Date(me.member_since).toLocaleDateString("tr-TR", {
                        year: "numeric",
                        month: "long",
                        day: "numeric",
                      })
                    : "—"}
                </td>
              </tr>
              <tr>
                <th>Plan</th>
                <td>
                  Pilot <span className="muted">(faturalama henüz açılmadı)</span>
                </td>
              </tr>
            </tbody>
          </table>
          <button
            type="button"
            className="ghost"
            onClick={() => {
              setToken("");
              setMe(null);
            }}
          >
            Çıkış yap
          </button>
        </div>
        <p className="muted">
          Aynı organizasyonda çok kullanıcı ve rol yönetimi yol haritasında;
          şimdilik tek moderatör hesabı yeterli.
        </p>
      </>
    );
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
