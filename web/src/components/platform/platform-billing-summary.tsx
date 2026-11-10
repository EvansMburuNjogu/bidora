"use client";
import { useEffect, useState } from "react";
import { useSession } from "@/components/session/session-provider";
import { PlatformMetricCard } from "@/components/platform/platform-metric-card";
import { Button } from "@/components/ui/button";

type Month = { month: string; total_minor_units: string };
type Summary = { invoiced_minor_units: string | null; paid_invoices: string | null; outstanding_minor_units: string | null; failed_payments: string | null; monthly_invoicing: Month[] };
const numeric = (value: unknown): value is string => typeof value === "string" && /^\d{1,40}$/.test(value);
function money(raw: string) { const value = BigInt(raw); return `KES ${(value / BigInt(100)).toLocaleString("en-KE")}.${(value % BigInt(100)).toString().padStart(2, "0")}`; }
function monthLabel(month: string) { return new Date(`${month}-01T00:00:00Z`).toLocaleDateString("en-KE", { month: "short", year: "2-digit", timeZone: "UTC" }); }

export function PlatformBillingSummary() {
 const { resolution, refresh } = useSession(); const generation = resolution.platform_context_generation;
 return <BillingSummaryData key={generation} generation={generation} refresh={refresh} />;
}
function BillingSummaryData({ generation, refresh }: { generation: number | null; refresh: () => void }) {
 const [summary, setSummary] = useState<Summary | null>(null), [busy, setBusy] = useState(true), [error, setError] = useState(""), [revision, setRevision] = useState(0);
 useEffect(() => {
  const controller = new AbortController();
  async function load() {
   if (!generation) { setBusy(false); return; } setBusy(true); setError("");
   try {
    const response = await fetch("/api/platform/overview", { credentials: "include", cache: "no-store", signal: controller.signal, headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
    if (controller.signal.aborted) return; if (response.status === 401 || response.status === 409) { refresh(); return; }
    const data = response.ok ? (await response.json())?.data : null;
    if (!data || !["invoiced_minor_units", "paid_invoices", "outstanding_minor_units", "failed_payments"].every(key => data[key] === null || numeric(data[key])) || !Array.isArray(data.monthly_invoicing) || data.monthly_invoicing.length > 12 || !data.monthly_invoicing.every((r: Record<string, unknown>) => r && typeof r.month === "string" && /^\d{4}-(0[1-9]|1[0-2])$/.test(r.month) && numeric(r.total_minor_units))) throw new Error("unavailable");
    if (!controller.signal.aborted) setSummary(data);
   } catch { if (!controller.signal.aborted) { setSummary(null); setError("Billing totals could not be loaded."); } }
   finally { if (!controller.signal.aborted) setBusy(false); }
  }
  void load(); return () => controller.abort();
 }, [generation, refresh, revision]);
 return <section className="space-y-4" aria-label="Billing summary" aria-busy={busy}>
  {error && <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-destructive">{error}<Button type="button" size="sm" variant="outline" onClick={() => setRevision(v => v + 1)}>Retry</Button></div>}
  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{[{ label: "Total Invoiced", value: summary?.invoiced_minor_units, format: money }, { label: "Paid Invoices", value: summary?.paid_invoices, format: (v: string) => BigInt(v).toLocaleString("en-KE") }, { label: "Outstanding", value: summary?.outstanding_minor_units, format: money }, { label: "Failed Payments", value: summary?.failed_payments, format: (v: string) => BigInt(v).toLocaleString("en-KE") }].filter(row => row.value !== null).map(row => <PlatformMetricCard key={row.label} label={row.label} value={row.value == null ? undefined : row.format(row.value)} />)}</div>
  {summary && summary.monthly_invoicing.length > 0 && <MonthlyInvoicingChart months={summary.monthly_invoicing} />}
 </section>;
}
export function MonthlyInvoicingChart({ months }: { months: Month[] }) {
 const maximum = months.reduce((max, row) => { const value = BigInt(row.total_minor_units); return value > max ? value : max; }, BigInt(0));
 return <section aria-labelledby="monthly-invoicing-title" className="rounded-lg border bg-card p-4">
  <h2 id="monthly-invoicing-title" className="text-sm font-medium">Monthly invoicing</h2>
  <p className="mt-1 text-xs text-muted-foreground">Invoice totals for the last {months.length} months, excluding void invoices. Not cash received or net revenue.</p>
  <div className="mt-4 overflow-x-auto"><div className="min-w-[600px]">
   <div className="flex items-start gap-2"><div className="flex h-44 w-28 shrink-0 flex-col justify-between text-right text-xs text-muted-foreground"><span>{money(String(maximum))}</span><span>KES 0.00</span></div>
    <div className="grid h-44 min-w-0 flex-1 grid-cols-12 items-end gap-2 border-b" aria-label="Monthly invoice totals">{months.map(row => {
     const amount = BigInt(row.total_minor_units), height = maximum === BigInt(0) ? 0 : Number(amount * BigInt(10000) / maximum) / 100;
     return <div key={row.month} className="flex h-full items-end justify-center" title={`${monthLabel(row.month)}: ${money(row.total_minor_units)}`}><span role="img" aria-label={`${monthLabel(row.month)}: ${money(row.total_minor_units)}`} className="w-full max-w-10 rounded-t-sm bg-primary/70" style={{ height: `${height}%` }} /></div>;
    })}</div>
   </div><div className="ml-[7.5rem] grid grid-cols-12 gap-2 pt-2 text-center text-xs text-muted-foreground">{months.map(row => <span key={row.month}>{monthLabel(row.month)}</span>)}</div>
  </div></div>
 </section>;
}
