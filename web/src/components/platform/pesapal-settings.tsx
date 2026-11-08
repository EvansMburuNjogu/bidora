"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Loader2Icon, PlugIcon, SaveIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PasswordInput } from "@/components/auth/password-input";
import { PlatformSelect } from "@/components/platform/platform-select";
import { SettingsField, SettingsSwitch, settingsStateLabel } from "@/components/platform/settings-draft-controls";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { parsePesaPalSetting, parsePesaPalSettingPage, type PesaPalSetting, type PesaPalSettingPage } from "@/lib/platform-pesapal-contract";
import { parseIntegrationChange, type SMTPChange } from "@/lib/platform-smtp-contract";
import { networkSubmissionError, submissionErrorMessage, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Change = Omit<SMTPChange, "setting_kind"> & { setting_kind: "PESAPAL" };
type Draft = {enabled:boolean;environment:"SANDBOX"|"LIVE";merchant:string;notification:string;key:string;secret:string;retain:boolean;reason:string};
const initial:Draft={enabled:false,environment:"SANDBOX",merchant:"",notification:"",key:"",secret:"",retain:false,reason:"Update payment configuration"};
const base = "/api/platform/integrations/pesapal";
const envelope = (v: unknown): { data: unknown; meta: unknown } | null => v !== null && typeof v === "object" && "data" in v && "meta" in v ? v as { data: unknown; meta: unknown } : null;
export function PesaPalSettings() {
  const { resolution } = useSession();
  return <main className="app-page"><div className="app-page-header"><h1 className="type-title">PesaPal</h1><p className="type-lead text-muted-foreground">Manage your payment connection.</p></div>{resolution.platform_context_generation !== null && resolution.capabilities.includes("platform.integration.read") ? <PesaPalConfiguration key={resolution.platform_context_generation} /> : <p className="mt-4 text-sm text-muted-foreground">You do not have access to these settings.</p>}</main>;
}
function PesaPalConfiguration() {
  const { resolution, csrfToken, refresh } = useSession(); const generation = resolution.platform_context_generation;
  const canManage = resolution.capabilities.includes("platform.integration.pesapal.manage"), canTest = resolution.capabilities.includes("platform.integration.pesapal.test"), canActivate = resolution.capabilities.includes("platform.integration.pesapal.activate"), canApprove = resolution.capabilities.includes("platform.governance.approve");
  const [active, setActive] = useState<PesaPalSetting | null>(null), [history, setHistory] = useState<PesaPalSettingPage | null>(null), [draft, setDraft] = useState<Draft>(initial);
  const [selected, setSelected] = useState<PesaPalSetting | null>(null), [change, setChange] = useState<Change | null>(null), [reason, setReason] = useState("");
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [loadError, setLoadError] = useState(false), [pageError, setPageError] = useState(false), [touched, setTouched] = useState(false);
  const [confirmation, setConfirmation] = useState<"test" | "propose" | "approve" | "reject" | null>(null);
  const lock = useRef(false), sequence = useRef(0), keys = useRef(new Map<string, { body: string; key: string }>());
  const headers = useCallback((write = false) => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation), ...(write ? { "Content-Type": "application/json", "X-CSRF-Token": csrfToken } : {}) }), [csrfToken, generation]);
  const load = useCallback(async () => {
    const current = ++sequence.current; setLoading(true); setLoadError(false);
    try {
      const responses = await Promise.all([fetch(base, { cache: "no-store", headers: headers() }), fetch(base + "/versions", { cache: "no-store", headers: headers() })]);
      if (current !== sequence.current) return;
      if (responses.some((r) => r.status === 401)) { refresh(); return; }
      const saved = responses[0].ok ? parsePesaPalSetting(await responses[0].json()) : null, versions = responses[1].ok ? parsePesaPalSettingPage(await responses[1].json()) : null;
      if (!versions || responses[0].status !== 404 && !saved) { setLoadError(true); return; }
      setActive(saved); setHistory(versions); setSelected(null);
      setDraft({...initial,enabled:saved?.configuration.enabled??false,environment:saved?.configuration.environment??"SANDBOX",merchant:saved?.configuration.merchant_name??"",notification:saved?.configuration.notification_id??"",retain:saved?.configuration.credential_reference_configured??false});setTouched(false);
    } catch { if (current === sequence.current) setLoadError(true); } finally { if (current === sequence.current) setLoading(false); }
  }, [headers, refresh]);
  useEffect(() => { const seq = sequence; const timer = setTimeout(() => void load(), 0); return () => { clearTimeout(timer); seq.current++; }; }, [load]);
  const retainCredentials = !!active?.configuration.credential_reference_configured && !draft.key && !draft.secret;
  const errors: Partial<Record<keyof Draft, string>> = {};
  if(!draft.merchant.trim()||draft.merchant.trim()!==draft.merchant||new TextEncoder().encode(draft.merchant).length>120||/[\r\n\u0000]/.test(draft.merchant))errors.merchant="Enter a merchant label of at most 120 bytes.";
  if(!retainCredentials&&(draft.enabled||draft.key||draft.secret)){for(const k of ["key","secret"] as const){if(draft[k].length<8||draft[k].length>512||draft[k].trim()!==draft[k]||/[\r\n\u0000]/.test(draft[k]))errors[k]="Provide both replacement credentials (8–512 characters each).";}}
  if(!draft.reason.trim()||draft.reason!==draft.reason.trim()||new TextEncoder().encode(draft.reason).length>1000)errors.reason="Enter a reason of at most 1,000 bytes.";
  function edit<K extends keyof Draft>(key: K, value: Draft[K]) { setDraft((v) => ({ ...v, [key]: value })); }
  async function post(operation: string, payload: object): Promise<unknown | null> {
    const body = JSON.stringify(payload), previous = keys.current.get(operation); if (previous?.body !== body) keys.current.set(operation, { body, key: crypto.randomUUID() });
    const response = await fetch(base + (operation === "create" ? "" : `/${operation}`), { method: "POST", credentials: "include", cache: "no-store", headers: { ...headers(true), "Idempotency-Key": keys.current.get(operation)!.key }, body });
    if (response.status === 401) { toast.dismiss("pesapal-settings"); refresh(); return null; }
    if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "pesapal-settings" }); return null; }
    return response.json();
  }
  async function save(event: FormEvent) {
    event.preventDefault(); setTouched(true); if (!canManage || !csrfToken || loading || loadError || lock.current || Object.keys(errors).length) return;
    lock.current = true; setBusy(true);
    try {
      const result = await post("create", {enabled:draft.enabled,environment:draft.environment,merchant_name:draft.merchant,notification_id:draft.notification,retain_active_credentials:retainCredentials,...(!retainCredentials&&draft.key?{consumer_key:draft.key}:{}),...(!retainCredentials&&draft.secret?{consumer_secret:draft.secret}:{}),reason:draft.reason});
      if (!result) return; const value = parsePesaPalSetting(result); if (!value || value.state !== "DRAFT") { toast.error(unconfirmedSubmissionError, { id: "pesapal-settings" }); return; }
      setSelected(value); setHistory((old) => old ? { ...old, versions: [value, ...old.versions.filter((v) => v.id !== value.id)] } : null); setDraft((v) => ({ ...v, key: "", secret: "" })); setTouched(false); keys.current.delete("create");
      toast.success("Changes saved. Test the connection before requesting approval.", { id: "pesapal-settings" });
    } catch { toast.error(networkSubmissionError, { id: "pesapal-settings" }); } finally { lock.current = false; setBusy(false); }
  }
  async function execute() {
    const operation = confirmation, version = selected, current = change;
    if (!operation || lock.current || !csrfToken || operation === "test" && (!version || !canTest) || operation === "propose" && (!version || !canActivate) || (operation === "approve" || operation === "reject") && (!current || !canApprove)) return;
    lock.current = true; setBusy(true); if (operation === "test") toast.loading("Checking the selected payment configuration…", { id: "pesapal-settings" });
    try {
      const payload = operation === "test" ? { version_id: version!.id, version: version!.version, reason: "Explicitly authorised provider authentication test" } : operation === "propose" ? { version_id: version!.id, version: version!.version, action: version!.state === "SUPERSEDED" ? "ROLLBACK" : "ACTIVATE", reason: "Activate reviewed payment configuration" } : { change_id: current!.id, version: current!.version, reason };
      const raw = await post(operation, payload); if (!raw) return;
      if (operation === "test") {
        const value = parsePesaPalSetting(raw); if (!value || value.id !== version!.id) { toast.error(unconfirmedSubmissionError, { id: "pesapal-settings" }); return; }
        setSelected(value); setHistory((old) => old ? { ...old, versions: old.versions.map((v) => v.id === value.id ? value : v) } : null);
        if (value.test_status === "PASSED") toast.success(value.configuration.enabled === false ? "Disabled configuration verified. No provider request was sent." : "Authentication verified only. No order, payment or callback registration was created.", { id: "pesapal-settings" }); else toast.error("Connection test failed. Review the configuration before retrying.", { id: "pesapal-settings" });
      } else if (operation === "propose" || operation === "reject") {
        const value = parseIntegrationChange(raw, "PESAPAL", operation === "reject" ? current!.id : undefined); if (!value) { toast.error(unconfirmedSubmissionError, { id: "pesapal-settings" }); return; } setChange(value);
        if (operation === "propose") { setSelected(null); toast.info(value.state === "PENDING_APPROVAL" ? "Activation requested. A different Owner must approve." : "This request has already been processed. Reload the current setting.", { id: "pesapal-settings" }); } else if (value.state === "REJECTED") { toast.success("Activation request rejected.", { id: "pesapal-settings" }); }
      } else {
        const env = envelope(raw), data = env?.data as { change?: unknown; active_version?: unknown } | null; const value = data ? parseIntegrationChange({ data: data.change, meta: env?.meta }, "PESAPAL", current!.id) : null; const activated = data ? parsePesaPalSetting({ data: data.active_version, meta: env?.meta }) : null;
        if (!value || value.state !== "APPROVED_EXECUTED" || !activated || activated.id !== value.target_version_id) { toast.error(unconfirmedSubmissionError, { id: "pesapal-settings" }); return; }
        setChange(value); setActive(activated); toast.success(activated.configuration.enabled === false ? "New payment orders are disabled by this configuration." : "Payment configuration activated. No payment was created or settled.", { id: "pesapal-settings" });
      }
      keys.current.delete(operation); setConfirmation(null);
    } catch { toast.error(networkSubmissionError, { id: "pesapal-settings" }); } finally { lock.current = false; setBusy(false); }
  }
  async function viewVersion(version: PesaPalSetting) {
    if (lock.current) return;
    setSelected(version); setChange(null); setReason("");
    if (!canApprove || !version.pending_change_id) return;
    lock.current = true; setBusy(true);
    try {
      const response = await fetch(base + `/changes/${version.pending_change_id}`, { cache: "no-store", headers: headers() });
      if (response.status === 401) { refresh(); return; }
      const value = response.ok ? parseIntegrationChange(await response.json(), "PESAPAL", version.pending_change_id) : null;
      if (!value || value.target_version_id !== version.id) { toast.error("The saved change could not be loaded. Refresh settings and try again.", { id: "pesapal-settings" }); return; }
      setChange(value);
    } catch { toast.error(networkSubmissionError, { id: "pesapal-settings" }); }
    finally { lock.current = false; setBusy(false); }
  }
  async function more() { if (!history?.next_cursor || lock.current) return; lock.current = true; setBusy(true); try { const response = await fetch(base + `/versions?cursor=${encodeURIComponent(history.next_cursor)}`, { cache: "no-store", headers: headers() }); if (response.status === 401) { refresh(); return; } const value = response.ok ? parsePesaPalSettingPage(await response.json()) : null; if (!value) { setPageError(true); return; } setHistory((old) => old ? { versions: [...old.versions, ...value.versions.filter((v) => !old.versions.some((row) => row.id === v.id))], next_cursor: value.next_cursor } : null); setPageError(false); } catch { setPageError(true); } finally { lock.current = false; setBusy(false); } }
  const validDecisionReason = reason.trim() === reason && reason.length > 0 && new TextEncoder().encode(reason).length <= 1000;
  return <div className="max-w-4xl">
{loading&&<p role="status" className="py-4 text-sm text-muted-foreground">Loading settings…</p>}
{loadError&&<p role="alert" className="py-4 text-sm text-destructive">Settings could not be loaded. <Button variant="outline" size="sm" onClick={()=>void load()}>Retry</Button></p>}
<form onSubmit={event=>void save(event)} noValidate autoComplete="off" className="space-y-5 py-5">
<fieldset disabled={!canManage||busy||loading||loadError} className="space-y-5">
<SettingsSwitch id="pesapal-enabled" label="Enable payments" checked={draft.enabled} onChange={v=>edit("enabled",v)}/>
<div className="grid gap-5 sm:grid-cols-2">
<SettingsField id="pesapal-environment" label="Environment"><PlatformSelect id="pesapal-environment" value={draft.environment} onChange={e=>edit("environment",e.target.value as Draft["environment"])}><option value="SANDBOX">Sandbox</option><option value="LIVE">Live</option></PlatformSelect></SettingsField>
<SettingsField id="pesapal-merchant" label="Merchant Name"><Input id="pesapal-merchant" value={draft.merchant} maxLength={120} onChange={e=>edit("merchant",e.target.value)} aria-invalid={touched&&!!errors.merchant}/>{touched&&errors.merchant&&<p role="alert" className="text-sm text-destructive">{errors.merchant}</p>}</SettingsField>
{([["key","Consumer Key"],["secret","Consumer Secret"]] as const).map(([field,label])=><SettingsField key={field} id={`pesapal-${field}`} label={label} hint={active?.configuration.credential_reference_configured?"Leave both blank to keep current credentials.":undefined}><PasswordInput id={`pesapal-${field}`} autoComplete="new-password" value={draft[field]} maxLength={512} onChange={e=>{edit(field,e.target.value);edit("retain",false);}} aria-invalid={touched&&!!errors[field]}/>{touched&&errors[field]&&<p role="alert" className="text-sm text-destructive">{errors[field]}</p>}</SettingsField>)}
</div></fieldset>
<p className="text-xs text-muted-foreground">Changes need a successful test and another Owner’s approval before use.</p>
<div className="flex flex-wrap items-center justify-end gap-3 border-t pt-5">
<Button type="button" variant="outline" disabled={busy||loading} onClick={()=>{setDraft({...initial,enabled:active?.configuration.enabled??false,environment:active?.configuration.environment??"SANDBOX",merchant:active?.configuration.merchant_name??"",notification:active?.configuration.notification_id??"",retain:active?.configuration.credential_reference_configured??false});setTouched(false);keys.current.delete("create");}}>Cancel</Button>
<Button type="button" variant="outline" disabled={busy||!canTest||!selected||!["DRAFT","TEST_PASSED","TEST_FAILED","SUPERSEDED"].includes(selected.state)} onClick={()=>setConfirmation("test")}><PlugIcon aria-hidden="true"/>Test Connection</Button>
<Button type="submit" disabled={!canManage||!csrfToken||busy||loading||loadError} aria-busy={busy}>{busy?<Loader2Icon aria-hidden="true" className="animate-spin"/>:<SaveIcon aria-hidden="true"/>}Save Changes</Button>
</div></form>
<p className="py-3 text-sm"><span className="font-medium">Connection Status: </span><span className="text-muted-foreground">{active?active.configuration.enabled?"Enabled":"Disabled":"Not configured"}</span></p>
{selected&&<section aria-label="Review selected payment version" className="space-y-3 border-t py-4 text-sm"><h2 className="font-medium">Saved changes</h2><p>{settingsStateLabel(selected.state)} · {settingsStateLabel(selected.test_status)}</p>{canActivate&&selected.test_status==="PASSED"&&["TEST_PASSED","SUPERSEDED"].includes(selected.state)&&<Button disabled={busy} onClick={()=>setConfirmation("propose")}>Request {selected.state==="SUPERSEDED"?"Rollback":"Approval"}</Button>}</section>}
{change&&<section className="space-y-3 border-t py-4 text-sm"><h2 className="font-medium">{settingsStateLabel(change.state)}</h2>{canApprove&&change.state==="PENDING_APPROVAL"&&<><label className="grid gap-2">Reason<Input value={reason} maxLength={1000} onChange={e=>setReason(e.target.value)} aria-invalid={!!reason&&!validDecisionReason}/></label><div className="flex gap-3"><Button disabled={busy||!validDecisionReason} onClick={()=>setConfirmation("approve")}>Approve</Button><Button variant="outline" disabled={busy||!validDecisionReason} onClick={()=>setConfirmation("reject")}>Reject</Button></div></>}</section>}
<details className="border-t py-4"><summary className="cursor-pointer text-sm font-medium">Previous settings</summary><ul className="mt-4 divide-y text-sm">{history?.versions.map(v=><li key={v.id} className="flex flex-wrap items-center justify-between gap-3 py-3"><div><p>{`${v.configuration.merchant_name} · ${v.configuration.environment}`}</p><p className="text-xs text-muted-foreground">{settingsStateLabel(v.state)} · {new Date(v.created_at).toLocaleString()}</p></div><Button type="button" variant="outline" size="sm" disabled={busy} onClick={()=>void viewVersion(v)}>View</Button></li>)}</ul>{history&&!history.versions.length&&<p className="py-3 text-sm text-muted-foreground">No previous settings.</p>}{pageError&&<p role="alert" className="text-sm text-destructive">Could not load more settings.</p>}{history?.next_cursor&&<Button variant="outline" disabled={busy} onClick={()=>void more()}>Load more</Button>}</details>
<AlertDialog open={confirmation!==null} onOpenChange={open=>{if(!lock.current&&!open)setConfirmation(null);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{confirmation==="test"?"Test Connection?":confirmation==="propose"?"Request approval?":confirmation==="approve"?"Approve changes?":"Reject changes?"}</AlertDialogTitle><AlertDialogDescription>{confirmation==="test"?selected?.configuration.enabled===false?"No provider request is made while disabled.":"This checks credentials only. No payment is created or moved.":confirmation==="propose"?"Another Owner must approve before these settings take effect.":"The current settings change only after approval."}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button disabled={busy} aria-busy={busy} onClick={()=>void execute()}>{busy&&<Loader2Icon aria-hidden="true" className="animate-spin"/>}Confirm</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
</div>;
}
