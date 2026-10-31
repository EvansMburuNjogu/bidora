"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { Loader2Icon, RotateCcwIcon, ShieldOffIcon } from "lucide-react";
import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors } from "@/lib/submission-errors";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
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
          : { kind: "error", message: "Couldn’t load user details." });
      } catch {
        if (!controller.signal.aborted) setState({ kind: "error", message: "Couldn’t load user details." });
      }
    }
    void load();
    return () => controller.abort();
  }, [generation, refreshSession, revision, userId]);

  async function revokeSessions(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || revoking || state.kind !== "ready" || !generation || !resolution.capabilities.includes("platform.admin.session.revoke")) return;
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

  return <main className="app-page">
    <div className="app-page-header flex-row items-start justify-between gap-4">
      <div className="min-w-0"><Link href="/platform/users" className="text-sm text-primary underline-offset-2 hover:underline">← Users</Link><h1 className="type-title mt-2">Administrator details</h1><p className="type-lead text-muted-foreground">Current account status and assigned responsibilities.</p></div>
    </div>
    {state.kind === "loading" ? <div role="status" aria-label="Loading administrator details" className="space-y-3"><Skeleton className="h-16 w-full rounded-lg" /><Skeleton className="h-28 w-full rounded-lg" /></div> : null}
    {state.kind === "not_found" ? <p role="status" className="rounded-lg border p-4 text-sm">User not found.</p> : null}
    {state.kind === "error" ? <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border p-4 text-sm"><span>{state.message}</span>{!state.denied ? <Button type="button" variant="outline" size="sm" onClick={() => setRevision((value) => value + 1)}><RotateCcwIcon aria-hidden="true" />Retry</Button> : null}</div> : null}
    {state.kind === "ready" ? <><section aria-label="Administrator account" className="space-y-5 rounded-lg border bg-card p-4 sm:p-6">
      <div><h2 className="break-all font-medium">{state.value.email}</h2><p className="text-sm text-muted-foreground">{state.value.state === "ACTIVE" ? "Active account" : "Disabled account"} · {state.value.totp_enrolled ? "Two-step enabled" : "Two-step not enabled"}</p></div>
      <div><h3 className="text-sm font-medium">Current responsibilities</h3>{state.value.assignments.some((assignment) => assignment.state === "ACTIVE") ? <ul className="mt-2 list-inside list-disc text-sm">{state.value.assignments.filter((assignment) => assignment.state === "ACTIVE").map((assignment) => <li key={assignment.role_key}>{platformRoleLabels[assignment.role_key]}</li>)}</ul> : <p className="mt-2 text-sm text-muted-foreground">No active roles assigned.</p>}</div>
      {state.value.effective_owner ? <p className="text-sm text-muted-foreground">Counts toward Platform Owner continuity.</p> : null}
      {resolution.capabilities.includes("platform.admin.session.revoke") ? <AlertDialog open={dialogOpen} onOpenChange={(value) => { if (!submitting.current) setDialogOpen(value); }}>
        <AlertDialogTrigger render={<Button type="button" variant="destructive" />}><ShieldOffIcon aria-hidden="true" />Revoke sessions</AlertDialogTrigger>
        <AlertDialogContent>
          <form onSubmit={(event) => void revokeSessions(event)} className="space-y-4">
            <AlertDialogHeader><AlertDialogTitle>Revoke administrator sessions?</AlertDialogTitle><AlertDialogDescription>This signs the administrator out of all sessions. Their account and role assignments remain unchanged. You cannot revoke your own sessions here.</AlertDialogDescription></AlertDialogHeader>
            <div className="space-y-2"><Label htmlFor="revocation-reason">Reason</Label><Input aria-invalid={!!reasonError} aria-describedby={reasonError ? "administrator-detail-reason-error" : undefined} onChange={() => setReasonError(null)} id="revocation-reason" name="reason" required minLength={1} maxLength={2000} disabled={revoking} />{reasonError && <p role="alert" id="administrator-detail-reason-error" className="text-sm text-destructive">{reasonError}</p>}</div>
            <AlertDialogFooter><AlertDialogCancel disabled={revoking}>Cancel</AlertDialogCancel><Button type="submit" variant="destructive" disabled={revoking} aria-busy={revoking}>{revoking ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <ShieldOffIcon aria-hidden="true" />}{revoking ? "Revoking…" : "Revoke sessions"}</Button></AlertDialogFooter>
          </form>
        </AlertDialogContent>
      </AlertDialog> : null}
    </section><AdministratorRoleChange administrator={state.value} onChanged={() => setRevision((value) => value + 1)} /><AdministratorIdentityState administrator={state.value} onChanged={() => setRevision((value) => value + 1)} /><AdministratorAssignmentHistory key={revision} userId={userId} /></> : null}
  </main>;
}
