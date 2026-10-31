type Tone = "success" | "warning" | "danger" | "neutral";
const success = new Set(["ACTIVE", "PAID", "DELIVERED", "COMPLETED", "APPROVED", "PASSED", "RESOLVED", "SUCCESSFUL"]);
const warning = new Set(["PENDING", "PROCESSING", "AWAITING_REVIEW", "OPEN", "REVERSED", "IN_PROGRESS", "SENT"]);
const danger = new Set(["FAILED", "REJECTED", "OVERDUE", "SUSPENDED", "DISABLED", "PAST_DUE"]);
const styles: Record<Tone, string> = {
  success: "bg-success/10 text-success-foreground",
  warning: "bg-warning/10 text-warning-foreground",
  danger: "bg-destructive/10 text-destructive-foreground",
  neutral: "bg-muted text-muted-foreground",
};

export function PlatformStatus({ value }: { value: string }) {
  const key = value.toUpperCase().replaceAll(" ", "_");
  const tone: Tone = success.has(key) ? "success" : warning.has(key) ? "warning" : danger.has(key) ? "danger" : "neutral";
  return <span className={`inline-flex max-w-full rounded-md px-2 py-0.5 text-xs font-medium ${styles[tone]}`}>{value.replaceAll("_", " ")}</span>;
}
