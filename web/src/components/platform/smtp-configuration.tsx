"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Loader2Icon, PlugIcon, RotateCcwIcon, SaveIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { normalizeNumericInput } from "@/lib/numeric-input";
import { PasswordInput } from "@/components/auth/password-input";
import { Label } from "@/components/ui/label";
import { parseSMTPVersion, parseSMTPVersions, type SMTPVersion, type SMTPVersionPage } from "@/lib/platform-smtp-contract";

import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Status = { kind: "loading" } | { kind: "unconfigured" } | { kind: "error"; message: string } | { kind: "ready"; value: SMTPVersion };
type Draft = { host: string; port: string; username: string; password: string; senderName: string; senderAddress: string; reason: string };
const emptyDraft: Draft = { host: "", port: "587", username: "", password: "", senderName: "", senderAddress: "", reason: "Configure Platform SMTP delivery" };
const fromVersion = (version: SMTPVersion | null): Draft => version ? { ...emptyDraft, host: version.configuration.host, username: version.configuration.username ?? "", port: String(version.configuration.port), senderName: version.configuration.sender_name, senderAddress: version.configuration.sender_address } : { ...emptyDraft };
function draftErrors(draft: Draft, canKeepPassword: boolean): Partial<Record<keyof Draft, string>> {
  const errors: Partial<Record<keyof Draft, string>> = {};
  if (!/^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$/.test(draft.host)) errors.host = "Enter a valid SMTP server name.";
  const port = Number(draft.port);
  if (!/^\d+$/.test(draft.port) || !Number.isInteger(port) || port < 1 || port > 65535) errors.port = "Enter a port from 1 to 65535.";
  if (!draft.username || draft.username.length > 256) errors.username = "Enter a username (maximum 256 characters).";
  if ((!draft.password && !canKeepPassword) || draft.password.length > 1024) errors.password = "Enter a password. Changing the server, port or username also requires a new password.";
  if (!draft.senderName.trim() || draft.senderName.length > 200) errors.senderName = "Enter a sender name.";
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.senderAddress) || draft.senderAddress.length > 254) errors.senderAddress = "Enter a valid sender email.";

  if (!draft.reason.trim() || draft.reason !== draft.reason.trim() || new TextEncoder().encode(draft.reason).length > 1000) errors.reason = "Enter a reason without surrounding spaces (maximum 1,000 bytes).";
  return errors;
}
const utc = (value: string) => `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC`;
const versionStates: Record<SMTPVersion["state"], string> = { DRAFT: "Draft", TEST_PASSED: "Test passed", TEST_FAILED: "Test failed", PENDING_APPROVAL: "Awaiting approval", ACTIVE: "Active setting", SUPERSEDED: "Replaced", REJECTED: "Rejected" };

