"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors } from "@/lib/submission-errors";
import { useSession } from "@/components/session/session-provider";
import { Loader2Icon, SendIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PLATFORM_ROLES, parsePlatformAdministratorChange, platformRoleLabels, type PlatformAdministrator, type PlatformRole } from "@/lib/platform-contract";

export function AdministratorRoleChange({ administrator, onChanged }: { administrator: PlatformAdministrator; onChanged: () => void }) {
  const { resolution, csrfToken, refresh } = useSession();
  const canAssign = resolution.capabilities.includes("platform.admin.role.assign");
  const canRevoke = resolution.capabilities.includes("platform.admin.role.revoke");
  const [busy, setBusy] = useState(false);
  const [selectedAction, setSelectedAction] = useState("");
  const submitting = useRef(false);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const retry = useRef<{ fingerprint: string; key: string } | null>(null);
  const active = administrator.assignments.filter((assignment) => assignment.state === "ACTIVE");
  const addable = PLATFORM_ROLES.filter((role) => !active.some((assignment) => assignment.role_key === role));
  const availableRoles = selectedAction === "ASSIGN_ROLE" ? addable : selectedAction === "REVOKE_ROLE" ? active.map((assignment) => assignment.role_key) : []; 
  if ((!canAssign && !canRevoke) || administrator.state==="REMOVED") return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || busy || !resolution.platform_context_generation) return;
    const form = new FormData(event.currentTarget);
    const action = form.get("action");
    const role = form.get("role_key");
    const reason = form.get("reason");
    if ((action !== "ASSIGN_ROLE" && action !== "REVOKE_ROLE") || typeof role !== "string" || !PLATFORM_ROLES.includes(role as PlatformRole) || typeof reason !== "string" || reason !== reason.trim() || new TextEncoder().encode(reason).byteLength < 1 || new TextEncoder().encode(reason).byteLength > 2000) {
      setReasonError("Enter a reason of at most 2,000 bytes without surrounding spaces."); return;
    }
    if (action === "ASSIGN_ROLE" && (!canAssign || administrator.state !== "ACTIVE" || !addable.includes(role as PlatformRole))) return;
    const assignment = active.find((entry) => entry.role_key === role);
    if (action === "REVOKE_ROLE" && (!canRevoke || !assignment)) return;
    const body = action === "ASSIGN_ROLE"
      ? { action, target_user_id: administrator.user_id, role_key: role, expected_user_version: administrator.user_version, expected_target_context_generation: administrator.platform_context_generation, reason }
      : { action, target_user_id: administrator.user_id, role_key: role, assignment_id: assignment!.id, expected_assignment_version: assignment!.version, expected_user_version: administrator.user_version, expected_target_context_generation: administrator.platform_context_generation, reason };
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
      if (response.status === 409) { toast.error(submissionErrorMessage(409), { id: "administrator-role-change" }); return; }
      if (response.status === 403) { toast.error(submissionErrorMessage(403), { id: "administrator-role-change" }); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["reason"] as const); if (fields.reason) setReasonError(fields.reason); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "administrator-role-change" }); return; }
      const change = response.status === 201 ? parsePlatformAdministratorChange(await response.json()) : null;
      if (!change || change.target_user_id?.toLowerCase() !== administrator.user_id.toLowerCase() || change.action !== action || change.role_key !== role || change.state !== "EXECUTED") {
        toast.error("No change is confirmed. Refresh before retrying.", { id: "administrator-role-change" }); return;
      }
      retry.current = null;
      if (change.state === "EXECUTED") {
        toast.success("Role change recorded. Current access is being refreshed.", { id: "administrator-role-change" });
        onChanged();
      }
    } catch { toast.error(networkSubmissionError, { id: "administrator-role-change" }); }
    finally { submitting.current = false; setBusy(false); }
  }

  return <section id="roles" aria-label="Manage administrator roles" className="space-y-3">
    <h2 className="text-base font-medium">Manage roles</h2>
    <p className="text-sm text-muted-foreground">Changes apply immediately. The last active Platform Owner cannot be removed.</p>
    <form onSubmit={(event) => void submit(event)} className="grid gap-4 border-t pt-4 sm:max-w-2xl sm:grid-cols-2">
      <div className="grid gap-2"><Label htmlFor="role-action">Action <span aria-hidden="true" className="text-destructive">*</span></Label><PlatformSelect id="role-action" name="action" required disabled={busy} value={selectedAction} onChange={(event) => setSelectedAction(event.target.value)}><option value="" disabled>Choose an action</option>{canAssign && administrator.state === "ACTIVE" && addable.length ? <option value="ASSIGN_ROLE">Assign role</option> : null}{canRevoke && active.length ? <option value="REVOKE_ROLE">Revoke role</option> : null}</PlatformSelect></div>
      <div className="grid gap-2"><Label htmlFor="role-key">Role <span aria-hidden="true" className="text-destructive">*</span></Label><PlatformSelect key={selectedAction} id="role-key" name="role_key" required defaultValue="" disabled={!selectedAction || busy}><option value="" disabled>Choose a role</option>{availableRoles.map((role) => <option key={role} value={role}>{platformRoleLabels[role]}</option>)}</PlatformSelect></div>
      <div className="grid gap-2 sm:col-span-2"><Label htmlFor="role-reason">Reason <span aria-hidden="true" className="text-destructive">*</span></Label><Input aria-invalid={!!reasonError} aria-describedby={reasonError ? "administrator-role-change-reason-error" : undefined} onChange={() => setReasonError(null)} id="role-reason" className="h-11 px-3 sm:h-10" name="reason" disabled={busy} required minLength={1} maxLength={2000} />{reasonError && <p role="alert" id="administrator-role-change-reason-error" className="text-sm text-destructive">{reasonError}</p>}</div>
      <div className="flex justify-end sm:col-span-2"><Button type="submit" disabled={busy || !selectedAction} aria-busy={busy}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <SendIcon aria-hidden="true" />}{busy ? "Submitting…" : "Submit change"}</Button></div>
    </form>
  </section>;
}
