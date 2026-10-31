// Presentation only: never display upstream messages, diagnostics or submitted values.
export const networkSubmissionError = "Connection lost. Check the current record before retrying.";
export const unconfirmedSubmissionError = "Couldn't confirm the outcome. Refresh before retrying.";
export function submissionErrorMessage(status: number, retryAfter: string | null = null, now = Date.now()): string {
  if (status === 401) return "Please sign in again.";
  if (status === 403) return "You don't have permission to do this. Recent verification may be required.";
  if (status === 409 || status === 412) return "This record changed. Refresh it before trying again.";
  if (status === 400 || status === 422) return "Check the form values and try again.";
  if (status === 429) {
    const seconds = retryAfter && /^\d{1,5}$/.test(retryAfter) ? Number(retryAfter) : retryAfter ? Math.ceil((Date.parse(retryAfter) - now) / 1000) : NaN;
    return Number.isFinite(seconds) && seconds > 0 && seconds <= 86400 ? `Too many requests. Try again in ${seconds} seconds.` : "Too many requests. Please try again later.";
  }
  if (status === 404) return "This record is no longer available. Refresh and try again.";
  if (status >= 500) return "Couldn't complete this action. Please try again.";
  return unconfirmedSubmissionError;
}
export async function submissionFieldErrors<T extends string>(response: Response, fields: readonly T[]): Promise<Partial<Record<T, string>>> {
  const result: Partial<Record<T, string>> = {};
  if (response.status !== 400 && response.status !== 422) return result;
  try {
    const payload = await response.json();
    const errors: unknown = payload?.error?.field_errors;
    if (!Array.isArray(errors) || errors.length > 100) return result;
    for (const entry of errors) {
      if (entry && typeof entry === "object" && typeof entry.field === "string" && fields.includes(entry.field as T)) result[entry.field as T] = "Check this value and try again.";
    }
  } catch { /* Malformed failures use the safe status message, never raw text. */ }
  return result;
}
