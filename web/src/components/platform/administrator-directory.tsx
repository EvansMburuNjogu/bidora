"use client";

import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { SearchIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { AdministratorInvite } from "@/components/platform/administrator-invite";
import { AdministratorInvitations } from "@/components/platform/administrator-invitations";
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
  const [tab, setTab] = useState<"users" | "invited">("users");
  const [invitationRevision, setInvitationRevision] = useState(0);
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const cursor = cursors[cursors.length - 1];
  const [filterState, setFilterState] = useState<"ACTIVE" | "DISABLED" | "">("");
  const [filterRole, setFilterRole] = useState<PlatformRole | "">("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<{ email: string; state: "ACTIVE" | "DISABLED" | ""; role: PlatformRole | "" }>({ email: "", state: "", role: "" });
  const submittedSearch = filters.email;
  function clearFilters() { setCursors([null]); setSearch(""); setFilterRole(""); setFilterState(""); setFilters({ email: "", state: "", role: "" }); }
  const sequence = useRef(0);
  const generation = resolution.platform_context_generation;

  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setState({ kind: "error", message: "You don’t have access to users.", denied: true }); return; }
    if (submittedSearch && !csrfToken) { setState({ kind: "error", message: "Search requires an active browser session.", denied: true }); return; }
    setState({ kind: "loading" });
    const query = new URLSearchParams();
    if (filters.state) query.set("state", filters.state);
    if (filters.role) query.set("role_key", filters.role);
    if (cursor && !submittedSearch) query.set("cursor", cursor);
    try {
      const response = await fetch(submittedSearch ? "/api/platform/administrators/search" : `/api/platform/administrators${query.size ? `?${query}` : ""}`, {
        method: submittedSearch ? "POST" : "GET", credentials: "include", cache: "no-store",
        headers: submittedSearch ? { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation) } :
          { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        body: submittedSearch ? JSON.stringify({ email: submittedSearch, ...(filters.state ? { state: filters.state } : {}), ...(filters.role ? { role_key: filters.role } : {}) }) : undefined,
      });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refreshSession(); return; }
      if (response.status === 403) { setState({ kind: "error", message: "You don’t have access to users.", denied: true }); return; }
      if (response.status === 400 && submittedSearch) { setState({ kind: "error", message: "Enter a valid full email address to search." }); return; }
      const value = response.ok ? parsePlatformAdministratorList(await response.json()) : null;
      if (current === sequence.current) setState(value ? { kind: "ready", value } : { kind: "error", message: "" });
    } catch { if (current === sequence.current) setState({ kind: "error", message: "" }); }
  }, [generation, filters, submittedSearch, cursor, csrfToken, refreshSession]);

  useEffect(() => {
    const sequenceRef = sequence;
    const timeout = window.setTimeout(() => { if (tab === "users") void load(); }, 0);
    return () => { window.clearTimeout(timeout); sequenceRef.current++; };
  }, [load, tab]);

  return (
    <main className="app-page">
      <div className="app-page-header flex-row items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="type-title">Users</h1>
          <p className="type-lead text-muted-foreground">Manage Platform Admin accounts, responsibilities and access.</p>
        </div>
        <AdministratorInvite key={generation ?? "none"} onCreated={() => { setInvitationRevision(value => value + 1); setTab("invited"); }} />
      </div>

      <Tabs value={tab} onValueChange={value => { if (value === "users" || value === "invited") setTab(value); }}>
        <TabsList aria-label="User directory views">
          <TabsTrigger value="users">Users</TabsTrigger>
          <TabsTrigger value="invited">Invited</TabsTrigger>
        </TabsList>
        <TabsContent value="invited" className="mt-5"><AdministratorInvitations key={invitationRevision} /></TabsContent>
        <TabsContent value="users" className="mt-5">
      <Card className="relative z-10 mb-5 overflow-visible" role="region" aria-label="User filters">
        <CardHeader><CardTitle role="heading" aria-level={2}>Filters</CardTitle></CardHeader>
        <CardContent>
      <form className="flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); const next = { email: search.trim(), state: filterState, role: filterRole }; if (JSON.stringify(next) === JSON.stringify(filters)) void load(); else { setCursors([null]); setFilters(next); } }}>
        <label className="grid min-w-0 gap-1 text-sm">Email address<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" type="email" maxLength={254} value={search} onChange={(event) => setSearch(event.target.value)} placeholder="admin@example.com" /></div></label>
        <div className="min-w-40"><SearchableFilterSelect id="user-state" label="Status" value={filterState} options={[{ value: "", label: "All statuses" }, { value: "ACTIVE", label: "Active" }, { value: "DISABLED", label: "Disabled" }]} onChange={value => setFilterState(value as "ACTIVE" | "DISABLED" | "")} /></div>
        <div className="min-w-48"><SearchableFilterSelect id="user-role" label="Role" value={filterRole} options={[{ value: "", label: "All roles" }, ...PLATFORM_ROLES.map(role => ({ value: role, label: platformRoleLabels[role] }))]} onChange={value => setFilterRole(value as PlatformRole | "")} /></div>
        {search || submittedSearch || filterRole || filterState ? <Button type="button" size="sm" variant="outline" onClick={clearFilters}>Clear filters</Button> : null}
        <Button type="submit" size="sm" disabled={!csrfToken || state.kind === "loading"}><SearchIcon aria-hidden="true" />Apply filters</Button>
      </form>
        </CardContent>
      </Card>
      {state.kind === "error" && state.denied ? <p role="alert" className="text-sm">{state.message}</p> : <>
          <DirectoryRows
            value={state.kind === "ready" ? state.value : { administrators: [], effective_owner_count: 0, continuity_warning: false, next_cursor: null }}
            loading={state.kind === "loading"}
            error={state.kind === "error"}
            message={state.kind === "error" ? state.message : undefined}
            onChanged={() => void load()}
          />
          <nav aria-label="Users pages" className="mt-4 flex items-center gap-3">
            <Button type="button" variant="outline" disabled={state.kind === "loading" || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button>
            <span className="text-sm text-muted-foreground">Page {cursors.length}</span>
            <Button type="button" variant="outline" disabled={state.kind !== "ready" || !state.value.next_cursor} onClick={() => { if (state.kind === "ready" && state.value.next_cursor) setCursors(old => [...old, state.value.next_cursor]); }}>Next</Button>
          </nav>
        </>}
        </TabsContent>
      </Tabs>
    </main>
  );
}

