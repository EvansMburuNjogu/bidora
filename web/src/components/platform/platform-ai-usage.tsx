"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";

type Usage = { id: string; tenant_id: string; kind: "DOCUMENT_PDF" | "DOCUMENT_DOCX" | "GENERATION_TEXT"; actual_credits: string; settled_at: string };
type Page = { data: Usage[]; meta: { trace_id: string; next_cursor: string | null } };
type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: Page };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const kinds = ["DOCUMENT_PDF", "DOCUMENT_DOCX", "GENERATION_TEXT"];
function parsePage(value: unknown): Page | null {
  if (!value || typeof value !== "object") return null;
  const body = value as Partial<Page>;
  if (!Array.isArray(body.data) || body.data.length > 25 || !body.meta || typeof body.meta.trace_id !== "string" ||
    !(body.meta.next_cursor === null || typeof body.meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(body.meta.next_cursor))) return null;
  for (const item of body.data) {
    if (!item || !uuid.test(item.id) || !uuid.test(item.tenant_id) || !kinds.includes(item.kind) ||
      typeof item.actual_credits !== "string" || !/^[1-9]\d{0,18}$/.test(item.actual_credits) || BigInt(item.actual_credits) > BigInt("9223372036854775807") || typeof item.settled_at !== "string" ||
      !/Z$/.test(item.settled_at) || !Number.isFinite(Date.parse(item.settled_at))) return null;
  }
  return body as Page;
}
export function PlatformAIUsage() {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [view, setView] = useState<View>({ kind: "loading" });
  const sequence = useRef(0);
  const cursor = cursors[cursors.length - 1];
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (generation === null) { setView({ kind: "denied" }); return; }
    setView({ kind: "loading" });
    try {
      const query = new URLSearchParams({ limit: "25" });
      if (cursor) query.set("cursor", cursor);
      const response = await fetch(`/api/platform/ai-usage?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setView({ kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setView({ kind: "denied" }); return; }
      const page = response.ok ? parsePage(await response.json()) : null;
      if (current === sequence.current) setView(page ? { kind: "ready", page } : { kind: "error" });
    } catch { if (current === sequence.current) setView({ kind: "error" }); }
  }, [generation, cursor, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  return <main className="app-page min-w-0 space-y-5">
    <div className="app-page-header"><h1 className="type-title">AI Usage</h1><p className="text-sm text-muted-foreground">Settled credits recorded by the control plane. Each row is one settled operation, not a bill or a tenant-wide total.</p></div>
    {view.kind === "loading" ? <Skeleton className="h-36 w-full" /> : null}
    {view.kind === "denied" ? <p role="alert" className="text-sm">This view requires a current Platform Billing Administrator session with recent authenticator assurance.</p> : null}
    {view.kind === "error" ? <div role="alert" className="flex items-center gap-3 text-sm">Usage could not be loaded. <Button type="button" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : null}
    {view.kind === "ready" ? <>
      <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Tenant ID</th><th scope="col" className="px-4 py-3">Usage kind</th><th scope="col" className="px-4 py-3">Settled credits</th><th scope="col" className="px-4 py-3">Settled at (UTC)</th></tr></thead><tbody>{view.page.data.map((item) => <tr key={item.id} className="border-t"><td className="break-all px-4 py-3 font-mono text-xs">{item.tenant_id}</td><td className="px-4 py-3">{item.kind.replaceAll("_", " ")}</td><td className="px-4 py-3">{BigInt(item.actual_credits).toLocaleString()}</td><td className="whitespace-nowrap px-4 py-3">{new Date(item.settled_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</td></tr>)}</tbody></table>{!view.page.data.length ? <p className="border-t p-4 text-sm text-muted-foreground">No settled operations on this page.</p> : null}</div>
      <nav aria-label="Usage pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={cursors.length === 1} onClick={() => setCursors((old) => old.slice(0, -1))}>Previous</Button><Button type="button" variant="outline" disabled={!view.page.meta.next_cursor} onClick={() => { if (view.page.meta.next_cursor) setCursors((old) => [...old, view.page.meta.next_cursor]); }}>Next</Button><span className="text-xs text-muted-foreground">Page {cursors.length}; no total available.</span></nav>
    </> : null}
  </main>;
}
