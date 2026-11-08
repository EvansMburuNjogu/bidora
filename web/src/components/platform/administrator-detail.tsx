"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { ArrowLeftIcon,Loader2Icon, RotateCcwIcon, ShieldOffIcon,ShieldCheckIcon,MailIcon,UserRoundIcon } from "lucide-react";
import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors } from "@/lib/submission-errors";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Card,CardHeader,CardTitle,CardDescription,CardContent } from "@/components/ui/card";
import { PlatformStatus } from "@/components/platform/platform-status";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAdministratorDetail, platformRoleLabels, type PlatformAdministrator } from "@/lib/platform-contract";
import { AdministratorAssignmentHistory } from "@/components/platform/administrator-assignment-history";
import { AdministratorRoleChange } from "@/components/platform/administrator-role-change";
import { AdministratorIdentityState } from "@/components/platform/administrator-identity-state";

type State = { kind: "loading" } | { kind: "not_found" } | { kind: "error"; message: string; denied?: boolean } | { kind: "ready"; value: PlatformAdministrator };

export function AdministratorDetail({ userId }: { userId: string }) {
  const { resolution, csrfToken, refresh: refreshSession } = useSession();
  const generation = resolution.platform_context_generation;
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<State>({ kind: "loading" });
  const [revoking, setRevoking] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const submitting = useRef(false);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const retry = useRef<{ fingerprint: string; key: string } | null>(null);

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
        if (response.status === 403) return setState({ kind: "error", message: "You don’t have access to this user. You may need to verify your account.", denied: true });
        const value = response.ok ? parsePlatformAdministratorDetail(await response.json()) : null;
        if (controller.signal.aborted) return;
        setState(value && value.user_id.toLowerCase() === userId.toLowerCase()
          ? { kind: "ready", value }
          : { kind: "error", message: "Data unavailable: user details." });
      } catch {
        if (!controller.signal.aborted) setState({ kind: "error", message: "Data unavailable: user details." });
      }
    }
    void load();
    return () => controller.abort();
  }, [generation, refreshSession, revision, userId]);

  async function revokeSessions(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || revoking || state.kind !== "ready" || state.value.is_current_user || !generation || !resolution.capabilities.includes("platform.admin.session.revoke")) return;
    const reason = String(new FormData(event.currentTarget).get("reason") ?? "");
    if (reason !== reason.trim() || new TextEncoder().encode(reason).byteLength < 1 || new TextEncoder().encode(reason).byteLength > 2000) {
      setReasonError("Enter a reason of at most 2,000 bytes without surrounding spaces."); return;
    }
    const target = state.value;
    const fingerprint = `${target.user_id}/${target.user_version}/${target.platform_context_generation}/${reason}`;
    if (retry.current?.fingerprint !== fingerprint) retry.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setReasonError(null); setRevoking(true);
    try {
      const response = await fetch(`/api/platform/administrators/${encodeURIComponent(userId)}/sessions/revoke`, {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": retry.current.key },
        body: JSON.stringify({ expected_user_version: target.user_version, expected_target_context_generation: target.platform_context_generation, reason }),
      });
      if (response.status === 401) { refreshSession(); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["reason"] as const); if (fields.reason) setReasonError(fields.reason); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "administrator-detail" }); return; }
      if (response.status === 204) {
        retry.current = null;
        setDialogOpen(false);
        setRevision((value) => value + 1);
        toast.success("Administrator sessions revoked.", { id: "administrator-detail" });
      } else toast.error("The outcome is unconfirmed. Refresh before retrying.", { id: "administrator-detail" });
    } catch { toast.error(networkSubmissionError, { id: "administrator-detail" }); }
    finally { submitting.current = false; setRevoking(false); }
  }

  const profile=state.kind==="ready"?state.value:null;
  const name=profile?[profile.first_name,profile.last_name].filter(Boolean).join(" "):"";
  const activeRoles=profile?.assignments.filter(assignment=>assignment.state==="ACTIVE")??[];
  return <main className="app-page">
    <div className="app-page-header gap-4">
      <Link href="/platform/users" className="inline-flex w-fit items-center gap-2 text-sm text-muted-foreground hover:text-foreground"><ArrowLeftIcon aria-hidden="true" className="size-4"/>Back to Users</Link>
      <div className="flex flex-wrap items-center gap-4">
        <div className="flex size-14 shrink-0 items-center justify-center rounded-xl bg-muted text-primary"><UserRoundIcon aria-hidden="true" className="size-7"/></div>
        <div className="min-w-0 flex-1"><h1 className="type-title break-words">{name||"User details"}</h1><p className="mt-1 break-all text-sm text-muted-foreground">{profile?.email??"Account details and access"}</p></div>
        {profile&&<div className="flex flex-wrap gap-2"><PlatformStatus value={profile.state}/>{profile.is_current_user&&<span className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">Your account</span>}</div>}
      </div>
    </div>
    {state.kind==="loading"&&<div role="status" aria-label="Loading administrator details" className="grid gap-4 lg:grid-cols-3"><Skeleton className="h-64 rounded-xl lg:col-span-2"/><Skeleton className="h-64 rounded-xl"/></div>}
    {state.kind==="not_found"&&<Card><CardContent><p role="status">User not found.</p></CardContent></Card>}
    {state.kind==="error"&&<div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>{state.message}</span>{!state.denied&&<Button variant="outline" size="sm" onClick={()=>setRevision(n=>n+1)}><RotateCcwIcon aria-hidden="true"/>Retry</Button>}</div>}
    {profile&&<div className="space-y-6">
      <div className="grid items-start gap-6 lg:grid-cols-3">
        <div className="min-w-0 space-y-6 lg:col-span-2">
          <Card role="region" aria-label="Administrator account">
            <CardHeader><CardTitle role="heading" aria-level={2}>Account overview</CardTitle><CardDescription>Identity and current Platform Admin responsibilities.</CardDescription></CardHeader>
            <CardContent className="space-y-6">
              <dl className="grid gap-5 sm:grid-cols-2">
                <div><dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Full name</dt><dd className="mt-1 break-words font-medium">{name||"Not provided"}</dd></div>
                <div><dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Email address</dt><dd className="mt-1 flex items-start gap-2 break-all"><MailIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-muted-foreground"/>{profile.email}</dd></div>
                <div><dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Sign-in protection</dt><dd className="mt-1 flex items-center gap-2"><ShieldCheckIcon aria-hidden="true" className="size-4 text-muted-foreground"/>{profile.totp_enrolled?"Authenticator enrolled":"Email-code verification"}</dd></div>
                <div><dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">User ID</dt><dd className="mt-1 break-all font-mono text-xs text-muted-foreground">{profile.user_id}</dd></div>
              </dl>
              <div className="space-y-3 border-t pt-5"><h2 className="text-sm font-medium">Assigned roles</h2><div className="flex flex-wrap gap-2">{activeRoles.length?activeRoles.map(role=><span key={role.id} className="rounded-md bg-muted px-2 py-1 text-xs font-medium">{platformRoleLabels[role.role_key]}</span>):<p className="text-sm text-muted-foreground">No active roles assigned.</p>}</div>{profile.effective_owner&&<p className="text-xs text-muted-foreground">This account counts toward Platform Owner continuity.</p>}</div>
            </CardContent>
          </Card>
          {!profile.is_current_user&&profile.state!=="REMOVED"&&resolution.capabilities.some(cap=>cap==="platform.admin.role.assign"||cap==="platform.admin.role.revoke")&&<Card><CardContent><AdministratorRoleChange administrator={profile} onChanged={()=>setRevision(n=>n+1)}/></CardContent></Card>}
        </div>
        <Card>
          <CardHeader><CardTitle role="heading" aria-level={2}>Access & security</CardTitle><CardDescription>Manage sign-in access without losing account history.</CardDescription></CardHeader>
          <CardContent className="space-y-6">
            {profile.is_current_user?<div className="space-y-4"><p className="text-sm text-muted-foreground">You cannot change your own roles, disable yourself or revoke your sessions here.</p><Link href="/account/security" className="inline-flex text-sm font-medium text-primary underline-offset-4 hover:underline">Account security</Link></div>:<>
              <AdministratorIdentityState administrator={profile} onChanged={()=>setRevision(n=>n+1)}/>
              {profile.state!=="REMOVED"&&resolution.capabilities.includes("platform.admin.session.revoke")&&<div className="space-y-3 border-t pt-5"><div><h2 className="text-sm font-medium">Active sessions</h2><p className="mt-1 text-sm text-muted-foreground">Sign this user out on every device. Their account remains available.</p></div>
                <AlertDialog open={dialogOpen} onOpenChange={value=>{if(!submitting.current)setDialogOpen(value)}}>
                  <AlertDialogTrigger render={<Button id="revoke-sessions" type="button" variant="outline"/>}><ShieldOffIcon aria-hidden="true"/>Revoke sessions</AlertDialogTrigger>
                  <AlertDialogContent><form onSubmit={event=>void revokeSessions(event)} className="space-y-4">
                    <AlertDialogHeader><AlertDialogTitle>Revoke administrator sessions?</AlertDialogTitle><AlertDialogDescription>This signs the administrator out of all sessions. Their account and roles remain unchanged.</AlertDialogDescription></AlertDialogHeader>
                    <div className="space-y-2"><Label htmlFor="revocation-reason">Reason</Label><Input aria-invalid={!!reasonError} aria-describedby={reasonError?"administrator-detail-reason-error":undefined} onChange={()=>setReasonError(null)} id="revocation-reason" name="reason" required minLength={1} maxLength={2000} disabled={revoking}/>{reasonError&&<p role="alert" id="administrator-detail-reason-error" className="text-sm text-destructive">{reasonError}</p>}</div>
                    <AlertDialogFooter><AlertDialogCancel disabled={revoking}>Cancel</AlertDialogCancel><Button type="submit" variant="destructive" disabled={revoking} aria-busy={revoking}>{revoking&&<Loader2Icon aria-hidden="true" className="animate-spin"/>}{revoking?"Revoking…":"Revoke sessions"}</Button></AlertDialogFooter>
                  </form></AlertDialogContent>
                </AlertDialog>
              </div>}
              {profile.state==="REMOVED"&&<p className="text-sm text-muted-foreground">This user has been removed. Their audit history is retained.</p>}
            </>}
          </CardContent>
        </Card>
      </div>
      <Card><CardContent><AdministratorAssignmentHistory key={revision} userId={userId}/></CardContent></Card>
    </div>}
  </main>;
}