function DirectoryRows({ value, loading, error, message, onChanged }: { value: PlatformAdministratorList; loading: boolean; error: boolean; message?: string; onChanged: () => void }) {
  const { resolution } = useSession();
  const [selected, setSelected] = useState<{ user: PlatformAdministratorList["administrators"][number]; kind: "roles" | "access" | "delete" } | null>(null);
  const can = (capability: string) => resolution.capabilities.includes(capability);
  function changed() { setSelected(null); onChanged(); }
  return <>
    <section aria-label="Platform administrators" aria-busy={loading} data-state={loading ? "loading" : error ? "error" : "ready"} className="overflow-x-auto rounded-lg border bg-card">
      <table className="w-full min-w-[680px] text-left text-sm">
        <thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">User</th><th scope="col" className="px-4 py-3">Roles</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead>
        <tbody className="divide-y">{value.administrators.map(user => {
          const roles = user.assignments.filter(assignment => assignment.state === "ACTIVE");
          const href = `/platform/users/${user.user_id}`;
          const ownerAccount = roles.some(role => role.role_key === "PLATFORM_OWNER");
          const access = can(user.state === "ACTIVE" ? "platform.admin.identity.disable" : "platform.admin.identity.reactivate") && (!ownerAccount || can("platform.governance.approve")) && !(user.effective_owner && value.effective_owner_count <= 1);
          return <tr key={user.user_id}>
            <th scope="row" className="max-w-72 px-4 py-4 font-medium"><Link href={href} className="break-all text-primary hover:underline">{user.email}</Link>{user.effective_owner && <span className="mt-1 block text-xs font-normal text-muted-foreground">Platform owner</span>}</th>
            <td className="max-w-80 px-4 py-4"><div className="flex flex-wrap gap-1.5">{roles.length ? roles.map(role => <span key={role.id} className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">{platformRoleLabels[role.role_key]}</span>) : <span className="text-muted-foreground">No active role</span>}</div></td>
            <td className="px-4 py-4"><PlatformStatus value={user.state === "ACTIVE" ? "Active" : "Disabled"} /></td>
            <td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={user.email} actions={[
              { label: "View details", href },
              ...(can("platform.admin.role.assign") || can("platform.admin.role.revoke") ? [{ label: "Manage roles", onSelect: () => setSelected({ user, kind: "roles" }) }] : []),
              ...(access ? [{ label: user.state === "ACTIVE" ? "Disable user" : "Reactivate user", destructive: user.state === "ACTIVE", onSelect: () => setSelected({ user, kind: "access" }) }] : []),
 ...(user.state==="DISABLED"&&can("platform.admin.identity.disable")&&can("platform.governance.approve")?[{label:"Delete user",destructive:true,onSelect:()=>setSelected({user,kind:"delete"})}]:[]),
            ]} /></div></td>
          </tr>;
        })}
          {loading || error || value.administrators.length === 0 ? <tr><td colSpan={4} className="px-4 py-8 text-center text-sm text-muted-foreground">
            {loading ? <Skeleton role="status" aria-label="Loading users" className="mx-auto h-6 w-48" /> : message ? <span role="alert">{message}</span> : "No data."}
          </td></tr> : null}
        </tbody>
      </table>
    </section>
    <Sheet open={selected?.kind === "roles"} onOpenChange={open => { if (!open) setSelected(null); }}><SheetContent className="overflow-y-auto"><SheetHeader><SheetTitle>Manage roles</SheetTitle><SheetDescription>{selected?.user.email}</SheetDescription></SheetHeader><div className="px-6 pb-6">{selected?.kind === "roles" && <AdministratorRoleChange administrator={selected.user} onChanged={changed} />}</div></SheetContent></Sheet>
    {selected?.kind === "delete" && <AdministratorIdentityState key={"delete:"+selected.user.user_id} administrator={selected.user} deleteMode onChanged={changed} initialOpen compact onClosed={()=>setSelected(null)}/>}
    {selected?.kind === "access" && <AdministratorIdentityState key={selected.user.user_id} administrator={selected.user} onChanged={changed} initialOpen compact onClosed={() => setSelected(null)} />}
  </>;
}
