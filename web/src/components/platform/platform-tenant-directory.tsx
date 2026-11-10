"use client";

import Link from "next/link";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon, SearchIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformTenantList, platformTenantStates, type PlatformTenantList, type PlatformTenantState } from "@/lib/platform-tenant-contract";
import { countryReferences } from "@/lib/country-reference";

type State = { kind: "loading" } | { kind: "error"; message: string; denied?: boolean } | { kind: "ready"; value: PlatformTenantList };
export function PlatformTenantDirectory() {
  const { resolution, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const [filter, setFilter] = useState<PlatformTenantState | "">("");
  const [search, setSearch] = useState("");
  const [country, setCountry] = useState("");
  const [applied, setApplied] = useState<{ search: string; filter: PlatformTenantState | ""; country: string }>({ search: "", filter: "", country: "" });
  const activeSearch = applied.search;
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const cursor = cursors[cursors.length - 1];
  const [state, setState] = useState<State>({ kind: "loading" });
  const sequence = useRef(0);
  const clearFilters = () => { setFilter(""); setSearch(""); setCountry(""); setApplied({ search: "", filter: "", country: "" }); setCursors([null]); };
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setState({ kind: "error", message: "You don’t have access to Tenderpreneurs.", denied: true }); return; }
    setState({ kind: "loading" });
    const query = new URLSearchParams();
    if (applied.filter) query.set("state", applied.filter);
    if (applied.country) query.set("country", applied.country);
    if (activeSearch) query.set("search", activeSearch);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/tenants${query.size ? `?${query}` : ""}`, {
        credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
      });
      if (sequence.current !== current) return;
      if (response.status === 401 || response.status === 409) { setState({ kind: "error", message: "Session changed. Please try again." }); refreshSession(); return; }
      if (response.status === 403) { setState({ kind: "error", message: "You don’t have access to Tenderpreneurs.", denied: true }); return; }
      const value = response.ok ? parsePlatformTenantList(await response.json()) : null;
      if (sequence.current === current) setState(value ? { kind: "ready", value } : { kind: "error", message: "Couldn’t load organisations." });
    } catch { if (sequence.current === current) setState({ kind: "error", message: "Couldn’t load organisations." }); }
  }, [generation, applied.filter, applied.country, activeSearch, cursor, refreshSession]);
  useEffect(() => { const sequenceRef = sequence; const timer = window.setTimeout(() => void load(), activeSearch ? 250 : 0); return () => { window.clearTimeout(timer); sequenceRef.current++; }; }, [load, activeSearch]);
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Tenderpreneurs</h1><p className="type-lead text-muted-foreground">Registered Tenderpreneur organisations and their current account status.</p></div>
    <Card className="relative z-10 mb-5 overflow-visible" role="region" aria-label="Tenderpreneur filters">
      <CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader>
      <CardContent><form className="flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); const trimmed = search.trim(); if (trimmed.length === 1) return; setApplied({ search: trimmed, filter, country }); setCursors([null]); }}>
        <label className="grid min-w-0 gap-1 text-sm">Search all organisations<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" value={search} maxLength={100} onChange={(event) => setSearch(event.target.value)} placeholder="Enter at least 2 characters" /></div></label>
        <div className="min-w-48"><SearchableFilterSelect id="tenant-country" label="Country" value={country} options={[{ value: "", label: "All countries" }, ...countryReferences.map(item => ({ value: item.code, label: `${item.name} (${item.code})` }))]} onChange={setCountry} /></div>
        <div className="min-w-40"><SearchableFilterSelect id="tenant-state" label="Status" value={filter} options={[{ value: "", label: "All statuses" }, ...platformTenantStates.map(item => ({ value: item, label: item.replaceAll("_", " ") }))]} onChange={value => setFilter(value as PlatformTenantState | "")} /></div>
        {filter || search || country || applied.search || applied.filter || applied.country ? <Button type="button" size="sm" variant="outline" onClick={clearFilters}>Clear filters</Button> : null}
        <Button type="submit" size="sm" disabled={search.trim().length === 1}><SearchIcon aria-hidden="true" />Apply filters</Button>
      </form>{search.trim().length === 1 && <p className="mt-3 text-xs text-muted-foreground">Enter at least two characters to search all organisations.</p>}</CardContent>
    </Card>
    {state.kind === "error" && state.denied ? <p role="alert" className="text-sm">{state.message}</p> : <>
      <section aria-label="Tenderpreneur organisations" aria-busy={state.kind === "loading"} className="overflow-x-auto rounded-lg border bg-card">
        <table className="w-full min-w-[680px] text-left text-sm">
          <thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Organisation</th><th scope="col" className="px-4 py-3">Country</th><th scope="col" className="px-4 py-3">Subscription</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3">Registered</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead>
          <tbody className="divide-y">{state.kind === "ready" && state.value.tenants.map((tenant) => <tr key={tenant.id}>
            <th scope="row" className="max-w-72 break-words px-4 py-4 font-medium"><Link href={`/platform/tenderpreneurs/${tenant.id}`} className="text-primary hover:underline">{tenant.name}</Link></th>
            <td className="px-4 py-4">{tenant.country}</td><td className="px-4 py-4 text-muted-foreground">{resolution.capabilities.includes("platform.subscription.read") ? <Link href={`/platform/tenderpreneurs/${tenant.id}`} className="text-primary hover:underline">View plan</Link> : "Restricted"}</td>
            <td className="px-4 py-4"><PlatformStatus value={tenant.state} /></td><td className="whitespace-nowrap px-4 py-4"><time dateTime={tenant.created_at}>{new Date(tenant.created_at).toLocaleDateString(undefined, { timeZone: "UTC", year: "numeric", month: "short", day: "numeric" })} UTC</time></td>
            <td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={tenant.name} actions={[{ label: "View details", href: `/platform/tenderpreneurs/${tenant.id}` }]} /></div></td>
          </tr>)}
          {state.kind !== "ready" || state.value.tenants.length === 0 ? <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">{state.kind === "loading" ? <Skeleton role="status" aria-label="Loading organisations" className="mx-auto h-6 w-48" /> : state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">{state.message}<Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : "No data."}</td></tr> : null}
          </tbody>
        </table>
      </section>
      <nav aria-label="Tenderpreneur pages" className="mt-4 flex items-center gap-3">
        <Button type="button" variant="outline" disabled={state.kind === "loading" || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button>
        <span className="text-sm text-muted-foreground">Page {cursors.length}</span>
        <Button type="button" variant="outline" disabled={state.kind !== "ready" || !state.value.next_cursor} onClick={() => { if (state.kind === "ready" && state.value.next_cursor) setCursors(old => [...old, state.value.next_cursor]); }}>Next</Button>
      </nav>
    </>}
  </main>;
}
