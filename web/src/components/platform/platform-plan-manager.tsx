"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";
import { normalizeNumericInput } from "@/lib/numeric-input";
import { useSession } from "@/components/session/session-provider";
import { parseManagedPlans, parsePlanReceipt, type ManagedPlanItem } from "@/lib/platform-plan-management-contract";
import type { PlanCatalogue, PlanCatalogueEntry, PlanKey, PlanMoney } from "@/lib/platform-plan-catalogue-contract";

type Values = Record<"monthly_price" | "annual_price" | "additional_licence_monthly_price" | "bid_workspaces" | "included_licences" | "storage_bytes" | "document_versions" | "ai_credits" | "concurrent_ai_jobs", string>;
type RequestKey = { fingerprint: string; value: string };
function initial(item: ManagedPlanItem): Values { const v = item.draft ?? item.published; return { monthly_price: v.monthly_price.amount, annual_price: v.annual_price.amount, additional_licence_monthly_price: v.additional_licence_monthly_price.amount, bid_workspaces: String(v.bid_workspaces), included_licences: String(v.included_licences), storage_bytes: String(v.storage_bytes), document_versions: String(v.document_versions), ai_credits: String(v.ai_credits), concurrent_ai_jobs: String(v.concurrent_ai_jobs) }; }
const fields = [["monthly_price", "Monthly base (KES)"], ["annual_price", "Annual base (KES)"], ["additional_licence_monthly_price", "Extra licence / month (KES)"], ["bid_workspaces", "Bid workspaces"], ["included_licences", "Included licences"], ["storage_bytes", "Storage"], ["ai_credits", "AI credits"], ["concurrent_ai_jobs", "Concurrent AI jobs"]] as const;
function groupAmount(value: string): string { return value.replace(/^(\d+)(?=\.|$)/, digits => digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",")); }
function price(value: PlanMoney | null): string { return value ? `${value.kind === "FROM" ? "From " : ""}KES ${groupAmount(value.amount)}` : "Not specified"; }
type StorageUnit = "GB" | "MB" | "GiB" | "MiB" | "bytes";
const storageUnits: ReadonlyArray<[StorageUnit, number]> = [["GB", 1_000_000_000], ["GiB", 1_073_741_824], ["MB", 1_000_000], ["MiB", 1_048_576], ["bytes", 1]];
function storageUnit(bytes: number): [StorageUnit, number] {
  return storageUnits.find(([, factor]) => bytes >= factor && bytes % factor === 0) ?? ["bytes", 1];
}
function storageBytes(value: string, factor: number): number | null {
  if (!/^[1-9]\d*$/.test(value)) return null;
  const result = BigInt(value) * BigInt(factor);
  return result <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(result) : null;
}
function count(value: number | null): string { return value === null ? "Not specified" : value.toLocaleString("en-KE"); }

