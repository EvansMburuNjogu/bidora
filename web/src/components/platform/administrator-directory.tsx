"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { RotateCcwIcon, SearchIcon, ShieldAlertIcon, UsersIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AdministratorInvite } from "@/components/platform/administrator-invite";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { Skeleton } from "@/components/ui/skeleton";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from "@/components/ui/sheet";
import { AdministratorRoleChange } from "@/components/platform/administrator-role-change";
import { AdministratorIdentityState } from "@/components/platform/administrator-identity-state";
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
  const [filters, setFilters] = useState<{ email: string; state: "ACTIVE" | "DISABLED" | ""; role: PlatformRole | "" }>({ email: "", state: "", role: "" });
  const submittedSearch = filters.email;
  function clearFilters() { setSearch(""); setFilterRole(""); setFilterState(""); setFilters({ email: "", state: "", role: "" }); }
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
    if (filters.state) query.set("state", filters.state);
    if (filters.role) query.set("role_key", filters.role);
    if (cursor) query.set("cursor", cursor);
    try {
      const response = await fetch(submittedSearch ? "/api/platform/administrators/search" : `/api/platform/administrators${query.size ? `?${query}` : ""}`, {
        method: submittedSearch ? "POST" : "GET", credentials: "include", cache: "no-store",
        headers: submittedSearch ? { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation) } :
          { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        body: submittedSearch ? JSON.stringify({ email: submittedSearch, ...(filters.state ? { state: filters.state } : {}), ...(filters.role ? { role_key: filters.role } : {}) }) : undefined,
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
  }, [generation, filters, submittedSearch, csrfToken, refreshSession]);

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
        <div className="flex flex-wrap gap-2"><AdministratorInvite key={generation ?? "none"} onCreated={() => void load()} /><Button type="button" variant="outline" disabled={state.kind === "loading" || loadingMore} onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Refresh</Button></div>
      </div>

      <form className="mb-5 flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); const next = { email: search.trim(), state: filterState, role: filterRole }; if (JSON.stringify(next) === JSON.stringify(filters)) void load(); else setFilters(next); }}>
        <label className="grid min-w-0 gap-1 text-sm">Email address<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" type="email" maxLength={254} value={search} onChange={(event) => setSearch(event.target.value)} placeholder="admin@example.com" /></div></label>
        <div className="flex flex-col gap-1 text-sm font-medium"><label htmlFor="user-state">Status</label><PlatformSelect id="user-state" value={filterState} onChange={(event) => setFilterState(event.target.value as "ACTIVE" | "DISABLED" | "")}><option value="">All statuses</option><option value="ACTIVE">Active</option><option value="DISABLED">Disabled</option></PlatformSelect></div>
        <div className="flex flex-col gap-1 text-sm font-medium"><label htmlFor="user-role">Role</label><PlatformSelect id="user-role" value={filterRole} onChange={(event) => setFilterRole(event.target.value as PlatformRole | "")}><option value="">All roles</option>{PLATFORM_ROLES.map((role) => <option key={role} value={role}>{platformRoleLabels[role]}</option>)}</PlatformSelect></div>
        {search || submittedSearch || filterRole || filterState ? <Button type="button" size="sm" variant="outline" onClick={clearFilters}>Clear filters</Button> : null}
        <Button type="submit" size="sm" disabled={!csrfToken || state.kind === "loading"}><SearchIcon aria-hidden="true" />Apply filters</Button>
      </form>
      <p className="mb-3 text-xs text-muted-foreground">{submittedSearch ? `Matching email: ${submittedSearch}.` : "All Platform Admin users, including disabled accounts. Invited users appear after accepting."} Email matches a full address; filters apply across all pages.</p>
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
              {search || submittedSearch || filterRole || filterState ? <Button type="button" variant="link" onClick={clearFilters}>Clear filters</Button> : null}
            </div>
          ) : <><p role="status" className="mb-3 text-sm text-muted-foreground">{state.value.administrators.length} users shown{state.value.next_cursor ? " · More available" : ""}</p><DirectoryRows value={state.value} onChanged={() => void load()} /></>}
          {pageError ? <p role="alert" className="text-sm text-destructive">Couldn’t load more users. Try again.</p> : null}
          {state.value.next_cursor ? <Button type="button" variant="outline" disabled={loadingMore} onClick={() => void load(state.value.next_cursor ?? undefined)}>{loadingMore ? "Loading…" : "Load more users"}</Button> : null}
        </>
      ) : null}
    </main>
  );
}

