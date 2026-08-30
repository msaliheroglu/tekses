// control-api istemcisi: tüm çağrılar Next.js rewrites üzerinden /control/…
// yoluna gider; oturum token'ı tarayıcıda saklanır.

const TOKEN_KEY = "tekses_panel_token";

export function getToken(): string {
  try {
    return localStorage.getItem(TOKEN_KEY) ?? "";
  } catch {
    return "";
  }
}

export function setToken(token: string): void {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    // site verisi engelliyse oturum yalnızca sekme ömrünce yaşar
  }
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(base: string, path: string, init?: { method?: string; body?: unknown; raw?: string }): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  let body: string | undefined;
  if (init?.raw !== undefined) {
    headers["Content-Type"] = "application/json";
    body = init.raw;
  } else if (init?.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(init.body);
  }
  const resp = await fetch(base + path, { method: init?.method ?? "GET", headers, body });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new ApiError(resp.status, (data as { error?: string }).error ?? `HTTP ${resp.status}`);
  }
  return data as T;
}

export const control = {
  get: <T>(path: string) => request<T>("/control", path),
  post: <T>(path: string, body: unknown) => request<T>("/control", path, { method: "POST", body }),
  postRaw: <T>(path: string, raw: string) => request<T>("/control", path, { method: "POST", raw }),
};

// Gateway çağrıları (canlı konsol): /gw/… üzerinden; gateway'in kendi
// yönetici token'ı ayrıca başlıkla taşınır.
export async function gatewayPost<T>(path: string, body: unknown, adminToken: string): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (adminToken) headers["Authorization"] = `Bearer ${adminToken}`;
  const resp = await fetch("/gw" + path, { method: "POST", headers, body: JSON.stringify(body) });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new ApiError(resp.status, (data as { error?: string }).error ?? `HTTP ${resp.status}`);
  }
  return data as T;
}

// Ortak tipler (control-api yanıtları).
export type Event = { id: string; name: string; venue?: string; created_at: string };
export type Room = {
  id: string;
  event_id: string;
  name: string;
  join_code: string;
  active_show_version_id?: string;
};
export type Show = { id: string; title: string; created_at: string };
export type ShowVersion = { id: string; show_id: string; version: number; sha256: string; created_at: string };
