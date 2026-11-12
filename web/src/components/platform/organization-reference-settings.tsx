"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { PlusIcon, SearchIcon } from "lucide-react";
import { toast } from "sonner";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PlatformSelect } from "@/components/platform/platform-select";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { SettingsHeader, SettingsSwitch } from "@/components/platform/settings-draft-controls";
import { useSession } from "@/components/session/session-provider";
import { parseOrganizationReferences, type OrganizationReference, type OrganizationReferenceKind } from "@/lib/organization-reference-contract";
import { regionalRequest } from "@/lib/platform-regional-contract";
import { networkSubmissionError, submissionErrorMessage, unconfirmedSubmissionError } from "@/lib/submission-errors";

type Form = { code: string; label: string; enabled: boolean; version: number };
async function readReferences(kind: OrganizationReferenceKind, generation: number) {
  const response = await regionalRequest(`/api/platform/organization-references/${kind}`, generation);
  const result = response.ok ? parseOrganizationReferences(await response.json(), kind) : null;
  if (!result) throw new Error("reference_unavailable");
  return result;
}
export function OrganizationReferenceSettings({ kind }: { kind: OrganizationReferenceKind }) {
  const { resolution } = useSession();
  const allowed = resolution.platform_context_generation !== null && resolution.capabilities.includes("platform.reference.read");
  const title = kind === "business-types" ? "Business types" : "Industries";
  return <main className="app-page"><SettingsHeader title={title} description="Manage options available to new organizations." />{allowed ? <ReferenceDirectory key={`${kind}-${resolution.platform_context_generation}`} kind={kind} title={title} /> : <p className="text-sm text-muted-foreground">You do not have access to reference settings.</p>}</main>;
}
function ReferenceDirectory({ kind, title }: { kind: OrganizationReferenceKind; title: string }) {
  const { resolution, csrfToken } = useSession();
  const generation = resolution.platform_context_generation!;
  const canManage = resolution.capabilities.includes("platform.reference.manage");
  const [rows, setRows] = useState<OrganizationReference[] | null>(null), [error, setError] = useState(""), [loading, setLoading] = useState(true);
  const [query, setQuery] = useState(""), [status, setStatus] = useState("all"), [applied, setApplied] = useState({ query: "", status: "all" }), [page, setPage] = useState(0), [size, setSize] = useState(10);
  const [form, setForm] = useState<Form | null>(null), [busy, setBusy] = useState(false), [attempted, setAttempted] = useState(false);
  const lock = useRef(false), command = useRef<{ fingerprint: string; key: string } | null>(null);
  const load = useCallback(async () => { try { setRows(await readReferences(kind, generation)); setError(""); } catch { setError("Reference settings could not be loaded. Previously displayed values may be stale."); } finally { setLoading(false); } }, [generation, kind]);
  useEffect(() => { let active = true; void readReferences(kind, generation).then(value => { if (active) { setRows(value); setError(""); } }).catch(() => { if (active) setError("Reference settings could not be loaded."); }).finally(() => { if (active) setLoading(false); }); return () => { active = false; }; }, [kind, generation]);
  const filtered = (rows ?? []).filter(row => `${row.label} ${row.code}`.toLowerCase().includes(applied.query.trim().toLowerCase()) && (applied.status === "all" || row.enabled === (applied.status === "enabled")));
  const pages = Math.max(1, Math.ceil(filtered.length / size)), current = Math.min(page, pages - 1), visible = filtered.slice(current * size, (current + 1) * size);
  const invalid = !!form && (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(form.code) || form.code.length > 64 || !form.label.trim() || form.label.trim() !== form.label || new TextEncoder().encode(form.label).length > 100 || /[\u0000-\u001f\u007f]/.test(form.label));
  function open(row?: OrganizationReference) { command.current = null; setAttempted(false); setForm(row ? { code: row.code, label: row.label, enabled: row.enabled, version: row.version } : { code: "", label: "", enabled: true, version: 0 }); }
  function update<K extends keyof Form>(key: K, value: Form[K]) { setForm(old => old && ({ ...old, [key]: value })); command.current = null; }
  async function save() {
    if (!form || lock.current || !canManage) return;
    setAttempted(true); if (invalid) return;
    if (!csrfToken) { toast.error("Refresh your session before saving.", { id: "reference-save" }); return; }
    const body = { code: form.code, label: form.label, enabled: form.enabled }, fingerprint = JSON.stringify({ body, version: form.version });
    if (command.current?.fingerprint !== fingerprint) command.current = { fingerprint, key: crypto.randomUUID() };
    lock.current = true; setBusy(true);
    try {
      const response = await regionalRequest(`/api/platform/organization-references/${kind}${form.version ? `/${form.code}` : ""}`, generation, { method: form.version ? "PUT" : "POST", csrf: csrfToken, key: command.current.key, version: form.version, body });
      if (!response.ok) { toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "reference-save" }); return; }
      const result = await response.json();
      if (!parseOrganizationReferences({ data: [result?.data], meta: result?.meta }, kind)) { toast.warning(unconfirmedSubmissionError, { id: "reference-save" }); return; }
      setForm(null); command.current = null; toast.success("Reference saved.", { id: "reference-save" }); await load();
    } catch { toast.error(networkSubmissionError, { id: "reference-save" }); } finally { lock.current = false; setBusy(false); }
  }
  return <div className="max-w-5xl">
    <p className="text-sm text-muted-foreground">Disabled options are hidden from new organization setup. Existing selections are retained.</p>
    {error ? <p role="alert" className="mt-3 text-sm text-destructive">{error} <Button size="sm" variant="outline" disabled={loading} onClick={() => { setLoading(true); void load(); }}>Retry</Button></p> : null}
    <form className="mt-5 flex flex-wrap items-end gap-3" onSubmit={event => { event.preventDefault(); setApplied({ query, status }); setPage(0); }}><div className="min-w-44 flex-1"><label htmlFor="reference-search" className="text-sm font-medium">Search {title.toLowerCase()}</label><div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"/><Input id="reference-search" className="pl-9" value={query} onChange={event => setQuery(event.target.value)} /></div></div><SearchableFilterSelect id="reference-status" label="Status" value={status} options={[{ value: "all", label: "All" }, { value: "enabled", label: "Enabled" }, { value: "disabled", label: "Disabled" }]} onChange={setStatus}/>{query || status !== "all" || applied.query || applied.status !== "all" ? <Button type="button" variant="outline" onClick={() => { setQuery(""); setStatus("all"); setApplied({ query: "", status: "all" }); setPage(0); }}>Clear filters</Button> : null}<Button type="submit">Apply filters</Button>{canManage ? <Button type="button" disabled={loading || !rows || !!error} onClick={() => open()}><PlusIcon aria-hidden="true"/>Add</Button> : null}</form>
    {loading ? <p role="status" className="mt-3 text-sm text-muted-foreground">Loading…</p> : null}
    <div className="mt-3 overflow-x-auto rounded-lg border"><table className="w-full text-left text-sm"><thead className="bg-muted/40 text-xs text-muted-foreground"><tr>{["Name", "Code", "Status", "Actions"].map(label => <th key={label} scope="col" className="px-3 py-3 font-medium">{label}</th>)}</tr></thead><tbody className="divide-y">{visible.map(row => <tr key={row.code}><th scope="row" className="px-3 py-3 font-medium">{row.label}</th><td className="px-3 py-3">{row.code}</td><td className="px-3 py-3"><PlatformStatus value={row.enabled ? "ACTIVE" : "DISABLED"}/></td><td className="px-3 py-3"><PlatformRowActions label={row.label} actions={[{ label: canManage ? "View / Edit" : "View", onSelect: () => open(row) }]}/></td></tr>)}{!loading && rows && visible.length === 0 ? <tr><td colSpan={4} className="px-3 py-6 text-center text-muted-foreground">No entries match these filters.</td></tr> : null}</tbody></table></div>
    <div className="mt-3 flex flex-wrap items-center justify-between gap-3 text-sm"><label className="flex items-center gap-2">Rows<PlatformSelect value={size} onChange={event => { setSize(Number(event.target.value)); setPage(0); }}><option value={5}>5</option><option value={10}>10</option><option value={25}>25</option></PlatformSelect></label><div className="flex items-center gap-2"><span>Page {current + 1} of {pages}</span><Button size="sm" variant="outline" disabled={current === 0} onClick={() => setPage(current - 1)}>Previous</Button><Button size="sm" variant="outline" disabled={current + 1 >= pages} onClick={() => setPage(current + 1)}>Next</Button></div></div>
    <AlertDialog open={form !== null} onOpenChange={open => { if (!open && !lock.current) setForm(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{form?.version ? `Edit ${title.toLowerCase()}` : `Add ${title.toLowerCase()}`}</AlertDialogTitle><AlertDialogDescription>Codes stay fixed after creation. Disable an option to hide it from new organization setup.</AlertDialogDescription></AlertDialogHeader><fieldset disabled={busy || !canManage} className="space-y-3"><label className="block text-sm font-medium" htmlFor="reference-label">Name</label><Input id="reference-label" value={form?.label ?? ""} maxLength={100} onChange={event => update("label", event.target.value)}/><label className="block text-sm font-medium" htmlFor="reference-code">Code</label><Input id="reference-code" value={form?.code ?? ""} maxLength={64} disabled={!!form?.version} onChange={event => update("code", event.target.value)}/><p className="text-xs text-muted-foreground">Use lowercase letters, numbers and hyphens.</p><SettingsSwitch id="reference-enabled" label="Enabled" checked={form?.enabled ?? true} onChange={value => update("enabled", value)}/>{attempted && invalid ? <p role="alert" className="text-sm text-destructive">Enter a valid name and code.</p> : null}</fieldset><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel>{canManage ? <Button type="button" disabled={busy} onClick={() => void save()}>{busy ? "Saving…" : "Save changes"}</Button> : null}</AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
}
