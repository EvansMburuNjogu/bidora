"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Loader2Icon, CheckIcon, PlugIcon, RotateCcwIcon, SaveIcon, SendIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PasswordInput } from "@/components/auth/password-input";
import { Label } from "@/components/ui/label";
import { parseSMTPChange, parseSMTPDecision, parseSMTPVersion, parseSMTPVersions, smtpUUID, type SMTPChange, type SMTPVersion, type SMTPVersionPage } from "@/lib/platform-smtp-contract";

import { SettingsApprovalReference } from "@/components/platform/settings-draft-controls";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Status = { kind: "loading" } | { kind: "unconfigured" } | { kind: "error"; message: string } | { kind: "ready"; value: SMTPVersion };
type Draft = { host: string; port: string; username: string; password: string; senderName: string; senderAddress: string; reason: string };
const emptyDraft: Draft = { host: "", port: "587", username: "", password: "", senderName: "", senderAddress: "", reason: "Configure Platform SMTP delivery" };
const fromVersion = (version: SMTPVersion | null): Draft => version ? { ...emptyDraft, host: version.configuration.host, port: String(version.configuration.port), senderName: version.configuration.sender_name, senderAddress: version.configuration.sender_address } : { ...emptyDraft };
function draftErrors(draft: Draft): Partial<Record<keyof Draft, string>> {
  const errors: Partial<Record<keyof Draft, string>> = {};
  if (!/^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$/.test(draft.host)) errors.host = "Enter a valid SMTP server name.";
  const port = Number(draft.port);
  if (!/^\d+$/.test(draft.port) || !Number.isInteger(port) || port < 1 || port > 65535) errors.port = "Enter a port from 1 to 65535.";
  if (!draft.username || draft.username.length > 256) errors.username = "Enter a username (maximum 256 characters).";
  if (!draft.password || draft.password.length > 1024) errors.password = "Enter a new password to save this version.";
  if (!draft.senderName.trim() || draft.senderName.length > 200) errors.senderName = "Enter a sender name.";
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.senderAddress) || draft.senderAddress.length > 254) errors.senderAddress = "Enter a valid sender email.";

  if (!draft.reason.trim() || draft.reason !== draft.reason.trim() || new TextEncoder().encode(draft.reason).length > 1000) errors.reason = "Enter a reason without surrounding spaces (maximum 1,000 bytes).";
  return errors;
}
const utc = (value: string) => `${new Date(value).toLocaleString(undefined, { timeZone: "UTC" })} UTC`;
const versionStates: Record<SMTPVersion["state"], string> = { DRAFT: "Draft", TEST_PASSED: "Test passed", TEST_FAILED: "Test failed", PENDING_APPROVAL: "Awaiting approval", ACTIVE: "Active setting", SUPERSEDED: "Replaced", REJECTED: "Rejected" };
const changeStates: Record<SMTPChange["state"], string> = { PENDING_APPROVAL: "Awaiting approval", APPROVED_EXECUTED: "Approved and recorded", REJECTED: "Rejected", EXPIRED: "Expired" };
export function SMTPConfiguration() {
  const { csrfToken, resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const capabilities = new Set(resolution.capabilities);
  const canManage = capabilities.has("platform.integration.smtp.manage");
  const canTest = capabilities.has("platform.integration.smtp.test");
  const canActivate = capabilities.has("platform.integration.smtp.activate");
  const canRollback = canActivate && capabilities.has("platform.integration.rollback");
  const canApprove = capabilities.has("platform.governance.approve");
  const [status, setStatus] = useState<Status>({ kind: "loading" });
  const [history, setHistory] = useState<SMTPVersionPage | null>(null);
  const [historyError, setHistoryError] = useState(false);
  const [selected, setSelected] = useState<SMTPVersion | null>(null);
  const [change, setChange] = useState<SMTPChange | null>(null);
  const [confirmActivation, setConfirmActivation] = useState(false);
  const [decisionIntent, setDecisionIntent] = useState<{ action: "approve" | "reject"; reason: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [baseline, setBaseline] = useState<Draft>(emptyDraft);
  const [submitted, setSubmitted] = useState(false);

  const [touched, setTouched] = useState<Partial<Record<keyof Draft, boolean>>>({});
  const [serverErrors, setServerErrors] = useState<Partial<Record<keyof Draft, string>>>({});
  const [changeIdError, setChangeIdError] = useState<string | null>(null);
  const [decisionReasonError, setDecisionReasonError] = useState<string | null>(null);
  const submitting = useRef(false);
  const dirtyRef = useRef(false);
  const sequence = useRef(0);
  const headers = useCallback((mutation = false) => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation ?? ""), ...(mutation ? { "Content-Type": "application/json", "X-CSRF-Token": csrfToken } : {}) }), [csrfToken, generation]);
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setStatus({ kind: "error", message: "SMTP access could not be confirmed." }); return; }
    setStatus({ kind: "loading" }); setHistory(null); setHistoryError(false); setSelected(null); setChange(null); setConfirmActivation(false); setDecisionIntent(null);
    try {
      const [activeResponse, versionsResponse] = await Promise.all([
        fetch("/api/platform/smtp", { credentials: "include", cache: "no-store", headers: headers() }),
        fetch("/api/platform/smtp/versions", { credentials: "include", cache: "no-store", headers: headers() }),
      ]);
      if (current !== sequence.current) return;
      if ([401, 409].includes(activeResponse.status) || [401, 409].includes(versionsResponse.status)) { refresh(); return; }
      if (activeResponse.status === 403 || versionsResponse.status === 403) { setStatus({ kind: "error", message: "SMTP settings require current integration access and recent email verification." }); return; }
      const versions = versionsResponse.ok ? parseSMTPVersions(await versionsResponse.json()) : null;
      const active = activeResponse.ok ? parseSMTPVersion(await activeResponse.json()) : null;
      if (!versions || activeResponse.status !== 404 && !active) { setStatus({ kind: "error", message: "SMTP settings are temporarily unavailable." }); return; }
      const loaded = fromVersion(active ?? versions.versions[0] ?? null);
      setBaseline(loaded);
      if (!dirtyRef.current) setDraft(loaded);
      setSelected(versions.versions.find((row) => ["DRAFT", "TEST_FAILED", "TEST_PASSED", "SUPERSEDED"].includes(row.state)) ?? null);
      setHistory(versions);
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
  const errors = { ...draftErrors(draft), ...serverErrors };
  const changed = (Object.keys(emptyDraft) as Array<keyof Draft>).some((key) => draft[key] !== baseline[key]);
  function editDraft(key: keyof Draft, value: string) { dirtyRef.current = true; setDraft((previous) => ({ ...previous, [key]: value })); setServerErrors({}); }
  function cancelDraft() { dirtyRef.current = false; setDraft({ ...baseline }); setSubmitted(false); setTouched({}); setServerErrors({}); }
  const fieldError = (key: keyof Draft) => (submitted || touched[key]) ? errors[key] : undefined;
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitted(true); setServerErrors({});
    if (Object.keys(errors).length) return;
    if (!canManage || !generation || !csrfToken || busy || submitting.current || status.kind === "error" || status.kind === "loading") return;
    const request = sequence.current;
    submitting.current = true; setBusy(true);
    try {
      const response = await fetch("/api/platform/smtp", { method: "POST", credentials: "include", headers: headers(true), body: JSON.stringify({
        setting_kind: "SMTP", host: draft.host, port: Number(draft.port), username: draft.username, password: draft.password, sender_name: draft.senderName, sender_address: draft.senderAddress, reply_to_address: status.kind === "ready" ? status.value.configuration.reply_to_address : null,
        connect_timeout_seconds: status.kind === "ready" ? status.value.configuration.connect_timeout_seconds : 10, command_timeout_seconds: status.kind === "ready" ? status.value.configuration.command_timeout_seconds : 15, delivery_timeout_seconds: status.kind === "ready" ? status.value.configuration.delivery_timeout_seconds : 30, max_concurrency: status.kind === "ready" ? status.value.configuration.max_concurrency : 4, reason: draft.reason,
      }) });
      if (request !== sequence.current) return;
      if (response.status === 401) { refresh(); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["host", "port", "username", "password", "sender_name", "sender_address", "reason"] as const); const mapped: Partial<Record<keyof Draft, string>> = {}; const names: Record<string, keyof Draft> = { host: "host", port: "port", username: "username", password: "password", sender_name: "senderName", sender_address: "senderAddress", reason: "reason" }; for (const [key, message] of Object.entries(fields)) mapped[names[key]] = message; setServerErrors(mapped); if (Object.keys(mapped).length) toast.error("Check the highlighted fields.", { id: "smtp-configuration" }); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "smtp-configuration" }); return; }
      const value = response.ok ? parseSMTPVersion(await response.json()) : null;
      if (!value) { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
      const saved = fromVersion(value);
      setBaseline(saved); setDraft(saved); dirtyRef.current = false; setSubmitted(false); setTouched({});
      updateVersion(value); toast.success("Changes saved. Test the connection before requesting approval.", { id: "smtp-configuration" });
    } catch { if (request === sequence.current) toast.error(networkSubmissionError, { id: "smtp-configuration" }); }
    finally { submitting.current = false; if (request === sequence.current) setBusy(false); }
  }
  async function command(operation: "test" | "propose") {
    const version = selected;
    if (!version || busy || submitting.current || operation === "test" && (!canTest || !["DRAFT", "TEST_FAILED", "TEST_PASSED", "SUPERSEDED"].includes(version.state)) ||
        operation === "propose" && (version.test_status !== "PASSED" || !(version.state === "TEST_PASSED" && canActivate || version.state === "SUPERSEDED" && canRollback))) return;
    submitting.current = true; setBusy(true);
    if (operation === "test") toast.loading("Checking SMTP connection…", { id: "smtp-configuration" });
    try {
      const response = await fetch(`/api/platform/smtp/${operation}`, { method: "POST", credentials: "include", headers: headers(true), body: JSON.stringify({ version_id: version.id, version: version.version, ...(operation === "test" ? { reason: "Verify authenticated STARTTLS configuration" } : { action: version.state === "SUPERSEDED" ? "ROLLBACK" : "ACTIVATE", reason: version.state === "SUPERSEDED" ? "Restore previously active verified SMTP configuration" : "Activate verified SMTP configuration" }) }) });
      if (response.status === 401) { toast.dismiss("smtp-configuration"); refresh(); return; }
      if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "smtp-configuration" }); return; }
      if (operation === "test") {
        const value = parseSMTPVersion(await response.json());
        if (!value) { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
        updateVersion(value);
        if (value.test_status === "PASSED") toast.success("Connection test passed. Delivery is not confirmed.", { id: "smtp-configuration" });
        else if (value.test_status === "FAILED") toast.error("Connection test failed. Check the settings and retry.", { id: "smtp-configuration" });
        else toast.warning("Test outcome is unconfirmed. Refresh before retrying.", { id: "smtp-configuration" });
      } else {
        const value = parseSMTPChange(await response.json());
        if (!value) { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
        setChange(value); setSelected(null); setConfirmActivation(false);
        toast.info(value.state === "PENDING_APPROVAL" ? version.state === "SUPERSEDED" ? "Rollback requested. A different Owner must approve; the active setting is unchanged." : "Your request has been submitted. Activation is awaiting approval." : "This request has already been processed. Review the current setting.", { id: "smtp-configuration" });
      }
    } catch { toast.error(networkSubmissionError, { id: "smtp-configuration" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  async function lookup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const id = String(new FormData(event.currentTarget).get("change_id") ?? "").trim();
    if (busy || submitting.current) return;
    if (!smtpUUID.test(id)) { setChangeIdError("Enter a valid approval reference."); return; }
    submitting.current = true; setChangeIdError(null); setBusy(true);
    try {
      const response = await fetch(`/api/platform/smtp/changes/${id}`, { credentials: "include", cache: "no-store", headers: headers() });
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      if (response.status === 404) { toast.error("This SMTP change was not found.", { id: "smtp-lookup" }); return; }
      if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "smtp-lookup" }); return; }
      const value = parseSMTPChange(await response.json(), id);
      if (!value) throw new Error("lookup_failed");
      setChange(value);
    } catch { toast.error(networkSubmissionError, { id: "smtp-lookup" }); } finally { submitting.current = false; setBusy(false); }
  }
  function prepareDecision(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const current = change;
    if (!current || !canApprove || current.state !== "PENDING_APPROVAL" || busy || !history?.versions.some((row) => row.id === current.target_version_id)) return;
    const form = new FormData(event.currentTarget);
    const submitter = (event.nativeEvent as SubmitEvent).submitter;
    const action = submitter instanceof HTMLButtonElement ? submitter.value : "";
    const reason = String(form.get("reason") ?? "");
    if ((action !== "approve" && action !== "reject") || reason !== reason.trim() || new TextEncoder().encode(reason).byteLength < 1 || new TextEncoder().encode(reason).byteLength > 1000) { setDecisionReasonError("Enter a reason of at most 1,000 bytes without surrounding spaces."); return; }
    setDecisionReasonError(null); setDecisionIntent({ action, reason });
  }
  async function decide() {
    const current = change, intent = decisionIntent;
    if (!current || !intent || !canApprove || current.state !== "PENDING_APPROVAL" || busy || submitting.current || !history?.versions.some((row) => row.id === current.target_version_id)) return;
    submitting.current = true; setBusy(true);
    try {
      const currentResponse = await fetch(`/api/platform/smtp/changes/${current.id}`, { credentials: "include", cache: "no-store", headers: headers() });
      if (currentResponse.status === 401) { refresh(); return; }
      if (!currentResponse.ok) { toast.error(submissionErrorMessage(currentResponse.status, currentResponse.headers.get("retry-after")), { id: "smtp-configuration" }); return; }
      const latest = parseSMTPChange(await currentResponse.json(), current.id);
      if (!latest || latest.state !== "PENDING_APPROVAL" || latest.version !== current.version || latest.target_version_id !== current.target_version_id) { toast.error(submissionErrorMessage(409), { id: "smtp-configuration" }); return; }
      const response = await fetch(`/api/platform/smtp/${intent.action}`, { method: "POST", credentials: "include", headers: headers(true), body: JSON.stringify({ change_id: latest.id, version: latest.version, reason: intent.reason }) });
      if (response.status === 401) { refresh(); return; }
      if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "smtp-configuration" }); return; }
      const value = intent.action === "approve" ? parseSMTPDecision(await response.json(), latest.id) : parseSMTPChange(await response.json(), latest.id);
      if (!value || intent.action === "reject" && value.state !== "REJECTED") { toast.error(unconfirmedSubmissionError, { id: "smtp-configuration" }); return; }
      setChange(value); setDecisionIntent(null);
      toast.success(intent.action === "approve" ? "Activation recorded. Delivery is not confirmed." : "Activation rejected.", { id: "smtp-configuration" }); await load();
    } catch { toast.error(networkSubmissionError, { id: "smtp-configuration" }); }
    finally { submitting.current = false; setBusy(false); }
  }
  const target = change && history?.versions.find((row) => row.id.toLowerCase() === change.target_version_id.toLowerCase());
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Email (SMTP)</h1><p className="type-lead text-muted-foreground">Manage the default SMTP for sign-in, recovery, verification, invitations, and notifications.</p></div>
    <form onSubmit={(event) => void create(event)} noValidate className="max-w-4xl space-y-6" aria-label="SMTP configuration">
      <fieldset disabled={busy} className="space-y-6">
      <section aria-labelledby="email-server-heading" className="space-y-4"><h2 id="email-server-heading" className="text-base font-semibold">Email Server</h2><div className="grid gap-4 sm:grid-cols-2">
        <Field label="Server" name="host" value={draft.host} onChange={(event) => editDraft("host", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, host: true }))} maxLength={253} autoCapitalize="none" spellCheck={false} placeholder="smtp.example.com" error={fieldError("host")} />
        <Field label="Port" name="port" type="number" value={draft.port} onChange={(event) => editDraft("port", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, port: true }))} min={1} max={65535} step={1} error={fieldError("port")} />
        <Field label="Username" name="username" value={draft.username} onChange={(event) => editDraft("username", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, username: true }))} autoComplete="username" maxLength={256} error={fieldError("username")} />
        <Field label="Password" name="password" type="password" value={draft.password} onChange={(event) => editDraft("password", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, password: true }))} autoComplete="new-password" maxLength={1024} placeholder="Enter new password" error={fieldError("password")} />
        <div className="grid gap-2"><Label htmlFor="smtp-encryption">Connection security</Label><PlatformSelect id="smtp-encryption" value="STARTTLS" disabled><option value="STARTTLS">STARTTLS (required)</option></PlatformSelect></div>
      </div><p className="text-xs text-muted-foreground">Your password is never displayed. Enter a new password when saving.</p></section>
      <section aria-labelledby="smtp-sender-heading" className="space-y-4 border-t pt-5"><h2 id="smtp-sender-heading" className="text-base font-semibold">Sender Information</h2><div className="grid gap-4 sm:grid-cols-2">
        <Field label="Sender Name" name="sender_name" value={draft.senderName} onChange={(event) => editDraft("senderName", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, senderName: true }))} maxLength={200} error={fieldError("senderName")} />
        <Field label="Sender Email" name="sender_address" type="email" value={draft.senderAddress} onChange={(event) => editDraft("senderAddress", event.target.value)} onBlur={() => setTouched((old) => ({ ...old, senderAddress: true }))} maxLength={254} autoCapitalize="none" spellCheck={false} placeholder="no-reply@example.com" error={fieldError("senderAddress")} />
      </div></section>

      </fieldset>

      <div className="flex flex-wrap items-center justify-end gap-2 border-t pt-5">{changed ? <span role="status" className="mr-auto text-sm text-muted-foreground">Unsaved changes</span> : selected && <span className="mr-auto text-xs text-muted-foreground">Tests use the selected saved settings.</span>}<Button type="button" variant="outline" onClick={cancelDraft} disabled={busy || !changed}>Cancel</Button><Button type="button" variant="outline" disabled={busy || changed || !selected || !canTest || !["DRAFT", "TEST_FAILED", "TEST_PASSED", "SUPERSEDED"].includes(selected.state)} aria-busy={busy} onClick={() => void command("test")}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <PlugIcon aria-hidden="true" />}Test Connection</Button><Button type="submit" disabled={busy || !canManage || !generation || !csrfToken || status.kind === "loading" || status.kind === "error"} aria-busy={busy}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <SaveIcon aria-hidden="true" />}{busy ? "Working…" : "Save Changes"}</Button></div>
    </form>
    <section aria-label="SMTP connection status" className="mt-6 max-w-4xl border-t pt-4 text-sm"><span className="font-medium">Connection status: </span><span className="text-muted-foreground">{status.kind === "loading" ? "Checking…" : status.kind === "error" ? "Could not load" : status.kind === "unconfigured" ? history?.versions.some(version => version.state === "PENDING_APPROVAL") ? "Awaiting approval · No active default SMTP" : "No active default SMTP" : "Enabled · Delivery not confirmed"}</span>{status.kind === "error" && <Button type="button" variant="link" size="sm" onClick={() => void load()}><RotateCcwIcon aria-hidden="true" />Retry</Button>}{status.kind === "ready" && <p className="mt-1 text-xs text-muted-foreground">{status.value.configuration.sender_address} · {status.value.configuration.host}:{status.value.configuration.port} · {status.value.activated_at ? utc(status.value.activated_at) : "activation time unavailable"}</p>}</section>
    {history ? <details className="mt-5" aria-label="SMTP configuration history"><summary className="cursor-pointer text-sm font-medium">Previous settings ({history.versions.length}{history.next_cursor ? "+" : ""})</summary><section aria-label="SMTP version history" className="mt-3 space-y-3">{history.versions.length === 0 ? <p className="text-sm text-muted-foreground">No previous settings.</p> : <ul className="space-y-2">{history.versions.map((row) => <li key={row.id} className="rounded-lg border bg-card p-3 text-sm"><div className="flex flex-wrap justify-between gap-2"><p>{versionStates[row.state]} · {row.test_status === "NOT_TESTED" ? "Not tested" : row.test_status === "PASSED" ? "Connection test passed" : "Connection test failed"}</p><span>{utc(row.created_at)}</span></div><p className="mt-1 break-all text-muted-foreground">{row.configuration.host}:{row.configuration.port} · {row.configuration.sender_address}</p>{row.test_code ? <p>{row.test_status === "PASSED" ? "Connection test completed." : "Connection test did not pass. Check the settings before testing again."}</p> : null}{canTest || canActivate ? <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => setSelected(row)} className="mt-2">Review</Button> : null}</li>)}</ul>}{historyError ? <p role="alert" className="text-sm text-destructive">Could not load more settings.</p> : null}{history.next_cursor ? <Button type="button" variant="outline" disabled={busy} onClick={() => void more(history.next_cursor!)}>Load more</Button> : null}</section></details> : null}
    {selected ? <section aria-label="Selected SMTP version" className="mt-5 rounded-lg border bg-card p-4 text-sm"><h2 className="font-medium">Saved changes</h2><p className="mt-1">{versionStates[selected.state]} · {selected.test_status === "PASSED" ? "Connection test passed" : selected.test_status === "FAILED" ? "Connection test failed" : "Not tested"} · {selected.configuration.sender_address}</p><p className="text-muted-foreground">{selected.configuration.host}:{selected.configuration.port} · created {utc(selected.created_at)}</p><div className="mt-3 flex flex-wrap gap-2">{selected.test_status === "PASSED" && (selected.state === "TEST_PASSED" && canActivate || selected.state === "SUPERSEDED" && canRollback) ? <Button type="button" disabled={busy} onClick={() => setConfirmActivation(true)}><SendIcon aria-hidden="true" />{selected.state === "SUPERSEDED" ? "Request rollback" : "Request activation"}</Button> : null}</div></section> : null}
    <AlertDialog open={confirmActivation} onOpenChange={(value) => { if (!submitting.current) setConfirmActivation(value); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{selected?.state === "SUPERSEDED" ? "Request SMTP rollback?" : "Request email activation?"}</AlertDialogTitle><AlertDialogDescription>Ask another Owner to approve {selected?.state === "SUPERSEDED" ? "restoring the previous settings" : "these settings"}. Current settings stay active until approval.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><AlertDialogAction type="button" disabled={busy || !selected} aria-busy={busy} onClick={() => void command("propose")}><SendIcon aria-hidden="true" />Request approval</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    <AlertDialog open={decisionIntent !== null} onOpenChange={(open) => { if (!open && !busy) setDecisionIntent(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{decisionIntent?.action === "approve" ? "Approve this email setting?" : "Reject this email setting?"}</AlertDialogTitle><AlertDialogDescription>{decisionIntent?.action === "approve" ? "Approval can make the reviewed version active if it still passes all checks. It does not prove email delivery." : "The request will be rejected. The current active setting will not change."}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><AlertDialogAction type="button" disabled={busy} variant={decisionIntent?.action === "approve" ? "default" : "destructive"} aria-busy={busy} onClick={() => void decide()}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <CheckIcon aria-hidden="true" />}{decisionIntent?.action === "approve" ? "Confirm approval" : "Confirm rejection"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    {change ? <Alert className="mt-5" role="status"><AlertTitle>Activation request: {changeStates[change.state]}</AlertTitle><AlertDescription>Reference <SettingsApprovalReference reference={change.id}/>. {change.state === "PENDING_APPROVAL" ? `A different authorised administrator must review this request before it can become active. Decision due ${utc(change.expires_at)}.` : "Check the current setting to confirm what is active."}</AlertDescription></Alert> : null}
    {canApprove && history ? <details className="mt-5"><summary className="cursor-pointer text-sm font-medium">Approvals</summary><section aria-label="Review SMTP activation change" className="mt-5 space-y-3 rounded-lg border bg-card p-4"><h2 className="font-medium">Review an approval</h2><p className="text-sm text-muted-foreground">Enter the approval reference shared with you.</p><form onSubmit={(event) => void lookup(event)} className="flex flex-wrap items-end gap-2"><Field label="Approval reference" name="change_id" error={changeIdError ?? undefined} onChange={() => setChangeIdError(null)} placeholder="Approval reference" className="min-w-60 flex-1"/><Button type="submit" variant="outline" aria-busy={busy} disabled={busy}>{busy && <Loader2Icon aria-hidden="true" className="animate-spin" />}Review</Button></form>{change ? <div className="space-y-2 border-t pt-3 text-sm"><p>{changeStates[change.state]} · Decision due {utc(change.expires_at)}</p>{target ? <p>Settings: {target.configuration.host}:{target.configuration.port} · {target.configuration.sender_address} · test {target.test_status}</p> : <p role="alert">Load the selected settings before approving.</p>}{change.state === "PENDING_APPROVAL" && target ? <form onSubmit={prepareDecision} className="flex flex-wrap items-end gap-2"><Field label="Decision reason" name="reason" error={decisionReasonError ?? undefined} onChange={() => setDecisionReasonError(null)} className="min-w-60 flex-1"/><Button type="submit" name="decision" value="approve" disabled={busy}><CheckIcon aria-hidden="true" />Approve</Button><Button type="submit" name="decision" value="reject" disabled={busy} variant="outline">Reject</Button></form> : null}</div> : null}</section></details> : null}
  </main>;
}
function Field({ label, name, className, error, type, ...props }: { label: string; name: string; className?: string; error?: string } & React.ComponentProps<typeof Input>) {
  const control = { id: name, name, required: true, "aria-invalid": !!error, "aria-describedby": error ? `${name}-error` : undefined, ...props };
  return <div className={`grid gap-2 ${className ?? ""}`}><Label htmlFor={name}>{label}</Label>{type === "password" ? <PasswordInput {...control} /> : <Input type={type} {...control} />}{error && <p id={`${name}-error`} className="text-sm text-destructive">{error}</p>}</div>;
}
