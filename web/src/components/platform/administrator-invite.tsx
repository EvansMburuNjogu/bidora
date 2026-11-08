"use client";

import { useId, useRef, useState } from "react";
import { PlusIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { PlatformSelect } from "@/components/platform/platform-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { PLATFORM_ROLES, platformRoleLabels, parsePlatformInvitationReceipt, type PlatformRole } from "@/lib/platform-contract";

export function AdministratorInvite({ onCreated }: { onCreated?: () => void }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const roleID = useId();
  const allowed = resolution.capabilities.includes("platform.admin.invite");
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<PlatformRole | "">("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [created, setCreated] = useState(false);
  const validEmail = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) && email.trim().length <= 254;
  const pendingKey = useRef<string | null>(null);
  const submitting = useRef(false);
  if (!allowed || !generation) return null;
  function changed() { pendingKey.current = null; setFeedback(""); setCreated(false); }
  async function submit() {
    if (submitting.current || !csrfToken || !generation || !validEmail || !role || !reason.trim()) return;
    if (new TextEncoder().encode(reason.trim()).length > 2000) { setFeedback("Enter a reason of at most 2,000 bytes."); return; }
    submitting.current = true; setBusy(true); setFeedback("");
    const key = pendingKey.current ?? crypto.randomUUID(); pendingKey.current = key;
    try {
      const response = await fetch("/api/platform/administrator-invitations", { method: "POST", credentials: "include", cache: "no-store", headers: { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": key }, body: JSON.stringify({ email: email.trim(), role_key: role, reason: reason.trim() }) });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 201) {
        const invitation = parsePlatformInvitationReceipt(await response.json());
        if (!invitation) { setFeedback("The invitation outcome could not be confirmed. Retry with the same details before issuing another invitation."); return; }
        setCreated(true);
        if (invitation.state === "ACTIVE") onCreated?.();
        setFeedback(invitation.state === "PENDING_APPROVAL" ? "This invitation is not active. No email has been sent. Refresh before issuing a new invitation." : invitation.delivery_state === "ACCEPTED_FOR_DELIVERY" ? "Invitation created and accepted for delivery. Recipient delivery is not confirmed." : "Invitation created; delivery is unconfirmed. Check the invitation before retrying.");
        return;
      }
      setFeedback(response.status === 403 ? "You cannot issue this invitation in the current session." : response.status === 409 ? "An invitation or account conflicts with this request. Review the existing record." : response.status === 503 ? "Invitation service or delivery is unavailable. The outcome may be uncertain; retry with the same details." : "Check the email, role and reason before trying again.");
    } catch { setFeedback("The outcome could not be confirmed. Retry with the same details before issuing another invitation."); }
    finally { submitting.current = false; setBusy(false); }
  }
  return <><Button type="button" onClick={() => { setOpen(true); setFeedback(""); }}><PlusIcon aria-hidden="true" />Invite User</Button>
    <AlertDialog open={open} onOpenChange={(next) => { if (busy) return; setOpen(next); if (!next) { setEmail(""); setRole(""); setReason(""); setFeedback(""); setCreated(false); pendingKey.current = null; } }}><AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto"><form onSubmit={event => { event.preventDefault(); void submit(); }} className="space-y-4"><AlertDialogHeader><AlertDialogTitle>Invite user</AlertDialogTitle><AlertDialogDescription>Choose a Platform Admin role. The invitation expires after 72 hours. The user appears in this list after accepting; tenant access is not granted.</AlertDialogDescription></AlertDialogHeader>
      <div className="grid gap-3"><label className="grid gap-1 text-sm">Email address<Input type="email" required maxLength={254} value={email} disabled={busy || created} onChange={(event) => { changed(); setEmail(event.target.value); }} /></label><div className="grid gap-1 text-sm"><label htmlFor={roleID}>Role</label><PlatformSelect id={roleID} value={role} disabled={busy || created} onChange={(event) => { changed(); setRole(event.target.value as PlatformRole | ""); }}><option value="">Select role</option>{PLATFORM_ROLES.map((key) => <option value={key} key={key}>{platformRoleLabels[key]}</option>)}</PlatformSelect></div><label className="grid gap-1 text-sm">Reason<Input required maxLength={2000} value={reason} disabled={busy || created} onChange={(event) => { changed(); setReason(event.target.value); }} /></label>{feedback && <p role="status" className="text-sm text-muted-foreground">{feedback}</p>}</div>
      <AlertDialogFooter><AlertDialogCancel disabled={busy}>{created ? "Close" : "Cancel"}</AlertDialogCancel>{!created && <Button type="submit" disabled={busy || !csrfToken || !validEmail || !role || !reason.trim()}>{busy ? "Submitting…" : "Issue invitation"}</Button>}</AlertDialogFooter></form></AlertDialogContent></AlertDialog>
  </>;
}
