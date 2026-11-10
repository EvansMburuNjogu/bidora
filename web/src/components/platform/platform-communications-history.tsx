"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { isAcceptedEmail } from "@/lib/auth-validation";
import { useSession } from "@/components/session/session-provider";

type Attempt = { recipient: string | null; subject: string | null; message: string | null; category: string; attempt_number: number; disposition: string | null; failure_code: string | null; started_at: string; finished_at: string | null };
type Page = { data: Attempt[]; meta: { trace_id: string; next_cursor: string | null } };
type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: Page };
type Filters = { recipient: string; category: string; status: string; from: string; to: string };
const emptyFilters: Filters = { recipient: "", category: "", status: "", from: "", to: "" };
const categories = ["LOGIN_CODE", "PASSWORD_RECOVERY", "EMAIL_VERIFICATION", "TENANT_INVITATION", "PLATFORM_INVITATION", "PLATFORM_SESSION_REVOCATION", "PLATFORM_IDENTITY_STATE_CHANGE", "PLATFORM_ROLE_CHANGE", "PLATFORM_TEMPLATE_TEST"];
const outcomes = { NOT_ATTEMPTED: "Not attempted", PENDING: "Awaiting result", SMTP_ACCEPTED: "SMTP accepted", FAILED: "Failed", AMBIGUOUS: "Outcome unknown" };
const categoryPattern = /^[A-Z][A-Z0-9_]{0,79}$/;
const failures = ["CONNECTIVITY_FAILED", "AUTHENTICATION_FAILED", "TLS_FAILED", "PROVIDER_REJECTED", "TIMEOUT", "PROCESS_INTERRUPTED", "PROVIDER_OUTCOME_UNKNOWN", "INTERNAL_ERROR"];
const date = (value: string) => /Z$/.test(value) && Number.isFinite(Date.parse(value));
const nullableText = (value: unknown, maximum: number) => value === null || typeof value === "string" && value.length <= maximum;
function parsePage(value: unknown): Page | null {
  if (!value || typeof value !== "object") return null;
  const page = value as Partial<Page>;
  if (!Array.isArray(page.data) || page.data.length > 25 || !page.meta || typeof page.meta.trace_id !== "string" || !(page.meta.next_cursor === null || typeof page.meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(page.meta.next_cursor))) return null;
  for (const row of page.data) {
    if (!row || !nullableText(row.recipient, 254) || !nullableText(row.subject, 600) || !nullableText(row.message, 6100) || typeof row.category !== "string" || !categoryPattern.test(row.category) || !Number.isInteger(row.attempt_number) || row.attempt_number < 0 || row.attempt_number > 100 || typeof row.started_at !== "string" || !date(row.started_at)) return null;
    if (row.attempt_number === 0 && row.disposition !== null) return null;
    if (row.disposition === null) { if (row.failure_code !== null || row.finished_at !== null) return null; }
    else if (row.disposition === "SMTP_ACCEPTED") { if (row.failure_code !== null || typeof row.finished_at !== "string" || !date(row.finished_at)) return null; }
    else if ((row.disposition === "FAILED" || row.disposition === "AMBIGUOUS") && failures.includes(row.failure_code ?? "")) { if (typeof row.finished_at !== "string" || !date(row.finished_at)) return null; }
    else return null;
  }
  return page as Page;
}
function outcome(row: Attempt) { return row.attempt_number === 0 ? outcomes.NOT_ATTEMPTED : row.disposition === null ? outcomes.PENDING : outcomes[row.disposition as keyof typeof outcomes] ?? row.disposition; }
export function PlatformCommunicationsHistory() {
  const { resolution, refresh, csrfToken } = useSession();
  const generation = resolution.platform_context_generation;
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [applied, setApplied] = useState<Filters>(emptyFilters);
  const [filterError, setFilterError] = useState("");
  const [view, setView] = useState<View>({ kind: "loading" });
  const sequence = useRef(0);
  const cursor = cursors[cursors.length - 1];
  const filterQuery = new URLSearchParams(Object.entries(applied).filter(([key, value]) => key !== "recipient" && value !== "")).toString();
  const recipient = applied.recipient;
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (generation === null) { setView({ kind: "denied" }); return; }
    setView({ kind: "loading" });
    try {
      const query = new URLSearchParams(filterQuery); query.set("limit", "25"); if (cursor) query.set("cursor", cursor);
      const response = await fetch(`/api/platform/communications/attempts?${query}`, { method: recipient ? "POST" : "GET", body: recipient ? JSON.stringify({recipient}) : undefined, credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation), ...(recipient ? {"Content-Type":"application/json","X-CSRF-Token":csrfToken} : {}) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { setView({ kind: "denied" }); refresh(); return; }
      if (response.status === 403) { setView({ kind: "denied" }); return; }
      const page = response.ok ? parsePage(await response.json()) : null;
      if (current === sequence.current) setView(page ? { kind: "ready", page } : { kind: "error" });
    } catch { if (current === sequence.current) setView({ kind: "error" }); }
  }, [generation, cursor, refresh, filterQuery, recipient, csrfToken]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  function apply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const recipient = filters.recipient.trim();
    if (recipient && !isAcceptedEmail(recipient)) { setFilterError("Enter a complete recipient email address."); return; }
    if (filters.from && filters.to && filters.from > filters.to) { setFilterError("From date must be on or before To date."); return; }
    setFilterError(""); setCursors([null]); setApplied({ ...filters, recipient });
  }
  function clearFilters() { setFilterError(""); setFilters({ ...emptyFilters }); setApplied({ ...emptyFilters }); setCursors([null]); }
  return <main className="app-page min-w-0 space-y-5">
    <div className="app-page-header"><h1 className="type-title">Communications</h1><p className="text-sm text-muted-foreground">Recipients, message previews and recorded send outcomes. Secret codes and links are redacted. SMTP acceptance does not confirm inbox delivery.</p></div>
    <form onSubmit={apply} aria-label="Communication filters" className="relative z-10 space-y-3 rounded-lg border bg-card p-4">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div className="grid gap-2"><Label htmlFor="communication-recipient">Recipient email</Label><Input id="communication-recipient" type="email" maxLength={254} autoComplete="off" placeholder="Exact email address" value={filters.recipient} onChange={event => setFilters(old => ({...old, recipient:event.target.value}))} /></div>
        <SearchableFilterSelect id="communication-category" label="Category" value={filters.category} options={[{ value: "", label: "All categories" }, ...categories.map(value => ({ value, label: value.replaceAll("_", " ") }))]} onChange={category => setFilters(old => ({ ...old, category }))} />
        <SearchableFilterSelect id="communication-status" label="Outcome" value={filters.status} options={[{ value: "", label: "All outcomes" }, ...Object.entries(outcomes).map(([value, label]) => ({ value, label }))]} onChange={status => setFilters(old => ({ ...old, status }))} />
        <div className="grid gap-2"><Label htmlFor="communication-from">From date (UTC)</Label><Input id="communication-from" type="date" value={filters.from} onChange={event => setFilters(old => ({ ...old, from: event.target.value }))} /></div>
        <div className="grid gap-2"><Label htmlFor="communication-to">To date (UTC)</Label><Input id="communication-to" type="date" value={filters.to} onChange={event => setFilters(old => ({ ...old, to: event.target.value }))} /></div>
      </div>
      {filterError ? <p role="alert" className="text-sm text-destructive">{filterError}</p> : null}
      <div className="flex flex-wrap gap-2"><Button type="submit">Apply filters</Button><Button type="button" variant="outline" onClick={clearFilters}>Clear filters</Button><Button type="button" variant="outline" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Refresh</Button></div>
    </form>
    {view.kind === "loading" ? <Skeleton className="h-36 w-full" /> : null}
    {view.kind === "denied" ? <p role="alert" className="text-sm">You do not have permission to view notification records.</p> : null}
    {view.kind === "error" ? <div role="alert" className="flex items-center gap-3 text-sm">History could not be loaded. <Button type="button" variant="outline" onClick={() => void load()}>Retry</Button></div> : null}
    {view.kind === "ready" ? <>
      <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[900px] text-left text-sm"><thead className="bg-muted/40"><tr>{["Recipient", "Message", "Category", "Send outcome", "Started (UTC)", "Finished (UTC)"].map(label => <th key={label} scope="col" className="px-4 py-3">{label}</th>)}</tr></thead><tbody>{view.page.data.map((row, index) => <tr key={`${row.started_at}-${index}`} className="border-t align-top">
        <td className="max-w-64 break-all px-4 py-3">{row.recipient ?? <span className="text-muted-foreground">Not recorded or no longer retained</span>}</td>
        <td className="min-w-64 max-w-md px-4 py-3">{row.subject ? <p className="break-words font-medium">{row.subject}</p> : null}{row.message ? <details className="mt-1"><summary className="cursor-pointer text-primary">View message</summary><p className="mt-2 whitespace-pre-wrap break-words text-muted-foreground">{row.message}</p></details> : <p className="text-muted-foreground">Message details were not recorded or are no longer retained.</p>}</td>
        <td className="px-4 py-3">{row.category.replaceAll("_", " ")}</td><td className="px-4 py-3"><p>{outcome(row)}</p><p className="mt-1 text-xs text-muted-foreground">{row.attempt_number ? `Attempt ${row.attempt_number}` : "No send attempt"}</p>{row.failure_code ? <p className="mt-1 text-xs text-muted-foreground">{row.failure_code.replaceAll("_", " ")}</p> : null}</td>
        <td className="whitespace-nowrap px-4 py-3">{new Date(row.started_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC</td><td className="whitespace-nowrap px-4 py-3">{row.finished_at ? `${new Date(row.finished_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : "—"}</td>
      </tr>)}</tbody></table>{!view.page.data.length ? <p className="border-t p-4 text-sm text-muted-foreground">No communications match these filters.</p> : null}</div>
      <nav aria-label="Communication pages" className="flex items-center gap-3"><Button type="button" variant="outline" disabled={cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button><Button type="button" variant="outline" disabled={!view.page.meta.next_cursor} onClick={() => { if (view.page.meta.next_cursor) setCursors(old => [...old, view.page.meta.next_cursor]); }}>Next</Button><span className="text-xs text-muted-foreground">Page {cursors.length}</span></nav>
    </> : null}
  </main>;
}
