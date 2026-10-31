"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Check, RotateCcwIcon, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { SettingsHeader } from "@/components/platform/settings-draft-controls";
import { PlatformRoleAssignmentPanel } from "@/components/platform/platform-role-assignment-panel";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformAdministratorList, platformRoleLabels, type PlatformAdministrator, type PlatformRole } from "@/lib/platform-contract";
import { parsePlatformRoleCatalogue, type PlatformRoleCatalogue } from "@/lib/platform-roles-contract";
import { fallbackRoles, granted, permissionMatrix, roleDescriptions, shortRoleLabels } from "@/lib/platform-role-matrix";

type RoleLoad = { status: "loading" | "ready" | "fallback" | "denied"; generation: number | null; roles: PlatformRoleCatalogue["roles"] };
type DirectoryLoad = { status: "loading" | "ready" | "error" | "denied"; generation: number | null; users: PlatformAdministrator[]; nextCursor: string | null; loaded: boolean };
const emptyDirectory: DirectoryLoad = { status: "loading", generation: null, users: [], nextCursor: null, loaded: false };

export function PlatformRoleCatalogue() {
  const { resolution } = useSession();
  const allowed = resolution.session.assurance_level === "AAL2" && resolution.platform_context_generation !== null && resolution.capabilities.includes("platform.admin.read");
  const generation = resolution.platform_context_generation;
  const [retryRoles, setRetryRoles] = useState(0);
  const [roles, setRoles] = useState<RoleLoad>({ status: "loading", generation: null, roles: fallbackRoles });
  const [directoryState, setDirectory] = useState<DirectoryLoad>(emptyDirectory);
  const directoryEpoch = useRef(0);
  const pendingPage = useRef<string | null>(null);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<PlatformRole | null>(null);
  const [firstSection, setFirstSection] = useState<"permissions" | "users">("permissions");

  useEffect(() => {
    if (!allowed || generation === null) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch("/api/platform/roles", { cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) }, signal: controller.signal });
        if (response.status === 403 || response.status === 401) { if (!controller.signal.aborted) setRoles({ status: "denied", generation, roles: [] }); return; }
        const parsed = response.ok ? parsePlatformRoleCatalogue(await response.json()) : null;
        if (!controller.signal.aborted) setRoles(parsed ? { status: "ready", generation, roles: parsed.roles } : { status: "fallback", generation, roles: fallbackRoles });
      } catch { if (!controller.signal.aborted) setRoles({ status: "fallback", generation, roles: fallbackRoles }); }
    })();
    return () => controller.abort();
  }, [allowed, generation, retryRoles]);

  const fetchPage = useCallback(async (cursor: string | null, epoch: number) => {
    if (!allowed || generation === null) return;
    const requestKey = `${generation}:${cursor ?? "first"}`;
    if (pendingPage.current === requestKey) return;
    pendingPage.current = requestKey;
    setDirectory((previous) => ({ ...(previous.generation === generation ? previous : emptyDirectory), generation, status: "loading" }));
    try {
      const url = new URL("/api/platform/administrators", window.location.origin);
      if (cursor) url.searchParams.set("cursor", cursor);
      const response = await fetch(url, { cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (epoch !== directoryEpoch.current) return;
      if (response.status === 403 || response.status === 401) { setDirectory({ ...emptyDirectory, generation, status: "denied" }); return; }
      const parsed = response.ok ? parsePlatformAdministratorList(await response.json()) : null;
      if (!parsed) { setDirectory((previous) => ({ ...previous, status: "error" })); return; }
      setDirectory((previous) => ({ status: "ready", generation, users: cursor && previous.generation === generation ? [...previous.users, ...parsed.administrators] : parsed.administrators, nextCursor: parsed.next_cursor, loaded: true }));
    } catch { if (epoch === directoryEpoch.current) setDirectory((previous) => ({ ...previous, status: "error" })); }
    finally { if (pendingPage.current === requestKey) pendingPage.current = null; }
  }, [allowed, generation]);

  useEffect(() => {
    const epoch = ++directoryEpoch.current;
    if (allowed && generation !== null) void Promise.resolve().then(() => { if (directoryEpoch.current === epoch) return fetchPage(null, epoch); });
    const epochRef = directoryEpoch;
    return () => { epochRef.current++; };
  }, [allowed, generation, fetchPage]);

  // Keep the source-defined catalogue visible during service outages, but never treat it as access authority.
  const accessible = allowed && !(roles.generation === generation && roles.status === "denied");
  const visibleRoles = accessible ? (roles.generation === generation ? roles.roles : fallbackRoles) : [];
  const directory = directoryState.generation === generation ? directoryState : emptyDirectory;
  const query = search.trim().toLowerCase();
  const filtered = visibleRoles.filter((role) => !query || `${platformRoleLabels[role.role_key]} ${roleDescriptions[role.role_key]}`.toLowerCase().includes(query));
  const completeDirectory = directory.status === "ready" && directory.loaded && directory.nextCursor === null;
  const count = (role: PlatformRole) => completeDirectory ? directory.users.filter((user) => user.assignments.some((assignment) => assignment.role_key === role && assignment.state === "ACTIVE")).length : null;
  const activeRole = visibleRoles.find((role) => role.role_key === selected);
  const assigned = activeRole ? directory.users.filter((user) => user.assignments.some((assignment) => assignment.role_key === activeRole.role_key && assignment.state === "ACTIVE")) : [];
  const openRole = (role: PlatformRole, section: "permissions" | "users") => { setFirstSection(section); setSelected(role); };

  const permissions = activeRole && <section aria-labelledby="role-capabilities-heading">
    <h3 id="role-capabilities-heading" className="mb-2 font-medium">Permissions</h3>
    <ul className="space-y-1.5 text-xs text-muted-foreground">{permissionMatrix.flatMap(group => group.rows).filter(row => granted(activeRole.capabilities, row.keys)).map((row,index) => <li key={`${row.label}-${index}`} className="py-1">{row.label}</li>)}</ul>
  </section>;
  const users = activeRole && <section aria-labelledby="role-users-heading">
    <div className="mb-2 flex items-center justify-between gap-2"><h3 id="role-users-heading" className="font-medium">Assigned administrators</h3>
      <Link href="/platform/users" className="text-xs text-primary underline-offset-4 hover:underline">User Management</Link></div>
    {directory.status === "denied" ? <p className="text-xs text-muted-foreground">Administrator assignments are unavailable for this session.</p> : <>
      {assigned.length > 0 ? <ul className="divide-y text-sm">{assigned.map((user) => <li key={user.user_id} className="py-2">{user.email}</li>)}</ul> : <p className="text-xs text-muted-foreground">{completeDirectory ? "No administrators assigned." : "No assignments in loaded results."}</p>}
      {!completeDirectory && <p className="mt-2 text-xs text-muted-foreground">{directory.status === "loading" ? "Loading administrators…" : "Assignments and totals may be incomplete."}</p>}
      {directory.status === "error" && <Button type="button" variant="outline" size="sm" className="mt-2" onClick={() => void fetchPage(directory.loaded ? directory.nextCursor : null, directoryEpoch.current)}><RotateCcwIcon aria-hidden="true" />Retry assignments</Button>}
      {directory.status === "ready" && directory.nextCursor && <Button type="button" variant="outline" size="sm" className="mt-2" onClick={() => void fetchPage(directory.nextCursor, directoryEpoch.current)}>Load more administrators</Button>}
    </>}
  </section>;

  return <main className="app-page"><SettingsHeader title="Roles & Permissions" description="Manage administrator access."/><div className="space-y-6">
    <section aria-labelledby="administrator-roles-heading">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <div><h2 id="administrator-roles-heading" className="text-lg font-semibold">Administrator Roles</h2><p className="text-sm text-muted-foreground">Choose a role to view permissions and administrators.</p></div>
        <div className="relative w-full sm:w-64"><Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" /><Input aria-label="Search administrator roles" placeholder="Search roles" value={search} onChange={(event) => setSearch(event.target.value)} className="pl-9" /></div>
      </div>
      {accessible && roles.generation === generation && roles.status === "fallback" && <p className="mb-2 text-xs text-muted-foreground">Showing fixed role definitions while live roles are unavailable. <button type="button" className="text-primary underline-offset-4 hover:underline" onClick={() => setRetryRoles((value) => value + 1)}>Retry roles</button></p>}
      <div className="overflow-x-auto rounded-lg border"><table className="w-full min-w-[720px] text-left text-sm"><thead className="bg-muted/40 text-xs text-muted-foreground"><tr><th scope="col" className="px-4 py-3 font-medium">Role</th><th scope="col" className="px-4 py-3 font-medium">Description</th><th scope="col" className="px-4 py-3 font-medium">Assigned Users</th><th scope="col" className="px-4 py-3 font-medium">Actions</th></tr></thead><tbody className="divide-y">
        {filtered.map((role) => <tr key={role.role_key}><th scope="row" className="px-4 py-3 font-medium">{platformRoleLabels[role.role_key]}</th><td className="px-4 py-3 text-muted-foreground">{roleDescriptions[role.role_key]}</td><td className="px-4 py-3">{count(role.role_key) ?? "—"}</td><td className="whitespace-nowrap px-4 py-3"><PlatformRowActions label={platformRoleLabels[role.role_key]} actions={[{ label: "View permissions", onSelect: () => openRole(role.role_key, "permissions") }, { label: "View assigned users", onSelect: () => openRole(role.role_key, "users") }]} /></td></tr>)}
        {filtered.length === 0 && <tr><td colSpan={4} className="px-4 py-5 text-center text-sm text-muted-foreground">{accessible ? "No matching roles." : "Role details require administrator read access and verified assurance."}</td></tr>}
      </tbody></table></div>
      {accessible && directory.status === "error" && <p className="mt-2 text-xs text-muted-foreground">Assigned users unavailable. <button type="button" className="text-primary hover:underline" onClick={() => void fetchPage(directory.loaded ? directory.nextCursor : null, directoryEpoch.current)}>Retry</button></p>}
      {accessible && directory.status === "ready" && directory.nextCursor && <p className="mt-2 text-xs text-muted-foreground">Counts are shown only after all administrators load. <button type="button" className="text-primary hover:underline" onClick={() => void fetchPage(directory.nextCursor, directoryEpoch.current)}>Load more</button></p>}
    </section>
    <section aria-labelledby="permissions-matrix-heading">
      <h2 id="permissions-matrix-heading" className="text-lg font-semibold">Permissions Matrix</h2><p className="mb-4 text-sm text-muted-foreground">Access included with each role.</p>
      <div className="overflow-x-auto rounded-lg border"><table className="w-full min-w-[880px] text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-3 py-3 text-left font-medium">Permission</th>{visibleRoles.map((role) => <th scope="col" key={role.role_key} title={platformRoleLabels[role.role_key]} className="px-2 py-3 text-center text-xs font-medium">{shortRoleLabels[role.role_key]}</th>)}</tr></thead><tbody className="divide-y">
        {accessible ? permissionMatrix.map((group) => <FragmentGroup key={group.name} group={group} roles={visibleRoles} />) : <tr><td colSpan={7} className="px-4 py-5 text-center text-muted-foreground">Permission details unavailable for this session.</td></tr>}
      </tbody></table></div>
    </section>
    {accessible && <PlatformRoleAssignmentPanel key={generation} onChanged={() => { pendingPage.current = null; const epoch = ++directoryEpoch.current; void fetchPage(null, epoch); }} />}
    <Sheet open={activeRole !== undefined} onOpenChange={(open) => { if (!open) setSelected(null); }}><SheetContent className="w-full sm:max-w-xl"><SheetHeader><SheetTitle>{activeRole ? platformRoleLabels[activeRole.role_key] : "Role details"}</SheetTitle><SheetDescription>{activeRole ? roleDescriptions[activeRole.role_key] : "Fixed platform role"}.</SheetDescription></SheetHeader><div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 pb-6">{firstSection === "permissions" ? <>{permissions}{users}</> : <>{users}{permissions}</>}</div></SheetContent></Sheet>
  </div></main>;
}

function FragmentGroup({ group, roles }: { group: (typeof permissionMatrix)[number]; roles: PlatformRoleCatalogue["roles"] }) {
  return <><tr className="bg-muted/20"><th scope="rowgroup" colSpan={7} className="px-3 py-2 text-left text-xs font-semibold">{group.name}</th></tr>{group.rows.map((row) => <tr key={row.label}><th scope="row" className="px-3 py-2 text-left font-normal">{row.label}{row.keys.length === 0 && <span className="ml-1 text-xs text-muted-foreground">(unavailable)</span>}</th>{roles.map((role) => <td key={role.role_key} className="px-2 py-2 text-center">{granted(role.capabilities, row.keys) ? <Check aria-label="Granted" className="mx-auto size-4 text-primary" /> : <span aria-label="Not granted" className="text-muted-foreground">—</span>}</td>)}</tr>)}</>;
}
