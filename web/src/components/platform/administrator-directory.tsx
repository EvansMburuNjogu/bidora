"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { RotateCcwIcon, SearchIcon, ShieldAlertIcon, UsersIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PlatformCreatePreview } from "@/components/platform/platform-create-preview";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAdministratorList, platformRoleLabels, PLATFORM_ROLES, type PlatformAdministratorList, type PlatformRole } from "@/lib/platform-contract";

type State = { kind: "loading" } | { kind: "error"; message: string; denied?: boolean } | { kind: "ready"; value: PlatformAdministratorList };

export function AdministratorDirectory() {
  const { resolution, csrfToken, refresh: refreshSession } = useSession();
  const [state, setState] = useState<State>({ kind: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  const [pageError, setPageError] = useState(false);
  const [filterState, setFilterState] = useState<"ACTIVE" | "DISABLED" | "">("");
  const [filterRole, setFilterRole] = useState<PlatformRole | "">("");
  const [search, setSearch] = useState("");
  const [submittedSearch, setSubmittedSearch] = useState("");
  const sequence = useRef(0);
  const generation = resolution.platform_context_generation;

  const load = useCallback(async (cursor?: string) => {
    const current = ++sequence.current;
    if (!generation) {
      setState({ kind: "error", message: "You don’t have access to users.", denied: true });
      return;
    }
    if (submittedSearch && !csrfToken) { setState({ kind: "error", message: "Search requires an active browser session.", denied: true }); return; }
    if (cursor) setLoadingMore(true);
    else { setState({ kind: "loading" }); setPageError(false); }
    const query = new URLSearchParams();
    if (filterState) query.set("state", filterState);
    if (filterRole) query.set("role_key", filterRole);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(submittedSearch ? "/api/platform/administrators/search" : `/api/platform/administrators${query.size ? `?${query}` : ""}`, {
        method: submittedSearch ? "POST" : "GET", credentials: "include", cache: "no-store",
        headers: submittedSearch ? { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation) } :
          { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        body: submittedSearch ? JSON.stringify({ email: submittedSearch, ...(filterState ? { state: filterState } : {}), ...(filterRole ? { role_key: filterRole } : {}) }) : undefined,
      });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) {
        refreshSession();
        return;
      }
      if (response.status === 403) {
        setState({ kind: "error", message: "You don’t have access to users. You may need to verify your account.", denied: true });
        return;
      }
      if (response.status === 400 && submittedSearch) { setState({ kind: "error", message: "Enter a valid full email address to search." }); return; }
      const value = response.ok ? parsePlatformAdministratorList(await response.json()) : null;
      if (!value) {
        if (cursor) setPageError(true);
        else setState({ kind: "error", message: "Couldn’t load users." });
        return;
      }
      if (cursor) {
        setState((previous) => {
          if (previous.kind !== "ready" || previous.value.next_cursor !== cursor) return previous;
          const ids = new Set(previous.value.administrators.map((administrator) => administrator.user_id));
          return { kind: "ready", value: { ...value, administrators: [...previous.value.administrators, ...value.administrators.filter((administrator) => !ids.has(administrator.user_id))] } };
        });
        setPageError(false);
      } else setState({ kind: "ready", value });
    } catch {
      if (current === sequence.current) { if (cursor) setPageError(true);
      else setState({ kind: "error", message: "Couldn’t load users." }); }
    } finally { if (current === sequence.current) setLoadingMore(false); }
  }, [generation, filterState, filterRole, submittedSearch, csrfToken, refreshSession]);

  useEffect(() => {
    const sequenceRef = sequence;
    const timeout = window.setTimeout(() => void load(), 0);
    return () => { window.clearTimeout(timeout); sequenceRef.current++; };
  }, [load]);

  return (
    <main className="app-page">
      <div className="app-page-header flex-row items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="type-title">Users</h1>
          <p className="type-lead text-muted-foreground">Manage Platform Admin accounts, responsibilities and access.</p>
        </div>
        <div className="flex flex-wrap gap-2"><PlatformCreatePreview kind="user" />{resolution.capabilities.includes("platform.governance.approve") || resolution.capabilities.includes("platform.governance.review") ? <Link href="/platform/users/approvals" className="rounded-lg border bg-card px-3 py-2 text-sm text-primary hover:underline">Review approvals</Link> : null}</div>
      </div>

      <div className="mb-5 flex flex-wrap items-center gap-3">
        <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => { event.preventDefault(); if (search && search === search.trim()) { if (search === submittedSearch) void load(); else setSubmittedSearch(search); } }}><label className="grid gap-1 text-sm">Exact administrator email<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" type="email" required maxLength={254} value={search} onChange={(event) => setSearch(event.target.value)} placeholder="admin@example.com" /></div></label><Button type="submit" size="sm" disabled={!csrfToken || !search || search !== search.trim()}><SearchIcon aria-hidden="true" />Search</Button></form>
        <label htmlFor="user-state" className="flex flex-col gap-1 text-sm font-medium">Status<PlatformSelect id="user-state" value={filterState} onChange={(event) => setFilterState(event.target.value as "ACTIVE" | "DISABLED" | "")}><option value="">All statuses</option><option value="ACTIVE">Active</option><option value="DISABLED">Disabled</option></PlatformSelect></label>
        <label htmlFor="user-role" className="flex flex-col gap-1 text-sm font-medium">Role<PlatformSelect id="user-role" value={filterRole} onChange={(event) => setFilterRole(event.target.value as PlatformRole | "")}><option value="">All roles</option>{PLATFORM_ROLES.map((role) => <option key={role} value={role}>{platformRoleLabels[role]}</option>)}</PlatformSelect></label>
        {search || submittedSearch || filterRole || filterState ? <Button type="button" size="sm" variant="outline" onClick={() => { setSearch(""); setSubmittedSearch(""); setFilterRole(""); setFilterState(""); }}>Clear Filters</Button> : null}
      </div>
      {submittedSearch && <p className="mb-3 text-xs text-muted-foreground">Showing the exact-email lookup, not a page-only or platform-wide search.</p>}
      {state.kind === "loading" ? <DirectoryLoading /> : null}
      {state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3 text-sm"><span>{state.message}</span>{!state.denied ? <Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button> : null}</div> : null}
      {state.kind === "ready" ? (
        <>
          {state.value.continuity_warning ? (
            <Alert variant="warning" role="status">
              <ShieldAlertIcon aria-hidden="true" />
              <AlertTitle>Owner continuity needs attention</AlertTitle>
              <AlertDescription>Platform ownership has limited coverage. Review owner assignments before making access changes.</AlertDescription>
            </Alert>
          ) : null}
          {state.value.administrators.length === 0 ? (
            <div className="rounded-lg border border-dashed p-8 text-center">
              <UsersIcon className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden="true" />
              <h2 className="font-medium">No records found.</h2>
              {search || submittedSearch || filterRole || filterState ? <Button type="button" variant="link" onClick={() => { setSearch(""); setSubmittedSearch(""); setFilterRole(""); setFilterState(""); }}>Clear Filters</Button> : null}
            </div>
          ) : <DirectoryRows value={state.value} />}
          {pageError ? <p role="alert" className="text-sm text-destructive">Couldn’t load more users. Try again.</p> : null}
          {state.value.next_cursor ? <Button type="button" variant="outline" disabled={loadingMore} onClick={() => void load(state.value.next_cursor ?? undefined)}>{loadingMore ? "Loading…" : "Load more administrators"}</Button> : null}
        </>
      ) : null}
    </main>
  );
}

