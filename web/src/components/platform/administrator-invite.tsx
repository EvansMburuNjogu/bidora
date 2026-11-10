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

const validName = (value: string) => !!value.trim() && new TextEncoder().encode(value.trim()).length <= 100 && !/[\u0000-\u001f\u007f]/.test(value);

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
  const valid = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) && email.trim().length <= 254 && validName(firstName) && validName(lastName) && !!role;
  if (!resolution.capabilities.includes("platform.admin.invite") || !generation) return null;
  function changed() { pendingKey.current = null; }
  function reset() { setEmail(""); setFirstName(""); setLastName(""); setRole(""); pendingKey.current = null; }
  async function submit() {
    if (submitting.current || !csrfToken || !generation || !valid) return;
    submitting.current = true; setBusy(true);
    const key = pendingKey.current ?? crypto.randomUUID(); pendingKey.current = key;
    try {
      const response = await fetch("/api/platform/administrator-invitations", {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": key },
        body: JSON.stringify({ first_name: firstName.trim(), last_name: lastName.trim(), email: email.trim(), role_key: role }),
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 201) {
        const invitation = parsePlatformInvitationReceipt(await response.json());
        if (!invitation) { toast.error("The outcome is unconfirmed. Retry with the same details.", { id: "administrator-invite" }); return; }
        if (invitation.state !== "ACTIVE") { toast.error("The invitation was not activated. No email was sent.", { id: "administrator-invite" }); return; }
        if (invitation.delivery_state === "ACCEPTED_FOR_DELIVERY") toast.success("Invitation created. Email accepted for delivery.", { id: "administrator-invite" });
        else toast.warning("Invitation created. Email delivery is unconfirmed.", { id: "administrator-invite" });
        setOpen(false); reset(); onCreated?.();
        return;
      }
      toast.error(response.status === 403 ? "You cannot invite users in this session." : response.status === 409 ? "An account or invitation already conflicts with these details." : response.status === 503 ? "Invitation service is unavailable. Retry with the same details." : "Check the names, email and role.", { id: "administrator-invite" });
    } catch { toast.error("The outcome is unconfirmed. Retry with the same details.", { id: "administrator-invite" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  return <>
    <Button type="button" onClick={() => setOpen(true)}><PlusIcon aria-hidden="true" />Invite User</Button>
    <AlertDialog open={open} onOpenChange={next => { if (!submitting.current) { setOpen(next); if (!next) reset(); } }}>
      <AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto">
        <form onSubmit={event => { event.preventDefault(); void submit(); }} className="space-y-4">
          <AlertDialogHeader><AlertDialogTitle>Invite user</AlertDialogTitle><AlertDialogDescription>Choose a Platform Admin role. The invitation is valid for 72 hours.</AlertDialogDescription></AlertDialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="grid gap-1 text-sm">First name<Input required autoComplete="given-name" maxLength={100} value={firstName} disabled={busy} onChange={event => { changed(); setFirstName(event.target.value); }} /></label>
              <label className="grid gap-1 text-sm">Last name<Input required autoComplete="family-name" maxLength={100} value={lastName} disabled={busy} onChange={event => { changed(); setLastName(event.target.value); }} /></label>
            </div>
            <label className="grid gap-1 text-sm">Email address<Input type="email" required maxLength={254} value={email} disabled={busy} onChange={event => { changed(); setEmail(event.target.value); }} /></label>
            <div className="grid gap-1 text-sm"><label htmlFor={roleID}>Role</label><PlatformSelect id={roleID} value={role} disabled={busy} onChange={event => { changed(); setRole(event.target.value as PlatformRole | ""); }}><option value="">Select role</option>{PLATFORM_ROLES.map(key => <option value={key} key={key}>{platformRoleLabels[key]}</option>)}</PlatformSelect></div>
          </div>
          <AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="submit" disabled={busy || !csrfToken || !valid}>{busy ? "Sending…" : "Send invitation"}</Button></AlertDialogFooter>
        </form>
      </AlertDialogContent>
    </AlertDialog>
  </>;
}
