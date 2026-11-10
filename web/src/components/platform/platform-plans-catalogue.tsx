"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformPlanCatalogue, type PlanCatalogue } from "@/lib/platform-plan-catalogue-contract";
import { PlatformPlanManager } from "@/components/platform/platform-plan-manager";
import { PlatformSubscriptionManager } from "@/components/platform/platform-subscription-manager";

type State = { generation: number | null; kind: "loading" | "denied" | "error" } | { generation: number | null; kind: "ready"; data: PlanCatalogue };

export function PlatformPlansCatalogue() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.plan.read") && generation !== null;
  const canReadSubscriptions = resolution.capabilities.includes("platform.subscription.read") && generation !== null;
  const [state, setState] = useState<State>({ generation, kind: "loading" });
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!allowed || generation === null) { setState({ generation, kind: "denied" }); return; }
    setState(current => current.generation === generation && current.kind === "ready" ? current : { generation, kind: "loading" });
    try {
      const response = await fetch("/api/platform/plans/catalogue", { credentials: "include", cache: "no-store", headers: {
        Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation),
      } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setState({ generation, kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setState({ generation, kind: "denied" }); return; }
      const data = response.ok ? parsePlatformPlanCatalogue(await response.json()) : null;
      if (current === sequence.current) setState(data ? { generation, kind: "ready", data } : { generation, kind: "error" });
    } catch { if (current === sequence.current) setState({ generation, kind: "error" }); }
  }, [allowed, generation, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  const visible = state.generation === generation && allowed ? state : { generation, kind: allowed ? "loading" : "denied" } as State;
  return <main className="app-page space-y-5">
    <div className="app-page-header"><h1 className="type-title">Plans &amp; Subscriptions</h1><p className="type-lead text-muted-foreground">Manage customer subscriptions and plan pricing.</p></div>
    {!allowed && !canReadSubscriptions ? <p className="text-sm text-muted-foreground">Plans and subscriptions are not available for this Platform session.</p> : <Tabs key={`${generation}:${allowed}:${canReadSubscriptions}`} defaultValue={canReadSubscriptions ? "subscriptions" : "plans"}>
      <TabsList aria-label="Plans and subscriptions views">
        <TabsTrigger value="subscriptions" disabled={!canReadSubscriptions}>Subscriptions</TabsTrigger>
        <TabsTrigger value="plans" disabled={!allowed}>Plans</TabsTrigger>
      </TabsList>
      <TabsContent value="subscriptions" className="mt-5 space-y-5">{canReadSubscriptions && <PlatformSubscriptionManager />}</TabsContent>
      <TabsContent value="plans" className="mt-5 space-y-5">
    {visible.kind === "loading" && <div role="status" aria-label="Loading plan catalogue" className="grid gap-3 sm:grid-cols-2"><Skeleton className="h-36 w-full" /><Skeleton className="h-36 w-full" /></div>}
    {visible.kind === "denied" && !canReadSubscriptions && <p className="text-sm text-muted-foreground">The plan catalogue is not available for this Platform session.</p>}
    {visible.kind === "error" && <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>Couldn’t load the plan catalogue. No example prices are shown in place of the service.</span><Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div>}
    {visible.kind === "ready" && <PlatformPlanManager catalogue={visible.data} onPublished={load} />}
      </TabsContent>
    </Tabs>}
  </main>;
}