function DirectoryRows({ value, onChanged }: { value: PlatformAdministratorList; onChanged: () => void }) {
  const { resolution } = useSession();
  const [selected, setSelected] = useState<{ user: PlatformAdministratorList["administrators"][number]; kind: "roles" | "access" } | null>(null);
  const can = (capability: string) => resolution.capabilities.includes(capability);
  function changed() { setSelected(null); onChanged(); }
  return <>
    <section aria-label="Platform administrators" className="overflow-x-auto rounded-lg border bg-card">
      <table className="w-full min-w-[680px] text-left text-sm">
        <thead className="border-b bg-muted/40 text-xs text-muted-foreground"><tr><th scope="col" className="px-4 py-3">User</th><th scope="col" className="px-4 py-3">Roles</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead>
        <tbody className="divide-y">{value.administrators.map(user => {
          const roles = user.assignments.filter(assignment => assignment.state === "ACTIVE");
          const href = `/platform/users/${user.user_id}`;
          const access = can(user.state === "ACTIVE" ? "platform.admin.identity.disable" : "platform.admin.identity.reactivate") && !(user.effective_owner && value.effective_owner_count <= 1);
          return <tr key={user.user_id}>
            <th scope="row" className="max-w-72 px-4 py-4 font-medium"><Link href={href} className="break-all text-primary hover:underline">{user.email}</Link>{user.effective_owner && <span className="mt-1 block text-xs font-normal text-muted-foreground">Platform owner</span>}</th>
            <td className="max-w-80 px-4 py-4"><div className="flex flex-wrap gap-1.5">{roles.length ? roles.map(role => <span key={role.id} className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">{platformRoleLabels[role.role_key]}</span>) : <span className="text-muted-foreground">No active role</span>}</div></td>
            <td className="px-4 py-4"><PlatformStatus value={user.state === "ACTIVE" ? "Active" : "Disabled"} /></td>
            <td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={user.email} actions={[
              { label: "View details", href },
              ...(can("platform.admin.role.assign") || can("platform.admin.role.revoke") ? [{ label: "Manage roles", onSelect: () => setSelected({ user, kind: "roles" }) }] : []),
              ...(access ? [{ label: user.state === "ACTIVE" ? "Disable user" : "Reactivate user", destructive: user.state === "ACTIVE", onSelect: () => setSelected({ user, kind: "access" }) }] : []),
              ...(can("platform.admin.session.revoke") ? [{ label: "Revoke sessions", href: href + "#revoke-sessions", destructive: true }] : []),
            ]} /></div></td>
          </tr>;
        })}</tbody>
      </table>
    </section>
    <Sheet open={selected?.kind === "roles"} onOpenChange={open => { if (!open) setSelected(null); }}><SheetContent className="overflow-y-auto"><SheetHeader><SheetTitle>Manage roles</SheetTitle><SheetDescription>{selected?.user.email}</SheetDescription></SheetHeader><div className="px-6 pb-6">{selected?.kind === "roles" && <AdministratorRoleChange administrator={selected.user} onChanged={changed} />}</div></SheetContent></Sheet>
    {selected?.kind === "access" && <AdministratorIdentityState key={selected.user.user_id} administrator={selected.user} onChanged={changed} initialOpen compact onClosed={() => setSelected(null)} />}
  </>;
}

function DirectoryLoading() {
  return <div className="space-y-3" role="status" aria-label="Loading administrators">{[0, 1, 2].map((row) => <Skeleton key={row} className="h-20 w-full rounded-lg" />)}</div>;
}
