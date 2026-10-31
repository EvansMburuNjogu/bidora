export async function revokeSession(sessionID: string, csrfToken: string, idempotencyKey: string): Promise<Response> {
  return fetch(`/api/session/sessions/${encodeURIComponent(sessionID)}`, {
    method: "DELETE",
    credentials: "include",
    cache: "no-store",
    headers: { "X-CSRF-Token": csrfToken, "Idempotency-Key": idempotencyKey },
  });
}

export async function revokeAllSessions(csrfToken: string): Promise<Response> {
  return fetch("/api/session/sessions", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: { "X-CSRF-Token": csrfToken },
  });
}
