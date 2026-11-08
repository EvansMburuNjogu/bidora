"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { PlatformTenantActions } from "@/components/platform/platform-tenant-actions";
import { PlatformSubscriptionManager } from "@/components/platform/platform-subscription-manager";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformTenantDetail, type PlatformTenant } from "@/lib/platform-tenant-contract";

type State = { kind: "loading" } | { kind: "not_found" } | { kind: "error"; message: string; denied?: boolean } | { kind: "ready"; value: PlatformTenant };
export function PlatformTenantDetail({ tenantId }: { tenantId: string }) {
 const { resolution } = useSession();
 return <TenantDetailWorkspace key={`${tenantId}|${resolution.platform_context_generation}`} tenantId={tenantId} />;
}
function TenantDetailWorkspace({ tenantId }: { tenantId: string }) {
  const { resolution, refresh: refreshSession } = useSession();
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<State>({ kind: "loading" });
  const generation = resolution.platform_context_generation;
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      if (!generation) { setState({ kind: "error", message: "You don’t have access to this organisation.", denied: true }); return; }
      setState({ kind: "loading" });
      try {
        const response = await fetch(`/api/platform/tenants/${encodeURIComponent(tenantId)}`, {
          credentials: "include", cache: "no-store", signal: controller.signal,
          headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        });
        if (controller.signal.aborted) return;
        if (response.status === 401 || response.status === 409) { refreshSession(); return; }
        if (response.status === 404) { setState({ kind: "not_found" }); return; }
        if (response.status === 403) { setState({ kind: "error", message: "You don’t have access to this organisation. You may need to verify your account.", denied: true }); return; }
        const value = response.ok ? parsePlatformTenantDetail(await response.json(), tenantId) : null;
        if (!controller.signal.aborted) setState(value ? { kind: "ready", value } : { kind: "error", message: "Data unavailable: organisation details." });
      } catch { if (!controller.signal.aborted) setState({ kind: "error", message: "Data unavailable: organisation details." }); }
    }
    void load();
    return () => controller.abort();
  }, [generation, refreshSession, revision, tenantId]);
  return <main className="app-page">
    <div className="app-page-header flex-row items-start justify-between gap-4"><div><Link href="/platform/tenderpreneurs" className="text-sm text-primary underline-offset-2 hover:underline">← Tenderpreneurs</Link><h1 className="type-title mt-2">Tenderpreneur details</h1><p className="type-lead text-muted-foreground">Organisation details only; customer files are private.</p></div></div>
    {state.kind === "loading" ? <div role="status" aria-label="Loading organization"><Skeleton className="h-40 w-full" /></div> : null}
    {state.kind === "not_found" ? <p role="status" className="rounded-lg border p-4 text-sm">Organisation not found.</p> : null}
    {state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>{state.message}</span>{!state.denied ? <Button type="button" variant="outline" size="sm" onClick={() => setRevision((value) => value + 1)}>Retry</Button> : null}</div> : null}
    {state.kind === "ready" ? <><section aria-label="Organization registry" className="space-y-5 rounded-lg border bg-card p-4 sm:p-6">
      <div><h2 className="text-lg font-medium">{state.value.name}</h2><p className="text-sm text-muted-foreground">Tenderpreneur · {state.value.state.replaceAll("_", " ")}</p></div>
      <dl className="grid gap-4 text-sm sm:grid-cols-2">
        <div><dt className="text-muted-foreground">Service readiness</dt><dd>{state.value.assignment_present && state.value.common_schema?.state === "COMPATIBLE" && state.value.tenant_schema?.state === "COMPATIBLE" ? "Services are ready" : state.value.state === "FAILED" ? "Organisation setup needs attention" : "Service setup is not complete"}</dd></div>
        <div><dt className="text-muted-foreground">Country</dt><dd>{state.value.country}</dd></div>
        <div><dt className="text-muted-foreground">Timezone</dt><dd>{state.value.timezone}</dd></div>
        <div><dt className="text-muted-foreground">Locale</dt><dd>{state.value.locale}</dd></div>
        <div><dt className="text-muted-foreground">Recorded</dt><dd>{new Date(state.value.created_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</dd></div>
        <div><dt className="text-muted-foreground">Updated</dt><dd>{new Date(state.value.updated_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</dd></div>
      </dl>
    </section><PlatformTenantActions tenantID={state.value.id} state={state.value.state} version={state.value.version} onChanged={() => setRevision(old => old + 1)} />{resolution.capabilities.includes("platform.subscription.read") && <PlatformSubscriptionManager key={state.value.id} tenantID={state.value.id} />}</> : null}
  </main>;
}
