"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import { MailIcon, RotateCcwIcon, Trash2Icon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel } from "@/components/ui/alert-dialog";
import { parseInvitationsForReissue, type ReissuableInvitation } from "@/lib/platform-invitation-reissue-contract";
import { parsePlatformInvitationReceipt, platformRoleLabels } from "@/lib/platform-contract";
import { PlatformStatus } from "@/components/platform/platform-status";

type Selection = { invitation: ReissuableInvitation; kind: "reissue" | "remove" };
export function AdministratorInvitations({ revision }: { revision: number }) {
  const router=useRouter();
  const { resolution, csrfToken, refresh } = useSession(), generation = resolution.platform_context_generation;
  const [data, setData] = useState<ReturnType<typeof parseInvitationsForReissue>>(null);
  const [error, setError] = useState(""), [loading, setLoading] = useState(true);
  const [cursors, setCursors] = useState<(string | null)[]>([null]), [page, setPage] = useState(0), [local, setLocal] = useState(0);
  const [selected, setSelected] = useState<Selection | null>(null), [reason, setReason] = useState(""), [busy, setBusy] = useState(false);
  const [fetchedAt, setFetchedAt] = useState(0);
  const retry = useRef<{ fingerprint: string; key: string } | null>(null), flight = useRef(false);
  const cursor = cursors[page];
  const load = useCallback(async () => {
    if (!generation) return;
    setLoading(true); setError("");
    try {
      const response = await fetch("/api/platform/administrator-invitations" + (cursor ? "?cursor=" + encodeURIComponent(cursor) : ""), { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (response.status === 401 || response.status === 409) { refresh(); setError("Your access changed. Please retry."); return; }
      const result = response.ok ? parseInvitationsForReissue(await response.json()) : null;
      if (result) { setData(result); setFetchedAt(Date.now()); }
      else setError(response.status === 403 ? "You do not have access to invitations." : "Invitation data is temporarily unavailable.");
    } catch { setError("Invitation data is temporarily unavailable."); }
    finally { setLoading(false); }
  }, [generation, cursor, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); return () => window.clearTimeout(timer); }, [load, revision, local]);

  async function submit() {
    if (!selected || flight.current || !csrfToken || (selected.kind === "reissue" && !reason.trim())) return;
    flight.current = true; setBusy(true);
    const { invitation, kind } = selected;
    const remove = kind === "remove";
    const toastID = toast.loading(remove ? "Removing invitation…" : "Reissuing invitation…");
    const fingerprint = `${kind}/${invitation.id}/${invitation.version}/${remove ? "" : reason.trim()}`;
    if (retry.current?.fingerprint !== fingerprint) retry.current = { fingerprint, key: crypto.randomUUID() };
    try {
      const response = await fetch(`/api/platform/administrator-invitations/${invitation.id}${remove ? "" : "/reissue"}`, {
        method: remove ? "DELETE" : "POST", credentials: "include", cache: "no-store",
        headers: { ...(remove ? { "If-Match": `"${invitation.version}"` } : { "Content-Type": "application/json" }), "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": retry.current.key },
        body: remove ? undefined : JSON.stringify({ expected_version: invitation.version, reason: reason.trim() }),
      });
      if (response.status === 401) { toast.error("Sign in again to manage invitations.", { id: toastID }); refresh(); return; }
      if (response.status === 403) {
        toast.error("Only a recently verified Platform Owner can manage invitations.", { id: toastID, action: { label: "Verify account", onClick: () => router.push("/login") } }); return;
      }
      const receipt = !remove && response.status === 201 ? parsePlatformInvitationReceipt(await response.json()) : null;
      if (remove ? response.status === 204 : !!receipt && receipt.state === "ACTIVE") {
        retry.current = null; setSelected(null); setPage(0); setCursors([null]); setLocal(n => n + 1);
        if (remove) toast.success("Invitation removed. Its link no longer works.", { id: toastID });
        else if (receipt?.delivery_state === "ACCEPTED_FOR_DELIVERY") toast.success("New invitation accepted for delivery. The previous link is invalid.", { id: toastID });
        else toast.warning("Invitation reissued; delivery is unconfirmed. Check Communications before retrying.", { id: toastID });
      } else toast.error(response.status === 409 || response.status === 404 ? "This invitation changed or was already accepted. Close this dialog and check the current list." : "The outcome could not be confirmed. Retry with the same details.", { id: toastID });
    } catch { toast.error("The outcome could not be confirmed. Retry with the same details.", { id: toastID }); }
    finally { flight.current = false; setBusy(false); }
  }

  const canManage = resolution.capabilities.includes("platform.admin.invite");
  return <section className="space-y-4" aria-label="Invited users">
    <div className="flex items-center gap-3"><div className="rounded-lg bg-muted p-2"><MailIcon aria-hidden="true" className="size-5 text-muted-foreground" /></div><div><h2 className="text-base font-semibold">Pending invitations</h2><p className="text-sm text-muted-foreground">Reissue a setup link or remove an invitation that is no longer needed.</p></div></div>
    <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[680px] text-left text-sm"><thead className="border-b bg-muted/40 text-xs text-muted-foreground"><tr>{["Invited user", "Role", "Status", "Expires", "Actions"].map(label => <th key={label} scope="col" className="px-4 py-3">{label}</th>)}</tr></thead><tbody className="divide-y">
      {!error && (data?.invitations ?? []).map(row => <tr key={row.id}>
        <th scope="row" className="px-4 py-4 font-medium"><span>{[row.first_name, row.last_name].filter(Boolean).join(" ") || row.email}</span>{(row.first_name || row.last_name) && <span className="block break-all text-xs font-normal text-muted-foreground">{row.email}</span>}</th>
        <td className="px-4 py-4">{platformRoleLabels[row.role_key]}</td><td className="px-4 py-4"><PlatformStatus value={row.state === "PENDING_APPROVAL" ? "Awaiting approval" : row.expires_at && Date.parse(row.expires_at) <= fetchedAt ? "Expired" : "Invited"} /></td>
        <td className="px-4 py-4">{row.expires_at ? new Date(row.expires_at).toLocaleDateString() : "—"}</td>
        <td className="px-4 py-4">{canManage && <div className="flex justify-end gap-2">
          {row.state === "ACTIVE" && <Button size="sm" variant="outline" onClick={() => { setSelected({ invitation: row, kind: "reissue" }); setReason(""); }}><RotateCcwIcon aria-hidden="true" />Reissue invitation</Button>}
          <Button size="sm" variant="outline" aria-label={`Remove invitation for ${row.email}`} onClick={() => setSelected({ invitation: row, kind: "remove" })}><Trash2Icon aria-hidden="true" />Remove</Button>
        </div>}</td>
      </tr>)}
      {(error || !data?.invitations.length) && <tr><td colSpan={5} className="px-4 py-8 text-center text-muted-foreground">{loading ? "Loading invitations…" : error || "No data found."}</td></tr>}
    </tbody></table></div>
    {error && <Button variant="outline" onClick={() => void load()}>Retry invitations</Button>}
    <nav aria-label="Invitation pages" className="flex items-center justify-end gap-3"><Button variant="outline" disabled={loading || page === 0} onClick={() => setPage(p => p - 1)}>Previous invitations</Button><span className="text-sm">Page {page + 1}</span><Button variant="outline" disabled={loading || !!error || !data?.next} onClick={() => { if (data?.next) { setCursors(old => [...old.slice(0, page + 1), data.next]); setPage(p => p + 1); } }}>Next invitations</Button></nav>
    <AlertDialog open={!!selected} onOpenChange={open => { if (!open && !busy) setSelected(null); }}><AlertDialogContent>
      <AlertDialogHeader><AlertDialogTitle>{selected?.kind === "remove" ? "Remove invitation?" : "Reissue invitation?"}</AlertDialogTitle><AlertDialogDescription>{selected?.kind === "remove" ? `This cancels the invitation for ${selected.invitation.email} and invalidates its link. Audit history is retained. No email will be sent.` : `Send a new setup link to ${selected?.invitation.email}. The previous invitation will no longer work.`}</AlertDialogDescription></AlertDialogHeader>
      {selected?.kind === "reissue" && <div className="space-y-2"><Label htmlFor="reissue-reason">Reason</Label><Input id="reissue-reason" disabled={busy} value={reason} maxLength={2000} onChange={event => setReason(event.target.value)} /></div>}
      <AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button variant={selected?.kind === "remove" ? "destructive" : "default"} disabled={busy || selected?.kind === "reissue" && !reason.trim()} onClick={() => void submit()}>{busy ? "Saving…" : selected?.kind === "remove" ? "Remove invitation" : "Reissue invitation"}</Button></AlertDialogFooter>
    </AlertDialogContent></AlertDialog>
  </section>;
}
