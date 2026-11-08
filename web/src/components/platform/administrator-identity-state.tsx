"use client";

import { useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors } from "@/lib/submission-errors";
import { useSession } from "@/components/session/session-provider";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Loader2Icon, BanIcon, RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { parsePlatformAdministratorChange, type PlatformAdministrator } from "@/lib/platform-contract";

export function AdministratorIdentityState({ administrator, onChanged, initialOpen = false, compact = false, onClosed }: { administrator: PlatformAdministrator; onChanged: () => void; initialOpen?: boolean; compact?: boolean; onClosed?: () => void }) {
  const { resolution, csrfToken, refresh } = useSession();
  const disable = administrator.state === "ACTIVE";
  const action = disable ? "DISABLE_IDENTITY" : "REACTIVATE_IDENTITY";
  const ownerAccount = administrator.assignments.some(assignment => assignment.state === "ACTIVE" && assignment.role_key === "PLATFORM_OWNER");
  const allowed = resolution.capabilities.includes(disable ? "platform.admin.identity.disable" : "platform.admin.identity.reactivate") && (!ownerAccount || resolution.capabilities.includes("platform.governance.approve"));
  const [open, setOpen] = useState(initialOpen);
  const [busy, setBusy] = useState(false);
  const submitting = useRef(false);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const retry = useRef<{ fingerprint: string; key: string } | null>(null);
  if (!allowed) return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || busy || !resolution.platform_context_generation) return;
    const reason = String(new FormData(event.currentTarget).get("reason") ?? "");
    if (reason !== reason.trim() || new TextEncoder().encode(reason).byteLength < 1 || new TextEncoder().encode(reason).byteLength > 2000) {
      setReasonError("Enter a reason of at most 2,000 bytes without surrounding spaces."); return;
    }
    const body = { action, target_user_id: administrator.user_id, expected_user_version: administrator.user_version, expected_target_context_generation: administrator.platform_context_generation, reason };
    const fingerprint = JSON.stringify(body);
    if (retry.current?.fingerprint !== fingerprint) retry.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setReasonError(null); setBusy(true);
    try {
      const response = await fetch("/api/platform/administrator-changes", {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(resolution.platform_context_generation), "Idempotency-Key": retry.current.key },
        body: fingerprint,
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 409) { toast.error(submissionErrorMessage(409), { id: "administrator-identity-state" }); return; }
      if (response.status === 403) { toast.error(submissionErrorMessage(403), { id: "administrator-identity-state" }); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["reason"] as const); if (fields.reason) setReasonError(fields.reason); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "administrator-identity-state" }); return; }
      const change = response.status === 201 ? parsePlatformAdministratorChange(await response.json()) : null;
      if (!change || change.target_user_id?.toLowerCase() !== administrator.user_id.toLowerCase() || change.action !== action || change.state !== "EXECUTED") {
        toast.error("No outcome is confirmed. Check the current record before retrying.", { id: "administrator-identity-state" }); return;
      }
      retry.current = null;
      setOpen(false);
      if (change.state === "EXECUTED") {
        toast.success(disable ? "Administrator disabled and sessions revoked." : "Administrator reactivated. Existing sessions remain invalid.", { id: "administrator-identity-state" });
        onChanged();
      }
    } catch { toast.error(networkSubmissionError, { id: "administrator-identity-state" }); }
    finally { submitting.current = false; setBusy(false); }
  }

  return <section id="account-access" aria-label="Administrator account status" className="space-y-3">
    {!compact && <h2 className="text-base font-medium">Account access</h2>}
    {!compact && <p className="text-sm text-muted-foreground">{disable ? "Disabling an administrator removes sign-in access and revokes their sessions when executed." : "Reactivation permits future sign-in; it does not restore old sessions."} Owner account changes require a recently verified Platform Owner.</p>}
    <AlertDialog open={open} onOpenChange={(value) => { if (!submitting.current) setOpen(value); if (!value && !submitting.current) onClosed?.(); }}>
      {!compact && <AlertDialogTrigger render={<Button type="button" variant={disable ? "destructive" : "outline"} />}>{disable ? "Disable administrator" : "Reactivate administrator"}</AlertDialogTrigger>}
      <AlertDialogContent><form onSubmit={(event) => void submit(event)} className="space-y-4">
        <AlertDialogHeader><AlertDialogTitle>{disable ? "Disable this administrator?" : "Reactivate this administrator?"}</AlertDialogTitle><AlertDialogDescription>Review the selected account and give a reason. You cannot change your own account here. Changes take effect immediately; the last active Owner remains protected.</AlertDialogDescription></AlertDialogHeader>
        <div className="space-y-2"><Label htmlFor="identity-change-reason">Reason</Label><Input aria-invalid={!!reasonError} aria-describedby={reasonError ? "administrator-identity-state-reason-error" : undefined} onChange={() => setReasonError(null)} id="identity-change-reason" name="reason" required maxLength={2000} disabled={busy} />{reasonError && <p role="alert" id="administrator-identity-state-reason-error" className="text-sm text-destructive">{reasonError}</p>}</div>
        <AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="submit" variant={disable ? "destructive" : "default"} disabled={busy} aria-busy={busy}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : disable ? <BanIcon aria-hidden="true" /> : <RotateCcwIcon aria-hidden="true" />}{busy ? "Saving…" : disable ? "Disable administrator" : "Reactivate administrator"}</Button></AlertDialogFooter>
      </form></AlertDialogContent>
    </AlertDialog>
  </section>;
}
