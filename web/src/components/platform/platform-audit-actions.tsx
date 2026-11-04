"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { PlatformSelect } from "@/components/platform/platform-select";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";
import { useSession } from "@/components/session/session-provider";
import { auditActions, auditModules, auditResults, auditUUID, parseAuditDetail, parseAuditPage, type AuditPage, type AuditRecord } from "@/lib/platform-audit-contract";

type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: AuditPage; key: string };
type Detail = { kind: "loading" | "error" } | { kind: "ready"; record: AuditRecord; key: string };
type Filters = { search: string; administrator: string; action: string; module: string; result: string; from: string; to: string; sort: "ASC" | "DESC" };
const initial: Filters = { search: "", administrator: "", action: "", module: "", result: "", from: "", to: "", sort: "DESC" };
const formatUTC = (value: string) => `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC`;
const actionLabel = (action: string) => ({ UserLoggedIn: "Session created", EmailLoginCodeVerified: "Email-code sign-in completed", MFALoginCompleted: "Protected sign-in completed" } as Record<string, string>)[action] ?? action;

export function PlatformAuditActions() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const canRead = resolution.capabilities.includes("platform.audit.read");
  const [filters, setFilters] = useState<Filters>(initial);
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [view, setView] = useState<View>({ kind: "loading" });
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail>({ kind: "loading" });
  const listSequence = useRef(0), detailSequence = useRef(0);
  const cursor = cursors[cursors.length - 1];
  const pageKey = JSON.stringify([generation, canRead, filters, cursor]);
  const detailKey = `${generation}|${canRead}|${selectedID}`;
  const ready = view.kind === "ready" && view.key === pageKey ? view.page : null;
  const readyDetail = detail.kind === "ready" && detail.key === detailKey ? detail.record : null;
  const invalidAdministrator = !!filters.administrator && !auditUUID.test(filters.administrator);
  const invalidRange = !!filters.from && !!filters.to && filters.from > filters.to;
  const accessDenied = generation === null || !canRead || view.kind === "denied";
  const change = (update: Partial<Filters>) => { setView({ kind: "loading" }); setFilters((current) => ({ ...current, ...update })); setCursors([null]); setSelectedID(null); };
  const load = useCallback(async () => {
    const current = ++listSequence.current;
    if (generation === null || !canRead) { setView({ kind: "denied" }); return; }
    if (invalidAdministrator || invalidRange) { setView({ kind: "error" }); return; }
    setView({ kind: "loading" });
    try {
      const query = new URLSearchParams({ limit: "10", sort: filters.sort });
      for (const key of ["search", "administrator", "action", "module", "result", "from", "to"] as const) if (filters[key]) query.set(key, filters[key]);
      if (cursor) query.set("cursor", cursor);
      const response = await fetch(`/api/platform/audit/actions?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== listSequence.current) return;
      if (response.status === 401 || response.status === 409) { setView({ kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setView({ kind: "denied" }); return; }
      const page = response.ok ? parseAuditPage(await response.json()) : null;
      if (current === listSequence.current) setView(page ? { kind: "ready", page, key: pageKey } : { kind: "error" });
    } catch { if (current === listSequence.current) setView({ kind: "error" }); }
  }, [generation, canRead, filters, cursor, invalidAdministrator, invalidRange, refresh, pageKey, setView]);
  useEffect(() => { const sequence = listSequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); sequence.current++; }; }, [load]);
  const loadDetail = useCallback(async (id: string) => {
    const current = ++detailSequence.current;
    if (generation === null || !canRead) { setDetail({ kind: "error" }); return; }
    setDetail({ kind: "loading" });
    try {
      const response = await fetch(`/api/platform/audit/actions/${id}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== detailSequence.current) return;
      if (response.status === 401 || response.status === 409) refresh();
      const record = response.ok ? parseAuditDetail(await response.json()) : null;
      if (current === detailSequence.current) setDetail(record && record.id === id ? { kind: "ready", record, key: `${generation}|${canRead}|${id}` } : { kind: "error" });
    } catch { if (current === detailSequence.current) setDetail({ kind: "error" }); }
  }, [generation, canRead, refresh, setDetail]);
  useEffect(() => { const sequence = detailSequence; if (selectedID) { const timer = window.setTimeout(() => void loadDetail(selectedID), 0); return () => { window.clearTimeout(timer); sequence.current++; }; } return () => { sequence.current++; }; }, [selectedID, loadDetail]);
  const fields = readyDetail ? [
    ["Date & Time", formatUTC(readyDetail.occurred_at)], ["Administrator", readyDetail.administrator_id ?? "System"],
    ["Action", actionLabel(readyDetail.action)], ["Module", readyDetail.module], ["Affected Record", readyDetail.affected_record_id ?? "Not shown"],
    ["Result", readyDetail.result], ["Audit Reference", readyDetail.id],
  ] : [];
  return <main className="app-page min-w-0 space-y-5">
    <div className="app-page-header"><h1 className="type-title">Audit Trails</h1><p className="text-sm text-muted-foreground">Review Platform activity. Pending changes may still need approval.</p></div>
    <div className="grid gap-3 rounded-lg border bg-card p-4 sm:grid-cols-2 lg:grid-cols-4">
      <label className="grid gap-1 text-sm">Search<Input value={filters.search} maxLength={80} placeholder="Action, module or reference" onChange={(e) => change({ search: e.target.value })} /></label>
      <label className="grid gap-1 text-sm">From (UTC)<Input type="date" value={filters.from} max={filters.to || undefined} onChange={(e) => change({ from: e.target.value })} /></label>
      <label className="grid gap-1 text-sm">To (UTC)<Input type="date" value={filters.to} min={filters.from || undefined} onChange={(e) => change({ to: e.target.value })} /></label>
      <label className="grid gap-1 text-sm">Administrator ID<Input value={filters.administrator} maxLength={36} aria-invalid={invalidAdministrator} placeholder="Exact administrator UUID" onChange={(e) => change({ administrator: e.target.value.toLowerCase() })} /></label>
      <label className="grid gap-1 text-sm">Action<PlatformSelect value={filters.action} onChange={(e) => change({ action: e.target.value })}><option value="">All actions</option>{auditActions.map((action) => <option key={action} value={action}>{actionLabel(action)}</option>)}</PlatformSelect></label>
      <label className="grid gap-1 text-sm">Module<PlatformSelect value={filters.module} onChange={(e) => change({ module: e.target.value })}><option value="">All modules</option>{auditModules.map((module) => <option key={module} value={module}>{module}</option>)}</PlatformSelect></label>
      <label className="grid gap-1 text-sm">Result<PlatformSelect value={filters.result} onChange={(e) => change({ result: e.target.value })}><option value="">All results</option>{auditResults.map((result) => <option key={result} value={result}>{result}</option>)}</PlatformSelect></label>
      <label className="grid gap-1 text-sm">Date order<PlatformSelect value={filters.sort} onChange={(e) => change({ sort: e.target.value as Filters["sort"] })}><option value="DESC">Newest first</option><option value="ASC">Oldest first</option></PlatformSelect></label>
      <div className="sm:col-span-2 lg:col-span-4"><Button type="button" variant="outline" onClick={() => change(initial)}>Clear filters</Button></div>
    </div>
    {invalidAdministrator ? <p role="alert" className="text-sm text-destructive">Enter an exact administrator ID.</p> : null}
    {invalidRange ? <p role="alert" className="text-sm text-destructive">Choose an end date on or after the start date.</p> : null}
    {accessDenied ? <p role="alert" className="text-sm">This view requires current Platform audit-read authority and recent email verification.</p> : <>
      <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[1100px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Date &amp; Time</th><th scope="col" className="px-4 py-3">Administrator</th><th scope="col" className="px-4 py-3">Action</th><th scope="col" className="px-4 py-3">Module</th><th scope="col" className="px-4 py-3">Affected Record</th><th scope="col" className="px-4 py-3">Result</th><th scope="col" className="px-4 py-3">Actions</th></tr></thead><tbody>
        {ready?.data.map((row) => <tr key={row.id} className="border-t"><td className="whitespace-nowrap px-4 py-3">{formatUTC(row.occurred_at)}</td><td className="break-all px-4 py-3 font-mono text-xs">{row.administrator_id ?? "System"}</td><td className="px-4 py-3">{actionLabel(row.action)}</td><td className="px-4 py-3">{row.module}</td><td className="break-all px-4 py-3 font-mono text-xs">{row.affected_record_id ?? "Not shown"}</td><td className="px-4 py-3"><PlatformStatus value={row.result} /></td><td className="px-4 py-3"><PlatformRowActions label={row.action} actions={[{ label: "View Details", onSelect: () => setSelectedID(row.id) }]} /></td></tr>)}
        {!ready ? <tr className="border-t"><td colSpan={7} className="px-4 py-8 text-center text-sm">{view.kind === "error" && !invalidAdministrator && !invalidRange ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">Couldn&apos;t load activity. <Button type="button" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : invalidAdministrator || invalidRange ? <span>Check the filters above.</span> : <Skeleton aria-label="Loading activity" className="mx-auto h-6 w-48" />}</td></tr> : null}
        {ready && !ready.data.length ? <tr className="border-t"><td colSpan={7} className="px-4 py-8 text-center text-sm text-muted-foreground">No activity found.</td></tr> : null}
      </tbody></table></div>
      <nav aria-label="Audit pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={!ready || cursors.length === 1} onClick={() => setCursors((old) => old.slice(0, -1))}>Previous</Button><Button type="button" variant="outline" disabled={!ready?.meta.next_cursor} onClick={() => { if (ready?.meta.next_cursor) setCursors((old) => [...old, ready.meta.next_cursor]); }}>Next</Button><span className="text-xs text-muted-foreground">Page {cursors.length}; no total available.</span></nav>
    </>}
    <Sheet open={selectedID !== null} onOpenChange={(open) => { if (!open) setSelectedID(null); }}><SheetContent className="w-full overflow-y-auto sm:max-w-md"><SheetHeader><SheetTitle>Activity details</SheetTitle><SheetDescription>Permitted Platform activity only.</SheetDescription></SheetHeader>
      {detail.kind === "loading" || detail.kind === "ready" && !readyDetail ? <Skeleton className="mx-4 h-36" /> : null}
      {detail.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 px-4 text-sm">Couldn&apos;t load details. <Button type="button" variant="outline" onClick={() => { if (selectedID) void loadDetail(selectedID); }}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : null}
      {readyDetail ? <dl className="space-y-4 px-4">{fields.map(([label, value]) => <div key={label}><dt className="text-xs text-muted-foreground">{label}</dt><dd className="break-all text-sm">{value}</dd></div>)}</dl> : null}
    </SheetContent></Sheet>
  </main>;
}
