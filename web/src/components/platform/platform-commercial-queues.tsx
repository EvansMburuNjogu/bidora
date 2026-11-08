"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformBillingSummary } from "@/components/platform/platform-billing-summary";
import { PlatformBillingExceptions } from "@/components/platform/platform-billing-exceptions";
import { PlatformReconciliationAction } from "@/components/platform/platform-reconciliation-action";
import { PlatformAdjustmentProposal } from "@/components/platform/platform-adjustment-proposal";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";
import { commercialQueueConfig, commercialQueues, parseCommercialPage, type CommercialPage, type CommercialQueue } from "@/lib/platform-commercial-contract";

type LoadState = { key: string; kind: "loading" } | { key: string; kind: "error"; message: string; denied?: boolean } | { key: string; kind: "ready"; page: CommercialPage };
const title = (value: string) => value.replaceAll("-", " ").replaceAll("_", " ");
function fieldValue(label: string, value: string): string {
  return label.endsWith(" at") && value !== "—" ? `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : value === "true" ? "Yes" : value === "false" ? "No" : /^[A-Z]+(?:_[A-Z]+)+$/.test(value) ? value.replaceAll("_", " ").toLowerCase() : value;
}
export function PlatformCommercialQueues({ initialQueue, initialState, initialReconciliation }: { initialQueue?: CommercialQueue; initialState?: string; initialReconciliation?: string }) {
  const { resolution, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const capabilities = new Set(resolution.capabilities);
  const available = commercialQueues.filter((name) => capabilities.has(commercialQueueConfig[name].capability));
  const [selected, setSelected] = useState<CommercialQueue>(initialQueue ?? "invoices");
  const [filter, setFilter] = useState(initialState ?? "");
  const [secondary, setSecondary] = useState(initialReconciliation ?? "");
  const [state, setState] = useState<LoadState>({ key: "", kind: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  const [pageError, setPageError] = useState(false);
  const sequence = useRef(0);
  const abort = useRef<AbortController | null>(null);
  const queue: CommercialQueue | undefined = capabilities.has(commercialQueueConfig[selected].capability) ? selected :
    capabilities.has(commercialQueueConfig.invoices.capability) ? "invoices" :
    capabilities.has(commercialQueueConfig.payments.capability) ? "payments" : undefined;
  const config = queue ? commercialQueueConfig[queue] : null;
  const stateFilter = config?.states.some((value) => value === filter) ? filter : "";
  const secondaryFilter = config?.secondary.some((value) => value === secondary) ? secondary : "";
  const key = `${queue}|${stateFilter}|${secondaryFilter}|${generation}`;
  const load = useCallback(async (cursor?: string) => {
    if (!queue || !generation) return;
    abort.current?.abort();
    const controller = new AbortController(); abort.current = controller;
    const current = ++sequence.current;
    if (cursor) setLoadingMore(true);
    else { setState({ key, kind: "loading" }); setPageError(false); }
    const query = new URLSearchParams({ limit: "25" });
    if (stateFilter) query.set("state", stateFilter);
    if (secondaryFilter) query.set(queue === "payments" ? "reconciliation_state" : "action", secondaryFilter);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/commercial/${queue}?${query}`, { credentials: "include", cache: "no-store", signal: controller.signal,
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refreshSession(); return; }
      if (response.status === 403) { setState({ key, kind: "error", message: "You don’t have access to these records. You may need to verify your account.", denied: true }); return; }
      const value = response.ok ? parseCommercialPage(await response.json(), queue) : null;
      if (!value) { if (cursor) setPageError(true); else setState({ key, kind: "error", message: "Couldn’t load records." }); return; }
      if (!cursor) setState({ key, kind: "ready", page: value });
      else {
        setState((previous) => {
          if (previous.key !== key || previous.kind !== "ready" || previous.page.next_cursor !== cursor) return previous;
          const seen = new Set(previous.page.items.map((row) => row.id.toLowerCase()));
          return { key, kind: "ready", page: { items: [...previous.page.items, ...value.items.filter((row) => !seen.has(row.id.toLowerCase()))], next_cursor: value.next_cursor } };
        });
        setPageError(false);
      }
    } catch (error) {
      if (current === sequence.current && !(error instanceof DOMException && error.name === "AbortError")) {
        if (cursor) setPageError(true); else setState({ key, kind: "error", message: "Couldn’t load records." });
      }
    } finally { if (current === sequence.current) setLoadingMore(false); }
  }, [queue, generation, key, stateFilter, secondaryFilter, refreshSession]);
  useEffect(() => { const sequenceRef = sequence; const abortRef = abort; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); sequenceRef.current++; abortRef.current?.abort(); }; }, [load]);
  const visible = state.key === key ? state : { key, kind: "loading" } as LoadState;
  return <main className="app-page">
    <div className="app-page-header flex-row items-start justify-between gap-4">
      <div><h1 className="type-title">Billing</h1><p className="text-sm text-muted-foreground">Financial records and payment activity.</p></div>
    </div>
    <PlatformBillingSummary />
    <PlatformBillingExceptions />
    {!queue ? <Alert role="alert" variant="destructive"><AlertTitle>Billing access unavailable</AlertTitle><AlertDescription>You do not have a financial read capability in this context.</AlertDescription></Alert> : <>
      <div className="mb-5 flex flex-wrap items-end gap-4">
        <label className="flex flex-col gap-1 text-sm font-medium" htmlFor="financial-queue">Queue<PlatformSelect id="financial-queue" value={queue} onChange={(event) => { setSelected(event.target.value as CommercialQueue); setFilter(""); setSecondary(""); }}>{available.map((name) => <option key={name} value={name}>{title(name)}</option>)}</PlatformSelect></label>
        <label className="flex flex-col gap-1 text-sm font-medium" htmlFor="financial-state">State<PlatformSelect id="financial-state" value={stateFilter} onChange={(event) => setFilter(event.target.value)}><option value="">All states</option>{config?.states.map((value) => <option key={value} value={value}>{title(value)}</option>)}</PlatformSelect></label>
        {config?.secondary.length ? <label className="flex flex-col gap-1 text-sm font-medium" htmlFor="financial-secondary">{queue === "payments" ? "Reconciliation" : "Action"}<PlatformSelect id="financial-secondary" value={secondaryFilter} onChange={(event) => setSecondary(event.target.value)}><option value="">All</option>{config.secondary.map((value) => <option key={value} value={value}>{title(value)}</option>)}</PlatformSelect></label> : null}
      </div>
      {visible.kind === "loading" ? <div role="status" aria-label="Loading financial queue" className="space-y-3"><Skeleton className="h-28 w-full" /><Skeleton className="h-28 w-full" /></div> : null}
      {visible.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3 text-sm"><span>{visible.message}</span>{visible.denied ? null : <Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button>}</div> : null}
      {visible.kind === "ready" ? <section aria-label={`${title(queue)} queue`} className="space-y-3">
        {visible.page.items.length === 0 ? <p className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">No records in this view.</p> : <ul className="space-y-3">{visible.page.items.map((row) => <li key={row.id} className="rounded-lg border bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2"><p className="break-all font-medium">{row.detail_id ? <Link className="text-primary underline-offset-2 hover:underline" href={`/platform/billing/${queue}/${row.detail_id}`}>{queue === "financial-adjustments" ? "View adjustment" : "View reversal"}</Link> : row.fields.find(field => field.label === "invoice number")?.value ?? (queue === "payments" ? "Payment" : "Financial record")}</p><PlatformStatus value={title(row.state)} /></div>
          <p className="mt-2 font-medium tabular-nums">{row.amount.amount} {row.amount.currency}</p><p className="text-muted-foreground">{row.tenant_name || "Organisation"}</p>
          <dl className="mt-2 grid gap-x-4 gap-y-1 sm:grid-cols-2">{row.fields.filter(({label}) => !label.endsWith(" id")).map(({ label, value }) => <div key={label} className="min-w-0"><dt className="inline text-muted-foreground capitalize">{label}: </dt><dd className="inline break-all">{fieldValue(label, value)}</dd></div>)}</dl>
          <p className="mt-2 text-xs text-muted-foreground">Observed {new Date(row.occurred_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</p>
          {queue === "invoices" && row.state === "PAID" && row.version && row.settled_payment_version ? <div className="mt-3"><PlatformAdjustmentProposal key={`${generation}|${row.id}|${row.version}`} invoiceID={row.id} invoiceVersion={row.version} paymentVersion={row.settled_payment_version} onProposed={() => void load()} /></div> : null}
          {queue === "payments" && row.state === "SETTLED" && row.fields.some(({ label, value }) => label === "reconciliation state" && value === "COMPLETE") && row.version ? <div className="mt-3"><PlatformReconciliationAction key={`${generation}|${row.id}|${row.version}`} paymentID={row.id} version={row.version} onScheduled={() => void load()} /></div> : null}
        </li>)}</ul>}
        {pageError ? <p role="alert" className="text-sm text-destructive">More records could not be loaded. Try again.</p> : null}
        {visible.page.next_cursor ? <Button type="button" variant="outline" disabled={loadingMore} onClick={() => void load(visible.page.next_cursor ?? undefined)}>{loadingMore ? "Loading…" : "Load more"}</Button> : null}
      </section> : null}
    </>}
  </main>;
}
