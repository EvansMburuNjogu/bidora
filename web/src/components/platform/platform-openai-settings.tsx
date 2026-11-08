"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Loader2Icon, PlugIcon, SaveIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { normalizeNumericInput } from "@/lib/numeric-input";
import { PasswordInput } from "@/components/auth/password-input";
import { PlatformSelect } from "@/components/platform/platform-select";
import { SettingsField, SettingsSwitch, settingsStateLabel } from "@/components/platform/settings-draft-controls";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { parseOpenAISetting, parseOpenAISettingPage, type OpenAISetting, type OpenAISettingPage } from "@/lib/platform-openai-contract";
import { parseIntegrationChange, type SMTPChange } from "@/lib/platform-smtp-contract";
import { networkSubmissionError, submissionErrorMessage, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Change = Omit<SMTPChange, "setting_kind"> & { setting_kind: "OPENAI" };
type Draft = { enabled: boolean; model: string; apiKey: string; timeout: string; concurrency: string; reason: string };
const initial: Draft = { enabled: false, model: "", apiKey: "", timeout: "60", concurrency: "2", reason: "Update AI configuration" };
const base = "/api/platform/integrations/openai";
const envelope = (v: unknown): { data: unknown; meta: unknown } | null => v !== null && typeof v === "object" && "data" in v && "meta" in v ? v as { data: unknown; meta: unknown } : null;
export function PlatformOpenAISettings() {
  const { resolution } = useSession();
  return <main className="app-page"><div className="app-page-header"><h1 className="type-title">OpenAI</h1><p className="type-lead text-muted-foreground">Manage your AI connection and usage limits.</p></div>{resolution.platform_context_generation !== null && resolution.capabilities.includes("platform.integration.read") ? <OpenAIConfiguration key={resolution.platform_context_generation} /> : <p className="mt-4 text-sm text-muted-foreground">You do not have access to these settings.</p>}</main>;
}
function OpenAIConfiguration() {
  const { resolution, csrfToken, refresh } = useSession(); const generation = resolution.platform_context_generation;
  const canManage = resolution.capabilities.includes("platform.integration.ai.manage"), canTest = resolution.capabilities.includes("platform.integration.ai.test"), canActivate = resolution.capabilities.includes("platform.integration.ai.activate"), canApprove = resolution.capabilities.includes("platform.governance.approve");
  const [active, setActive] = useState<OpenAISetting | null>(null), [history, setHistory] = useState<OpenAISettingPage | null>(null), [models, setModels] = useState<string[]>([]), [draft, setDraft] = useState<Draft>(initial);
  const [selected, setSelected] = useState<OpenAISetting | null>(null), [change, setChange] = useState<Change | null>(null), [reason, setReason] = useState("");
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [loadError, setLoadError] = useState(false), [pageError, setPageError] = useState(false), [touched, setTouched] = useState(false);
  const [confirmation, setConfirmation] = useState<"test" | "propose" | "approve" | "reject" | null>(null);
  const lock = useRef(false), sequence = useRef(0), keys = useRef(new Map<string, { body: string; key: string }>());
  const headers = useCallback((write = false) => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation), ...(write ? { "Content-Type": "application/json", "X-CSRF-Token": csrfToken } : {}) }), [csrfToken, generation]);
  const load = useCallback(async () => {
    const current = ++sequence.current; setLoading(true); setLoadError(false);
    try {
      const responses = await Promise.all([fetch(base, { cache: "no-store", headers: headers() }), fetch(base + "/versions", { cache: "no-store", headers: headers() }), fetch(base + "/models", { cache: "no-store", headers: headers() })]);
      if (current !== sequence.current) return;
      if (responses.some((r) => r.status === 401)) { refresh(); return; }
      const saved = responses[0].ok ? parseOpenAISetting(await responses[0].json()) : null, versions = responses[1].ok ? parseOpenAISettingPage(await responses[1].json()) : null, choices = responses[2].ok ? envelope(await responses[2].json())?.data : null;
      if (!versions || responses[0].status !== 404 && !saved || !Array.isArray(choices) || !choices.length || choices.length > 50 || choices.some((v) => typeof v !== "string" || !/^[a-z0-9.-]{1,100}$/.test(v))) { setLoadError(true); return; }
      setActive(saved); setHistory(versions); setModels(choices); setSelected(null);
      setDraft({ ...initial, enabled: saved?.configuration.enabled ?? !!saved, model: saved?.configuration.model ?? choices[0], timeout: String(saved?.configuration.timeout_seconds ?? 60), concurrency: String(saved?.configuration.max_concurrency ?? 2) }); setTouched(false);
    } catch { if (current === sequence.current) setLoadError(true); } finally { if (current === sequence.current) setLoading(false); }
  }, [headers, refresh]);
  useEffect(() => { const seq = sequence; const timer = setTimeout(() => void load(), 0); return () => { clearTimeout(timer); seq.current++; }; }, [load]);
  const errors: Partial<Record<keyof Draft, string>> = {};
  if (!models.includes(draft.model)) errors.model = "Choose a supported model.";
  if ((!active || draft.apiKey) && (draft.apiKey.length < 8 || draft.apiKey.length > 1024 || draft.apiKey.trim() !== draft.apiKey || /[\r\n\u0000]/.test(draft.apiKey))) errors.apiKey = "Enter a new API key or leave blank to retain the active key.";

  if (!/^\d+$/.test(draft.timeout) || +draft.timeout < 10 || +draft.timeout > 900) errors.timeout = "Choose 10–900 seconds.";
  if (!/^\d+$/.test(draft.concurrency) || +draft.concurrency < 1 || +draft.concurrency > 100) errors.concurrency = "Choose 1–100 concurrent requests per process.";
  if (!draft.reason.trim() || draft.reason !== draft.reason.trim() || new TextEncoder().encode(draft.reason).length > 1000) errors.reason = "Enter a reason of at most 1,000 bytes without surrounding spaces.";
  function edit<K extends keyof Draft>(key: K, value: Draft[K]) { setDraft((v) => ({ ...v, [key]: value })); }
  async function post(operation: string, payload: object): Promise<unknown | null> {
    const body = JSON.stringify(payload), previous = keys.current.get(operation); if (previous?.body !== body) keys.current.set(operation, { body, key: crypto.randomUUID() });
    const response = await fetch(base + (operation === "create" ? "" : `/${operation}`), { method: "POST", credentials: "include", cache: "no-store", headers: { ...headers(true), "Idempotency-Key": keys.current.get(operation)!.key }, body });
    if (response.status === 401) { toast.dismiss("openai-settings"); refresh(); return null; }
    if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "openai-settings" }); return null; }
    return response.json();
  }
  async function save(event: FormEvent) {
    event.preventDefault(); setTouched(true); if (!canManage || !csrfToken || loading || loadError || lock.current || Object.keys(errors).length) return;
    lock.current = true; setBusy(true);
    try {
      const result = await post("create", { enabled: draft.enabled, model: draft.model, ...(draft.apiKey ? { api_key: draft.apiKey } : {}), ...(!active ? { use_default_trust: true } : {}), timeout_seconds: +draft.timeout, max_concurrency: +draft.concurrency, reason: draft.reason });
      if (!result) return; const value = parseOpenAISetting(result); if (!value || value.state !== "DRAFT") { toast.error(unconfirmedSubmissionError, { id: "openai-settings" }); return; }
      setSelected(value); setHistory((old) => old ? { ...old, versions: [value, ...old.versions.filter((v) => v.id !== value.id)] } : null); setDraft((v) => ({ ...v, apiKey: "" })); setTouched(false); keys.current.delete("create");
      toast.success("Changes saved. Test the connection before requesting approval.", { id: "openai-settings" });
    } catch { toast.error(networkSubmissionError, { id: "openai-settings" }); } finally { lock.current = false; setBusy(false); }
  }
  async function execute() {
    const operation = confirmation, version = selected, current = change;
    if (!operation || lock.current || !csrfToken || operation === "test" && (!version || !canTest) || operation === "propose" && (!version || !canActivate) || (operation === "approve" || operation === "reject") && (!current || !canApprove)) return;
    lock.current = true; setBusy(true); if (operation === "test") toast.loading("Checking the selected AI configuration…", { id: "openai-settings" });
    try {
      const payload = operation === "test" ? { version_id: version!.id, version: version!.version, reason: "Explicitly authorised AI connection test", confirm_billable_test: true } : operation === "propose" ? { version_id: version!.id, version: version!.version, action: version!.state === "SUPERSEDED" ? "ROLLBACK" : "ACTIVATE", reason: "Activate reviewed AI configuration" } : { change_id: current!.id, version: current!.version, reason };
      const raw = await post(operation, payload); if (!raw) return;
      if (operation === "test") {
        const value = parseOpenAISetting(raw); if (!value || value.id !== version!.id) { toast.error(unconfirmedSubmissionError, { id: "openai-settings" }); return; }
        setSelected(value); setHistory((old) => old ? { ...old, versions: old.versions.map((v) => v.id === value.id ? value : v) } : null);
        if (value.test_status === "PASSED") toast.success(value.configuration.enabled === false ? "Disabled configuration verified. No provider request was sent." : "Structured response test passed. This does not activate AI.", { id: "openai-settings" }); else toast.error("Connection test failed. Review the configuration before retrying.", { id: "openai-settings" });
      } else if (operation === "propose" || operation === "reject") {
        const value = parseIntegrationChange(raw, "OPENAI", operation === "reject" ? current!.id : undefined); if (!value) { toast.error(unconfirmedSubmissionError, { id: "openai-settings" }); return; } setChange(value);
        if (operation === "propose") { setSelected(null); toast.info(value.state === "PENDING_APPROVAL" ? "Activation requested. A different Owner must approve." : "This request has already been processed. Reload the current setting.", { id: "openai-settings" }); } else if (value.state === "REJECTED") { toast.success("Activation request rejected.", { id: "openai-settings" }); }
      } else {
        const env = envelope(raw), data = env?.data as { change?: unknown; active_version?: unknown } | null; const value = data ? parseIntegrationChange({ data: data.change, meta: env?.meta }, "OPENAI", current!.id) : null; const activated = data ? parseOpenAISetting({ data: data.active_version, meta: env?.meta }) : null;
        if (!value || value.state !== "APPROVED_EXECUTED" || !activated || activated.id !== value.target_version_id) { toast.error(unconfirmedSubmissionError, { id: "openai-settings" }); return; }
        setChange(value); setActive(activated); toast.success(activated.configuration.enabled === false ? "AI is disabled by the active configuration." : "AI configuration activated.", { id: "openai-settings" });
      }
      keys.current.delete(operation); setConfirmation(null);
    } catch { toast.error(networkSubmissionError, { id: "openai-settings" }); } finally { lock.current = false; setBusy(false); }
  }
  async function viewVersion(version: OpenAISetting) {
    if (lock.current) return;
    setSelected(version); setChange(null); setReason("");
    if (!canApprove || !version.pending_change_id) return;
    lock.current = true; setBusy(true);
    try {
      const response = await fetch(base + `/changes/${version.pending_change_id}`, { cache: "no-store", headers: headers() });
      if (response.status === 401) { refresh(); return; }
      const value = response.ok ? parseIntegrationChange(await response.json(), "OPENAI", version.pending_change_id) : null;
      if (!value || value.target_version_id !== version.id) { toast.error("The saved change could not be loaded. Refresh settings and try again.", { id: "openai-settings" }); return; }
      setChange(value);
    } catch { toast.error(networkSubmissionError, { id: "openai-settings" }); }
    finally { lock.current = false; setBusy(false); }
  }
  async function more() { if (!history?.next_cursor || lock.current) return; lock.current = true; setBusy(true); try { const response = await fetch(base + `/versions?cursor=${encodeURIComponent(history.next_cursor)}`, { cache: "no-store", headers: headers() }); if (response.status === 401) { refresh(); return; } const value = response.ok ? parseOpenAISettingPage(await response.json()) : null; if (!value) { setPageError(true); return; } setHistory((old) => old ? { versions: [...old.versions, ...value.versions.filter((v) => !old.versions.some((row) => row.id === v.id))], next_cursor: value.next_cursor } : null); setPageError(false); } catch { setPageError(true); } finally { lock.current = false; setBusy(false); } }
  const validDecisionReason = reason.trim() === reason && reason.length > 0 && new TextEncoder().encode(reason).length <= 1000;
  return <div className="max-w-4xl">
{loading&&<p role="status" className="py-4 text-sm text-muted-foreground">Loading settings…</p>}
{loadError&&<p role="alert" className="py-4 text-sm text-destructive">Settings could not be loaded. <Button variant="outline" size="sm" onClick={()=>void load()}>Retry</Button></p>}
<form onSubmit={event=>void save(event)} noValidate autoComplete="off" className="space-y-5 py-5">
<fieldset disabled={!canManage||busy||loading||loadError} className="space-y-5">
<SettingsSwitch id="openai-enabled" label="Enable AI" checked={draft.enabled} onChange={v=>edit("enabled",v)}/>
<div className="grid gap-5 sm:grid-cols-2">
<SettingsField id="openai-key" label="API Key" hint={active?"Leave blank to keep the current key.":undefined}><PasswordInput id="openai-key" autoComplete="new-password" value={draft.apiKey} onChange={e=>edit("apiKey",e.target.value)} maxLength={1024} aria-invalid={touched&&!!errors.apiKey}/>{touched&&errors.apiKey&&<p role="alert" className="text-sm text-destructive">{errors.apiKey}</p>}</SettingsField>
<SettingsField id="openai-model" label="Model"><PlatformSelect id="openai-model" value={draft.model} onChange={e=>edit("model",e.target.value)}>{!draft.model&&<option value="">Choose a model</option>}{models.map(model=><option key={model} value={model}>{model}</option>)}</PlatformSelect>{touched&&errors.model&&<p role="alert" className="text-sm text-destructive">{errors.model}</p>}</SettingsField>
<SettingsField id="openai-concurrency" label="Concurrent requests" hint="Other usage limits follow your subscription."><Input id="openai-concurrency" type="number" min={1} max={100} value={draft.concurrency} onChange={e=>edit("concurrency",normalizeNumericInput(e.target.value))} aria-invalid={touched&&!!errors.concurrency}/>{touched&&errors.concurrency&&<p role="alert" className="text-sm text-destructive">{errors.concurrency}</p>}</SettingsField>
</div></fieldset>
<p className="text-xs text-muted-foreground">Changes need a successful test and another Owner’s approval before use.</p>
<div className="flex flex-wrap items-center justify-end gap-3 border-t pt-5">
<Button type="button" variant="outline" disabled={busy||loading} onClick={()=>{setDraft({...initial,enabled:active?.configuration.enabled??false,model:active?.configuration.model??models[0]??"",timeout:String(active?.configuration.timeout_seconds??60),concurrency:String(active?.configuration.max_concurrency??2)});setTouched(false);keys.current.delete("create");}}>Cancel</Button>
<Button type="button" variant="outline" disabled={busy||!canTest||!selected||!["DRAFT","TEST_PASSED","TEST_FAILED","SUPERSEDED"].includes(selected.state)} onClick={()=>setConfirmation("test")}><PlugIcon aria-hidden="true"/>Test Connection</Button>
<Button type="submit" disabled={!canManage||!csrfToken||busy||loading||loadError} aria-busy={busy}>{busy?<Loader2Icon aria-hidden="true" className="animate-spin"/>:<SaveIcon aria-hidden="true"/>}Save Changes</Button>
</div></form>
<p className="py-3 text-sm"><span className="font-medium">Connection Status: </span><span className="text-muted-foreground">{active?active.configuration.enabled?"Enabled":"Disabled":"Not configured"}</span></p>
{selected&&<section aria-label="Review selected AI version" className="space-y-3 border-t py-4 text-sm"><h2 className="font-medium">Saved changes</h2><p>{settingsStateLabel(selected.state)} · {settingsStateLabel(selected.test_status)}</p>{canActivate&&selected.test_status==="PASSED"&&["TEST_PASSED","SUPERSEDED"].includes(selected.state)&&<Button disabled={busy} onClick={()=>setConfirmation("propose")}>Request {selected.state==="SUPERSEDED"?"Rollback":"Approval"}</Button>}</section>}
{change&&<section className="space-y-3 border-t py-4 text-sm"><h2 className="font-medium">{settingsStateLabel(change.state)}</h2>{canApprove&&change.state==="PENDING_APPROVAL"&&<><label className="grid gap-2">Reason<Input value={reason} maxLength={1000} onChange={e=>setReason(e.target.value)} aria-invalid={!!reason&&!validDecisionReason}/></label><div className="flex gap-3"><Button disabled={busy||!validDecisionReason} onClick={()=>setConfirmation("approve")}>Approve</Button><Button variant="outline" disabled={busy||!validDecisionReason} onClick={()=>setConfirmation("reject")}>Reject</Button></div></>}</section>}
<details className="border-t py-4"><summary className="cursor-pointer text-sm font-medium">Previous settings</summary><ul className="mt-4 divide-y text-sm">{history?.versions.map(v=><li key={v.id} className="flex flex-wrap items-center justify-between gap-3 py-3"><div><p>{v.configuration.model}</p><p className="text-xs text-muted-foreground">{settingsStateLabel(v.state)} · {new Date(v.created_at).toLocaleString()}</p></div><Button type="button" variant="outline" size="sm" disabled={busy} onClick={()=>void viewVersion(v)}>View</Button></li>)}</ul>{history&&!history.versions.length&&<p className="py-3 text-sm text-muted-foreground">No previous settings.</p>}{pageError&&<p role="alert" className="text-sm text-destructive">Could not load more settings.</p>}{history?.next_cursor&&<Button variant="outline" disabled={busy} onClick={()=>void more()}>Load more</Button>}</details>
<AlertDialog open={confirmation!==null} onOpenChange={open=>{if(!lock.current&&!open)setConfirmation(null);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{confirmation==="test"?"Test Connection?":confirmation==="propose"?"Request approval?":confirmation==="approve"?"Approve changes?":"Reject changes?"}</AlertDialogTitle><AlertDialogDescription>{confirmation==="test"?selected?.configuration.enabled===false?"No provider request is made while disabled.":"This uses a small paid AI request. No tenant data is sent.":confirmation==="propose"?"Another Owner must approve before these settings take effect.":"The current settings change only after approval."}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button disabled={busy} aria-busy={busy} onClick={()=>void execute()}>{busy&&<Loader2Icon aria-hidden="true" className="animate-spin"/>}Confirm</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
</div>;
}
