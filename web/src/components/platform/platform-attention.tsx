"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformChangePage, type PlatformChangePage } from "@/lib/platform-contract";
import { parsePlatformTenantList, type PlatformTenantList } from "@/lib/platform-tenant-contract";

type ReadState<T> = { kind: "loading" } | { kind: "error" } | { kind: "denied" } | { kind: "ready"; data: T };

function AttentionSource<T>({ title, path, link, parse, render }: {
  title: string;
  path: string;
  link: string;
  parse: (value: unknown) => T | null;
  render: (data: T) => React.ReactNode;
}) {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [state, setState] = useState<ReadState<T>>({ kind: "loading" });
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setState({ kind: "denied" }); return; }
    setState({ kind: "loading" });
    try {
      const response = await fetch(path, { credentials: "include", cache: "no-store", headers: {
        Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation),
      } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); setState({ kind: "denied" }); return; }
      if (response.status === 403) { setState({ kind: "denied" }); return; }
      const data = response.ok ? parse(await response.json()) : null;
      if (current === sequence.current) setState(data ? { kind: "ready", data } : { kind: "error" });
    } catch { if (current === sequence.current) setState({ kind: "error" }); }
  }, [generation, path, parse, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  return <section className="rounded-lg border bg-card p-4" aria-label={title}>
    <div className="flex items-center justify-between gap-3"><h3 className="font-medium">{title}</h3><Link href={link} className="text-sm text-primary hover:underline">Open list</Link></div>
    {state.kind === "loading" && <Skeleton className="mt-3 h-16 w-full" />}
    {state.kind === "error" && <div role="alert" className="mt-3 flex flex-wrap items-center gap-2 text-sm"><span>Couldn’t load {title.toLowerCase()}.</span><Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div>}
    {state.kind === "denied" && <p className="mt-3 text-sm text-muted-foreground">Unavailable for this session.</p>}
    {state.kind === "ready" && render(state.data)}
  </section>;
}

function AdministratorAttention() {
  return <AttentionSource title="Pending administrator requests" path="/api/platform/administrator-changes?state=PENDING_APPROVAL" link="/platform/users/approvals" parse={parsePlatformChangePage} render={(data: PlatformChangePage) => <>
    {data.changes.length === 0 ? <p className="mt-3 text-sm text-muted-foreground">No pending requests in this view.</p> : <ul className="mt-3 space-y-2 text-sm">{data.changes.slice(0, 3).map((change) => <li key={change.id} className="border-t pt-2">{change.action.replaceAll("_", " ").toLowerCase()} · <time dateTime={change.created_at}>{new Date(change.created_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</time></li>)}</ul>}
    {(data.next_cursor || data.changes.length > 3) && <p className="mt-2 text-xs text-muted-foreground">Showing recent requests only; open the list for more.</p>}
    <p className="mt-2 text-xs text-muted-foreground">A request is not necessarily eligible for you to decide.</p>
  </>} />;
}

function TenantAttention() {
  return <AttentionSource title="Tenderpreneurs in FAILED state" path="/api/platform/tenants?state=FAILED" link="/platform/tenderpreneurs" parse={parsePlatformTenantList} render={(data: PlatformTenantList) => <>
    {data.tenants.length === 0 ? <p className="mt-3 text-sm text-muted-foreground">No failed records in this view.</p> : <ul className="mt-3 space-y-2 text-sm">{data.tenants.slice(0, 3).map((tenant) => <li key={tenant.id} className="border-t pt-2"><Link href={`/platform/tenderpreneurs/${tenant.id}`} className="text-primary hover:underline">{tenant.name}</Link></li>)}</ul>}
    {(data.next_cursor || data.tenants.length > 3) && <p className="mt-2 text-xs text-muted-foreground">Showing recent records only; open the list for more.</p>}
  </>} />;
}

export function PlatformAttention() {
  const { resolution } = useSession();
  const generation = resolution.platform_context_generation;
  const governance = resolution.capabilities.includes("platform.admin.read") && (resolution.capabilities.includes("platform.governance.review") || resolution.capabilities.includes("platform.governance.approve"));
  const tenants = resolution.capabilities.includes("platform.tenant.read");
  return <section aria-labelledby="platform-attention-heading" className="space-y-3">
    <h2 id="platform-attention-heading" className="text-base font-semibold">Attention from current records</h2>
    <p className="text-xs text-muted-foreground">Recent bounded views, not platform-wide counts. Each source has its own access check and retry.</p>
    {governance || tenants ? <div className="grid gap-3 lg:grid-cols-2">
      {governance && <AdministratorAttention key={`governance-${generation}`} />}
      {tenants && <TenantAttention key={`tenants-${generation}`} />}
    </div> : <p className="text-sm text-muted-foreground">No accessible attention sources for this session.</p>}
  </section>;
}
