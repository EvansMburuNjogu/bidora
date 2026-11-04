// Display hints only: never store passwords, codes or challenge tokens here.
export function maskLoginEmail(email: string): string | null {
  const parts = email.trim().split("@");
  if (parts.length !== 2 || !parts[0] || !parts[1]) return null;
  const [local, domain] = parts;
  const suffix = domain.includes(".") ? domain.slice(domain.lastIndexOf(".")) : "";
  return `${local[0]}***@${domain[0]}***${suffix}`;
}

export function challengeDeadline(value: unknown): number | null {
  if (typeof value !== "string") return null;
  const deadline = Date.parse(value);
  return Number.isFinite(deadline) ? deadline : null;
}

export function countdownLabel(deadline: number, now: number): string {
  const seconds = Math.max(0, Math.ceil((deadline - now) / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
