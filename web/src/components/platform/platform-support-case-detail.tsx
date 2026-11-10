"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { useSession } from "@/components/session/session-provider";
import { parseSupportDetail, parseSupportReceipt, type SupportCase, type SupportMessage } from "@/lib/platform-support-contract";

type Detail = { generation: number; case: SupportCase; messages: SupportMessage[] };
type Action = "assign" | "reply" | "resolve" | "reopen";
export function PlatformSupportCaseDetail({ caseID }: { caseID: string }) {
 const {resolution}=useSession();
 return <SupportCaseWorkspace key={`${caseID}|${resolution.platform_context_generation}`} caseID={caseID} />;
}
function SupportCaseWorkspace({ caseID }: { caseID: string }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const read = resolution.capabilities.includes("platform.support.case.read");
  const manage = resolution.capabilities.includes("platform.support.case.manage");
  const [detail, setDetail] = useState<Detail | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [assignee, setAssignee] = useState("");
  const [message, setMessage] = useState("");
  const pending = useRef<{ action: Action; key: string; payload: string } | null>(null);
  const sequence = useRef(0);
  const locked = useRef(false);
  const load = useCallback(async () => {
    if (!generation || !read) return;
    const current = ++sequence.current; setBusy(true); setError("");
    try {
      const response = await fetch(`/api/platform/support/cases/${caseID}`, { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const result = response.ok ? parseSupportDetail(await response.json(), caseID) : null;
      if (!result) { setError(response.status === 403 ? "You do not have permission to view this support case." : "The case could not be confirmed."); return; }
      setDetail({ ...result, generation }); setAssignee(result.case.assigned_to ?? "");
    } catch { if (current === sequence.current) setError("The case is unavailable."); }
    finally { if (current === sequence.current) setBusy(false); }
  }, [caseID, generation, read, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); const current = sequence; return () => { window.clearTimeout(timer); current.current++; }; }, [load]);
  const visible = detail?.generation === generation && detail.case.id === caseID && read ? detail : null;
  async function command(action: Action) {
    if (locked.current || !visible || !manage || !csrfToken || !generation) return;
    const payload = JSON.stringify(action === "assign" ? { assignee_id: assignee, expected_version: visible.case.version } : action === "reply" ? { message: message.trim(), expected_version: visible.case.version } : { expected_version: visible.case.version });
    if (action === "reply" && !message.trim() || action === "assign" && !assignee) return;
    locked.current = true; setBusy(true); setError("");
    const current = ++sequence.current;
    const key = pending.current?.action === action && pending.current.payload === payload ? pending.current.key : crypto.randomUUID();
    pending.current = { action, payload, key };
    try {
      const response = await fetch(`/api/platform/support/cases/${caseID}/${action}`, { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": key }, body: payload });
      if (current !== sequence.current) return;
      if (response.status === 401) { refresh(); return; }
      const receipt = response.ok ? parseSupportReceipt(await response.json()) : null;
      if (!receipt || receipt.case_id !== caseID) { setError(response.status === 403 ? "Support management requires a verified session." : response.status === 409 ? "Case version changed. Reload before trying again." : "The outcome could not be confirmed. Retry unchanged with the same key."); return; }
      pending.current = null; setMessage(""); setAssignee("");
      await load();
    } catch { if (current === sequence.current) setError("The outcome is uncertain. Retry unchanged with the same key."); }
    finally { locked.current = false; if (current === sequence.current) setBusy(false); }
  }
  return <main className="app-page space-y-5"><Link href="/platform/support" className="text-sm text-primary underline">Back to support cases</Link><div className="app-page-header"><h1 className="type-title">Support case</h1><p className="type-lead text-muted-foreground">Coordinate the response, record updates and resolve the customer’s request. Private customer documents remain protected.</p></div>
    {!read || !generation ? <p role="status">Support case-read permission is required.</p> : <>
      <Button type="button" variant="outline" disabled={busy} onClick={() => void load()}>Refresh case</Button>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {!visible ? <p role="status">{busy ? "Loading case…" : "No case detail is confirmed."}</p> : <><section className="space-y-2 rounded-lg border bg-card p-4"><h2 className="font-semibold">{visible.case.title}</h2><p className="whitespace-pre-wrap break-words text-sm">{visible.case.description}</p><PlatformReferencePicker kind="organizations" id="case-organisation" label="Organisation" value={visible.case.tenant_id} disabled onChange={() => {}} /><p className="text-xs text-muted-foreground">{visible.case.state} · {visible.case.assigned_to ? "Assigned" : "Unassigned"}</p></section>
        <section aria-label="Replies" className="space-y-2"><h2 className="font-semibold">Replies</h2>{visible.messages.map((row) => <div className="rounded-lg border p-3" key={row.id}><p className="whitespace-pre-wrap break-words text-sm">{row.body}</p><p className="mt-1 text-xs text-muted-foreground">Administrator reply · {new Date(row.created_at).toLocaleString()}</p></div>)}{!visible.messages.length && <p className="text-sm text-muted-foreground">No replies yet.</p>}</section>
        {manage && <section aria-label="Case operations" className="space-y-3 rounded-lg border bg-card p-4"><h2 className="font-semibold">Manage case</h2>{visible.case.state === "OPEN" ? <><div className="grid gap-1 text-sm"><PlatformReferencePicker kind="support-assignees" id="support-assignee" label="Support administrator" value={assignee} disabled={busy} onChange={(id) => { pending.current = null; setAssignee(id); }} /></div><Button type="button" variant="outline" disabled={busy || !csrfToken || !assignee || assignee === visible.case.assigned_to} onClick={() => void command("assign")}>Assign</Button><label className="grid gap-1 text-sm">Reply<textarea className="min-h-28 rounded-lg border border-input bg-background px-3 py-2 text-sm" maxLength={2000} value={message} disabled={busy || visible.messages.length >= 100} onChange={(e) => { pending.current = null; setMessage(e.target.value); }} /></label><Button type="button" disabled={busy || !csrfToken || !message.trim() || visible.messages.length >= 100} onClick={() => void command("reply")}>Add reply</Button><Button type="button" variant="outline" disabled={busy || !csrfToken} onClick={() => void command("resolve")}>Resolve</Button></> : <Button type="button" disabled={busy || !csrfToken} onClick={() => void command("reopen")}>Reopen</Button>}</section>}
      </>}
    </>}
  </main>;
}
