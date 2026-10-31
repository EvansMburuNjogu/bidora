"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors } from "@/lib/submission-errors";
import { useSession } from "@/components/session/session-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Loader2Icon, CheckIcon, RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAdministratorChange, parsePlatformChangePage, platformRoleLabels, type PlatformAdministratorChange, type PlatformChangePage } from "@/lib/platform-contract";

type State = { kind: "loading" } | { kind: "error" } | { kind: "ready"; value: PlatformChangePage };
const actionLabels = { ASSIGN_ROLE: "Assign role", REVOKE_ROLE: "Revoke role", DISABLE_IDENTITY: "Disable administrator", REACTIVATE_IDENTITY: "Reactivate administrator", ACTIVATE_OWNER_INVITATION: "Administrator invitation" } as const;

export function AdministratorApprovals() {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const canDecide = resolution.capabilities.includes("platform.governance.approve") || resolution.capabilities.includes("platform.governance.review");
  const [state, setState] = useState<State>({ kind: "loading" });
  const [busy, setBusy] = useState<string | null>(null);
  const [pageError, setPageError] = useState(false);
  const submitting = useRef(false);
  const [reasonError, setReasonError] = useState<Record<string, string>>({});
  const retry = useRef<{ fingerprint: string; key: string } | null>(null);

  const load = useCallback(async (cursor?: string) => {
    if (!generation) { setState({ kind: "error" }); return; }
    if (!cursor) { setState({ kind: "loading" }); setPageError(false); }
    const query = `?state=PENDING_APPROVAL${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
    try {
      const response = await fetch(`/api/platform/administrator-changes${query}`, {
        credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 403) { setState({ kind: "error" }); return; }
      const value = response.ok ? parsePlatformChangePage(await response.json()) : null;
      if (!value) { if (cursor) setPageError(true); else setState({ kind: "error" }); return; }
      if (!cursor) setState({ kind: "ready", value });
      else {
        setState((previous) => {
          if (previous.kind !== "ready" || previous.value.next_cursor !== cursor) return previous;
          const ids = new Set(previous.value.changes.map((change) => change.id));
          return { kind: "ready", value: { changes: [...previous.value.changes, ...value.changes.filter((change) => !ids.has(change.id))], next_cursor: value.next_cursor } };
        });
        setPageError(false);
      }
    } catch { if (cursor) setPageError(true); else setState({ kind: "error" }); }
  }, [generation, refresh]);

  useEffect(() => { const timeout = window.setTimeout(() => void load(), 0); return () => window.clearTimeout(timeout); }, [load]);

  async function decide(event: FormEvent<HTMLFormElement>, change: PlatformAdministratorChange) {
    event.preventDefault();
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const decision = submitter?.value;
    const reason = String(new FormData(event.currentTarget).get("reason") ?? "");
    if (submitting.current || busy || !canDecide || !generation || change.state !== "PENDING_APPROVAL" || change.action === "ACTIVATE_OWNER_INVITATION" ||
        (decision !== "approve" && decision !== "reject")) return;
    if (reason !== reason.trim() || new TextEncoder().encode(reason).byteLength < 1 || new TextEncoder().encode(reason).byteLength > 2000) {
      setReasonError({ [change.id]: "Enter a reason of at most 2,000 bytes without surrounding spaces." }); return;
    }
    const fingerprint = `${change.id}/${change.version}/${decision}/${reason}`;
    if (retry.current?.fingerprint !== fingerprint) retry.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setReasonError({}); setBusy(change.id);
    try {
      const response = await fetch(`/api/platform/administrator-changes/${change.id}/${decision}`, {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": retry.current.key },
        body: JSON.stringify({ expected_change_version: change.version, reason }),
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 403) { toast.error("You cannot decide this change. Check your access and recent verification.", { id: "administrator-approvals" }); return; }
      if (response.status === 409) { toast.error(submissionErrorMessage(409), { id: "administrator-approvals" }); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["reason"] as const); if (fields.reason) setReasonError({ [change.id]: fields.reason }); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "administrator-approvals" }); return; }
      const result = response.status === 200 ? parsePlatformAdministratorChange(await response.json()) : null;
      if (!result || result.id !== change.id || result.state !== (decision === "approve" ? "EXECUTED" : "REJECTED")) {
        toast.error("The outcome is not confirmed. Refresh the queue before retrying.", { id: "administrator-approvals" }); return;
      }
      retry.current = null;
      toast.success(decision === "approve" ? "Change approved and applied." : "Change rejected.", { id: "administrator-approvals" });
      await load();
    } catch { toast.error(networkSubmissionError, { id: "administrator-approvals" }); }
    finally { submitting.current = false; setBusy(null); }
  }

  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Approvals</h1><p className="type-lead text-muted-foreground">Review pending administrator access changes. A separate eligible reviewer must decide protected changes.</p></div>
    {state.kind === "loading" ? <div role="status" aria-label="Loading approvals"><Skeleton className="h-24 w-full" /></div> : null}
    {state.kind === "error" ? <Alert variant="destructive" role="alert"><AlertTitle>Approvals unavailable</AlertTitle><AlertDescription>Access or pending work could not be confirmed. <Button type="button" variant="outline" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Try again</Button></AlertDescription></Alert> : null}
    {state.kind === "ready" ? <section className="space-y-3" aria-label="Pending administrator changes">
      {state.value.changes.length === 0 ? <p className="text-sm text-muted-foreground">No pending administrator changes.</p> : <ul className="space-y-3">{state.value.changes.map((change) => <li key={change.id} className="space-y-3 rounded-lg border bg-card p-4">
        <div><h2 className="font-medium">{actionLabels[change.action]}{change.role_key ? ` · ${platformRoleLabels[change.role_key]}` : ""}</h2><p className="text-sm text-muted-foreground">Requested {new Date(change.created_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC{change.expires_at ? ` · Expires ${new Date(change.expires_at).toLocaleString(undefined, { timeZone: "UTC" })} UTC` : ""}</p></div>
        <p className="wrap-break-word text-sm">Reason: {change.reason}</p>
        {change.target_user_id ? <Link href={`/platform/users/${change.target_user_id}`} className="text-sm text-primary underline">View administrator</Link> : null}
        {change.action === "ACTIVATE_OWNER_INVITATION" ? <p className="text-sm text-muted-foreground">Invitation decisions are unavailable until delivery is ready.</p> : canDecide ? <form onSubmit={(event) => void decide(event, change)} className="flex flex-wrap items-end gap-2">
          <div className="grid min-w-48 flex-1 gap-1"><Label htmlFor={`reason-${change.id}`}>Decision reason</Label><Input aria-invalid={!!reasonError[change.id]} aria-describedby={reasonError[change.id] ? `approval-reason-error-${change.id}` : undefined} onChange={() => setReasonError({})} id={`reason-${change.id}`} name="reason" required minLength={1} maxLength={2000} disabled={busy !== null} />{reasonError[change.id] && <p role="alert" id={`approval-reason-error-${change.id}`} className="text-sm text-destructive">{reasonError[change.id]}</p>}</div>
          <Button type="submit" name="decision" value="approve" disabled={busy !== null}>{busy === change.id ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <CheckIcon aria-hidden="true" />}Approve</Button><Button type="submit" name="decision" value="reject" variant="outline" disabled={busy !== null} aria-busy={busy === change.id}>{busy === change.id && <Loader2Icon aria-hidden="true" className="animate-spin" />}Reject</Button>
        </form> : null}
      </li>)}</ul>}
      {pageError ? <p role="alert" className="text-sm text-destructive">More approvals could not be loaded. Try again.</p> : null}
      {state.value.next_cursor ? <Button type="button" variant="outline" disabled={busy !== null} onClick={() => void load(state.value.next_cursor ?? undefined)}>Load more</Button> : null}
    </section> : null}
  </main>;
}
