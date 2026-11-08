"use client";

import { useId, useRef, useState } from "react";
import { PlusIcon } from "lucide-react";
import { toast } from "sonner";
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
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [role, setRole] = useState<PlatformRole | "">("");
  const [busy, setBusy] = useState(false);
  const pendingKey = useRef<string | null>(null);
  const submitting = useRef(false);
  const validNames = [firstName, lastName].every(name => !!name.trim() && new TextEncoder().encode(name.trim()).length <= 100);
  const validEmail = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) && email.trim().length <= 254;
  if (!resolution.capabilities.includes("platform.admin.invite") || !generation) return null;

  function changed() { pendingKey.current = null; }
  function reset() { setEmail(""); setFirstName(""); setLastName(""); setRole(""); pendingKey.current = null; }
  async function submit() {
    if (submitting.current || !csrfToken || !generation || !validEmail || !validNames || !role) return;
    submitting.current = true;
    setBusy(true);
    const key = pendingKey.current ?? crypto.randomUUID();
    pendingKey.current = key;
    const id = toast.loading("Sending invitation…");
    try {
      const response = await fetch("/api/platform/administrator-invitations", {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": key },
        // Keep the backend's audited command contract without requiring a form field.
        body: JSON.stringify({ first_name: firstName.trim(), last_name: lastName.trim(), email: email.trim(), role_key: role, reason: "Administrator invitation" }),
      });
      if (response.status === 401) { toast.error("Sign in again to invite this user.", { id }); refresh(); return; }
      if (response.status === 201) {
        const invitation = parsePlatformInvitationReceipt(await response.json());
        if (!invitation) { toast.error("The invitation outcome could not be confirmed. Retry with the same details.", { id }); return; }
        if (invitation.state === "PENDING_APPROVAL") toast.warning("Invitation saved but not active. No email has been sent.", { id });
        else if (invitation.delivery_state === "ACCEPTED_FOR_DELIVERY") toast.success("Invitation created and accepted for delivery.", { id });
        else toast.warning("Invitation created; email delivery is unconfirmed. Check Communications before retrying.", { id });
        setOpen(false);
        reset();
        onCreated?.();
        return;
      }
      toast.error(response.status === 403 ? "You cannot issue this invitation in the current session." : response.status === 409 ? "An invitation or account conflicts with this request. Review the existing record." : response.status === 503 ? "Invitation service or delivery is unavailable. Retry with the same details." : "Check the user details before trying again.", { id });
    } catch { toast.error("The outcome could not be confirmed. Retry with the same details.", { id }); }
    finally { submitting.current = false; setBusy(false); }
  }

  return <><Button type="button" onClick={() => setOpen(true)}><PlusIcon aria-hidden="true" />Invite User</Button>
    <AlertDialog open={open} onOpenChange={next => { if (busy) return; setOpen(next); if (!next) reset(); }}><AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto"><form onSubmit={event => { event.preventDefault(); void submit(); }} className="space-y-4">
      <AlertDialogHeader><AlertDialogTitle>Invite user</AlertDialogTitle><AlertDialogDescription className="sr-only">Enter the user details.</AlertDialogDescription></AlertDialogHeader>
      <div className="grid gap-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="grid gap-1 text-sm">First name<Input required maxLength={100} value={firstName} disabled={busy} autoComplete="given-name" onChange={event => { changed(); setFirstName(event.target.value); }} /></label>
          <label className="grid gap-1 text-sm">Last name<Input required maxLength={100} value={lastName} disabled={busy} autoComplete="family-name" onChange={event => { changed(); setLastName(event.target.value); }} /></label>
        </div>
        <label className="grid gap-1 text-sm">Email address<Input type="email" required maxLength={254} value={email} disabled={busy} onChange={event => { changed(); setEmail(event.target.value); }} /></label>
        <div className="grid gap-1 text-sm"><label htmlFor={roleID}>Role</label><PlatformSelect id={roleID} value={role} disabled={busy} onChange={event => { changed(); setRole(event.target.value as PlatformRole | ""); }}><option value="">Select role</option>{PLATFORM_ROLES.map(key => <option value={key} key={key}>{platformRoleLabels[key]}</option>)}</PlatformSelect></div>
      </div>
      <AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="submit" disabled={busy || !csrfToken || !validEmail || !validNames || !role}>{busy ? "Submitting…" : "Issue invitation"}</Button></AlertDialogFooter>
    </form></AlertDialogContent></AlertDialog>
  </>;
}
