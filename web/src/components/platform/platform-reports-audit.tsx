"use client";

import { useEffect, useRef, useState } from "react";
import { DownloadIcon, RotateCcwIcon, SearchIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PlatformBillingSummary } from "@/components/platform/platform-billing-summary";
import { PlatformStatus } from "@/components/platform/platform-status";
import { useSession } from "@/components/session/session-provider";
import { parseAuditPage, type AuditRecord } from "@/lib/platform-audit-contract";

type Period = { from: string; to: string };
type Preview = { rows: AuditRecord[]; next: string | null; generation: number; period: Period; cursor: string | null };
const emptyPeriod: Period = { from: "", to: "" };
export function PlatformReportsAudit() {
 const { resolution } = useSession();
 const billing = resolution.capabilities.includes("platform.invoice.read");
 const audit = resolution.capabilities.includes("platform.audit.read");
 const generation = resolution.platform_context_generation;
 return <main className="app-page space-y-5"><div className="app-page-header"><h1 className="type-title">Reports</h1><p className="type-lead text-muted-foreground">Review financial and platform activity.</p></div>
  {!generation || !billing && !audit ? <p role="status" className="text-sm text-muted-foreground">You do not have permission to view reports.</p> : <Tabs key={`${generation}|${billing}|${audit}`} defaultValue={billing ? "billing" : "activity"}>
   <TabsList aria-label="Report views"><TabsTrigger value="billing" disabled={!billing}>Billing</TabsTrigger><TabsTrigger value="activity" disabled={!audit}>Administrator activity</TabsTrigger></TabsList>
   <TabsContent value="billing" className="mt-5 space-y-5">{billing && <><div><h2 className="text-base font-semibold">Billing overview</h2><p className="text-sm text-muted-foreground">Current invoice totals and monthly invoicing.</p></div><PlatformBillingSummary /></>}</TabsContent>
   <TabsContent value="activity" className="mt-5">{audit && <ReportsWorkspace key={`${generation}|${resolution.capabilities.join("|")}`} />}</TabsContent>
  </Tabs>}
 </main>;
}
function ReportsWorkspace() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.audit.read");
  const exportAllowed = resolution.capabilities.includes("platform.audit.export");
  const [period, setPeriod] = useState<Period>(emptyPeriod), [applied, setApplied] = useState<Period | null>(null);
  const [preview, setPreview] = useState<Preview | null>(null), [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const sequence = useRef(0), locked = useRef(false);
  useEffect(() => { const active = sequence; return () => { active.current++; }; }, [generation]);
  async function load(nextPeriod: Period, cursor: string | null, nextCursors: (string | null)[]) {
    if (!generation || !allowed || locked.current || nextPeriod.from && nextPeriod.to && nextPeriod.from > nextPeriod.to) return;
    locked.current = true; const current = ++sequence.current;
    setBusy(true); setError(""); setApplied(nextPeriod); setCursors(nextCursors); setPreview(null);
    const query = new URLSearchParams({ limit: "25", sort: "DESC" });
    if (nextPeriod.from) query.set("from", nextPeriod.from);
    if (nextPeriod.to) query.set("to", nextPeriod.to);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/audit/actions?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setError("Session changed. Please try again."); refresh(); return; }
      if (response.status === 403) { setError("You do not have permission to read Platform audit activity."); return; }
      const page = response.ok ? parseAuditPage(await response.json()) : null;
      if (!page) { setError("Report data could not be confirmed. Please try again."); return; }
      setPreview({ rows: page.data, next: page.meta.next_cursor, generation, period: nextPeriod, cursor });
    } catch { if (current === sequence.current) setError("Report data is unavailable."); }
    finally { locked.current = false; if (current === sequence.current) setBusy(false); }
  }
  async function exportReport() {
    if (!generation || !allowed || !exportAllowed || !applied?.from || !applied.to || applied.from > applied.to || locked.current) return;
    const { from, to } = applied;
    const days = (Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000;
    if (days >= 31) { setError("Choose a UTC window of at most 31 days."); return; }
    locked.current = true; const current = ++sequence.current;
    setBusy(true); setError("");
    try {
      const query = new URLSearchParams({ from, to });
      const response = await fetch(`/api/platform/audit/actions/export?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "text/csv", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setError("Session changed. Please try again."); refresh(); return; }
      if (response.status === 403) { const body = await response.json().catch(() => null); setError(body?.error?.code === "RECENT_VERIFICATION_REQUIRED" ? "Verify your sign-in again before exporting. Ordinary report browsing remains available." : "You do not have permission to export audit activity."); return; }
      if (response.status === 400) { setError("Reduce the UTC window; exports are limited to 500 actions in 31 days."); return; }
      if (!response.ok || response.headers.get("content-type")?.split(";")[0] !== "text/csv") { setError("Export could not be confirmed."); return; }
      const blob = await response.blob();
      if (current !== sequence.current) return;
      const url = URL.createObjectURL(blob), link = document.createElement("a");
      link.href = url; link.download = `platform-audit-${from}-${to}.csv`;
      document.body.append(link); link.click(); link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
    } catch { if (current === sequence.current) setError("Export is unavailable."); }
    finally { locked.current = false; if (current === sequence.current) setBusy(false); }
  }
  const cursor = cursors[cursors.length - 1];
  const visible = preview?.generation === generation && preview.cursor === cursor && preview.period.from === applied?.from && preview.period.to === applied?.to && allowed ? preview : null;
  const invalidRange = !!period.from && !!period.to && period.from > period.to;
  const filtersChanged = applied !== null && (period.from !== applied.from || period.to !== applied.to);
  return <div className="space-y-5">
    {!allowed || !generation ? <p role="status" className="text-sm text-muted-foreground">You do not have permission to view activity reports.</p> : <>
      <Card role="region" aria-label="Report filters"><CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader><CardContent><form className="space-y-4" onSubmit={event => { event.preventDefault(); if (!invalidRange) void load(period, null, [null]); }}>
        <div className="grid gap-4 sm:grid-cols-2 lg:max-w-xl">
          <label className="grid min-w-0 gap-1 text-sm font-medium">From (UTC)<Input type="date" value={period.from} max={period.to || undefined} aria-invalid={invalidRange} aria-describedby={invalidRange ? "report-date-error" : undefined} onChange={event => setPeriod(old => ({ ...old, from: event.target.value }))} /></label>
          <label className="grid min-w-0 gap-1 text-sm font-medium">Through (UTC)<Input type="date" value={period.to} min={period.from || undefined} aria-invalid={invalidRange} aria-describedby={invalidRange ? "report-date-error" : undefined} onChange={event => setPeriod(old => ({ ...old, to: event.target.value }))} /></label>
        </div>
        <div className="flex flex-wrap items-center gap-2"><Button type="submit" size="sm" disabled={busy || invalidRange}><SearchIcon aria-hidden="true" />Apply filters</Button>
        {(period.from || period.to || applied?.from || applied?.to) && <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => { setPeriod(emptyPeriod); void load(emptyPeriod, null, [null]); }}>Clear filters</Button>}
        {exportAllowed && <Button type="button" size="sm" variant="outline" disabled={busy || !visible || !applied?.from || !applied.to || filtersChanged} onClick={() => void exportReport()}><DownloadIcon aria-hidden="true" />Export CSV</Button>}</div>
      </form>{invalidRange && <p id="report-date-error" role="alert" className="mt-3 text-sm text-destructive">The end date must not precede the start date.</p>}</CardContent></Card>
      <section aria-label="Platform activity report" className="space-y-4">
        {filtersChanged && <p className="text-xs text-muted-foreground">Apply changes before exporting the selected dates.</p>}
        {error && <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-destructive">{error}{applied && !visible && <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void load(applied, cursor, cursors)}><RotateCcwIcon aria-hidden="true" />Retry report</Button>}</div>}
        <div className="overflow-x-auto rounded-lg border bg-card" aria-busy={busy}><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr>{["Time (UTC)", "Action", "Module", "Result", "Administrator"].map(label => <th scope="col" className="px-4 py-3" key={label}>{label}</th>)}</tr></thead><tbody className="divide-y">
          {visible?.rows.map(row => <tr key={row.id}><td className="whitespace-nowrap px-4 py-4">{new Date(row.occurred_at).toLocaleString(undefined, { timeZone: "UTC" })}</td><td className="px-4 py-4">{row.action.replace(/([a-z])([A-Z])/g, "$1 $2")}</td><td className="px-4 py-4">{row.module}</td><td className="px-4 py-4"><PlatformStatus value={row.result} /></td><td className="px-4 py-4">{row.administrator_name ?? (row.administrator_id ? "Administrator" : "System")}</td></tr>)}
          {(!visible || visible.rows.length === 0) && <tr><td colSpan={5} className="px-4 py-8 text-center text-sm text-muted-foreground">{busy && !visible ? <Skeleton role="status" aria-label="Loading activity report" className="mx-auto h-6 w-48" /> : error && !visible ? "Report unavailable." : visible ? "No activity found for these filters." : "No report loaded."}</td></tr>}
        </tbody></table></div>
        <nav aria-label="Report pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={busy || !applied || cursors.length === 1} onClick={() => { if (applied) { const next = cursors.slice(0, -1); void load(applied, next[next.length - 1], next); } }}>Previous</Button><span className="text-sm text-muted-foreground">Page {cursors.length}</span><Button type="button" variant="outline" disabled={busy || !visible?.next} onClick={() => { if (applied && visible?.next) void load(applied, visible.next, [...cursors, visible.next]); }}>Next</Button></nav>
      </section>
    </>}
  </div>;
}
