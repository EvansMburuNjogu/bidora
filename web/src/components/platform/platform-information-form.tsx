"use client";

import { usePlatformIdentity } from "@/components/session/platform-identity-provider";
import { parseSharedCountries, type SharedCountries } from "@/lib/platform-regional-contract";
import { languageLabel } from "@/lib/country-reference";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Loader2Icon, SaveIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformInformation, type PlatformInformation } from "@/lib/platform-information-contract";

import { toast } from "sonner";
import { networkSubmissionError, submissionErrorMessage, submissionFieldErrors, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Draft = { platform_name: string; website: string; support_email: string; contact_phone: string };
const defaults: Draft = { platform_name: "", website: "", support_email: "", contact_phone: "" };
const fromServer = (value: PlatformInformation): Draft => ({ ...defaults, platform_name: value.platform_name, website: value.website ?? "", support_email: value.support_email ?? "", contact_phone: value.contact_phone });
function errorsFor(draft: Draft): Partial<Record<keyof Draft, string>> {
  const errors: Partial<Record<keyof Draft, string>> = {};
  if (!draft.platform_name.trim() || draft.platform_name.trim() !== draft.platform_name || /[\u0000-\u001f\u007f-\u009f]/.test(draft.platform_name) || Array.from(draft.platform_name).length > 120) errors.platform_name = "Enter a platform name of at most 120 characters.";
  try {
    const url = new URL(draft.website);
    if (url.protocol !== "https:" || (!url.hostname.includes(".") || /^\d+\.\d+\.\d+\.\d+$/.test(url.hostname) || url.hostname.endsWith(".local")) || url.port || url.username || url.password || url.search || url.hash || draft.website.includes("?") || draft.website.includes("#") || draft.website !== url.href && draft.website !== url.origin && draft.website !== url.origin + url.pathname || draft.website.length > 512) errors.website = "Enter an HTTPS website address without a port, query or fragment.";
  } catch { errors.website = "Enter a valid HTTPS website address."; }
  const [local, host, extra] = draft.support_email.split("@");
  if (extra !== undefined || !local || local.length > 64 || local.startsWith(".") || local.endsWith(".") || local.includes("..") || !/^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+$/.test(local) || !host || !host.includes(".") || host.split(".").some((label) => !label || label.length > 63 || !/^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$/.test(label)) || draft.support_email.length > 254) errors.support_email = "Enter a valid support email address.";
  if (draft.contact_phone && (!/^\+?[0-9 ()-]{7,32}$/.test(draft.contact_phone) || (draft.contact_phone.match(/\d/g) ?? []).length < 7)) errors.contact_phone = "Enter a valid phone number or leave this blank.";
  return errors;
}
export function PlatformInformationForm() {
  const { resolution, csrfToken, refresh } = useSession();
  const identity = usePlatformIdentity();
  const [regional, setRegional] = useState<SharedCountries|null>(null);
  const regionalDefault = regional?.countries.find(country=>country.country_code===regional.default_country);
  useEffect(()=>{const controller=new AbortController();void fetch("/api/session/countries",{cache:"no-store",signal:controller.signal}).then(async response=>{const value=response.ok?parseSharedCountries(await response.json()):null;if(!controller.signal.aborted)setRegional(value);}).catch(()=>{if(!controller.signal.aborted)setRegional(null);});return()=>controller.abort();},[resolution.platform_context_generation]);
  const generation = resolution.platform_context_generation;
  const allowed = resolution.capabilities.includes("platform.admin.read");
  const canSave = resolution.capabilities.includes("platform.governance.approve");
  const [loaded, setLoaded] = useState<{ value: PlatformInformation; generation: number } | null>(null);
  const current = loaded?.generation === generation && allowed ? loaded.value : null;
  const [draft, setDraft] = useState<Draft>(defaults);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ text: string; kind: "error" | "success" } | null>(null);
  const [touched, setTouched] = useState<Partial<Record<keyof Draft, boolean>>>({});
  const submitting = useRef(false);
  const [serverErrors, setServerErrors] = useState<Partial<Record<keyof Draft, string>>>({});
  const sequence = useRef(0);
  const load = useCallback(async () => {
    const request = ++sequence.current;
    setLoading(true); setBusy(false); setLoaded(null); setMessage(null);
    if (!generation || !allowed) { setLoading(false); setMessage({ kind: "error", text: "You cannot view platform information." }); return; }
    try {
      const response = await fetch("/api/platform/information", { credentials: "include", cache: "no-store", signal: AbortSignal.timeout(10_000), headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (sequence.current !== request) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const value = response.ok ? parsePlatformInformation(await response.json()) : null;
      if (sequence.current !== request) return;
      if (!value) { setMessage({ kind: "error", text: response.status === 403 ? "Recent verification is required to view these settings." : "Saved details unavailable." }); return; }
      setLoaded({ value, generation }); setDraft(fromServer(value)); setServerErrors({}); setTouched({});
    } catch { if (sequence.current === request) setMessage({ kind: "error", text: "Saved details unavailable." }); }
    finally { if (sequence.current === request) setLoading(false); }
  }, [allowed, generation, refresh]);
  useEffect(() => { const requestSequence = sequence; const timer = window.setTimeout(() => void load(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [load]);
  const errors = { ...errorsFor(draft), ...serverErrors };
  const baseline = current ? fromServer(current) : defaults;
  const changed = !!current && (Object.keys(defaults) as Array<keyof Draft>).some((key) => draft[key] !== baseline[key]);
  const readyToSave = !!current && canSave && !busy && changed && Object.keys(errors).length === 0 && !!csrfToken;
  function change(key: keyof Draft, value: string) { setServerErrors({}); setDraft((old) => ({ ...old, [key]: value })); if (current) setMessage(null); }
  function fieldError(key: keyof Draft) { return touched[key] ? errors[key] : undefined; }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setTouched({ platform_name: true, website: true, support_email: true, contact_phone: true });
    if (!readyToSave || !current || !generation || submitting.current) return;
    const request = sequence.current;
    submitting.current = true; setBusy(true); setMessage(null); setServerErrors({});
    try {
      const response = await fetch("/api/platform/information", { method: "PUT", credentials: "include", cache: "no-store", signal: AbortSignal.timeout(10_000), headers: { Accept: "application/json", "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": String(generation), "X-CSRF-Token": csrfToken, "If-Match": `"${current.version}"`, "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ platform_name: draft.platform_name, contact_phone: draft.contact_phone, website: draft.website, support_email: draft.support_email }) });
      if (sequence.current !== request) return;
      if (response.status === 401) { refresh(); return; }
      if (response.status === 409) { setMessage({ kind: "error", text: "These settings have changed. Reload saved details before saving again." }); return; }
      if (!response.ok) { const fields = await submissionFieldErrors(response, ["platform_name", "website", "support_email", "contact_phone"] as const); if (sequence.current !== request) return; setServerErrors(fields); if (Object.keys(fields).length) toast.error("Check the highlighted fields.", { id: "platform-information" }); else toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "platform-information" }); return; }
      const saved = parsePlatformInformation(await response.json());
      if (sequence.current !== request) return;
      if (!saved || saved.platform_name !== draft.platform_name || saved.contact_phone !== draft.contact_phone || saved.website !== draft.website || saved.support_email !== draft.support_email) { toast.error(unconfirmedSubmissionError, { id: "platform-information" }); return; }
      // The successful write is authoritative even if the follow-up read fails.
      // Retain its version so a later retry never submits the old If-Match value.
      setLoaded({ value: saved, generation }); setDraft(fromServer(saved)); setTouched({}); identity.refresh();
      const confirmed = await fetch("/api/platform/information", { credentials: "include", cache: "no-store", signal: AbortSignal.timeout(10_000), headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (sequence.current !== request) return;
      if (confirmed.status === 401) { refresh(); return; }
      const persisted = confirmed.ok ? parsePlatformInformation(await confirmed.json()) : null;
      if (sequence.current !== request) return;
      if (!persisted || persisted.version !== saved.version || persisted.platform_name !== saved.platform_name || persisted.contact_phone !== saved.contact_phone || persisted.website !== saved.website || persisted.support_email !== saved.support_email) { setMessage({ kind: "error", text: "Changes were accepted, but saved details could not be confirmed. Reload saved details before editing again." }); toast.error(unconfirmedSubmissionError, { id: "platform-information" }); return; }
      setLoaded({ value: persisted, generation }); setDraft(fromServer(persisted)); setTouched({}); toast.success("Changes saved successfully.", { id: "platform-information" });
    } catch { if (sequence.current === request) { setMessage({ kind: "error", text: networkSubmissionError }); toast.error(networkSubmissionError, { id: "platform-information" }); } }
    finally { submitting.current = false; if (sequence.current === request) setBusy(false); }
  }
  return <main className="app-page"><div className="app-page-header mb-7"><h1 className="type-title">Platform Information</h1><p className="type-lead text-muted-foreground">Manage Bidora&apos;s general information and preferences.</p></div>
    {loading ? <div role="status" aria-label="Loading platform information" className="max-w-4xl space-y-3"><Skeleton className="h-56 w-full rounded-xl" /><Skeleton className="h-44 w-full rounded-xl" /></div> : null}
    {!loading && !current ? <div className="max-w-4xl space-y-3">
      <p role="alert" className="text-sm text-destructive">{message?.text ?? "Saved details unavailable."}</p>
      {allowed && generation ? <Button type="button" variant="outline" onClick={() => void load()}>Retry loading saved details</Button> : null}
    </div> : null}
    {!loading && current ? <form onSubmit={(event) => void save(event)} noValidate className="max-w-4xl" aria-label="Platform information">
      <section aria-labelledby="general-heading" className="space-y-6 py-5 sm:py-7"><div><h2 id="general-heading" className="text-base font-semibold">General Information</h2><p className="mt-1 text-sm text-muted-foreground">Your platform identity and support contact.</p></div><div className="grid gap-5 sm:grid-cols-2">
        <div className="grid min-w-0 gap-2"><div><Label htmlFor="platform-name">Platform Name <span aria-hidden="true" className="text-destructive">*</span></Label><p id="platform-name-hint" className="mt-1 text-xs text-muted-foreground">Shown throughout the signed-in portals.</p></div><div className="space-y-2"><Input id="platform-name" disabled={busy || !canSave} required maxLength={120} spellCheck={false} placeholder="e.g. Bidora" value={draft.platform_name} onChange={(event) => change("platform_name", event.target.value)} onBlur={() => setTouched((v) => ({ ...v, platform_name: true }))} aria-invalid={!!fieldError("platform_name")} aria-describedby={fieldError("platform_name") ? "platform-name-hint name-error" : "platform-name-hint"} />{fieldError("platform_name") ? <p id="name-error" className="text-sm text-destructive">{errors.platform_name}</p> : null}</div></div>
        <div className="grid min-w-0 gap-2"><Label htmlFor="platform-website">Website <span aria-hidden="true" className="text-destructive">*</span></Label><div className="space-y-2"><Input id="platform-website" disabled={busy || !canSave} type="url" inputMode="url" autoComplete="url" autoCapitalize="none" spellCheck={false} required maxLength={512} placeholder="https://example.com" value={draft.website} onChange={(event) => change("website", event.target.value)} onBlur={() => setTouched((v) => ({ ...v, website: true }))} aria-invalid={!!fieldError("website")} aria-describedby={fieldError("website") ? "website-error" : undefined} />{fieldError("website") ? <p id="website-error" className="text-sm text-destructive">{errors.website}</p> : null}</div></div>
        <div className="grid min-w-0 gap-2"><Label htmlFor="platform-support-email">Support Email <span aria-hidden="true" className="text-destructive">*</span></Label><div className="space-y-2"><Input id="platform-support-email" disabled={busy || !canSave} type="email" inputMode="email" autoComplete="email" autoCapitalize="none" spellCheck={false} required maxLength={254} placeholder="support@example.com" value={draft.support_email} onChange={(event) => change("support_email", event.target.value)} onBlur={() => setTouched((v) => ({ ...v, support_email: true }))} aria-invalid={!!fieldError("support_email")} aria-describedby={fieldError("support_email") ? "email-error" : undefined} />{fieldError("support_email") ? <p id="email-error" className="text-sm text-destructive">{errors.support_email}</p> : null}</div></div>
        <div className="grid min-w-0 gap-2"><Label htmlFor="platform-phone">Contact Phone <span className="text-xs font-normal text-muted-foreground">Optional</span></Label><div className="space-y-2"><Input id="platform-phone" disabled={busy || !canSave} type="tel" inputMode="tel" autoComplete="tel" maxLength={32} placeholder="+254 700 000 000" value={draft.contact_phone} onChange={(event) => change("contact_phone", event.target.value)} onBlur={() => setTouched((v) => ({ ...v, contact_phone: true }))} aria-invalid={!!fieldError("contact_phone")} aria-describedby={fieldError("contact_phone") ? "phone-error" : undefined} />{fieldError("contact_phone") ? <p id="phone-error" className="text-sm text-destructive">{errors.contact_phone}</p> : null}</div></div>
      </div></section>
      <section aria-labelledby="preferences-heading" className="space-y-5 border-t py-6"><h2 id="preferences-heading" className="text-base font-semibold">Regional defaults</h2><div className="grid gap-5 sm:grid-cols-2">{([["default_country","Country",regionalDefault?.name],["default_language","Language",regionalDefault?languageLabel(regionalDefault.language):undefined],["default_currency","Currency",regional?.billing_currency],["default_time_zone","Time Zone",regionalDefault?.timezone]] as const).map(([id,label,value])=><div key={id} className="grid gap-2"><Label htmlFor={id}>{label}</Label><Input id={id} readOnly value={value??""} placeholder="Unavailable"/></div>)}</div></section>
      <div className="space-y-3 border-t py-5 sm:py-6">
        {message ? <div className="space-y-3"><p role={message.kind === "error" ? "alert" : "status"} className={message.kind === "error" ? "text-sm text-destructive" : "text-sm"}>{message.text}</p><Button type="button" variant="outline" disabled={busy} onClick={() => void load()}>Reload saved details</Button></div> : null}
        <div className="flex flex-wrap items-center justify-end gap-3"><Button type="button" variant="outline" disabled={busy||!changed} onClick={()=>{if(current)setDraft(fromServer(current));setTouched({});setServerErrors({});}}>Cancel</Button>{changed ? <span role="status" className="mr-auto text-sm text-muted-foreground">Unsaved changes</span> : null}<Button type="submit" className="min-w-36" disabled={!readyToSave} aria-busy={busy}>{busy ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <SaveIcon aria-hidden="true" />}{busy ? "Saving…" : "Save Changes"}</Button></div>
      </div>
    </form> : null}
  </main>;
}
