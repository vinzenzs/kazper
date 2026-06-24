// A thin typed fetch wrapper over the Kazper REST API. Same-origin: the SPA is
// served from `/` and the API from `/api/v1` of the same host, so no base URL
// and no CORS. The browser holds the HTTP Basic credential natively and
// auto-attaches it to every same-origin request — there is no token handling
// here on purpose.

export const API_BASE = "/api/v1";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(`API ${status}: ${code}`);
    this.name = "ApiError";
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { Accept: "application/json" },
    // Ensure the browser attaches its cached Basic credential.
    credentials: "same-origin",
  });
  if (!res.ok) {
    let code = res.statusText || "request_failed";
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) code = body.error;
    } catch {
      // non-JSON error body — keep the status text
    }
    throw new ApiError(res.status, code);
  }
  return (await res.json()) as T;
}
