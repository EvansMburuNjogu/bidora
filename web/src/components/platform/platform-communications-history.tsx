"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/session/session-provider";

type Attempt = { category: string; attempt_number: number; disposition: string | null; failure_code: string | null; started_at: string; finished_at: string | null };
type Page = { data: Attempt[]; meta: { trace_id: string; next_cursor: string | null } };
type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: Page };
const categoryPattern = /^[A-Z][A-Z0-9_]{0,79}$/;
const failures = ["CONNECTIVITY_FAILED", "AUTHENTICATION_FAILED", "TLS_FAILED", "PROVIDER_REJECTED", "TIMEOUT", "PROCESS_INTERRUPTED", "PROVIDER_OUTCOME_UNKNOWN", "INTERNAL_ERROR"];
const date = (value: string) => /Z$/.test(value) && Number.isFinite(Date.parse(value));
function parsePage(value: unknown): Page | null {
  if (!value || typeof value !== "object") return null;
  const page = value as Partial<Page>;
  if (!Array.isArray(page.data) || page.data.length > 25 || !page.meta || typeof page.meta.trace_id !== "string" ||
    !(page.meta.next_cursor === null || typeof page.meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(page.meta.next_cursor))) return null;
  for (const row of page.data) {
    if (!row || typeof row.category !== "string" || !categoryPattern.test(row.category) || !Number.isInteger(row.attempt_number) || row.attempt_number < 0 || row.attempt_number > 100 || typeof row.started_at !== "string" || !date(row.started_at)) return null;
    if (row.attempt_number === 0 && row.disposition !== null) return null;
    if (row.disposition === null) { if (row.failure_code !== null || row.finished_at !== null) return null; }
    else if (row.disposition === "SMTP_ACCEPTED") { if (row.failure_code !== null || typeof row.finished_at !== "string" || !date(row.finished_at)) return null; }
    else if ((row.disposition === "FAILED" || row.disposition === "AMBIGUOUS") && failures.includes(row.failure_code ?? "")) {
      if (typeof row.finished_at !== "string" || !date(row.finished_at)) return null;
    } else return null;
  }
  return page as Page;
}
export function PlatformCommunicationsHistory() {
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
      const response = await fetch(`/api/platform/communications/attempts?${query}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setView({ kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setView({ kind: "denied" }); return; }
      const page = response.ok ? parsePage(await response.json()) : null;
      if (current === sequence.current) setView(page ? { kind: "ready", page } : { kind: "error" });
    } catch { if (current === sequence.current) setView({ kind: "error" }); }
  }, [generation, cursor, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  return <main className="app-page min-w-0 space-y-5">
    <div className="app-page-header"><h1 className="type-title">Communications</h1><p className="text-sm text-muted-foreground">All system notification records, including sign-in codes, recovery, verification and invitations. SMTP acceptance is provider handoff, not confirmed inbox delivery. Message contents and secret codes are never shown.</p></div>
    {view.kind === "loading" ? <Skeleton className="h-36 w-full" /> : null}
    {view.kind === "denied" ? <p role="alert" className="text-sm">Only authorised Platform Administrators with recent verification can view notification records.</p> : null}
    {view.kind === "error" ? <div role="alert" className="flex items-center gap-3 text-sm">History could not be loaded. <Button type="button" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : null}
    {view.kind === "ready" ? <>
      <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[760px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Category</th><th scope="col" className="px-4 py-3">Attempt</th><th scope="col" className="px-4 py-3">Recorded outcome</th><th scope="col" className="px-4 py-3">Safe failure code</th><th scope="col" className="px-4 py-3">Started (UTC)</th><th scope="col" className="px-4 py-3">Finished (UTC)</th></tr></thead><tbody>{view.page.data.map((row, index) => <tr key={`${row.started_at}-${index}`} className="border-t"><td className="px-4 py-3">{row.category.replaceAll("_", " ")}</td><td className="px-4 py-3">{row.attempt_number || "—"}</td><td className="px-4 py-3">{row.attempt_number === 0 ? "Not attempted" : row.disposition === "SMTP_ACCEPTED" ? "SMTP accepted (not delivered)" : row.disposition?.replaceAll("_", " ") ?? "No recorded result"}</td><td className="px-4 py-3">{row.failure_code?.replaceAll("_", " ") ?? "—"}</td><td className="whitespace-nowrap px-4 py-3">{new Date(row.started_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</td><td className="whitespace-nowrap px-4 py-3">{row.finished_at ? `${new Date(row.finished_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : "—"}</td></tr>)}</tbody></table>{!view.page.data.length ? <p className="border-t p-4 text-sm text-muted-foreground">No notification records on this page.</p> : null}</div>
      <nav aria-label="Notification attempt pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={cursors.length === 1} onClick={() => setCursors((old) => old.slice(0, -1))}>Previous</Button><Button type="button" variant="outline" disabled={!view.page.meta.next_cursor} onClick={() => { if (view.page.meta.next_cursor) setCursors((old) => [...old, view.page.meta.next_cursor]); }}>Next</Button><span className="text-xs text-muted-foreground">Page {cursors.length}; no delivery total available.</span></nav>
    </> : null}
  </main>;
}
