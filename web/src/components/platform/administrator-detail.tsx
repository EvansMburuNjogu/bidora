"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeftIcon, RotateCcwIcon, ShieldCheckIcon, ShieldOffIcon, UserRoundIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { PlatformStatus } from "@/components/platform/platform-status";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAdministratorDetail, platformRoleLabels, type PlatformAdministrator } from "@/lib/platform-contract";
import { AdministratorAssignmentHistory } from "@/components/platform/administrator-assignment-history";
import { AdministratorRoleChange } from "@/components/platform/administrator-role-change";
import { AdministratorIdentityState } from "@/components/platform/administrator-identity-state";

type State = { kind: "loading" } | { kind: "not_found" } | { kind: "error"; message: string; denied?: boolean } | { kind: "ready"; value: PlatformAdministrator };

export function AdministratorDetail({ userId }: { userId: string }) {
  const { resolution, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<State>({ kind: "loading" });
  const canManageRoles = resolution.capabilities.some(capability => ["platform.admin.role.assign", "platform.admin.role.revoke"].includes(capability));
  const canManageAccess = state.kind === "ready" && resolution.capabilities.includes(state.value.state === "ACTIVE" ? "platform.admin.identity.disable" : "platform.admin.identity.reactivate") && (!state.value.assignments.some(assignment => assignment.state === "ACTIVE" && assignment.role_key === "PLATFORM_OWNER") || resolution.capabilities.includes("platform.governance.approve"));

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      if (!generation) {
        setState({ kind: "error", message: "You don’t have access to this user.", denied: true });
        return;
      }
      setState({ kind: "loading" });
      try {
        const response = await fetch(`/api/platform/administrators/${encodeURIComponent(userId)}`, {
          credentials: "include", cache: "no-store", signal: controller.signal,
          headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        });
        if (controller.signal.aborted) return;
        if (response.status === 401 || response.status === 409) return refreshSession();
        if (response.status === 404) return setState({ kind: "not_found" });
        if (response.status === 403) return setState({ kind: "error", message: "You don’t have access to this user.", denied: true });
        const value = response.ok ? parsePlatformAdministratorDetail(await response.json()) : null;
        if (controller.signal.aborted) return;
        setState(value && value.user_id.toLowerCase() === userId.toLowerCase()
          ? { kind: "ready", value }
          : { kind: "error", message: "Couldn’t load user details." });
      } catch {
        if (!controller.signal.aborted) setState({ kind: "error", message: "Couldn’t load user details." });
      }
    }
    void load();
    return () => controller.abort();
  }, [generation, refreshSession, revision, userId]);


  return <main className="app-page">
    <div className="app-page-header flex-row items-start justify-between gap-4">
      <div className="min-w-0"><h1 className="type-title">User details</h1><p className="type-lead text-muted-foreground">Account information, roles and access.</p></div>
      <Button variant="outline" render={<Link href="/platform/users" />}><ArrowLeftIcon aria-hidden="true" />Back to users</Button>
    </div>
    {state.kind === "loading" ? <div role="status" aria-label="Loading administrator details" className="space-y-3"><Skeleton className="h-16 w-full rounded-lg" /><Skeleton className="h-28 w-full rounded-lg" /></div> : null}
    {state.kind === "not_found" ? <p role="status" className="rounded-lg border p-4 text-sm">User not found.</p> : null}
    {state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>{state.message}</span>{!state.denied ? <Button type="button" variant="outline" size="sm" onClick={() => setRevision((value) => value + 1)}><RotateCcwIcon aria-hidden="true" />Retry</Button> : null}</div> : null}
    {state.kind === "ready" ? <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
      <div className="space-y-5">
        <Card role="region" aria-label="Administrator account">
          <CardHeader className="place-items-center gap-3 text-center">
            <div aria-hidden="true" className="flex size-14 items-center justify-center rounded-full bg-muted text-muted-foreground"><UserRoundIcon className="size-7" /></div>
            <div className="space-y-1"><CardTitle role="heading" aria-level={2} className="break-all">{state.value.email}</CardTitle><p className="text-sm text-muted-foreground">Platform administrator</p></div>
            <PlatformStatus value={state.value.state === "ACTIVE" ? "Active" : "Disabled"} />
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="flex items-center gap-2 rounded-lg bg-muted/50 px-3 py-2 text-sm"><ShieldCheckIcon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />Email verification required</div>
            <div className="space-y-2"><h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Assigned roles</h3>
              <div className="flex flex-wrap gap-2">{state.value.assignments.some(assignment => assignment.state === "ACTIVE")
                ? state.value.assignments.filter(assignment => assignment.state === "ACTIVE").map(assignment => <span key={assignment.id} className="rounded-md bg-muted px-2 py-1 text-xs">{platformRoleLabels[assignment.role_key]}</span>)
                : <p className="text-sm text-muted-foreground">No active roles assigned.</p>}</div>
            </div>
          </CardContent>
          {resolution.capabilities.includes("platform.admin.session.revoke") ? <CardFooter><Button type="button" variant="outline" disabled title="Manual session revocation is disabled."><ShieldOffIcon aria-hidden="true" />Revoke sessions</Button></CardFooter> : null}
        </Card>
        {canManageAccess ? <Card><CardContent><AdministratorIdentityState administrator={state.value} onChanged={() => setRevision(value => value + 1)} /></CardContent></Card> : null}
      </div>
      <div className="space-y-5">
        {canManageRoles ? <Card><CardContent><AdministratorRoleChange administrator={state.value} onChanged={() => setRevision(value => value + 1)} /></CardContent></Card> : null}
        <Card><CardContent><AdministratorAssignmentHistory key={revision} userId={userId} /></CardContent></Card>
      </div>
    </div> : null}
  </main>;
}
