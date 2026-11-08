"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { useSession } from "@/components/session/session-provider";
import { parseSupportPage, parseSupportReceipt, type SupportCase } from "@/lib/platform-support-contract";

type View = { generation: number; rows: SupportCase[]; next: string | null };
export function PlatformSupportCases() {
 const {resolution}=useSession();
 return <SupportCasesWorkspace key={resolution.platform_context_generation} />;
}
function SupportCasesWorkspace() {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const read = resolution.capabilities.includes("platform.support.case.read");
  const manage = resolution.capabilities.includes("platform.support.case.manage");
  const [view, setView] = useState<View | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [tenant, setTenant] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [created, setCreated] = useState<string | null>(null);
  const key = useRef<string | null>(null);
  const sequence = useRef(0);
  const locked = useRef(false);
  const load = useCallback(async (cursor?: string) => {
    if (!generation || !read) return;
    const current = ++sequence.current;
    setBusy(true); setError("");
    const params = new URLSearchParams({ limit: "20" }); if (cursor) params.set("cursor", cursor);
    try {
      const response = await fetch(`/api/platform/support/cases?${params}`, { cache: "no-store", credentials: "include", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const page = response.ok ? parseSupportPage(await response.json()) : null;
      if (!page) { setError(response.status === 403 ? "A verified Support session is required." : "Support cases could not be confirmed."); return; }
      setView((old) => ({ generation, rows: cursor && old?.generation === generation ? [...old.rows, ...page.cases] : page.cases, next: page.next }));
    } catch { if (current === sequence.current) setError("Support cases are unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }, [generation, read, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); const current = sequence; return () => { window.clearTimeout(timer); current.current++; }; }, [load]);
  async function create() {
    if (locked.current || !generation || !csrfToken || !manage || !tenant || !title.trim() || !description.trim()) return;
    locked.current = true; setBusy(true); setError("");
    const current = ++sequence.current;
    const idempotency = key.current ?? crypto.randomUUID(); key.current = idempotency;
    try {
      const response = await fetch("/api/platform/support/cases", { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": idempotency }, body: JSON.stringify({ tenant_id: tenant, title: title.trim(), description: description.trim() }) });
      if (current !== sequence.current) return;
      if (response.status === 401) { refresh(); return; }
      const receipt = response.ok ? parseSupportReceipt(await response.json()) : null;
      if (!receipt) { setError(response.status === 403 ? "Support management requires a verified session." : response.status === 404 ? "That tenant was not found in the control registry." : response.status === 409 ? "The case changed. Review before resubmitting." : "The outcome could not be confirmed. Retry unchanged with the same key."); return; }
      key.current = null; setCreated(receipt.case_id); setTenant(""); setTitle(""); setDescription("");
      await load();
    } catch { if (current === sequence.current) setError("The outcome is uncertain; retry unchanged with the same key."); }
    finally { locked.current = false; if (current === sequence.current) setBusy(false); }
  }
  const visible = view?.generation === generation && read ? view : null;
  return <main className="app-page space-y-5"><div className="app-page-header"><h1 className="type-title">Support</h1><p className="type-lead text-muted-foreground">Create, assign and resolve customer support cases. Keep passwords, verification codes and private customer documents out of case notes.</p></div>
    {!read || !generation ? <p role="status">You do not have access to support cases.</p> : <>
      {manage && <section aria-label="Create support case" className="space-y-3 rounded-lg border bg-card p-4"><h2 className="font-semibold">New case</h2><PlatformReferencePicker kind="organizations" id="support-organisation" label="Organisation" value={tenant} disabled={busy} onChange={(id) => { key.current = null; setTenant(id); }} /><label className="grid gap-1 text-sm">Title<Input value={title} maxLength={120} disabled={busy} onChange={(e) => { key.current = null; setTitle(e.target.value); }} /></label><label className="grid gap-1 text-sm">Description<textarea className="min-h-28 rounded-lg border border-input bg-background px-3 py-2 text-sm" value={description} maxLength={2000} disabled={busy} onChange={(e) => { key.current = null; setDescription(e.target.value); }} /></label><Button type="button" disabled={busy || !csrfToken || !tenant || !title.trim() || !description.trim()} onClick={() => void create()}>Create case</Button></section>}
      {created && <p role="status" className="text-sm">Support case created. <Link href={`/platform/support/${created}`} className="text-primary underline">Open case</Link></p>}
      <Button type="button" variant="outline" disabled={busy} onClick={() => void load()}>Refresh cases</Button>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {!visible ? <p role="status" className="text-sm text-muted-foreground">{busy ? "Loading cases…" : "No case list is confirmed."}</p> : <section aria-label="Support cases" className="space-y-3"><div className="space-y-2">{visible.rows.map((row) => <Link href={`/platform/support/${row.id}`} key={row.id} className="block rounded-lg border bg-card p-4 hover:bg-muted/30"><span className="font-medium">{row.title}</span><span className="ml-2 text-xs text-muted-foreground">{row.state} · Version {row.version}</span><p className="text-xs text-muted-foreground">Updated {row.updated_at} UTC</p></Link>)}{!visible.rows.length && <p>No support cases found.</p>}</div>{visible.next && <Button type="button" variant="outline" disabled={busy} onClick={() => void load(visible.next ?? undefined)}>Load more</Button>}</section>}
    </>}
  </main>;
}
