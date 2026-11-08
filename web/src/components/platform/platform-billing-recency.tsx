"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformBillingDashboard, type PlatformBillingDashboard } from "@/lib/platform-dashboard-contract";
import type { CommercialQueue } from "@/lib/platform-commercial-contract";

type State = { generation: number | null; kind: "loading" | "error" | "denied" } | { generation: number | null; kind: "ready"; data: PlatformBillingDashboard };
const sections: { queue: CommercialQueue; title: string }[] = [
  { queue: "invoices", title: "Invoices" }, { queue: "payments", title: "Payments" },
  { queue: "financial-adjustments", title: "Financial adjustments" }, { queue: "payment-reversals", title: "Payment reversals" },
];
const utc = (value: string) => `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC`;
export function PlatformBillingRecency() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [state, setState] = useState<State>({ generation, kind: "loading" });
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setState({ generation, kind: "denied" }); return; }
    setState({ generation, kind: "loading" });
    try {
      const response = await fetch("/api/platform/dashboard", { credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      if (response.status === 403) { setState({ generation, kind: "denied" }); return; }
      const data = response.ok ? parsePlatformBillingDashboard(await response.json()) : null;
      setState(data ? { generation, kind: "ready", data } : { generation, kind: "error" });
    } catch { if (current === sequence.current) setState({ generation, kind: "error" }); }
  }, [generation, refresh]);
  useEffect(() => { const sequenceRef = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); sequenceRef.current++; }; }, [load]);
  const visible = state.generation === generation ? state : { generation, kind: "loading" } as State;
  return <section aria-label="Recent Transactions" className="mt-5 space-y-3">
    <h2 className="text-base font-semibold">Recent Transactions</h2>
    {visible.kind === "loading" ? <div role="status" aria-label="Loading transactions" className="grid gap-3 sm:grid-cols-2"><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div> : null}
    {visible.kind === "denied" ? <p className="text-sm text-muted-foreground">You don’t have access to transactions.</p> : null}
    {visible.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3 text-sm"><span>Data unavailable: transactions.</span><Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : null}
    {visible.kind === "ready" ? <><p className="text-xs text-muted-foreground">As of {utc(visible.data.observed_at)}</p><div className="grid gap-3 lg:grid-cols-2">{sections.map(({ queue, title }) => {
      const window = visible.data.windows[queue];
      return <section key={queue} aria-label={`Recent ${title.toLowerCase()}`} className="rounded-lg border bg-card p-4"><div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-medium">{title}</h3><Link href={`/platform/billing?queue=${queue}`} className="text-sm text-primary hover:underline">Open queue</Link></div>
        {window.items.length ? <ul className="mt-3 space-y-3">{window.items.slice(0, 2).map((row) => <li key={row.id} className="border-t pt-2 text-sm"><p className="break-all font-medium">{row.detail_href ? <Link href={row.detail_href} className="text-primary hover:underline">{row.label}</Link> : row.label}</p><p>{row.amount.amount} {row.amount.currency} · {row.state.replaceAll("_", " ")}</p><p className="text-xs text-muted-foreground">{utc(row.occurred_at)}</p></li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">No recent entries.</p>}
        {window.has_more || window.items.length > 2 ? <p className="mt-2 text-xs text-muted-foreground">View all in {title.toLowerCase()}.</p> : null}
      </section>;
    })}</div></> : null}
  </section>;
}
