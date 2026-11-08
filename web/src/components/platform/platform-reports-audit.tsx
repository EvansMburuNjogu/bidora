"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import { parseAuditPage, type AuditRecord } from "@/lib/platform-audit-contract";

type Preview = { rows: AuditRecord[]; next: string | null; generation: number; from: string; to: string };

export function PlatformReportsAudit() {
 const {resolution}=useSession();
 return <ReportsWorkspace key={`${resolution.platform_context_generation}|${resolution.capabilities.join("|")}`} />;
}
function ReportsWorkspace() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.audit.read");
  const exportAllowed = resolution.capabilities.includes("platform.audit.export");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const sequence = useRef(0);
  useEffect(() => { const active = sequence; return () => { active.current++; }; }, [generation]);
  async function load(cursor?: string) {
    if (!generation || !allowed || busy || from && to && from > to) return;
    const current = ++sequence.current;
    setBusy(true); setError("");
    const query = new URLSearchParams({ limit: "25", sort: "DESC" });
    if (from) query.set("from", from);
    if (to) query.set("to", to);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/audit/actions?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      if (response.status === 403) { setError("A verified audit session is required."); return; }
      const page = response.ok ? parseAuditPage(await response.json()) : null;
      if (!page) { setError("Report data could not be confirmed. Retry without changing filters."); return; }
      setPreview((old) => ({ rows: cursor && old && old.generation === generation && old.from === from && old.to === to ? [...old.rows, ...page.data] : page.data, next: page.meta.next_cursor, generation, from, to }));
    } catch { if (current === sequence.current) setError("Report data is unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }
  async function exportReport() {
    if (!generation || !allowed || !exportAllowed || !from || !to || from > to || busy) return;
    const days = (Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000;
    if (days >= 31) { setError("Choose a UTC window of at most 31 days."); return; }
    const current = ++sequence.current;
    setBusy(true); setError("");
    try {
      const query = new URLSearchParams({ from, to });
      const response = await fetch(`/api/platform/audit/actions/export?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "text/csv", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      if (response.status === 403) { setError("A verified audit-export session is required."); return; }
      if (response.status === 400) { setError("Reduce the UTC window; exports are limited to 500 actions in 31 days."); return; }
      if (!response.ok || response.headers.get("content-type")?.split(";")[0] !== "text/csv") { setError("Export could not be confirmed."); return; }
      const blob = await response.blob();
      if (current !== sequence.current) return;
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url; link.download = `platform-audit-${from}-${to}.csv`;
      document.body.append(link); link.click(); link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
    } catch { if (current === sequence.current) setError("Export is unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }
  const visible = preview?.generation === generation && preview.from === from && preview.to === to && allowed ? preview : null;
  return <main className="app-page space-y-5"><div className="app-page-header"><h1 className="type-title">Reports</h1><p className="type-lead text-muted-foreground">Review and export administrator activity for a selected period. Private customer records are excluded.</p></div>
    {!allowed || !generation ? <p role="status" className="text-sm text-muted-foreground">You do not have permission to view activity reports.</p> : <>
      <section aria-label="Report filters" className="flex flex-wrap items-end gap-3"><label className="grid gap-1 text-sm">From (UTC)<Input type="date" value={from} onChange={(event) => { setFrom(event.target.value); setPreview(null); }} /></label><label className="grid gap-1 text-sm">Through (UTC)<Input type="date" value={to} onChange={(event) => { setTo(event.target.value); setPreview(null); }} /></label><Button type="button" disabled={busy || !!(from && to && from > to)} onClick={() => void load()}>{busy ? "Loading…" : "Preview report"}</Button>{exportAllowed && <Button type="button" variant="outline" disabled={busy || !from || !to || from > to} onClick={() => void exportReport()}>Export CSV</Button>}</section>
      {from && to && from > to && <p role="alert" className="text-sm text-destructive">The end date must not precede the start date.</p>}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {visible && <section aria-label="Platform activity report" className="space-y-3"><p className="text-xs text-muted-foreground">Showing {visible.rows.length} real audit actions{visible.next ? "; additional pages available" : ""}. This preview is not a platform-wide total.</p><div className="overflow-x-auto rounded-lg border"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr>{["Time (UTC)", "Action", "Module", "Result", "Administrator"].map((label) => <th scope="col" className="p-3" key={label}>{label}</th>)}</tr></thead><tbody className="divide-y">{visible.rows.map((row) => <tr key={row.id}><td className="p-3">{row.occurred_at}</td><td className="p-3">{row.action.replace(/([a-z])([A-Z])/g, "$1 $2")}</td><td className="p-3">{row.module}</td><td className="p-3">{row.result}</td><td className="p-3">{row.administrator_name ?? (row.administrator_id ? "Administrator" : "System")}</td></tr>)}{!visible.rows.length && <tr><td colSpan={5} className="p-4 text-muted-foreground">No audit actions match these filters.</td></tr>}</tbody></table></div>{visible.next && <Button type="button" variant="outline" disabled={busy} onClick={() => void load(visible.next ?? undefined)}>Load more</Button>}</section>}
      <p className="text-xs text-muted-foreground">CSV export requires both audit-read and audit-export permissions, a verified session, a UTC window of at most 31 days and no more than 500 matching actions. Larger windows must be split; no partial CSV is returned.</p>
    </>}
  </main>;
}
