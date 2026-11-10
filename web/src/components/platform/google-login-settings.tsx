"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { Loader2Icon, PencilIcon, SaveIcon, PlugIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PasswordInput } from "@/components/auth/password-input";
import { Label } from "@/components/ui/label";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useSession } from "@/components/session/session-provider";
import { SettingsSwitch } from "@/components/platform/settings-draft-controls";
import { googleDraftErrors, parseGoogleSettings, settingsData, type GoogleSettings } from "@/lib/platform-managed-settings-contract";

import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Draft = { login: boolean; signup: boolean; clientId: string; clientSecret: string };
const initialDraft: Draft = { login: false, signup: false, clientId: "", clientSecret: "" };
const fromSaved = (v: GoogleSettings): Draft => ({ login: v.enabled, signup: v.allow_signup, clientId: v.client_id, clientSecret: "" });
function FieldRow({ children, label, htmlFor, hint, error }: { children: ReactNode; label: string; htmlFor: string; hint?: string; error?: string }) { return <div className="grid min-w-0 gap-2"><div className="space-y-1"><Label htmlFor={htmlFor}>{label}</Label>{hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}</div><div className="min-w-0 space-y-2">{children}{error && <p id={`${htmlFor}-error`} role="alert" className="text-sm text-destructive">{error}</p>}</div></div>; }
export function GoogleLoginSettings() {
  const { resolution } = useSession();
  const permitted = resolution.capabilities.includes("platform.integration.read") && resolution.platform_context_generation !== null;
  return permitted ? <GoogleSettingsForm key={resolution.platform_context_generation} /> : <main className="app-page"><h1 className="type-title">Google Login &amp; Signup</h1><p className="mt-4 text-sm text-muted-foreground">You do not have permission to view these settings.</p></main>;
}
function GoogleSettingsForm() {
  const { resolution, csrfToken, refresh } = useSession(); const generation = resolution.platform_context_generation;
  const canSave = resolution.capabilities.includes("platform.governance.approve");
  const [saved, setSaved] = useState<GoogleSettings | null>(null); const [draft, setDraft] = useState<Draft>(initialDraft);
  const [loading, setLoading] = useState(true); const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null); const [replacing, setReplacing] = useState(false); const [confirmDisable, setConfirmDisable] = useState(false);
  const submitting = useRef(false);
  const [serverErrors, setServerErrors] = useState<Partial<Record<"client_id" | "client_secret", string>>>({});
  const sequence = useRef(0); const receipt = useRef<{ fingerprint: string; key: string } | null>(null);
  const headers = useCallback((write = false) => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation), ...(write ? { "Content-Type": "application/json", "X-CSRF-Token": csrfToken } : {}) }), [generation, csrfToken]);

  const load = useCallback(async () => {
    const current = ++sequence.current; setLoading(true); setMessage(null);
    try { const response = await fetch("/api/platform/settings/google", { cache: "no-store", headers: headers() }); if (current !== sequence.current) return; if (response.status === 401) { refresh(); return; } const value = response.ok ? parseGoogleSettings(settingsData(await response.json())) : null; if (!value) { setSaved(null); setMessage("Google settings could not be loaded. Check access or retry."); return; } setSaved(value); setDraft(fromSaved(value)); setServerErrors({}); setReplacing(false);  }
    catch { if (current === sequence.current) { setSaved(null); setMessage("Google settings could not be loaded."); } } finally { if (current === sequence.current) setLoading(false); }
  }, [headers, refresh]);
  useEffect(() => { const seq = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); seq.current++; }; }, [load]);
  const baseline = saved ? fromSaved(saved) : initialDraft; const changed = JSON.stringify(draft) !== JSON.stringify(baseline);
  // Preserve existing restrictions; editing other settings must not widen access.
  const domains = saved?.allowed_domains ?? [];
  const requiresSecret = !!saved && (draft.login && !saved.secret_configured || draft.clientId !== saved.client_id && saved.secret_configured);
  const errors = { ...googleDraftErrors({ enabled: draft.login, client_id: draft.clientId, client_secret: draft.clientSecret, allowed_domains: domains }, requiresSecret), ...serverErrors };
  const valid = Object.keys(errors).length === 0;
  async function save(event: FormEvent) {
    event.preventDefault(); if (!saved || !valid || !canSave || !csrfToken || busy || submitting.current) return;
    const payload = { enabled: draft.login, allow_signup: draft.login && draft.signup, client_id: draft.clientId, allowed_domains: domains, ...(draft.clientSecret ? { client_secret: draft.clientSecret } : {}) };
    const fingerprint = JSON.stringify([payload, saved.version, generation]); if (receipt.current?.fingerprint !== fingerprint) receipt.current = { fingerprint, key: crypto.randomUUID() };
    submitting.current = true; setBusy(true); setServerErrors({});
    try {
      const response = await fetch("/api/platform/settings/google", { method: "PUT", credentials: "include", cache: "no-store", headers: { ...headers(true), "If-Match": `"${saved.version}"`, "Idempotency-Key": receipt.current.key }, body: JSON.stringify(payload) });
      if (response.status === 401) { refresh(); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["client_id", "client_secret"] as const); setServerErrors(fields); if (Object.keys(fields).length) toast.error("Check the highlighted fields.", { id: "google-settings" }); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "google-settings" }); return; }
      const value = parseGoogleSettings(settingsData(await response.json())); if (!value) { toast.error(unconfirmedSubmissionError, { id: "google-settings" }); return; }
      setSaved(value); setDraft(fromSaved(value)); setReplacing(false); receipt.current = null;
      toast.success("Changes saved successfully.", { id: "google-settings" }); 
    } catch { toast.error(networkSubmissionError, { id: "google-settings" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  async function testConnection() {
    if (!saved || busy || submitting.current) return; submitting.current = true; setBusy(true);
    toast.loading("Checking Google connection…", { id: "google-settings" });
    try {
      const response = await fetch("/api/platform/settings/google/test", { method: "POST", credentials: "include", headers: { ...headers(true), "If-Match": `"${saved.version}"`, "Idempotency-Key": crypto.randomUUID() }, body: "{}" });
      if (response.status === 401) { toast.dismiss("google-settings"); refresh(); return; }
      if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "google-settings" }); return; }
      const data = settingsData(await response.json()) as { status?: string } | null;
      if (data?.status === "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED") toast.info("Google keys are reachable. Sign in with Google to verify credentials.", { id: "google-settings" });
      else if (data?.status === "PENDING") toast.warning("The test is pending. Refresh before trying again.", { id: "google-settings" });
      else toast.error("Connection could not be verified. Please try again.", { id: "google-settings" });
      if (data?.status === "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED" || data?.status === "UNAVAILABLE") setSaved((old) => old ? { ...old, connection_status: data.status! } : null);
    } catch { toast.error(networkSubmissionError, { id: "google-settings" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  const status = !saved ? "Unavailable" : !saved.enabled ? "Disabled" : !saved.runtime_configured ? "Setup required" : !saved.secret_configured ? "Client secret required" : saved.connection_status === "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED" ? "Google reachable · Sign-in not verified" : saved.connection_status === "UNAVAILABLE" ? "Last connection test failed" : "Saved · Sign-in not verified";
  return <main className="app-page"><div className="app-page-header mb-7"><h1 className="type-title">Google Login &amp; Signup</h1><p className="type-lead text-muted-foreground">Manage how users sign in and register with Google.</p></div>
    {message && <p role="status" className="mb-4 text-sm text-muted-foreground">{message}</p>}{loading && <p role="status" className="text-sm text-muted-foreground">Loading settings…</p>}{!saved && !loading && <Button variant="outline" onClick={() => void load()}>Retry</Button>}
    <form onSubmit={(event) => void save(event)} noValidate autoComplete="off" className="max-w-4xl" aria-label="Google Login and Signup settings"><fieldset disabled={!saved || busy || !canSave}>
      <section aria-labelledby="google-auth-heading" className="space-y-5 py-6"><h2 id="google-auth-heading" className="text-base font-semibold">Google Authentication</h2><div className="divide-y border-y"><SettingsSwitch id="google-login" label="Enable Google Login" hint="Email and password sign-in remains available." checked={draft.login} onChange={(next) => { if (!next && draft.login) setConfirmDisable(true); else setDraft((old) => ({ ...old, login: next })); }} /><SettingsSwitch id="google-signup" label="Allow Google Signup" hint="New Google users receive no administrator privileges." checked={draft.signup} onChange={(next) => setDraft((old) => ({ ...old, signup: next, login: old.login || next }))} /></div>
        <FieldRow htmlFor="google-client-id" label="Client ID" error={changed ? errors.client_id : undefined}><Input id="google-client-id" value={draft.clientId} onChange={(event) => { setServerErrors({}); setDraft((old) => ({ ...old, clientId: event.target.value })); }} autoComplete="off" autoCapitalize="none" spellCheck={false} maxLength={255} placeholder="OAuth client ID" aria-invalid={changed && !!errors.client_id} aria-describedby={changed && errors.client_id ? "google-client-id-error" : undefined} /></FieldRow>
        <FieldRow htmlFor="google-client-secret" label="Client Secret" error={changed ? errors.client_secret : undefined} hint={saved?.secret_configured ? "A secret is stored. Leave blank to keep it." : "No secret is stored."}><div className="flex gap-2"><PasswordInput id="google-client-secret" wrapperClassName="min-w-0 flex-1" autoComplete="new-password" value={draft.clientSecret} aria-invalid={changed && !!errors.client_secret} aria-describedby={changed && errors.client_secret ? "google-client-secret-error" : undefined} onChange={(event) => { setServerErrors({}); setDraft((old) => ({ ...old, clientSecret: event.target.value })); }} disabled={!replacing} placeholder={replacing ? "Enter new secret" : "Secret is never displayed"} maxLength={1024} /><Button type="button" variant="outline" onClick={() => setReplacing(true)} disabled={replacing}><PencilIcon aria-hidden="true" />Replace Secret</Button></div></FieldRow>
      </section></fieldset>
      <p className="py-3 text-sm"><span className="font-medium">Connection Status: </span><span className="text-muted-foreground">{status}</span></p>
      <div className="flex flex-wrap items-center justify-end gap-3 border-t py-6">{changed && <span className="mr-auto text-sm text-muted-foreground">Unsaved changes</span>}<Button type="button" variant="outline" disabled={busy} onClick={() => { if (saved) setDraft(fromSaved(saved)); setServerErrors({}); setReplacing(false); receipt.current = null; }}>Cancel</Button><Button type="button" variant="outline" disabled={!saved?.runtime_configured || !canSave || busy || !csrfToken || changed} aria-busy={busy} onClick={() => void testConnection()}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <PlugIcon aria-hidden="true"/>}Test Connection</Button><Button type="submit" aria-busy={busy} disabled={!saved || !canSave || busy || !changed || !valid || !csrfToken}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <SaveIcon aria-hidden="true" />}{busy ? "Working…" : "Save Changes"}</Button></div>
    </form>

    <AlertDialog open={confirmDisable} onOpenChange={setConfirmDisable}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Turn off Google login?</AlertDialogTitle><AlertDialogDescription>This takes effect when you save. Email and password sign-in stays available.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Keep on</AlertDialogCancel><Button type="button" variant="destructive" onClick={() => { setDraft((old) => ({ ...old, login: false, signup: false })); setConfirmDisable(false); }}>Turn off</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </main>;
}
