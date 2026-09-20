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
async function gatewayRequest<T>(path: string, adminToken: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...((init?.headers as Record<string, string>) ?? {}),
  };
  // Terminalden kopyalanan token'ın başına/sonuna bulaşan boşluk ve satır
  // sonu 401'e yol açar; kırparak gönderilir. Alan boşsa panel oturum token'ı
  // kullanılır: gateway, panel oturumlarını control-api'ye doğrulatır —
  // moderatörün ayrıca TEKSES_ADMIN_TOKEN bilmesi gerekmez.
  const token = adminToken.trim() || getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const resp = await fetch("/gw" + path, { ...init, headers });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new ApiError(resp.status, (data as { error?: string }).error ?? `HTTP ${resp.status}`);
  }
  return data as T;
}

export function gatewayPost<T>(path: string, body: unknown, adminToken: string): Promise<T> {
  return gatewayRequest<T>(path, adminToken, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function gatewayGet<T>(path: string, adminToken: string): Promise<T> {
  return gatewayRequest<T>(path, adminToken);
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

// Manifest özeti (konsolun kue seçicisi için gereken kadarı).
export type ManifestSummary = {
  sequences: { id: string; title: string }[];
  hasProgram: boolean;
};

export async function fetchManifestSummary(showVersionID: string): Promise<ManifestSummary> {
  const resp = await control.get<{
    manifest: { sequences?: { id: string; title?: string }[]; program?: unknown[] };
  }>(`/api/v1/show-versions/${showVersionID}`);
  return {
    sequences: (resp.manifest.sequences ?? []).map((s) => ({ id: s.id, title: s.title ?? s.id })),
    hasProgram: (resp.manifest.program ?? []).length > 0,
  };
}

// Ses varlığı yükleme: ham gövde, Content-Type dosyanın ses türü.
export async function uploadAsset(
  file: File,
): Promise<{ asset_id: string; url: string; bytes: number }> {
  const headers: Record<string, string> = {
    "Content-Type": file.type || "audio/mpeg",
  };
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const resp = await fetch("/control/api/v1/assets", { method: "POST", headers, body: file });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new ApiError(resp.status, (data as { error?: string }).error ?? `HTTP ${resp.status}`);
  }
  return data as { asset_id: string; url: string; bytes: number };
}

// Deneysel: sesten zamanlı söz taslağı çıkarma (sunucuda TEKSES_TRANSCRIBER
// yapılandırılmışsa; değilse 501 döner).
export type TranscriptionLyricLine = { at_ms: number; duration_ms: number; text: string };
export type TranscriptionResult = {
  transcription_id: string;
  status: "running" | "done" | "error";
  lyric_lines?: TranscriptionLyricLine[];
  error?: string;
};

export function startTranscription(assetId: string): Promise<{ transcription_id: string }> {
  return control.post(`/api/v1/assets/${assetId}/transcribe`, {});
}

export function getTranscription(id: string): Promise<TranscriptionResult> {
  return control.get(`/api/v1/transcriptions/${id}`);
}

// Gateway çalıştırma kaydı (asgari telemetri).
export type RunRecord = {
  run_id?: string;
  kind: string;
  cue_id?: string;
  room_id?: string;
  fire_at_server_ms?: number;
  issued_at_server_ms: number;
  clients: number;
};
