"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformSubscriptionInventory, platformSubscriptionStates, type PlatformSubscriptionInventoryRow } from "@/lib/platform-subscription-contract";

type Filters = { state: string; plan: string; cadence: string };
const emptyFilters: Filters = { state: "", plan: "", cadence: "" };
type Page = { generation: number; filter: string; cursor: string | null; rows: PlatformSubscriptionInventoryRow[]; next: string | null };
export function PlatformSubscriptionInventory({ onSelect, revision }: { onSelect: (tenantID: string) => void; revision: number }) {
 const { resolution, refresh } = useSession();
 const generation = resolution.platform_context_generation, allowed = resolution.capabilities.includes("platform.subscription.read");
 const [draftFilter, setDraftFilter] = useState<Filters>(emptyFilters), [filters, setFilters] = useState<Filters>(emptyFilters), [page, setPage] = useState<Page | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState("");
 const [cursors, setCursors] = useState<(string | null)[]>([null]);
 const cursor = cursors[cursors.length - 1];
 const filter = JSON.stringify(filters);
 const sequence = useRef(0);
 const load = useCallback(async () => {
  if (!generation || !allowed) return;
  const current = ++sequence.current; setBusy(true); setError("");
  const params = new URLSearchParams({ limit: "20" }); if (filters.state) params.set("state", filters.state); if (filters.plan) params.set("plan", filters.plan); if (filters.cadence) params.set("cadence", filters.cadence); if (cursor) params.set("cursor", cursor);
  try {
   const response = await fetch(`/api/platform/subscriptions?${params}`, { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
   if (current !== sequence.current) return;
   if (response.status === 401 || response.status === 409) { setPage(null); setError("Session changed. Please try again."); refresh(); return; }
   const data = response.ok ? parsePlatformSubscriptionInventory(await response.json()) : null;
   if (!data || data.subscriptions.length > 20 || cursor && data.subscriptions.some((row) => row.subscription_id.toLowerCase() <= cursor.toLowerCase())) { setPage(null); setError(response.status === 403 ? "Verified Billing access is required." : "Subscription inventory could not be confirmed."); return; }
   setPage({ generation, filter, cursor, rows: data.subscriptions, next: data.next_cursor });
  } catch { if (current === sequence.current) { setPage(null); setError("Subscription inventory is unavailable."); } }
  finally { if (current === sequence.current) setBusy(false); }
 }, [generation, allowed, filter, filters, cursor, refresh]);
 useEffect(() => { const timer = window.setTimeout(() => { void load(); }, 0); const ref = sequence; return () => { window.clearTimeout(timer); ref.current++; }; }, [load, revision]);
 const visible = page?.generation === generation && page.filter === filter && page.cursor === cursor && allowed ? page : null;
 if (!allowed || !generation) return <p role="status" className="text-sm text-muted-foreground">Platform subscription read permission is required.</p>;
 return <section aria-label="Subscription inventory" className="space-y-4">
  <Card className="relative z-10 overflow-visible" role="region" aria-label="Subscription filters"><CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader><CardContent>
   <form className="flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); if (JSON.stringify(draftFilter) === filter && cursor === null) void load(); else { setFilters(draftFilter); setCursors([null]); } }}>
    <div className="min-w-40"><SearchableFilterSelect id="subscription-state" label="Status" value={draftFilter.state} options={[{ value: "", label: "All statuses" }, ...platformSubscriptionStates.map(state => ({ value: state, label: state.replaceAll("_", " ") }))]} onChange={state => setDraftFilter(old => ({ ...old, state }))} /></div>
    <div className="min-w-40"><SearchableFilterSelect id="subscription-plan" label="Plan" value={draftFilter.plan} options={[{ value: "", label: "All plans" }, ...["TRIAL", "BASIC", "GROWTH", "PROFESSIONAL", "ENTERPRISE"].map(value => ({ value, label: value[0] + value.slice(1).toLowerCase() }))]} onChange={plan => setDraftFilter(old => ({ ...old, plan }))} /></div>
    <div className="min-w-40"><SearchableFilterSelect id="subscription-cadence" label="Billing period" value={draftFilter.cadence} options={[{ value: "", label: "All periods" }, ...["TRIAL", "MONTHLY", "ANNUAL", "CONTRACT"].map(value => ({ value, label: value[0] + value.slice(1).toLowerCase() }))]} onChange={cadence => setDraftFilter(old => ({ ...old, cadence }))} /></div>
    {(Object.values(draftFilter).some(Boolean) || Object.values(filters).some(Boolean)) && <Button type="button" size="sm" variant="outline" onClick={() => { setDraftFilter(emptyFilters); setFilters(emptyFilters); setCursors([null]); }}>Clear filters</Button>}
    <Button type="submit" size="sm" disabled={busy}>Apply filters</Button>
   </form>
  </CardContent></Card>
  <div className="overflow-x-auto rounded-lg border bg-card" aria-busy={busy}><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Organisation</th><th scope="col" className="px-4 py-3">Plan</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3">Revision</th><th scope="col" className="px-4 py-3">Term end (UTC)</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y">
   {visible && !busy && visible.rows.map(row => <tr key={row.subscription_id}><th scope="row" className="px-4 py-4 font-medium">{row.tenant_name || "Organisation"}</th><td className="px-4 py-4">{row.plan_key || "Not yet assigned"}</td><td className="px-4 py-4"><PlatformStatus value={row.state} /></td><td className="px-4 py-4">{row.version}</td><td className="px-4 py-4">{row.commercial_period_end ?? "No current term"}</td><td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={row.tenant_name || "Organisation"} actions={row.has_current_term ? [{ label: "View details", onSelect: () => onSelect(row.tenant_id) }] : []} /></div></td></tr>)}
   {(busy || error || !visible || visible.rows.length === 0) && <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">{busy ? <Skeleton role="status" aria-label="Loading subscriptions" className="mx-auto h-6 w-48" /> : error ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">{error}<Button type="button" size="sm" variant="outline" onClick={() => void load()}>Retry</Button></div> : visible ? "No data." : "Loading subscriptions…"}</td></tr>}
  </tbody></table></div>
  <nav aria-label="Subscription pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={busy || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button><span className="text-sm text-muted-foreground">Page {cursors.length}</span><Button type="button" variant="outline" disabled={busy || !visible?.next} onClick={() => { if (visible?.next) setCursors(old => [...old, visible.next]); }}>Next</Button></nav>
 </section>;
}