function PlanEditor({ item, manage, publish, generation, csrf, requestKeyRef, onBusy, onSaved }: { item: ManagedPlanItem; manage: boolean; publish: boolean; generation: number; csrf: string | null; requestKeyRef: React.RefObject<RequestKey | null>; onBusy: (busy: boolean) => void; onSaved: (message: string) => Promise<void> }) {
  const [values, setValues] = useState<Values>(() => initial(item));
  const [[unit, factor]] = useState(() => storageUnit((item.draft ?? item.published).storage_bytes));
  const [storage, setStorage] = useState(() => String((item.draft ?? item.published).storage_bytes / factor));
  const [focusedMoney, setFocusedMoney] = useState<string | null>(null);
  const [busy, setBusy] = useState(false), [message, setMessage] = useState("");
  const locked = useRef(false);
  async function submit(action: "drafts" | "publish") {
    if (locked.current || !csrf || action === "drafts" && !manage || action === "publish" && (!publish || !item.draft || !item.independent_publisher)) return;
    const moneyFields = ["monthly_price", "annual_price", "additional_licence_monthly_price"] as const;
    const numericFields = ["bid_workspaces", "included_licences", "document_versions", "ai_credits", "concurrent_ai_jobs"] as const;
    const bytes = storageBytes(storage, factor);
    if (action === "drafts" && (moneyFields.some(f => !/^(0|[1-9]\d*)\.\d{2}$/.test(values[f]) || Number(values[f]) <= 0) || numericFields.some(f => !/^[1-9]\d*$/.test(values[f]) || !Number.isSafeInteger(Number(values[f]))) || bytes === null)) { setMessage(`Enter positive whole limits, two-decimal KES amounts and a valid storage value in ${unit}.`); return; }
    locked.current = true; setBusy(true); onBusy(true); setMessage("");
    const body = action === "publish" ? { expected_head_version: item.head_version, version: item.draft?.version } : { expected_head_version: item.head_version, ...Object.fromEntries(moneyFields.map(f => [f, { currency: "KES", amount: values[f] }])), ...Object.fromEntries(numericFields.map(f => [f, Number(values[f])])), storage_bytes: bytes };
    const fingerprint = JSON.stringify([generation, item.plan_key, action, body]);
    if (requestKeyRef.current?.fingerprint !== fingerprint) requestKeyRef.current = { fingerprint, value: crypto.randomUUID() };
    try {
      const response = await fetch(`/api/platform/plans/${item.plan_key}/${action}`, { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": requestKeyRef.current.value }, body: JSON.stringify(body) });
      const receipt = response.ok ? parsePlanReceipt(await response.json()) : null;
      if (!receipt || receipt.plan_key !== item.plan_key || receipt.published !== (action === "publish")) { setMessage(response.status === 409 ? "Plan changed or a second administrator is required. Reload before a new action." : response.status === 403 ? "This operation requires a verified Billing administrator." : "The outcome is uncertain. Retry unchanged with the same request."); return; }
      requestKeyRef.current = null;
      await onSaved(action === "publish" ? "Plan changes published. Existing terms are unchanged." : "Plan changes saved for publication by a different administrator.");
    } catch { setMessage("The outcome is uncertain. Retry unchanged with the same request."); }
    finally { locked.current = false; setBusy(false); onBusy(false); }
  }
  return <div className="space-y-4">
    <p className="text-xs text-muted-foreground">{item.draft ? "Showing saved changes awaiting publication." : "Showing current plan settings."} Published changes apply to new terms only.</p>
    {manage && <div className="grid gap-3 sm:grid-cols-2">{fields.map(([name, label]) => <label key={name} className="grid gap-1 text-sm">{name === "storage_bytes" ? `${label} (${unit})` : label}<Input inputMode={name.includes("price") ? "decimal" : "numeric"} maxLength={32} disabled={busy} value={name === "storage_bytes" ? storage : name.includes("price") && focusedMoney !== name ? groupAmount(values[name]) : values[name]} onFocus={() => { if (name.includes("price")) setFocusedMoney(name); }} onBlur={() => setFocusedMoney(null)} onChange={event => { if (name === "storage_bytes") setStorage(normalizeNumericInput(event.target.value)); else setValues(previous => ({ ...previous, [name]: normalizeNumericInput(event.target.value.replaceAll(",", "")) })); }} /></label>)}</div>}
    {manage && <p className="text-xs text-muted-foreground">Storage must be a whole number in {unit}. Existing limits are preserved exactly.</p>}
    {item.draft && publish && <p className="text-xs text-muted-foreground">{item.independent_publisher ? "Publishing applies saved changes, not unsaved edits." : "Publishing is unavailable for this session. A different administrator must publish these changes."}</p>}
    {message && <p role="alert" className="text-sm text-destructive">{message}</p>}
    <AlertDialogFooter><AlertDialogCancel disabled={busy}>Close</AlertDialogCancel>{manage && <Button type="button" disabled={busy || !csrf} onClick={() => void submit("drafts")}>Save changes for publication</Button>}{publish && item.draft && item.independent_publisher && <Button type="button" variant="outline" disabled={busy || !csrf} onClick={() => void submit("publish")}>Publish saved changes</Button>}</AlertDialogFooter>
  </div>;
}

export function PlatformPlanManager({ catalogue, onPublished }: { catalogue: PlanCatalogue; onPublished: () => Promise<void> }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const read = resolution.capabilities.includes("platform.plan.read"), manage = resolution.capabilities.includes("platform.plan.manage"), publish = resolution.capabilities.includes("platform.plan.publish");
  const [items, setItems] = useState<{ generation: number; plans: ManagedPlanItem[] } | null>(null), [error, setError] = useState(""), [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<PlanKey | null>(null), [editorBusy, setEditorBusy] = useState(false), [message, setMessage] = useState("");
  const sequence = useRef(0), requestKeyRef = useRef<RequestKey | null>(null);
  const load = useCallback(async () => {
    if (!generation || !read) return;
    const current = ++sequence.current; setBusy(true); setError("");
    try { const response = await fetch("/api/platform/plans/management", { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setError("Session changed. Please try again."); refresh(); return; }
      const plans = response.ok ? parseManagedPlans(await response.json()) : null;
      if (!plans) { setError(response.status === 403 ? "Verified Billing access is required." : "Plan settings are unavailable."); return; }
      setItems({ generation, plans });
    } catch { if (current === sequence.current) setError("Plan settings are unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }, [generation, read, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); const ref = sequence; return () => { window.clearTimeout(timer); ref.current++; }; }, [load]);
  const visible = items?.generation === generation && read ? items.plans : null;
  const selectedPlan = catalogue.plans.find(plan => plan.key === selected);
  const selectedItem = visible?.find(item => item.plan_key === selected);
  async function saved(text: string) { setSelected(null); setMessage(text); await Promise.all([load(), onPublished()]); }
  return <section aria-label="Plans" className="space-y-4">
    {error && <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-destructive"><span>{error} Editing is unavailable until settings are confirmed.</span><Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void load()}>Retry</Button></div>}
    {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
    <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Plan</th><th scope="col" className="px-4 py-3">Monthly</th><th scope="col" className="px-4 py-3">Annual</th><th scope="col" className="px-4 py-3">Licences</th><th scope="col" className="px-4 py-3">AI credits</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y">{catalogue.plans.map(plan => {
      const item = visible?.find(value => value.plan_key === plan.key);
      return <tr key={plan.key}><th scope="row" className="px-4 py-4 font-medium">{plan.name}</th><td className="px-4 py-4">{price(plan.monthly_base_price)}</td><td className="px-4 py-4">{price(plan.annual_base_price)}</td><td className="px-4 py-4">{count(plan.included_licences)}</td><td className="px-4 py-4">{count(plan.limits.ai_credits)}</td><td className="px-4 py-4"><div className="flex flex-wrap gap-1"><PlatformStatus value={plan.key === "TRIAL" || plan.key === "ENTERPRISE" ? "Available" : "Published"} />{item?.draft && <PlatformStatus value="Pending" />}</div></td><td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={plan.name} actions={[{ label: item && !error && (manage || publish) ? "Edit plan" : "View details", onSelect: () => setSelected(plan.key) }]} /></div></td></tr>;
    })}</tbody></table></div>
    {busy && !visible && <p role="status" className="text-xs text-muted-foreground">Loading plan settings…</p>}
    <AlertDialog open={selected !== null} onOpenChange={open => { if (!open && !editorBusy) setSelected(null); }}><AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto data-[size=default]:max-w-[calc(100vw-2rem)] data-[size=default]:sm:max-w-3xl"><AlertDialogHeader><AlertDialogTitle>{selectedPlan?.name ?? "Plan"}</AlertDialogTitle><AlertDialogDescription>Prices are in KES before VAT. Existing subscriptions and invoices are not changed.</AlertDialogDescription></AlertDialogHeader>
      {selectedPlan && selectedItem && !error && generation && (manage || publish) ? <PlanEditor key={`${generation}:${selectedItem.plan_key}:${selectedItem.head_version}`} item={selectedItem} manage={manage} publish={publish} generation={generation} csrf={csrfToken} requestKeyRef={requestKeyRef} onBusy={setEditorBusy} onSaved={saved} /> : selectedPlan && <PlanDetails plan={selectedPlan} />}
    </AlertDialogContent></AlertDialog>
  </section>;
}
function PlanDetails({ plan }: { plan: PlanCatalogueEntry }) {
  return <><dl className="grid gap-3 text-sm sm:grid-cols-2"><div><dt className="text-muted-foreground">Monthly</dt><dd>{price(plan.monthly_base_price)}</dd></div><div><dt className="text-muted-foreground">Annual</dt><dd>{price(plan.annual_base_price)}</dd></div>{plan.trial_base_price && <div><dt className="text-muted-foreground">Trial</dt><dd>{price(plan.trial_base_price)}</dd></div>}<div><dt className="text-muted-foreground">Bid workspaces / {plan.bid_workspace_allowance.period.replaceAll("_", " ").toLowerCase()}</dt><dd>{count(plan.bid_workspace_allowance.count)}</dd></div><div><dt className="text-muted-foreground">Included licences</dt><dd>{count(plan.included_licences)}</dd></div><div><dt className="text-muted-foreground">AI credits / {plan.limits.reset_period.replaceAll("_", " ").toLowerCase()}</dt><dd>{count(plan.limits.ai_credits)}</dd></div></dl>{plan.contract_required && <p className="text-xs text-muted-foreground">Contract terms are required.</p>}<AlertDialogFooter><AlertDialogCancel>Close</AlertDialogCancel></AlertDialogFooter></>;
}