export function SMTPConfiguration() {
  const { csrfToken, resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const capabilities = new Set(resolution.capabilities);
  const canManage = capabilities.has("platform.integration.smtp.manage");
  const canTest = capabilities.has("platform.integration.smtp.test");
  const canSave = canManage && capabilities.has("platform.integration.smtp.activate");
  const [status, setStatus] = useState<Status>({ kind: "loading" });
  const [history, setHistory] = useState<SMTPVersionPage | null>(null);
  const [historyError, setHistoryError] = useState(false);
  const [selected, setSelected] = useState<SMTPVersion | null>(null);
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [baseline, setBaseline] = useState<Draft>(emptyDraft);
  const [submitted, setSubmitted] = useState(false);

  const [touched, setTouched] = useState<Partial<Record<keyof Draft, boolean>>>({});
  const [serverErrors, setServerErrors] = useState<Partial<Record<keyof Draft, string>>>({});
  const submitting = useRef(false);
  const dirtyFields = useRef(new Set<keyof Draft>());
  const loadedGeneration = useRef(generation);
  const sequence = useRef(0);
  const headers = useCallback((mutation = false) => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation ?? ""), ...(mutation ? { "Content-Type": "application/json", "X-CSRF-Token": csrfToken } : {}) }), [csrfToken, generation]);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (loadedGeneration.current !== generation) {
      loadedGeneration.current = generation;
      dirtyFields.current.clear();
      setDraft({ ...emptyDraft }); setBaseline({ ...emptyDraft });
      setSubmitted(false); setTouched({}); setServerErrors({});
    }
    if (!generation) { setStatus({ kind: "error", message: "SMTP access could not be confirmed." }); return; }
    setStatus({ kind: "loading" }); setHistory(null); setHistoryError(false); setSelected(null);
    const options = { credentials: "include" as const, cache: "no-store" as const, headers: headers() };
    // History must never delay or prevent the current SMTP form from loading.
    void fetch("/api/platform/smtp/versions", options).then(async (response) => {
      const versions = response.ok ? parseSMTPVersions(await response.json()) : null;
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) refresh();
      setHistory(versions); setHistoryError(!versions);
    }).catch(() => { if (current === sequence.current) setHistoryError(true); });
    try {
      const response = await fetch("/api/platform/smtp", options);
      const active = response.ok ? parseSMTPVersion(await response.json()) : null;
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) {
        setStatus({ kind: "error", message: "SMTP access could not be confirmed." }); refresh(); return;
      }
      if (response.status === 403) { setStatus({ kind: "error", message: "You do not have permission to view SMTP settings." }); return; }
      if (response.status !== 404 && !active || active && active.state !== "ACTIVE") {
        setStatus({ kind: "error", message: "SMTP settings are temporarily unavailable." }); return;
      }
      const loaded = fromVersion(active);
      setBaseline(loaded);
      // Browser credential autofill or one edited field must not suppress all
      // the saved server/sender values. Preserve only fields actually edited.
      setDraft((previous) => Object.fromEntries(
        (Object.keys(loaded) as Array<keyof Draft>).map((key) => [key, dirtyFields.current.has(key) ? previous[key] : loaded[key]]),
      ) as Draft);
      setSelected(active);
      setStatus(active ? { kind: "ready", value: active } : { kind: "unconfigured" });
    } catch { if (current === sequence.current) setStatus({ kind: "error", message: "SMTP settings are temporarily unavailable." }); }
  }, [generation, headers, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);

  async function more(cursor: string) {
    if (!generation || busy) return;
    setBusy(true); setHistoryError(false);
    try {
      const response = await fetch(`/api/platform/smtp/versions?cursor=${encodeURIComponent(cursor)}`, { credentials: "include", cache: "no-store", headers: headers() });
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const page = response.ok ? parseSMTPVersions(await response.json()) : null;
      if (!page) { setHistoryError(true); return; }
      setHistory((previous) => {
        if (!previous || previous.next_cursor !== cursor) return previous;
        const ids = new Set(previous.versions.map((row) => row.id.toLowerCase()));
        return { versions: [...previous.versions, ...page.versions.filter((row) => !ids.has(row.id.toLowerCase()))], next_cursor: page.next_cursor };
      });
    } catch { setHistoryError(true); } finally { setBusy(false); }
  }
  function updateVersion(value: SMTPVersion) {
    setSelected(value);
    setHistory((previous) => previous && { ...previous, versions: [value, ...previous.versions.filter((row) => row.id !== value.id)].sort((a, b) => b.sequence - a.sequence) });
  }
  const hasSavedPassword = status.kind === "ready" && baseline.username.length > 0;
  const canKeepPassword = hasSavedPassword && draft.username === baseline.username && draft.host === baseline.host && draft.port === baseline.port;
  const errors = { ...draftErrors(draft, canKeepPassword), ...serverErrors };
  const changed = (Object.keys(emptyDraft) as Array<keyof Draft>).some((key) => draft[key] !== baseline[key]);
  function editDraft(key: keyof Draft, value: string) { if (value === baseline[key]) dirtyFields.current.delete(key); else dirtyFields.current.add(key); setDraft((previous) => ({ ...previous, [key]: value })); setServerErrors({}); }
  function cancelDraft() { dirtyFields.current.clear(); setDraft({ ...baseline }); setSubmitted(false); setTouched({}); setServerErrors({}); }
  const fieldError = (key: keyof Draft) => (submitted || touched[key]) ? errors[key] : undefined;
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitted(true); setServerErrors({});
    if (Object.keys(errors).length) return;
    if (!canSave || !generation || !csrfToken || busy || submitting.current || status.kind === "error" || status.kind === "loading") return;
    const request = sequence.current;
    submitting.current = true; setBusy(true);
    try {
      const response = await fetch("/api/platform/smtp", { method: "POST", credentials: "include", headers: headers(true), body: JSON.stringify({
        setting_kind: "SMTP", ...(canKeepPassword && !draft.password && status.kind === "ready" ? { retain_credentials: true, expected_active_version_id: status.value.id, expected_active_version: status.value.version } : {}), host: draft.host, port: Number(draft.port), username: draft.username, password: draft.password, sender_name: draft.senderName, sender_address: draft.senderAddress, reply_to_address: status.kind === "ready" ? status.value.configuration.reply_to_address : null,
        connect_timeout_seconds: status.kind === "ready" ? status.value.configuration.connect_timeout_seconds : 10, command_timeout_seconds: status.kind === "ready" ? status.value.configuration.command_timeout_seconds : 15, delivery_timeout_seconds: status.kind === "ready" ? status.value.configuration.delivery_timeout_seconds : 30, max_concurrency: status.kind === "ready" ? status.value.configuration.max_concurrency : 4, reason: draft.reason,
      }) });
      if (request !== sequence.current) return;
      if (response.status === 401) { refresh(); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["host", "port", "username", "password", "sender_name", "sender_address", "reason"] as const); const mapped: Partial<Record<keyof Draft, string>> = {}; const names: Record<string, keyof Draft> = { host: "host", port: "port", username: "username", password: "password", sender_name: "senderName", sender_address: "senderAddress", reason: "reason" }; for (const [key, message] of Object.entries(fields)) mapped[names[key]] = message; setServerErrors(mapped); if (Object.keys(mapped).length) toast.error("Check the highlighted fields.", { id: "smtp-configuration" }); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "smtp-configuration" }); return; }
      const value = response.ok ? parseSMTPVersion(await response.json()) : null;
      if (!value || value.state !== "ACTIVE") { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
      setStatus({kind:"ready",value});
      const saved = fromVersion(value);
      setBaseline(saved); setDraft(saved); dirtyFields.current.clear(); setSubmitted(false); setTouched({});
      updateVersion(value); toast.success("SMTP saved. All system emails use these settings.", { id: "smtp-configuration" });
    } catch { if (request === sequence.current) toast.error(networkSubmissionError, { id: "smtp-configuration" }); }
    finally { submitting.current = false; if (request === sequence.current) setBusy(false); }
  }
  async function command() {
    const version = status.kind === "ready" ? status.value : null;
    if (!version || busy || submitting.current || !canTest) return;
    submitting.current = true; setBusy(true);
    try {
      const response = await fetch("/api/platform/smtp/test", { method: "POST", credentials: "include", headers: headers(true), body: JSON.stringify({ version_id: version.id, version: version.version, reason: "Test the platform default SMTP" }) });
      if (response.status === 401) { refresh(); return; }
      if (!response.ok) { toast.error(submissionErrorMessage(response.status), { id: "smtp-configuration" }); return; }
      const value = parseSMTPVersion(await response.json());
      if (value && value.configuration.username == null) value.configuration.username = version.configuration.username;
      if (!value || value.state !== "ACTIVE") { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
      updateVersion(value);setStatus({kind:"ready",value});
      if (value.test_status === "PASSED") toast.success("Test email accepted by the SMTP server.", { id: "smtp-configuration" });
      else toast.error("Test email could not be sent. Check the SMTP settings.", { id: "smtp-configuration" });
    } catch { toast.error(networkSubmissionError, { id: "smtp-configuration" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Email (SMTP)</h1><p className="type-lead text-muted-foreground">Manage the default SMTP for sign-in, recovery, verification, invitations, and notifications.</p></div>
    <form onSubmit={(event) => void create(event)} noValidate className="max-w-4xl space-y-6" aria-label="SMTP configuration">
      <fieldset disabled={busy || status.kind === "loading" || status.kind === "error" || !canSave} aria-busy={status.kind === "loading"} className="space-y-6">
      <section aria-labelledby="email-server-heading" className="space-y-4"><h2 id="email-server-heading" className="text-base font-semibold">Email Server</h2><div className="grid gap-4 sm:grid-cols-2">
        <Field label="Server" name="host" value={draft.host} onChange={(event) => editDraft("host", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, host: true }))} maxLength={253} autoCapitalize="none" spellCheck={false} placeholder="smtp.example.com" error={fieldError("host")} />
        <Field label="Port" name="port" type="number" value={draft.port} onChange={(event) => editDraft("port", normalizeNumericInput(event.target.value))} onBlur={() => setTouched((old) => ({ ...old, port: true }))} min={1} max={65535} step={1} error={fieldError("port")} />
        <Field label="Username" name="username" value={draft.username} onChange={(event) => editDraft("username", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, username: true }))} autoComplete="username" maxLength={256} error={fieldError("username")} />
        <Field label="Password" name="password" type="password" value={draft.password} onChange={(event) => editDraft("password", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, password: true }))} autoComplete="new-password" maxLength={1024} required={!canKeepPassword} placeholder={hasSavedPassword ? "•••••••• (saved)" : "Enter password"} error={fieldError("password")} />
      </div><p className="text-xs text-muted-foreground">{hasSavedPassword ? "Password saved securely. Leave it blank to keep the current password, or enter a new one to replace it. Changing the server, port or username requires a new password." : "Enter your SMTP username and password. Passwords are stored securely and never displayed."}</p></section>
      <section aria-labelledby="smtp-sender-heading" className="space-y-4 border-t pt-5"><h2 id="smtp-sender-heading" className="text-base font-semibold">Sender Information</h2><div className="grid gap-4 sm:grid-cols-2">
        <Field label="Sender Name" name="sender_name" value={draft.senderName} onChange={(event) => editDraft("senderName", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, senderName: true }))} maxLength={200} error={fieldError("senderName")} />
        <Field label="Sender Email" name="sender_address" type="email" value={draft.senderAddress} onChange={(event) => editDraft("senderAddress", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, senderAddress: true }))} maxLength={254} autoCapitalize="none" spellCheck={false} placeholder="no-reply@example.com" error={fieldError("senderAddress")} />
      </div></section>

      </fieldset>

      <div className="flex flex-wrap items-center justify-end gap-2 border-t pt-5">{changed ? <span role="status" className="mr-auto text-sm text-muted-foreground">Unsaved changes</span> : selected && <span className="mr-auto text-xs text-muted-foreground">Tests use the selected saved settings.</span>}<Button type="button" variant="outline" onClick={cancelDraft} disabled={busy || !changed}>Cancel</Button><Button type="button" variant="outline" disabled={busy || changed || status.kind !== "ready" || !canTest} aria-busy={busy} onClick={() => void command()}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <PlugIcon aria-hidden="true" />}Send Test Email</Button><Button type="submit" disabled={busy || !canSave || !generation || !csrfToken || status.kind === "loading" || status.kind === "error"} aria-busy={busy}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <SaveIcon aria-hidden="true" />}{busy ? "Working…" : "Save Changes"}</Button></div>
    </form>
    <section aria-label="SMTP connection status" className="mt-6 max-w-4xl border-t pt-4 text-sm"><span className="font-medium">Connection status: </span><span className="text-muted-foreground">{status.kind === "loading" ? "Checking…" : status.kind === "error" ? "Could not load" : status.kind === "unconfigured" ? "No SMTP saved" : "Default SMTP active"}</span>{status.kind === "error" && <Button type="button" variant="link" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button>}{status.kind === "ready" && <p className="mt-1 text-xs text-muted-foreground">{status.value.configuration.sender_address} · {status.value.configuration.host}:{status.value.configuration.port} · {status.value.activated_at ? utc(status.value.activated_at) : "activation time unavailable"}</p>}</section>
    {historyError && !history && <p role="alert" className="mt-4 text-sm text-muted-foreground">Previous settings could not be loaded. The current SMTP settings are shown above when available.</p>}
    {history ? <details className="mt-5" aria-label="SMTP configuration history"><summary className="cursor-pointer text-sm font-medium">Previous settings ({history.versions.length}{history.next_cursor ? "+" : ""})</summary><section aria-label="SMTP version history" className="mt-3 space-y-3">{history.versions.length === 0 ? <p className="text-sm text-muted-foreground">No previous settings.</p> : <ul className="space-y-2">{history.versions.map((row) => <li key={row.id} className="rounded-lg border bg-card p-3 text-sm"><div className="flex flex-wrap justify-between gap-2"><p>{versionStates[row.state]} · {row.test_status === "NOT_TESTED" ? "Not tested" : row.test_status === "PASSED" ? "Connection test passed" : "Connection test failed"}</p><span>{utc(row.created_at)}</span></div><p className="mt-1 break-all text-muted-foreground">{row.configuration.host}:{row.configuration.port} · {row.configuration.sender_address}</p>{row.test_code ? <p>{row.test_status === "PASSED" ? "Connection test completed." : "Connection test did not pass. Check the settings before testing again."}</p> : null}</li>)}</ul>}{historyError ? <p role="alert" className="text-sm text-destructive">Could not load more settings.</p> : null}{history.next_cursor ? <Button type="button" variant="outline" disabled={busy} onClick={() => void more(history.next_cursor!)}>Load more</Button> : null}</section></details> : null}
  </main>;
}
function Field({ label, name, className, error, type, ...props }: { label: string; name: string; className?: string; error?: string } & React.ComponentProps<typeof Input>) {
  const control = { id: name, name, required: true, "aria-invalid": !!error, "aria-describedby": error ? `${name}-error` : undefined, ...props };
  return <div className={`grid gap-2 ${className ?? ""}`}><Label htmlFor={name}>{label}</Label>{type === "password" ? <PasswordInput {...control} /> : <Input type={type} {...control} />}{error && <p id={`${name}-error`} className="text-sm text-destructive">{error}</p>}</div>;
}
