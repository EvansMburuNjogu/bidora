"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PlatformStatus } from "@/components/platform/platform-status";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { commercialQueueConfig, commercialUUID, parseCommercialDetail, type CommercialDetail } from "@/lib/platform-commercial-contract";

type DetailKind = "financial-adjustments" | "payment-reversals";
type State = { key: string; kind: "loading" } | { key: string; kind: "error"; message: string; denied?: boolean } | { key: string; kind: "not_found" } | { key: string; kind: "ready"; value: CommercialDetail };
export function PlatformCommercialDetail({ queue, recordID }: { queue: string; recordID: string }) {
  const { resolution, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const allowedQueue = queue === "financial-adjustments" || queue === "payment-reversals" ? queue as DetailKind : null;
  const canRead = allowedQueue && resolution.capabilities.includes(commercialQueueConfig[allowedQueue].capability);
  const [state, setState] = useState<State>({ key: "", kind: "loading" });
  const key = `${queue}|${recordID}|${generation}|${canRead}`;
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!allowedQueue || !commercialUUID.test(recordID) || !canRead || !generation) { setState({ key, kind: "not_found" }); return; }
    setState({ key, kind: "loading" });
    try {
      const response = await fetch(`/api/platform/commercial/${allowedQueue}/${recordID}`, { credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refreshSession(); return; }
      if (response.status === 404) { setState({ key, kind: "not_found" }); return; }
      if (response.status === 403) { setState({ key, kind: "error", message: "You don’t have access to this record. You may need to verify your account.", denied: true }); return; }
      const value = response.ok ? parseCommercialDetail(await response.json(), allowedQueue, recordID) : null;
      setState(value ? { key, kind: "ready", value } : { key, kind: "error", message: "Couldn’t load this record." });
    } catch { if (current === sequence.current) setState({ key, kind: "error", message: "Couldn’t load this record." }); }
  }, [allowedQueue, recordID, canRead, generation, key, refreshSession]);
  useEffect(() => { const sequenceRef = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); sequenceRef.current++; }; }, [load]);
  const visible = state.key === key ? state : { key, kind: "loading" } as State;
  return <main className="app-page">
    <div className="app-page-header"><Link href="/platform/billing" className="text-sm text-primary hover:underline">← Billing</Link><h1 className="type-title">Financial record</h1><p className="type-lead text-muted-foreground">Check the status before treating a refund as complete.</p></div>
    {visible.kind === "loading" ? <div role="status" aria-label="Loading financial record"><Skeleton className="h-52 w-full" /></div> : null}
    {visible.kind === "not_found" ? <p role="status" className="rounded-lg border p-4 text-sm">Record not found.</p> : null}
    {visible.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>{visible.message}</span>{!visible.denied ? <Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button> : null}</div> : null}
    {visible.kind === "ready" ? <section aria-label={visible.value.title} className="rounded-lg border bg-card p-5">
      <h2 className="text-lg font-semibold">{visible.value.title}</h2>
      <p className="mt-2 break-all text-sm text-muted-foreground">{visible.value.id}</p><p className="mt-3 text-xl font-medium tabular-nums">{visible.value.amount.amount} {visible.value.amount.currency}</p><p className="mt-2 text-sm"><PlatformStatus value={visible.value.state} /></p>
      <dl className="mt-5 grid gap-x-5 gap-y-3 text-sm sm:grid-cols-2">{visible.value.fields.map(({ label, value }) => <div key={label} className="min-w-0 border-t pt-2"><dt className="capitalize text-muted-foreground">{label}</dt><dd className="break-all">{label.endsWith(" at") && value !== "—" ? `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : value}</dd></div>)}</dl>
    </section> : null}
  </main>;
}
