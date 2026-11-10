"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { ActivityIcon, CoinsIcon, RotateCcwIcon, SearchIcon, SparklesIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { useSession } from "@/components/session/session-provider";

type Usage = { id: string; tenant_id: string; tenant_name: string; kind: string; actual_credits: string; settled_at: string };
type Page = { data: Usage[]; meta: { trace_id: string; next_cursor: string | null } };
type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: Page };
type Filters = { tenant: string; kind: string; from: string; to: string };
const emptyFilters: Filters = { tenant: "", kind: "", from: "", to: "" };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const kinds: Record<string, string> = { DOCUMENT_PDF: "PDF analysis", DOCUMENT_DOCX: "Word document analysis", GENERATION_TEXT: "Writing assistance" };
function parsePage(value: unknown): Page | null {
 if (!value || typeof value !== "object") return null;
 const body = value as Partial<Page>;
 if (!Array.isArray(body.data) || body.data.length > 25 || !body.meta || typeof body.meta.trace_id !== "string" || !(body.meta.next_cursor === null || typeof body.meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(body.meta.next_cursor))) return null;
 for (const row of body.data) { if (!row || !uuid.test(row.id) || !uuid.test(row.tenant_id) || typeof row.tenant_name !== "string" || row.tenant_name.length > 200 || !kinds[row.kind] || typeof row.actual_credits !== "string" || !/^[1-9]\d{0,18}$/.test(row.actual_credits) || BigInt(row.actual_credits) > BigInt("9223372036854775807") || !Number.isFinite(Date.parse(row.settled_at))) return null; }
 return body as Page;
}
export function PlatformAIUsage() {
 const { resolution } = useSession();
 return <UsageWorkspace key={resolution.platform_context_generation} />;
}
function UsageWorkspace() {
 const [filters, setFilters] = useState<Filters>(emptyFilters);
 const [applied, setApplied] = useState<Filters>(emptyFilters);
 const [filterRevision, setFilterRevision] = useState(0);
 const invalidRange = !!filters.from && !!filters.to && filters.from > filters.to;
 return <main className="app-page min-w-0 space-y-5">
  <div className="app-page-header"><h1 className="type-title">AI Usage</h1><p className="type-lead text-muted-foreground">Review completed AI work and the credits charged to each organisation.</p></div>
  <Card className="relative z-10 overflow-visible" role="region" aria-label="AI usage filters"><CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader><CardContent>
   <form className="space-y-4" onSubmit={event => { event.preventDefault(); if (!invalidRange) { setApplied(filters); setFilterRevision(value => value + 1); } }}>
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
     <div className="min-w-0"><PlatformReferencePicker kind="organizations" id="usage-organisation" label="Organisation" value={filters.tenant} onChange={tenant => setFilters(old => ({ ...old, tenant }))} /></div>
     <SearchableFilterSelect id="usage-activity" label="Activity" value={filters.kind} options={[{ value: "", label: "All activities" }, ...Object.entries(kinds).map(([value, label]) => ({ value, label }))]} onChange={kind => setFilters(old => ({ ...old, kind }))} />
     <label className="grid min-w-0 content-start gap-1 text-sm font-medium">From (UTC)<Input type="date" className="w-full" value={filters.from} max={filters.to || undefined} aria-invalid={invalidRange} aria-describedby={invalidRange ? "usage-date-error" : undefined} onChange={event => setFilters(old => ({ ...old, from: event.target.value }))} /></label>
     <label className="grid min-w-0 content-start gap-1 text-sm font-medium">Through (UTC)<Input type="date" className="w-full" value={filters.to} min={filters.from || undefined} aria-invalid={invalidRange} aria-describedby={invalidRange ? "usage-date-error" : undefined} onChange={event => setFilters(old => ({ ...old, to: event.target.value }))} /></label>
    </div>
    <div className="flex flex-wrap items-center gap-2"><Button type="submit" size="sm" disabled={invalidRange}><SearchIcon aria-hidden="true" />Apply filters</Button>{(Object.values(filters).some(Boolean) || Object.values(applied).some(Boolean)) && <Button type="button" size="sm" variant="outline" onClick={() => { setFilters(emptyFilters); setApplied(emptyFilters); setFilterRevision(value => value + 1); }}>Clear filters</Button>}</div>
   </form>
   {invalidRange && <p id="usage-date-error" role="alert" className="mt-3 text-sm text-destructive">The end date must not precede the start date.</p>}
  </CardContent></Card>
  <UsageResults key={`${JSON.stringify(applied)}:${filterRevision}`} filters={applied} />
 </main>;
}
function UsageResults({ filters }: { filters: Filters }) {
 const { resolution, refresh } = useSession(); const generation = resolution.platform_context_generation;
 const [cursors, setCursors] = useState<(string | null)[]>([null]), [view, setView] = useState<View>({ kind: "loading" }); const sequence = useRef(0); const cursor = cursors[cursors.length - 1];
 const load = useCallback(async () => {
  const current = ++sequence.current; if (!generation) { setView({ kind: "denied" }); return; } setView({ kind: "loading" });
  try {
   const query = new URLSearchParams({ limit: "25" }); if (cursor) query.set("cursor", cursor); if (filters.tenant) query.set("tenant_id", filters.tenant); if (filters.kind) query.set("kind", filters.kind); if (filters.from) query.set("from", filters.from); if (filters.to) query.set("to", filters.to);
   const response = await fetch(`/api/platform/ai-usage?${query}`, { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
   if (current !== sequence.current) return;
   if (response.status === 401 || response.status === 409) { setView({ kind: "error" }); refresh(); return; } if (response.status === 403) { setView({ kind: "denied" }); return; }
   const page = response.ok ? parsePage(await response.json()) : null; if (current === sequence.current) setView(page ? { kind: "ready", page } : { kind: "error" });
  } catch { if (current === sequence.current) setView({ kind: "error" }); }
 }, [generation, cursor, filters, refresh]);
 useEffect(() => { const active = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); active.current++; }; }, [load]);
 return <section className="space-y-4" aria-label="Completed AI work">
  {view.kind === "denied" ? <p role="alert" className="text-sm">You do not have access to AI usage.</p> : <>
   {view.kind === "ready" && <div className="grid gap-3 sm:grid-cols-2">
    <Card><CardContent className="flex items-center gap-4"><span className="grid size-10 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"><ActivityIcon aria-hidden="true" className="size-5" /></span><div><p className="text-sm text-muted-foreground">Completed operations · this page</p><p className="text-xl font-semibold tabular-nums">{view.page.data.length.toLocaleString("en-KE")}</p></div></CardContent></Card>
    <Card><CardContent className="flex items-center gap-4"><span className="grid size-10 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"><CoinsIcon aria-hidden="true" className="size-5" /></span><div><p className="text-sm text-muted-foreground">Credits charged · this page</p><p className="text-xl font-semibold tabular-nums">{view.page.data.reduce((total, row) => total + BigInt(row.actual_credits), BigInt(0)).toLocaleString("en-KE")}</p></div></CardContent></Card>
   </div>}
   <div className="overflow-hidden rounded-lg border bg-card" aria-busy={view.kind === "loading"}><div className="border-b px-4 py-4"><h2 className="text-sm font-semibold">Completed activity</h2><p className="mt-1 text-xs text-muted-foreground">Measured credits for completed AI work.</p></div><div className="overflow-x-auto"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-muted/40"><tr>{["Organisation", "Activity", "Credits charged", "Completed (UTC)"].map(label => <th key={label} scope="col" className="px-4 py-3">{label}</th>)}</tr></thead><tbody className="divide-y">
    {view.kind === "ready" && view.page.data.map(row => <tr key={row.id}><th scope="row" className="px-4 py-4 font-medium">{row.tenant_name || "Organisation"}</th><td className="px-4 py-4">{kinds[row.kind]}</td><td className="px-4 py-4 tabular-nums">{BigInt(row.actual_credits).toLocaleString("en-KE")}</td><td className="whitespace-nowrap px-4 py-4">{new Date(row.settled_at).toLocaleString(undefined, { timeZone: "UTC" })}</td></tr>)}
    {(view.kind !== "ready" || view.page.data.length === 0) && <tr><td colSpan={4} className="px-4 py-8 text-center text-sm text-muted-foreground">{view.kind === "loading" ? <Skeleton role="status" aria-label="Loading AI usage" className="mx-auto h-6 w-48" /> : view.kind === "error" ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">Usage could not be loaded.<Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : <div className="flex flex-col items-center gap-2"><SparklesIcon aria-hidden="true" className="size-5 text-muted-foreground" /><span className="font-medium text-foreground">No completed activity</span><span>There is no AI usage for the selected filters.</span></div>}</td></tr>}
   </tbody></table></div></div>
   <nav aria-label="Usage pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={view.kind === "loading" || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button><span className="text-sm text-muted-foreground">Page {cursors.length}</span><Button type="button" variant="outline" disabled={view.kind !== "ready" || !view.page.meta.next_cursor} onClick={() => { if (view.kind === "ready" && view.page.meta.next_cursor) setCursors(old => [...old, view.page.meta.next_cursor]); }}>Next</Button></nav>
  </>}
 </section>;
}
