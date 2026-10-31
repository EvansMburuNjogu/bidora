"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { parseCommercialPage, type CommercialPage, type CommercialQueue } from "@/lib/platform-commercial-contract";

type View = { kind: "loading" | "error" | "denied" } | { kind: "ready"; page: CommercialPage };
const sources = [
  { queue: "invoices", title: "Invoices in review", filter: "state=REVIEW_REQUIRED", capability: "platform.invoice.read" },
  { queue: "payments", title: "Payments with reconciliation due", filter: "reconciliation_state=DUE", capability: "platform.payment.read" },
] as const;

function BillingSource({ source, generation }: { source: typeof sources[number]; generation: number }) {
  const { refresh } = useSession();
  const [view, setView] = useState<View>({ kind: "loading" });
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    setView({ kind: "loading" });
    try {
      const response = await fetch(`/api/platform/commercial/${source.queue}?limit=3&${source.filter}`, {
        credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
      });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setView({ kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setView({ kind: "denied" }); return; }
      const page = response.ok ? parseCommercialPage(await response.json(), source.queue as CommercialQueue) : null;
      if (current === sequence.current) setView(page ? { kind: "ready", page } : { kind: "error" });
    } catch { if (current === sequence.current) setView({ kind: "error" }); }
  }, [generation, source, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  return <section className="rounded-lg border bg-card p-4" aria-label={source.title}>
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-medium">{source.title}</h3><Link href={`/platform/billing?queue=${source.queue}&${source.filter}`} className="text-sm text-primary hover:underline">Open filtered queue</Link></div>
    {view.kind === "loading" && <Skeleton className="mt-3 h-16 w-full" />}
    {view.kind === "denied" && <p className="mt-3 text-sm text-muted-foreground">This view is not available for your current session.</p>}
    {view.kind === "error" && <div role="alert" className="mt-3 flex flex-wrap items-center gap-2 text-sm"><span>Couldn’t load {source.title.toLowerCase()}.</span><Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div>}
    {view.kind === "ready" && <>
      {view.page.items.length ? <ul className="mt-3 space-y-2 text-sm">{view.page.items.map((item) => <li key={item.id} className="border-t pt-2"><p className="break-all font-medium">{item.fields.find((field) => field.label === "invoice number")?.value ?? item.id}</p><p className="text-muted-foreground">{item.amount.amount} {item.amount.currency} · {item.state.replaceAll("_", " ")} · {new Date(item.occurred_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</p></li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">No records in this filtered view.</p>}
      {view.page.next_cursor && <p className="mt-2 text-xs text-muted-foreground">More records may be available in the filtered queue; this is not a total.</p>}
    </>}
  </section>;
}

export function PlatformBillingExceptions() {
  const { resolution } = useSession();
  const generation = resolution.platform_context_generation;
  const available = generation === null ? [] : sources.filter((source) => resolution.capabilities.includes(source.capability));
  if (!available.length) return null;
  return <section className="mb-5 space-y-3" aria-labelledby="billing-review-heading"><h2 id="billing-review-heading" className="text-base font-semibold">Billing review views</h2>
    <p className="text-xs text-muted-foreground">Current source-filtered queue rows, not platform totals or authority to reconcile or adjust a payment.</p>
    <div className="grid gap-3 lg:grid-cols-2">{available.map((source) => <BillingSource key={`${source.queue}-${generation}`} source={source} generation={generation!} />)}</div>
  </section>;
}
