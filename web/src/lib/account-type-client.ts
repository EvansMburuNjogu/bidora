import type { AccountTypeSelectionRequest } from "@/contracts/auth.generated";

export async function selectAccountType(
  payload: AccountTypeSelectionRequest,
  csrfToken: string,
  idempotencyKey: string,
): Promise<Response> {
  return fetch("/api/session/account-type", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrfToken,
      "Idempotency-Key": idempotencyKey,
    },
    body: JSON.stringify(payload),
  });
}
