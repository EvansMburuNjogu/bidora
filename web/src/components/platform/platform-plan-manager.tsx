"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import { parseManagedPlans, parsePlanReceipt, type ManagedPlanItem } from "@/lib/platform-plan-management-contract";

type Values = Record<"monthly_price" | "annual_price" | "additional_licence_monthly_price" | "bid_workspaces" | "included_licences" | "storage_bytes" | "document_versions" | "ai_credits" | "concurrent_ai_jobs", string>;
function initial(item: ManagedPlanItem): Values { const v = item.draft ?? item.published; return { monthly_price: v.monthly_price.amount, annual_price: v.annual_price.amount, additional_licence_monthly_price: v.additional_licence_monthly_price.amount, bid_workspaces: String(v.bid_workspaces), included_licences: String(v.included_licences), storage_bytes: String(v.storage_bytes), document_versions: String(v.document_versions), ai_credits: String(v.ai_credits), concurrent_ai_jobs: String(v.concurrent_ai_jobs) }; }
const fields = [["monthly_price", "Monthly base (KES)"], ["annual_price", "Annual base (KES)"], ["additional_licence_monthly_price", "Extra licence / month (KES)"], ["bid_workspaces", "Bid workspaces"], ["included_licences", "Included licences"], ["storage_bytes", "Storage bytes"], ["document_versions", "Document versions"], ["ai_credits", "AI credits"], ["concurrent_ai_jobs", "Concurrent AI jobs"]] as const;
function PlanEditor({ item, manage, publish, generation, csrf, reload }: { item: ManagedPlanItem; manage: boolean; publish: boolean; generation: number; csrf: string | null; reload: () => Promise<void> }) {
  const [values, setValues] = useState<Values>(() => initial(item));
  const [busy, setBusy] = useState(false), [message, setMessage] = useState("");
  const key = useRef<string | null>(null), locked = useRef(false);
  useEffect(() => { const timer = window.setTimeout(() => { setValues(initial(item)); setMessage(""); key.current = null; }, 0); return () => window.clearTimeout(timer); }, [item]);
  async function submit(action: "drafts" | "publish") {
    if (locked.current || !csrf) return;
    const moneyFields = ["monthly_price", "annual_price", "additional_licence_monthly_price"] as const;
    const numericFields = ["bid_workspaces", "included_licences", "storage_bytes", "document_versions", "ai_credits", "concurrent_ai_jobs"] as const;
    if (action === "drafts" && (moneyFields.some((f) => !/^(0|[1-9]\d*)\.\d{2}$/.test(values[f]) || Number(values[f]) <= 0) || numericFields.some((f) => !/^[1-9]\d*$/.test(values[f]) || !Number.isSafeInteger(Number(values[f]))))) { setMessage("Enter positive whole limits and two-decimal KES amounts."); return; }
    locked.current = true; setBusy(true); setMessage("");
    const idempotency = key.current ?? crypto.randomUUID(); key.current = idempotency;
    const body = action === "publish" ? { expected_head_version: item.head_version, version: item.draft?.version } : { expected_head_version: item.head_version, ...Object.fromEntries(moneyFields.map((f) => [f, { currency: "KES", amount: values[f] }])), ...Object.fromEntries(numericFields.map((f) => [f, Number(values[f])])) };
    try {
      const response = await fetch(`/api/platform/plans/${item.plan_key}/${action}`, { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": idempotency }, body: JSON.stringify(body) });
      const receipt = response.ok ? parsePlanReceipt(await response.json()) : null;
      if (!receipt || receipt.plan_key !== item.plan_key || receipt.published !== (action === "publish")) { setMessage(response.status === 409 ? "Plan changed or a second administrator is required. Refresh before a new action." : response.status === 403 ? "This operation requires a verified Billing administrator." : "The outcome is uncertain. Retry unchanged with the same key."); return; }
      key.current = null; setMessage(action === "publish" ? `Published version ${receipt.version}. Existing terms are unchanged.` : `Draft version ${receipt.version} saved; a different administrator must publish it.`);
      await reload();
    } catch { setMessage("The outcome is uncertain. Retry unchanged with the same key."); }
    finally { locked.current = false; setBusy(false); }
  }
  return <section className="space-y-3 rounded-lg border bg-card p-4" aria-label={`${item.plan_key} management`}><div><h3 className="font-semibold">{item.plan_key}</h3><p className="text-xs text-muted-foreground">Published v{item.active_version} · Head {item.head_version}{item.draft ? ` · Latest draft v${item.draft.version}` : " · No draft"}</p></div>
    {manage && <div className="grid gap-3 sm:grid-cols-2">{fields.map(([name, label]) => <label key={name} className="grid gap-1 text-sm">{label}<Input disabled={busy} value={values[name]} onChange={(event) => { key.current = null; setValues((previous) => ({ ...previous, [name]: event.target.value })); }} /></label>)}</div>}
    <div className="flex flex-wrap gap-2">{manage && <Button type="button" disabled={busy || !csrf} onClick={() => void submit("drafts")}>Save new draft</Button>}{publish && item.draft && <Button type="button" variant="outline" disabled={busy || !csrf} onClick={() => void submit("publish")}>Publish draft v{item.draft.version}</Button>}</div>
    {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
  </section>;
}
export function PlatformPlanManager({ onPublished }: { onPublished: () => Promise<void> }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const read = resolution.capabilities.includes("platform.plan.read"), manage = resolution.capabilities.includes("platform.plan.manage"), publish = resolution.capabilities.includes("platform.plan.publish");
  const [items, setItems] = useState<{ generation: number; plans: ManagedPlanItem[] } | null>(null), [error, setError] = useState(""), [busy, setBusy] = useState(false);
  const sequence = useRef(0);
  const load = useCallback(async () => {
    if (!generation || !read) return;
    const current = ++sequence.current; setBusy(true); setError("");
    try { const response = await fetch("/api/platform/plans/management", { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const plans = response.ok ? parseManagedPlans(await response.json()) : null;
      if (!plans) { setError(response.status === 403 ? "Verified Billing access is required." : "Managed plan versions are unavailable."); return; }
      setItems({ generation, plans });
    } catch { if (current === sequence.current) setError("Managed plan versions are unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }, [generation, read, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); const ref = sequence; return () => { window.clearTimeout(timer); ref.current++; }; }, [load]);
  const visible = items?.generation === generation && read ? items.plans : null;
  async function reload() { await Promise.all([load(), onPublished()]); }
  return <section className="space-y-3" aria-label="Managed paid plans"><div><h2 className="font-semibold">Paid plan versions</h2><p className="text-xs text-muted-foreground">Only Basic, Growth and Professional have published fixed-price versions. Trial and Enterprise terms are not editable here. Publication affects subsequent quotes, not existing subscriptions or invoices.</p></div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {busy && !visible && <p role="status" className="text-sm text-muted-foreground">Loading versioned plans…</p>}
    {visible && <div className="grid gap-3 xl:grid-cols-3">{visible.map((item) => <PlanEditor key={`${generation}:${item.plan_key}:${item.head_version}`} item={item} manage={manage} publish={publish} generation={generation!} csrf={csrfToken} reload={reload} />)}</div>}
    {visible && <Button type="button" variant="outline" disabled={busy} onClick={() => void load()}>Refresh versions</Button>}
  </section>;
}