function DirectoryRows({ value }: { value: PlatformAdministratorList }) {
  return (
    <section aria-label="Platform administrators" className="overflow-hidden rounded-lg border bg-card">
      <div className="hidden grid-cols-[minmax(0,1.4fr)_minmax(0,1.5fr)_minmax(7rem,.6fr)_minmax(8rem,.7fr)_minmax(4rem,.3fr)] gap-4 border-b bg-muted/40 px-4 py-2.5 text-xs font-medium text-muted-foreground sm:grid">
        <span>Administrator</span><span>Responsibilities</span><span>Status</span><span>Verification</span><span>Actions</span>
      </div>
      <div className="divide-y">
        {value.administrators.map((administrator) => {
          const roles = administrator.assignments.filter((assignment) => assignment.state === "ACTIVE").map((assignment) => platformRoleLabels[assignment.role_key]);
          return (
            <article key={administrator.user_id} className="grid min-w-0 gap-3 px-4 py-4 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,1.5fr)_minmax(7rem,.6fr)_minmax(8rem,.7fr)_minmax(4rem,.3fr)] sm:items-center sm:gap-4">
              <div className="min-w-0"><Link href={`/platform/users/${administrator.user_id}`} className="break-all text-sm font-medium text-primary underline-offset-2 hover:underline focus-visible:underline">{administrator.email}</Link>{administrator.effective_owner ? <p className="mt-1 text-xs text-muted-foreground">Platform owner</p> : null}</div>
              <div className="flex min-w-0 flex-wrap gap-1.5">{roles.length ? roles.map((label) => <span key={label} className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">{label}</span>) : <span className="text-sm text-muted-foreground">No active assignment</span>}</div>
              <div><PlatformStatus value={administrator.state === "ACTIVE" ? "Active" : "Disabled"} /></div>
              <p className="text-sm text-muted-foreground">Email code required</p>
              <PlatformRowActions label={administrator.email} actions={[{ label: "View details", href: `/platform/users/${administrator.user_id}` }]} />
            </article>
          );
        })}
      </div>
    </section>
  );
}

function DirectoryLoading() {
  return <div className="space-y-3" role="status" aria-label="Loading administrators">{[0, 1, 2].map((row) => <Skeleton key={row} className="h-20 w-full rounded-lg" />)}</div>;
}
