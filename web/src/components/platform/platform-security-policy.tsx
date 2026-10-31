"use client";

import { parsePlatformSessionPolicy, type PlatformSessionPolicy } from "@/lib/platform-session-policy-contract";
import { PlatformSelect } from "@/components/platform/platform-select";
import Link from "next/link";
import { Loader2Icon, RotateCcwIcon, ShieldOffIcon, SaveIcon } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { SessionInventoryResponse } from "@/contracts/auth.generated";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
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
function FixedValue({ value }: { value: string }) {
  return <p className="py-2 text-sm text-muted-foreground">{value}</p>;
}
function FixedNumber({ id, label, value }: { id: string; label: string; value: number }) {
  return <p id={id} aria-label={label} className="py-2 text-sm text-muted-foreground">{value}</p>;
}
function FixedSelect({ label, value }: { label: string; value: string }) {
  return <p aria-label={label} className="py-2 text-sm text-muted-foreground">{value}</p>;
}

export function PlatformSecurityPolicy() {
  const { resolution, csrfToken } = useSession();
  const allowed = resolution.capabilities.includes("platform.admin.read") && resolution.platform_context_generation !== null;
  return <main className="app-page">
    <div className="app-page-header mb-7"><h1 className="type-title">Security &amp; Sessions</h1><p className="type-lead text-muted-foreground">Manage platform login security and session preferences.</p></div>
    {!allowed ? <p className="text-sm text-muted-foreground">You do not currently have access to this section.</p> : <div className="max-w-4xl">
      
      <section aria-labelledby="login-security-heading" className="space-y-5 py-6"><h2 id="login-security-heading" className="text-base font-semibold">Login Security</h2>
        <PolicyField label="Require Two-Step Verification for Administrators"><FixedValue value="Required for administrator access" /></PolicyField>
        <PolicyField label="Maximum Failed Login Attempts" hint="Within 15 minutes."><FixedNumber id="failed-logins" label="Maximum failed login attempts per identity" value={5} /></PolicyField>
        <PolicyField label="Account Lockout Duration"><FixedSelect label="Account lockout duration" value="15 minutes" /></PolicyField>
        <PolicyField label="Minimum Password Length" hint="Required minimum."><FixedNumber id="minimum-password" label="Minimum password length in bytes" value={12} /></PolicyField>
      </section>
      <section aria-labelledby="session-management-heading" className="space-y-5 border-t py-6"><h2 id="session-management-heading" className="text-base font-semibold">Session Management</h2>
        <SessionPolicyFields />
        <PolicyField label="Allow Multiple Active Sessions"><FixedValue value="Allowed" /></PolicyField>
        <PolicyField label="Require Reauthentication for Sensitive Actions"><FixedValue value="Required · recent two-step verification" /></PolicyField>
      </section>
      <section aria-labelledby="account-protection-heading" className="space-y-5 border-t py-6"><h2 id="account-protection-heading" className="text-base font-semibold">Account Protection</h2>
        <PolicyField label="Email Verification Required"><FixedValue value="Required for sign-in" /></PolicyField>
        <PolicyField label="Password Reset Expiry"><FixedSelect label="Password reset expiry" value="30 minutes" /></PolicyField>
        
      </section>
      <ActiveSessions key={csrfToken} csrfToken={csrfToken} />
    </div>}
  </main>;
}

function SessionPolicyFields() {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [loaded, setLoaded] = useState<{ generation: number; value: PlatformSessionPolicy } | null>(null);
  const policy = loaded?.generation === generation ? loaded.value : null;
  const [idle, setIdle] = useState(1800);
  const [absolute, setAbsolute] = useState(43200);
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const submitting = useRef(false);
  const receipt = useRef<{ fingerprint: string; key: string } | null>(null);
  const canSave = resolution.capabilities.includes("platform.governance.approve");
  useEffect(() => {
    if (!generation) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch("/api/platform/security/session-policy", { cache: "no-store", signal: controller.signal, headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
        const value = response.ok ? parsePlatformSessionPolicy(await response.json()) : null;
        if (controller.signal.aborted) return;
        if (!value) { setLoaded(null); setMessage("Session settings could not be loaded."); return; }
        setLoaded({ generation, value }); setIdle(value.idle_seconds); setAbsolute(value.absolute_seconds); setMessage(null);
      } catch { if (!controller.signal.aborted) { setLoaded(null); setMessage("Session settings could not be loaded."); } }
    })();
    return () => controller.abort();
  }, [generation, revision]);
  async function save() {
    if (!policy || !generation || !csrfToken || busy || submitting.current) return;
    const fingerprint = JSON.stringify({ idle_seconds: idle, absolute_seconds: absolute, version: policy.version, generation });
    if (receipt.current?.fingerprint !== fingerprint) receipt.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setBusy(true); setMessage(null);
    try {
      const response = await fetch("/api/platform/security/session-policy", { method: "PUT", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": String(generation), "X-CSRF-Token": csrfToken, "If-Match": `"${policy.version}"`, "Idempotency-Key": receipt.current.key }, body: JSON.stringify({ idle_seconds: idle, absolute_seconds: absolute }) });
      if (response.status === 401) { refresh(); return; }
      const value = response.ok ? parsePlatformSessionPolicy(await response.json()) : null;
      if (!value) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "session-policy" }); return; }
      setLoaded({ generation, value }); setConfirm(false); receipt.current = null; toast.success("Session settings saved. Ordinary sessions were signed out.", { id: "session-policy" }); refresh();
    } catch { toast.error(networkSubmissionError, { id: "session-policy" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  return <>
    <PolicyField label="Session Inactivity Timeout" hint="For standard user sessions."><PlatformSelect aria-label="Ordinary session inactivity timeout" value={idle} disabled={!policy || !canSave || busy} onChange={(event) => setIdle(Number(event.target.value))}>{[300, 600, 900, 1800].map((seconds) => <option value={seconds} key={seconds}>{seconds / 60} minutes</option>)}</PlatformSelect></PolicyField>
    <PolicyField label="Maximum Session Duration" hint="Administrator sessions are unchanged."><PlatformSelect aria-label="Ordinary maximum session duration" value={absolute} disabled={!policy || !canSave || busy} onChange={(event) => setAbsolute(Number(event.target.value))}>{[3600, 14400, 28800, 43200].map((seconds) => <option value={seconds} key={seconds}>{seconds / 3600} hours</option>)}</PlatformSelect></PolicyField>
    {message && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
    <div className="flex justify-end gap-3"><Button type="button" variant="outline" disabled={busy||!policy} onClick={() => { if(policy){setIdle(policy.idle_seconds);setAbsolute(policy.absolute_seconds);} }}>Cancel</Button><Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => setRevision(value=>value+1)} aria-label="Refresh session settings"><RotateCcwIcon aria-hidden="true"/></Button><Button type="button" disabled={!policy || !canSave || busy || !csrfToken || idle === policy.idle_seconds && absolute === policy.absolute_seconds} onClick={() => setConfirm(true)}><SaveIcon aria-hidden="true"/>Save Changes</Button></div>
    <AlertDialog open={confirm} onOpenChange={(open) => { if (!busy) setConfirm(open); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Update session settings?</AlertDialogTitle><AlertDialogDescription>Standard users will be signed out. Administrator sessions stay active.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="button" disabled={busy} aria-busy={busy} onClick={() => void save()}>{busy && <Loader2Icon aria-hidden="true" className="animate-spin" />}{busy ? "Saving…" : "Save Changes"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
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
