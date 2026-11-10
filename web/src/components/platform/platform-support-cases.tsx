"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { PlusIcon, RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";
import { useSession } from "@/components/session/session-provider";
import { parseSupportPage, parseSupportReceipt, type SupportCase } from "@/lib/platform-support-contract";

type Filters = { state: string; assignment: string };
const emptyFilters: Filters = { state: "", assignment: "" };
type View = { generation: number; cursor: string | null; filterKey: string; rows: SupportCase[]; next: string | null };
export function PlatformSupportCases() {
 const { resolution } = useSession();
 return <SupportCasesWorkspace key={resolution.platform_context_generation} />;
}
function SupportCasesWorkspace() {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const read = resolution.capabilities.includes("platform.support.case.read");
  const manage = resolution.capabilities.includes("platform.support.case.manage");
  const [view, setView] = useState<View | null>(null);
  const [filters, setFilters] = useState<Filters>(emptyFilters), [applied, setApplied] = useState<Filters>(emptyFilters);
  const filterKey = JSON.stringify(applied);
  const [loading, setLoading] = useState(false), [error, setError] = useState("");
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const cursor = cursors[cursors.length - 1];
  const [open, setOpen] = useState(false), [submitting, setSubmitting] = useState(false), [createError, setCreateError] = useState("");
  const [tenant, setTenant] = useState(""), [title, setTitle] = useState(""), [description, setDescription] = useState("");
  const [created, setCreated] = useState<string | null>(null);
  const key = useRef<string | null>(null), sequence = useRef(0), locked = useRef(false);
  const load = useCallback(async () => {
    if (!generation || !read) return;
    const current = ++sequence.current; setLoading(true); setError("");
    const params = new URLSearchParams({ limit: "20" }); if (cursor) params.set("cursor", cursor); if (applied.state) params.set("state", applied.state); if (applied.assignment) params.set("assignment", applied.assignment);
    try {
      const response = await fetch(`/api/platform/support/cases?${params}`, { cache: "no-store", credentials: "include", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setView(null); setError("Session changed. Please try again."); refresh(); return; }
      const page = response.ok ? parseSupportPage(await response.json()) : null;
      if (!page) { setView(null); setError(response.status === 403 ? "You do not have permission to view support cases." : "Support cases could not be confirmed."); return; }
      setView({ generation, cursor, filterKey, rows: page.cases, next: page.next });
    } catch { if (current === sequence.current) { setView(null); setError("Support cases are unavailable."); } }
    finally { if (current === sequence.current) setLoading(false); }
  }, [generation, read, cursor, applied, filterKey, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); const current = sequence; return () => { window.clearTimeout(timer); current.current++; }; }, [load]);
  async function create() {
    if (locked.current || !generation || !csrfToken || !manage || !tenant || title.trim().length < 3 || description.trim().length < 3) return;
    locked.current = true; setSubmitting(true); setCreateError("");
    const idempotency = key.current ?? crypto.randomUUID(); key.current = idempotency;
    try {
      const response = await fetch("/api/platform/support/cases", { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": idempotency }, body: JSON.stringify({ tenant_id: tenant, title: title.trim(), description: description.trim() }) });
      if (response.status === 401) { setCreateError("Your session changed. Please try again."); refresh(); return; }
      const receipt = response.ok ? parseSupportReceipt(await response.json()) : null;
      if (!receipt) { setCreateError(response.status === 403 ? "Support management requires a verified session." : response.status === 404 ? "That organisation was not found." : response.status === 409 ? "The case changed. Review before resubmitting." : "The outcome could not be confirmed. Retry without changing the details."); return; }
      key.current = null; setCreated(receipt.case_id); setTenant(""); setTitle(""); setDescription(""); setOpen(false);
      if (cursor) setCursors([null]); else await load();
    } catch { setCreateError("The outcome is uncertain. Retry without changing the details."); }
    finally { locked.current = false; setSubmitting(false); }
  }
  const visible = view?.generation === generation && view.cursor === cursor && view.filterKey === filterKey && read ? view : null;
  return <main className="app-page space-y-5">
    <div className="app-page-header flex-row items-start justify-between gap-4"><div><h1 className="type-title">Support</h1><p className="type-lead text-muted-foreground">Manage customer support cases.</p></div>{read && manage && generation && <Button type="button" onClick={() => { setCreateError(""); setOpen(true); }}><PlusIcon aria-hidden="true" />New case</Button>}</div>
    {!read || !generation ? <p role="status" className="text-sm">You do not have access to support cases.</p> : <>
      {created && <p role="status" className="text-sm">Support case created. <Link href={`/platform/support/${created}`} className="text-primary underline-offset-2 hover:underline">Open case</Link></p>}
      <Card className="relative z-10 overflow-visible" role="region" aria-label="Support filters"><CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader><CardContent><form className="flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); if (JSON.stringify(filters) === filterKey && cursor === null) void load(); else { setApplied(filters); setCursors([null]); } }}>
        <div className="min-w-40"><SearchableFilterSelect id="support-status-filter" label="Status" value={filters.state} options={[{ value: "", label: "All statuses" }, { value: "OPEN", label: "Open" }, { value: "RESOLVED", label: "Resolved" }]} onChange={state => setFilters(old => ({ ...old, state }))} /></div>
        <div className="min-w-40"><SearchableFilterSelect id="support-assignment-filter" label="Assignment" value={filters.assignment} options={[{ value: "", label: "All assignments" }, { value: "ASSIGNED", label: "Assigned" }, { value: "UNASSIGNED", label: "Unassigned" }]} onChange={assignment => setFilters(old => ({ ...old, assignment }))} /></div>
        {(Object.values(filters).some(Boolean) || Object.values(applied).some(Boolean)) && <Button type="button" size="sm" variant="outline" onClick={() => { setFilters(emptyFilters); setApplied(emptyFilters); setCursors([null]); }}>Clear filters</Button>}
        <Button type="submit" size="sm">Apply filters</Button>
      </form></CardContent></Card>
      <section aria-label="Support cases" aria-busy={loading} className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Case</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3">Assignment</th><th scope="col" className="px-4 py-3">Updated (UTC)</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y">
        {!loading && visible?.rows.map(row => <tr key={row.id}><th scope="row" className="max-w-96 px-4 py-4 font-medium"><Link href={`/platform/support/${row.id}`} className="break-words text-primary hover:underline">{row.title}</Link></th><td className="px-4 py-4"><PlatformStatus value={row.state} /></td><td className="px-4 py-4">{row.assigned_to ? "Assigned" : "Unassigned"}</td><td className="whitespace-nowrap px-4 py-4">{new Date(row.updated_at).toLocaleString(undefined, { timeZone: "UTC" })}</td><td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={row.title} actions={[{ label: "View details", href: `/platform/support/${row.id}` }]} /></div></td></tr>)}
        {(loading || error || !visible || visible.rows.length === 0) && <tr><td colSpan={5} className="px-4 py-8 text-center text-sm text-muted-foreground">{loading || !visible && !error ? <Skeleton role="status" aria-label="Loading support cases" className="mx-auto h-6 w-48" /> : error ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">{error}<Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : "No data."}</td></tr>}
      </tbody></table></section>
      <nav aria-label="Support case pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={loading || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button><span className="text-sm text-muted-foreground">Page {cursors.length}</span><Button type="button" variant="outline" disabled={loading || !visible?.next} onClick={() => { if (visible?.next) setCursors(old => [...old, visible.next]); }}>Next</Button></nav>
    </>}
    {manage && read && <AlertDialog open={open} onOpenChange={next => { if (!locked.current) setOpen(next); }}><AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto data-[size=default]:max-w-[calc(100vw-2rem)] data-[size=default]:sm:max-w-xl"><form className="space-y-4" onSubmit={event => { event.preventDefault(); void create(); }}><AlertDialogHeader><AlertDialogTitle>New support case</AlertDialogTitle><AlertDialogDescription>Keep passwords, verification codes and private customer documents out of case notes.</AlertDialogDescription></AlertDialogHeader>
      <div className="space-y-4"><PlatformReferencePicker kind="organizations" id="support-organisation" label="Organisation" value={tenant} disabled={submitting} onChange={id => { key.current = null; setTenant(id); }} /><label className="grid gap-1 text-sm">Title<Input required minLength={3} maxLength={120} value={title} disabled={submitting} onChange={event => { key.current = null; setTitle(event.target.value); }} /></label><label className="grid gap-1 text-sm">Description<textarea required minLength={3} maxLength={2000} className="min-h-28 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={description} disabled={submitting} onChange={event => { key.current = null; setDescription(event.target.value); }} /></label></div>
      {createError && <p role="alert" className="text-sm text-destructive">{createError}</p>}
      <AlertDialogFooter><AlertDialogCancel disabled={submitting}>Cancel</AlertDialogCancel><Button type="submit" disabled={submitting || !csrfToken || !tenant || title.trim().length < 3 || description.trim().length < 3}>{submitting ? "Creating…" : "Create case"}</Button></AlertDialogFooter>
    </form></AlertDialogContent></AlertDialog>}
  </main>;
}
