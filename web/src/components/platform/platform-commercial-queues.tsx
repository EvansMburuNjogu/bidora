"use client";

import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformBillingSummary } from "@/components/platform/platform-billing-summary";
import { PlatformReconciliationAction } from "@/components/platform/platform-reconciliation-action";
import { PlatformAdjustmentProposal } from "@/components/platform/platform-adjustment-proposal";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/components/session/session-provider";
import { commercialQueueConfig, commercialQueues, parseCommercialPage, type CommercialItem, type CommercialPage, type CommercialQueue } from "@/lib/platform-commercial-contract";

type LoadState = { key: string; kind: "loading" } | { key: string; kind: "error"; message: string; denied?: boolean } | { key: string; kind: "ready"; page: CommercialPage };
const title = (value: string) => value.replaceAll("-", " ").replaceAll("_", " ");
function fieldValue(label: string, value: string): string {
  return label.endsWith(" at") && value !== "—" ? `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : value === "true" ? "Yes" : value === "false" ? "No" : /^[A-Z]+(?:_[A-Z]+)+$/.test(value) ? value.replaceAll("_", " ").toLowerCase() : value;
}
export function PlatformCommercialQueues({ initialQueue, initialState, initialReconciliation }: { initialQueue?: CommercialQueue; initialState?: string; initialReconciliation?: string }) {
  const { resolution } = useSession();
  const canInvoices = resolution.capabilities.includes("platform.invoice.read");
  const canPayments = resolution.capabilities.includes("platform.payment.read");
  const requestedTab = initialQueue ? (initialQueue === "payments" || initialQueue === "payment-reversals" ? "payments" : "invoices") : "overview";
  const initialTab = requestedTab === "invoices" && !canInvoices || requestedTab === "payments" && !canPayments ? "overview" : requestedTab;
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Billing</h1><p className="text-sm text-muted-foreground">Financial records and payment activity.</p></div>
    <Tabs key={`${resolution.platform_context_generation}:${canInvoices}:${canPayments}`} defaultValue={initialTab}>
      <TabsList aria-label="Billing views"><TabsTrigger value="overview">Overview</TabsTrigger><TabsTrigger value="invoices" disabled={!canInvoices}>Invoices</TabsTrigger><TabsTrigger value="payments" disabled={!canPayments}>Payments</TabsTrigger></TabsList>
      <TabsContent value="overview" className="mt-5 space-y-5"><PlatformBillingSummary /></TabsContent>
      <TabsContent value="invoices" className="mt-5">{canInvoices && <FinancialQueue group="invoices" initialQueue={initialQueue === "financial-adjustments" ? initialQueue : "invoices"} initialState={initialTab === "invoices" ? initialState : undefined} />}</TabsContent>
      <TabsContent value="payments" className="mt-5">{canPayments && <FinancialQueue group="payments" initialQueue={initialQueue === "payment-reversals" ? initialQueue : "payments"} initialState={initialTab === "payments" ? initialState : undefined} initialReconciliation={initialTab === "payments" ? initialReconciliation : undefined} />}</TabsContent>
    </Tabs>
  </main>;
}

function FinancialQueue({ group, initialQueue, initialState, initialReconciliation }: { group: "invoices" | "payments"; initialQueue: CommercialQueue; initialState?: string; initialReconciliation?: string }) {
  const { resolution, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const capabilities = new Set(resolution.capabilities);
  const available = commercialQueues.filter((name) => (group === "invoices" ? name === "invoices" || name === "financial-adjustments" : name === "payments" || name === "payment-reversals") && capabilities.has(commercialQueueConfig[name].capability));
  const [selected, setSelected] = useState<CommercialQueue>(initialQueue ?? "invoices");
  const [filter, setFilter] = useState(initialState ?? "");
  const [secondary, setSecondary] = useState(initialReconciliation ?? "");
  const [applied, setApplied] = useState({ selected: initialQueue ?? "invoices", filter: initialState ?? "", secondary: initialReconciliation ?? "" });
  const [state, setState] = useState<LoadState>({ key: "", kind: "loading" });
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const cursor = cursors[cursors.length - 1];
  const [selectedRecord, setSelectedRecord] = useState<CommercialItem | null>(null);
  const sequence = useRef(0);
  const abort = useRef<AbortController | null>(null);
  const queue: CommercialQueue | undefined = capabilities.has(commercialQueueConfig[applied.selected].capability) && (group === "invoices" ? applied.selected === "invoices" || applied.selected === "financial-adjustments" : applied.selected === "payments" || applied.selected === "payment-reversals") ? applied.selected : capabilities.has(commercialQueueConfig[group].capability) ? group : undefined;
  const config = queue ? commercialQueueConfig[queue] : null;
  const draftQueue = capabilities.has(commercialQueueConfig[selected].capability) ? selected : queue;
  const draftConfig = draftQueue ? commercialQueueConfig[draftQueue] : null;
  const stateFilter = config?.states.some((value) => value === applied.filter) ? applied.filter : "";
  const secondaryFilter = config?.secondary.some((value) => value === applied.secondary) ? applied.secondary : "";
  const key = `${queue}|${stateFilter}|${secondaryFilter}|${generation}|${cursor}`;
  const load = useCallback(async () => {
    if (!queue || !generation) return;
    abort.current?.abort();
    const controller = new AbortController(); abort.current = controller;
    const current = ++sequence.current;
    setState({ key, kind: "loading" });
    const query = new URLSearchParams({ limit: "25" });
    if (stateFilter) query.set("state", stateFilter);
    if (secondaryFilter) query.set(queue === "payments" ? "reconciliation_state" : "action", secondaryFilter);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/commercial/${queue}?${query}`, { credentials: "include", cache: "no-store", signal: controller.signal,
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setState({ key, kind: "error", message: "Session changed. Please try again." }); refreshSession(); return; }
      if (response.status === 403) { setState({ key, kind: "error", message: "You don’t have access to these records.", denied: true }); return; }
      const value = response.ok ? parseCommercialPage(await response.json(), queue) : null;
      setState(value ? { key, kind: "ready", page: value } : { key, kind: "error", message: "Couldn’t load records." });
    } catch (error) {
      if (current === sequence.current && !(error instanceof DOMException && error.name === "AbortError")) {
        setState({ key, kind: "error", message: "Couldn’t load records." });
      }
    }
  }, [queue, generation, key, stateFilter, secondaryFilter, cursor, refreshSession]);
  useEffect(() => { const sequenceRef = sequence; const abortRef = abort; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); sequenceRef.current++; abortRef.current?.abort(); }; }, [load]);
  const visible = state.key === key ? state : { key, kind: "loading" } as LoadState;
  return <>
    {!queue ? <Alert role="alert" variant="destructive"><AlertTitle>Billing access unavailable</AlertTitle><AlertDescription>You do not have a financial read capability in this context.</AlertDescription></Alert> : <>
      <Card className="relative z-10 mb-5 overflow-visible" role="region" aria-label={`${title(group)} filters`}><CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader><CardContent>
      <form className="flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); if (!draftQueue) return; const next = { selected: draftQueue, filter, secondary }; setSelectedRecord(null); if (JSON.stringify(next) === JSON.stringify(applied) && cursor === null) void load(); else { setApplied(next); setCursors([null]); } }}>
        <div className="min-w-44"><SearchableFilterSelect id={`${group}-queue`} label="Type" value={draftQueue ?? ""} options={available.map(value => ({ value, label: title(value) }))} onChange={value => { setSelected(value as CommercialQueue); setFilter(""); setSecondary(""); }} /></div>
        <div className="min-w-44"><SearchableFilterSelect id={`${group}-state`} label="Status" value={filter} options={[{ value: "", label: "All statuses" }, ...(draftConfig?.states ?? []).map(value => ({ value, label: title(value) }))]} onChange={setFilter} /></div>
        {draftConfig?.secondary.length ? <div className="min-w-44"><SearchableFilterSelect id={`${group}-secondary`} label={draftQueue === "payments" ? "Reconciliation" : "Action"} value={secondary} options={[{ value: "", label: "All" }, ...draftConfig.secondary.map(value => ({ value, label: title(value) }))]} onChange={setSecondary} /></div> : null}
        {(filter || secondary || selected !== group || stateFilter || secondaryFilter || queue !== group) && <Button type="button" size="sm" variant="outline" onClick={() => { setSelected(group); setFilter(""); setSecondary(""); setApplied({ selected: group, filter: "", secondary: "" }); setCursors([null]); setSelectedRecord(null); }}>Clear filters</Button>}
        <Button type="submit" size="sm">Apply filters</Button>
      </form></CardContent></Card>
      <section aria-label={`${title(queue)} records`} aria-busy={visible.kind === "loading"} className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Record</th><th scope="col" className="px-4 py-3">Organisation</th><th scope="col" className="px-4 py-3">Amount</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3">Date (UTC)</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y">
        {visible.kind === "ready" && visible.page.items.map(row => <tr key={row.id}><th scope="row" className="px-4 py-4 font-medium">{row.fields.find(field => field.label === "invoice number")?.value ?? (queue === "payments" ? "Payment" : queue === "financial-adjustments" ? "Adjustment" : "Reversal")}</th><td className="px-4 py-4">{row.tenant_name || "Organisation"}</td><td className="whitespace-nowrap px-4 py-4 tabular-nums">{row.amount.currency} {row.amount.amount.replace(/\B(?=(\d{3})+(?!\d))/g, ",")}</td><td className="px-4 py-4"><PlatformStatus value={title(row.state)} /></td><td className="whitespace-nowrap px-4 py-4">{new Date(row.occurred_at).toLocaleString(undefined, { timeZone: "UTC" })}</td><td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={`${title(queue)} ${row.tenant_name || "record"}`} actions={[{ label: "View details", onSelect: () => setSelectedRecord(row) }]} /></div></td></tr>)}
        {(visible.kind !== "ready" || visible.page.items.length === 0) && <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">{visible.kind === "loading" ? <Skeleton role="status" aria-label="Loading records" className="mx-auto h-6 w-48" /> : visible.kind === "error" ? <div role="alert" className="flex flex-wrap items-center justify-center gap-3">{visible.message}{!visible.denied && <Button type="button" size="sm" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button>}</div> : "No data."}</td></tr>}
      </tbody></table></section>
      <nav aria-label={`${title(group)} pages`} className="mt-4 flex items-center gap-3"><Button type="button" variant="outline" disabled={visible.kind === "loading" || cursors.length === 1} onClick={() => { setSelectedRecord(null); setCursors(old => old.slice(0, -1)); }}>Previous</Button><span className="text-sm text-muted-foreground">Page {cursors.length}</span><Button type="button" variant="outline" disabled={visible.kind !== "ready" || !visible.page.next_cursor} onClick={() => { if (visible.kind === "ready" && visible.page.next_cursor) { setSelectedRecord(null); setCursors(old => [...old, visible.page.next_cursor]); } }}>Next</Button></nav>
      <Sheet open={selectedRecord !== null} onOpenChange={open => { if (!open) setSelectedRecord(null); }}><SheetContent className="overflow-y-auto"><SheetHeader><SheetTitle>Record details</SheetTitle><SheetDescription>{selectedRecord?.tenant_name || "Organisation"}</SheetDescription></SheetHeader>{selectedRecord && <div className="space-y-4 px-6 pb-6 text-sm"><div><PlatformStatus value={title(selectedRecord.state)} /><p className="mt-2 font-medium">{selectedRecord.amount.currency} {selectedRecord.amount.amount}</p></div><dl className="space-y-3">{selectedRecord.fields.filter(({ label }) => !label.endsWith(" id")).map(({ label, value }) => <div key={label}><dt className="capitalize text-muted-foreground">{label}</dt><dd className="break-words">{fieldValue(label, value)}</dd></div>)}</dl><p className="text-xs text-muted-foreground">Observed {new Date(selectedRecord.occurred_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</p>
        {selectedRecord.detail_id && <Link className="text-primary underline-offset-2 hover:underline" href={`/platform/billing/${queue}/${selectedRecord.detail_id}`}>Open {queue === "financial-adjustments" ? "adjustment" : "reversal"}</Link>}
        {queue === "invoices" && selectedRecord.state === "PAID" && selectedRecord.version && selectedRecord.settled_payment_version ? <PlatformAdjustmentProposal key={`${generation}|${selectedRecord.id}|${selectedRecord.version}`} invoiceID={selectedRecord.id} invoiceVersion={selectedRecord.version} paymentVersion={selectedRecord.settled_payment_version} onProposed={() => { setSelectedRecord(null); void load(); }} /> : null}
        {queue === "payments" && selectedRecord.state === "SETTLED" && selectedRecord.fields.some(({ label, value }) => label === "reconciliation state" && value === "COMPLETE") && selectedRecord.version ? <PlatformReconciliationAction key={`${generation}|${selectedRecord.id}|${selectedRecord.version}`} paymentID={selectedRecord.id} version={selectedRecord.version} onScheduled={() => { setSelectedRecord(null); void load(); }} /> : null}
      </div>}</SheetContent></Sheet>
    </>}
  </>;
}
