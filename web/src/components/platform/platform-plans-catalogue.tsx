"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformPlanCatalogue, type PlanCatalogue, type PlanMoney } from "@/lib/platform-plan-catalogue-contract";

type State = { generation: number | null; kind: "loading" | "denied" | "error" } | { generation: number | null; kind: "ready"; data: PlanCatalogue };
function price(value: PlanMoney | null): string { return value ? `${value.kind === "FROM" ? "From " : ""}KES ${value.amount}` : "Not specified"; }
function count(value: number | null): string { return value === null ? "Contract / not specified" : value.toLocaleString("en-KE"); }

export function PlatformPlansCatalogue() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.plan.read") && generation !== null;
  const [state, setState] = useState<State>({ generation, kind: "loading" });
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!allowed || generation === null) { setState({ generation, kind: "denied" }); return; }
    setState({ generation, kind: "loading" });
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
    <div className="app-page-header"><h1 className="type-title">Plans &amp; Subscriptions</h1><p className="type-lead text-muted-foreground">Source-defined Tenderpreneur plans. Platform plan management and subscription inventory are not available here.</p></div>
    {visible.kind === "loading" && <div role="status" aria-label="Loading plan catalogue" className="grid gap-3 sm:grid-cols-2"><Skeleton className="h-36 w-full" /><Skeleton className="h-36 w-full" /></div>}
    {visible.kind === "denied" && <p className="text-sm text-muted-foreground">The plan catalogue is not available for this Platform session.</p>}
    {visible.kind === "error" && <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>Couldn’t load the plan catalogue. No example prices are shown in place of the service.</span><Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div>}
    {visible.kind === "ready" && <>
      <p className="text-xs text-muted-foreground">Catalogue {visible.data.catalogue_version} · KES before VAT. Prices do not include a subscription, invoice, payment or entitlement action.</p>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{visible.data.plans.map((plan) => <section key={plan.key} className="rounded-lg border bg-card p-4" aria-label={`${plan.name} plan`}>
        <h2 className="font-semibold">{plan.name}</h2><p className="mt-1 text-xs text-muted-foreground">{plan.available_cadences.join(" · ")}</p>
        <dl className="mt-4 grid gap-3 text-sm">
          <div><dt className="text-muted-foreground">Monthly base</dt><dd>{price(plan.monthly_base_price)}</dd></div>
          <div><dt className="text-muted-foreground">Annual base</dt><dd>{price(plan.annual_base_price)}</dd></div>
          {plan.trial_base_price && <div><dt className="text-muted-foreground">Trial base</dt><dd>{price(plan.trial_base_price)}</dd></div>}
          <div><dt className="text-muted-foreground">Bid workspaces / {plan.bid_workspace_allowance.period.replaceAll("_", " ").toLowerCase()}</dt><dd>{count(plan.bid_workspace_allowance.count)}</dd></div>
          <div><dt className="text-muted-foreground">Included licences</dt><dd>{count(plan.included_licences)}</dd></div>
          <div><dt className="text-muted-foreground">AI credits / {plan.limits.reset_period.replaceAll("_", " ").toLowerCase()}</dt><dd>{count(plan.limits.ai_credits)}</dd></div>
        </dl>
        {plan.contract_required && <p className="mt-4 text-xs text-muted-foreground">Contract terms required; missing amounts and limits are not estimated.</p>}
      </section>)}</div>
      <p className="text-sm text-muted-foreground">Customer subscriptions and plan editing require separate governed Platform operations; this catalogue does not report their status.</p>
    </>}
  </main>;
}
