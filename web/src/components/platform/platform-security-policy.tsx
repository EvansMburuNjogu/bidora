"use client";

import { parsePlatformSessionPolicy, validSessionPolicy, type SecurityControls, type PlatformSessionPolicy } from "@/lib/platform-session-policy-contract";
import { PlatformSelect } from "@/components/platform/platform-select";
import Link from "next/link";
import { Loader2Icon, RotateCcwIcon, ShieldOffIcon, SaveIcon } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { SessionInventoryResponse } from "@/contracts/auth.generated";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { useSession } from "@/components/session/session-provider";
import { revokeSession } from "@/lib/session-administration-client";
import { parseSessionInventory } from "@/lib/session-contract";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";

import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage } from "@/lib/submission-errors";

function PolicyField({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return <div className="grid gap-2 sm:grid-cols-[minmax(12rem,0.9fr)_minmax(0,1.1fr)] sm:items-center sm:gap-6"><div><p className="text-sm font-medium">{label}</p>{hint && <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>}</div><div className="min-w-0">{children}</div></div>;
}
export function PlatformSecurityPolicy() {
  const { resolution, csrfToken } = useSession();
  const [inventoryRevision, setInventoryRevision] = useState(0);
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.admin.read") && generation !== null;
  return <main className="app-page">
    <div className="app-page-header mb-7"><h1 className="type-title">Security &amp; Sessions</h1><p className="type-lead text-muted-foreground">Manage login protection and session limits across the platform.</p></div>
    {!allowed ? <p className="text-sm text-muted-foreground">You do not currently have access to this section.</p> : <div className="max-w-4xl">
      <SessionPolicyFields key={`${generation}:${csrfToken}`} onSaved={() => setInventoryRevision(value => value + 1)} />
      <section aria-labelledby="required-protection-heading" className="space-y-2 border-t py-6"><h2 id="required-protection-heading" className="text-base font-semibold">Required Protections</h2><p className="text-sm text-muted-foreground">Verified email and administrator two-step verification are always required. Sensitive actions require recent verification. These protections cannot be disabled.</p></section>
      <ActiveSessions key={`${csrfToken}:${inventoryRevision}`} csrfToken={csrfToken} />
    </div>}
  </main>;
}

function duration(seconds: number) { return seconds >= 86400 ? `${seconds / 86400} ${seconds === 86400 ? "day" : "days"}` : seconds >= 3600 ? `${seconds / 3600} ${seconds === 3600 ? "hour" : "hours"}` : `${seconds / 60} minutes`; }
function clonePolicy(value: PlatformSessionPolicy): PlatformSessionPolicy { return { ...value, security: { ...value.security } }; }
function SessionPolicyFields({ onSaved }: { onSaved: () => void }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [policy, setPolicy] = useState<PlatformSessionPolicy | null>(null);
  const [draft, setDraft] = useState<PlatformSessionPolicy | null>(null);
  const [revision, setRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<"save" | "reload" | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const submitting = useRef(false);
  const mounted = useRef(true);
  const receipt = useRef<{ fingerprint: string; key: string } | null>(null);
  const canSave = resolution.capabilities.includes("platform.governance.approve");
  const dirty = policy !== null && draft !== null && JSON.stringify(policy) !== JSON.stringify(draft);
  const valid = draft !== null && validSessionPolicy(draft);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    if (!generation) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch("/api/platform/security/session-policy", { credentials: "include", cache: "no-store", signal: controller.signal, headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
        const value = response.ok ? parsePlatformSessionPolicy(await response.json()) : null;
        if (controller.signal.aborted) return;
        if (!value) { setMessage(submissionErrorMessage(response.status)); return; }
        setPolicy(value); setDraft(clonePolicy(value)); receipt.current = null; setMessage(null);
      } catch { if (!controller.signal.aborted) setMessage("Security settings could not be loaded. Please retry."); }
      finally { if (!controller.signal.aborted) setLoading(false); }
    })();
    return () => controller.abort();
  }, [generation, revision]);
  function reload() { setDialog(null); setLoading(true); setRevision(value => value + 1); }
  function change<K extends keyof SecurityControls>(key: K, value: SecurityControls[K]) { setDraft(current => current ? { ...current, security: { ...current.security, [key]: value } } : null); }
  async function save() {
    if (!policy || !draft || !valid || !dirty || !canSave || !generation || !csrfToken || busy || submitting.current) return;
    const body = { idle_seconds: draft.idle_seconds, absolute_seconds: draft.absolute_seconds, security: draft.security };
    const fingerprint = JSON.stringify({ body, version: policy.version, generation });
    if (receipt.current?.fingerprint !== fingerprint) receipt.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setBusy(true); setMessage(null);
    try {
      const response = await fetch("/api/platform/security/session-policy", { method: "PUT", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": String(generation), "X-CSRF-Token": csrfToken, "If-Match": `"${policy.version}"`, "Idempotency-Key": receipt.current.key }, body: JSON.stringify(body) });
      if (!mounted.current) return;
      if (response.status === 401) { refresh(); return; }
      const value = response.ok ? parsePlatformSessionPolicy(await response.json()) : null;
      if (!mounted.current) return;
      if (!value) { const error = response.status === 409 ? "Saved settings changed. Reload and review the latest values before saving." : submissionErrorMessage(response.status, response.headers.get("retry-after")); setMessage(error); toast.error(error, { id: "session-policy" }); return; }
      setPolicy(value); setDraft(clonePolicy(value)); setDialog(null); receipt.current = null; toast.success("Security settings saved. Other sessions were signed out.", { id: "session-policy" }); onSaved(); refresh();
    } catch { if (mounted.current) { setMessage(networkSubmissionError); toast.error(networkSubmissionError, { id: "session-policy" }); } }
    finally { submitting.current = false; if (mounted.current) setBusy(false); }
  }
  if (!draft || !policy) return <div className="space-y-3 py-6"><p role="status" className="text-sm text-muted-foreground">{loading ? "Loading security settings…" : message ?? "Security settings are unavailable."}</p>{!loading && <Button variant="outline" onClick={reload}><RotateCcwIcon aria-hidden="true" />Retry security settings</Button>}</div>;
  const disabled = !canSave || busy || loading;
  const number = (label: string, key: Exclude<keyof SecurityControls, "allow_multiple_sessions">, min: number, max: number, unit = 1) => <Input type="number" inputMode="numeric" aria-label={label} value={draft.security[key] / unit} min={min} max={max} step={1} required disabled={disabled} onChange={event => change(key, Number(event.target.value) * unit)} />;
  return <>
    <form onSubmit={event => { event.preventDefault(); if (valid && dirty && canSave) setDialog("save"); }}>
      <section aria-labelledby="login-security-heading" className="space-y-5 py-6"><h2 id="login-security-heading" className="text-base font-semibold">Login Security</h2>
        <PolicyField label="Maximum Failed Password Attempts" hint="Per account, within the window below. Network protection stays enabled.">{number("Maximum failed password attempts", "failed_login_attempts", 1, 5)}</PolicyField>
        <PolicyField label="Failed Attempt Window" hint="Minutes for new failure windows. Existing windows keep their expiry.">{number("Failed attempt window in minutes", "failure_window_seconds", 1, 60, 60)}</PolicyField>
        <PolicyField label="Account Lockout Duration" hint="Minutes for new lockouts. Existing lockouts keep their original expiry.">{number("Account lockout duration in minutes", "lockout_seconds", 1, 60, 60)}</PolicyField>
        <PolicyField label="Minimum Password Length" hint="12–128 UTF-8 bytes. Applies to new and changed passwords, not existing sign-ins.">{number("Minimum password length in bytes", "minimum_password_bytes", 12, 128)}</PolicyField>
      </section>
      <section aria-labelledby="session-management-heading" className="space-y-5 border-t py-6"><h2 id="session-management-heading" className="text-base font-semibold">Session Management</h2>
        <PolicyField label="User Inactivity Timeout"><PlatformSelect aria-label="Ordinary session inactivity timeout" value={draft.idle_seconds} disabled={disabled} onChange={event => setDraft({ ...draft, idle_seconds: Number(event.target.value) })}>{[300, 600, 900, 1800].map(seconds => <option value={seconds} key={seconds}>{duration(seconds)}</option>)}</PlatformSelect></PolicyField>
        <PolicyField label="Maximum User Session Duration"><PlatformSelect aria-label="Ordinary maximum session duration" value={draft.absolute_seconds} disabled={disabled} onChange={event => setDraft({ ...draft, absolute_seconds: Number(event.target.value) })}>{[3600, 14400, 28800, 43200].map(seconds => <option value={seconds} key={seconds}>{duration(seconds)}</option>)}</PlatformSelect></PolicyField>
        <PolicyField label="Administrator Inactivity Timeout" hint="Minutes, from 5 minutes to 7 days.">{number("Administrator inactivity timeout in minutes", "admin_idle_seconds", 5, 10080, 60)}</PolicyField>
        <PolicyField label="Maximum Administrator Session Duration" hint="Minutes, from 1 hour to 7 days.">{number("Administrator maximum session duration in minutes", "admin_absolute_seconds", 60, 10080, 60)}</PolicyField>
        <PolicyField label="Allow Multiple Active Sessions" hint="When off, each new sign-in replaces other sessions for that account."><label className="flex items-center gap-3"><Checkbox aria-label="Allow multiple active sessions" checked={draft.security.allow_multiple_sessions} disabled={disabled} onCheckedChange={checked => change("allow_multiple_sessions", checked === true)} /><span className="text-sm">{draft.security.allow_multiple_sessions ? "Allowed" : "One session per account"}</span></label></PolicyField>
      </section>
      <section aria-labelledby="account-protection-heading" className="space-y-5 border-t py-6"><h2 id="account-protection-heading" className="text-base font-semibold">Account Protection</h2><PolicyField label="Password Reset Expiry" hint="Minutes for new recovery codes. Existing codes keep their expiry.">{number("Password reset expiry in minutes", "password_reset_seconds", 5, 30, 60)}</PolicyField></section>
      {!canSave && <p className="mb-3 text-sm text-muted-foreground">Only the Platform Owner can change security settings.</p>}
      {message && <p role="alert" className="mb-3 text-sm text-destructive">{message}</p>}
      {!valid && <p role="alert" className="mb-3 text-sm text-destructive">Check the allowed ranges. Maximum session durations must not be shorter than inactivity timeouts.</p>}
      <div className="flex flex-wrap justify-end gap-3 pb-6"><Button type="button" variant="ghost" disabled={busy || loading} onClick={() => dirty ? setDialog("reload") : reload()}><RotateCcwIcon aria-hidden="true" />Reload settings</Button><Button type="button" variant="outline" disabled={busy || !dirty} onClick={() => { setDraft(clonePolicy(policy)); setMessage(null); }}>Cancel</Button><Button type="submit" disabled={disabled || !csrfToken || !dirty || !valid}><SaveIcon aria-hidden="true" />Save Changes</Button></div>
    </form>
    <AlertDialog open={dialog !== null} onOpenChange={open => { if (!open && !busy) setDialog(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{dialog === "reload" ? "Reload saved settings?" : "Update security settings?"}</AlertDialogTitle><AlertDialogDescription>{dialog === "reload" ? "Your unsaved changes will be replaced with the latest saved settings." : "Other sessions will be signed out. Your session may also end if it no longer meets the new limits. Password and lockout rules apply to new attempts and requests."}</AlertDialogDescription></AlertDialogHeader>{message && dialog === "save" && <p role="alert" className="text-sm text-destructive">{message}</p>}<AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="button" disabled={busy} aria-busy={busy} onClick={() => dialog === "reload" ? reload() : void save()}>{busy && <Loader2Icon aria-hidden="true" className="animate-spin" />}{dialog === "reload" ? "Reload settings" : busy ? "Saving…" : "Save Changes"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </>;
}

function localTime(value: string): string { return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)); }
async function readSessions(signal?: AbortSignal): Promise<SessionInventoryResponse["data"] | null> {
  const response = await fetch("/api/session/sessions", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal });
  return response.ok ? parseSessionInventory(await response.json())?.data ?? null : null;
}
function ActiveSessions({ csrfToken }: { csrfToken: string }) {
  const [inventory, setInventory] = useState<SessionInventoryResponse["data"] | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const { refresh } = useSession();
  const submitting = useRef(false);
  const keys = useRef(new Map<string, string>());
  const mounted = useRef(true);
  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true); setFailed(false);
    try {
      const data = await readSessions(signal);
      if (signal?.aborted || !mounted.current) return;
      if (!data) { setInventory(null); setFailed(true); return; }
      setInventory(data);
    } catch { if (!signal?.aborted && mounted.current) { setInventory(null); setFailed(true); } }
    finally { if (!signal?.aborted && mounted.current) setLoading(false); }
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    mounted.current = true;
    void Promise.resolve().then(() => { if (!controller.signal.aborted) return load(controller.signal); });
    const requestKeys = keys.current;
    return () => { mounted.current = false; controller.abort(); requestKeys.clear(); };
  }, [load]);
  async function revokeSelected() {
    const item = inventory?.sessions.find((session) => session.session_id === selected);
    if (!item || item.current || !inventory?.other_session_revocation_available || pending || submitting.current || !csrfToken) return;
    const id = item.session_id;
    let key = keys.current.get(id);
    if (!key) { key = crypto.randomUUID(); keys.current.set(id, key); }
    submitting.current = true; setPending(true);
    try {
      const response = await revokeSession(id, csrfToken, key);
      if (!mounted.current) return;
      if (response.status === 204) { keys.current.delete(id); setSelected(null); toast.success("Session revoked.", { id: "active-session-revoke" }); setInventory(null); await load(); }
      else if (response.status === 401) refresh();
      else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "active-session-revoke" });
    } catch { if (mounted.current) toast.error(networkSubmissionError, { id: "active-session-revoke" }); }
    finally { submitting.current = false; if (mounted.current) setPending(false); }
  }
  return <section aria-labelledby="active-sessions-heading" className="border-t py-6"><div className="mb-4 flex flex-wrap items-center justify-between gap-3"><div><h2 id="active-sessions-heading" className="text-base font-semibold">Active Sessions</h2><p className="mt-1 text-xs text-muted-foreground">Manage where your account is signed in. Times use your local timezone.</p></div><Link href="/account/security" className="text-sm text-primary hover:underline">Account security</Link></div>
    <div className="overflow-x-auto rounded-lg border"><table className="w-full min-w-[560px] text-left text-sm"><thead className="bg-muted/40 text-xs text-muted-foreground"><tr><th scope="col" className="px-3 py-3 font-medium">Session</th><th scope="col" className="px-3 py-3 font-medium">Last Active</th><th scope="col" className="px-3 py-3 font-medium">Status</th><th scope="col" className="px-3 py-3 font-medium">Action</th></tr></thead><tbody className="divide-y">
      {inventory?.sessions.map((item) => <tr key={item.session_id}><td className="px-3 py-3">{item.current ? "Current session" : "Other session"}</td><td className="px-3 py-3">{localTime(item.last_seen_at)}</td><td className="px-3 py-3"><PlatformStatus value={item.current ? "Current" : "Active"} /></td><td className="px-3 py-3">{!item.current && inventory.other_session_revocation_available && !pending ? <PlatformRowActions label="other session" actions={[{ label: "Revoke session", onSelect: () => setSelected(item.session_id), destructive: true }]} /> : <span className="text-muted-foreground">—</span>}</td></tr>)}
      {!inventory && <tr><td colSpan={4} className="px-3 py-5 text-center text-muted-foreground">{loading ? "Loading sessions…" : "Session list could not be loaded."}</td></tr>}
    </tbody></table></div>
    {failed && <Button type="button" size="sm" variant="outline" className="mt-3" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry sessions</Button>}
    <AlertDialog open={selected !== null} onOpenChange={(open) => { if (!open && !pending) setSelected(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Revoke this session?</AlertDialogTitle><AlertDialogDescription>This signs out the selected other session. Your current session stays active.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel><Button type="button" variant="destructive" disabled={pending || !csrfToken} onClick={() => void revokeSelected()} aria-busy={pending}>{pending ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <ShieldOffIcon aria-hidden="true" />}{pending ? "Revoking…" : "Revoke Session"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </section>;
}
