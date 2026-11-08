"use client";

import Link from "next/link";
import { PlatformSelect } from "@/components/platform/platform-select";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon, SearchIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
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
  const activeSearch = search.trim().length >= 2 ? search.trim() : "";
  const [country, setCountry] = useState("");
  const [state, setState] = useState<State>({ kind: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  const [pageError, setPageError] = useState(false);
  const sequence = useRef(0);
  const load = useCallback(async (cursor?: string) => {
    const current = ++sequence.current;
    if (!generation) { setState({ kind: "error", message: "You don’t have access to Tenderpreneurs.", denied: true }); return; }
    if (cursor) setLoadingMore(true);
    else { setState({ kind: "loading" }); setPageError(false); }
    const query = new URLSearchParams();
    if (filter) query.set("state", filter);
    if (country) query.set("country", country);
    if (activeSearch) query.set("search", activeSearch);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/tenants${query.size ? `?${query}` : ""}`, {
        credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
      });
      if (sequence.current !== current) return;
      if (response.status === 401 || response.status === 409) { refreshSession(); return; }
      if (response.status === 403) { setState({ kind: "error", message: "You don’t have access to Tenderpreneurs. You may need to verify your account.", denied: true }); return; }
      const value = response.ok ? parsePlatformTenantList(await response.json()) : null;
      if (!value) { if (cursor) setPageError(true); else setState({ kind: "error", message: "Couldn’t load organisations." }); return; }
      if (!cursor) setState({ kind: "ready", value });
      else {
        setState((previous) => {
          if (previous.kind !== "ready" || previous.value.next_cursor !== cursor) return previous;
          const ids = new Set(previous.value.tenants.map((item) => item.id.toLowerCase()));
          return { kind: "ready", value: { tenants: [...previous.value.tenants, ...value.tenants.filter((item) => !ids.has(item.id.toLowerCase()))], next_cursor: value.next_cursor } };
        });
        setPageError(false);
      }
    } catch { if (sequence.current === current) { if (cursor) setPageError(true); else setState({ kind: "error", message: "Couldn’t load organisations." }); } }
    finally { if (sequence.current === current) setLoadingMore(false); }
  }, [generation, filter, country, activeSearch, refreshSession]);
  useEffect(() => { const sequenceRef = sequence; const timer = window.setTimeout(() => void load(), activeSearch ? 250 : 0); return () => { window.clearTimeout(timer); sequenceRef.current++; }; }, [load, activeSearch]);
  return <main className="app-page">
    <div className="app-page-header flex-row items-start justify-between gap-4">
      <div><h1 className="type-title">Tenderpreneurs</h1><p className="type-lead text-muted-foreground">Registered Tenderpreneur organisations and their current account status.</p></div>
      <Button type="button" variant="outline" onClick={() => void load()}>Refresh organisations</Button>
    </div>
    <div className="mb-5 flex flex-wrap items-end gap-3"><label className="grid gap-1 text-sm">Search all organisations<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" value={search} maxLength={100} onChange={(event) => setSearch(event.target.value)} placeholder="Enter at least 2 characters" /></div></label><label className="grid gap-1 text-sm">Country<PlatformSelect value={country} onChange={(event) => setCountry(event.target.value)}><option value="">All countries</option>{countryReferences.map((item) => <option key={item.code} value={item.code}>{item.name} ({item.code})</option>)}</PlatformSelect></label><label htmlFor="tenant-state" className="flex flex-col gap-1 text-sm font-medium">Status<PlatformSelect id="tenant-state" value={filter} onChange={(event) => setFilter(event.target.value as PlatformTenantState | "")}><option value="">All statuses</option>{platformTenantStates.map((item) => <option key={item} value={item}>{item.replaceAll("_", " ")}</option>)}</PlatformSelect></label>{filter || search || country ? <Button type="button" size="sm" variant="outline" onClick={() => { setFilter(""); setSearch(""); setCountry(""); }}>Clear Filters</Button> : null}</div>
    {search.trim().length === 1 && <p className="mb-3 text-xs text-muted-foreground">Enter at least two characters to search all organisations.</p>}
    {state.kind === "loading" ? <div role="status" aria-label="Loading organizations" className="space-y-3"><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div> : null}
    {state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3 text-sm"><span>{state.message}</span>{!state.denied ? <Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button> : null}</div> : null}
    {state.kind === "ready" ? <section aria-label="Tenderpreneur organizations" className="space-y-3">
      {state.value.tenants.length === 0 ? <div className="rounded-lg border border-dashed p-6"><h2 className="text-sm font-medium">No records found.</h2>{filter || search || country ? <Button type="button" variant="link" className="mt-2" onClick={() => { setFilter(""); setSearch(""); setCountry(""); }}>Clear Filters</Button> : null}</div> : <div className="overflow-hidden rounded-lg border bg-card"><div className="hidden grid-cols-[minmax(0,2fr)_minmax(5rem,.6fr)_minmax(6rem,.7fr)_minmax(8rem,.9fr)_minmax(10rem,1fr)_minmax(4rem,.5fr)] gap-3 border-b bg-muted/40 px-4 py-3 text-xs font-medium text-muted-foreground md:grid"><span>Organisation</span><span>Country</span><span>Subscription</span><span>Status</span><span>Registered</span><span>Action</span></div><ul className="divide-y">{state.value.tenants.map((tenant) => <li key={tenant.id} className="grid min-w-0 gap-2 px-4 py-4 text-sm md:grid-cols-[minmax(0,2fr)_minmax(5rem,.6fr)_minmax(6rem,.7fr)_minmax(8rem,.9fr)_minmax(10rem,1fr)_minmax(4rem,.5fr)] md:items-center md:gap-3"><span className="min-w-0 break-words font-medium">{tenant.name}</span><span><span className="text-xs text-muted-foreground md:hidden">Country · </span>{tenant.country}</span><span className="text-muted-foreground">{resolution.capabilities.includes("platform.subscription.read") ? <Link href={`/platform/tenderpreneurs/${tenant.id}`} className="text-primary hover:underline">View plan</Link> : "Restricted"}</span><span><span className="text-xs text-muted-foreground md:hidden">Status · </span><PlatformStatus value={tenant.state} /></span><span><span className="text-xs text-muted-foreground md:hidden">Registered · </span><time dateTime={tenant.created_at}>{new Date(tenant.created_at).toLocaleDateString(undefined, { timeZone: "UTC", year: "numeric", month: "short", day: "numeric" })} UTC</time></span><PlatformRowActions label={tenant.name} actions={[{ label: "View details", href: `/platform/tenderpreneurs/${tenant.id}` }]} /></li>)}</ul></div>}
      {pageError ? <p role="alert" className="text-sm text-destructive">Couldn’t load more organisations. Try again.</p> : null}
      {state.value.next_cursor ? <Button type="button" variant="outline" disabled={loadingMore} onClick={() => void load(state.value.next_cursor ?? undefined)}>{loadingMore ? "Loading…" : "Load more organizations"}</Button> : null}
    </section> : null}
  </main>;
}
