"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { PlatformStatus } from "@/components/platform/platform-status";
import { platformRoleLabels } from "@/lib/platform-contract";
import { parseInvitationsForReissue, type ReissuableInvitation } from "@/lib/platform-invitation-reissue-contract";

type State = { kind: "loading" | "error" | "denied" } | { kind: "ready"; rows: ReissuableInvitation[]; next: string | null; observedAt: number };

export function AdministratorInvitations() {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [state, setState] = useState<State>({ kind: "loading" });
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const cursor = cursors[cursors.length - 1];
  const sequence = useRef(0);
  const [selected, setSelected] = useState<ReissuableInvitation | null>(null);
  const [removing, setRemoving] = useState(false);
  const submitting = useRef(false);
  const retry = useRef<{ id: string; version: number; key: string } | null>(null);
  const canRemove = resolution.capabilities.includes("platform.admin.invite");
  const load = useCallback(async () => {
    const current = ++sequence.current;
    if (!generation) { setState({ kind: "denied" }); return; }
    setState({ kind: "loading" });
    try {
      const response = await fetch(`/api/platform/administrator-invitations${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`, { credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      if (response.status === 403) { setState({ kind: "denied" }); return; }
      const page = response.ok ? parseInvitationsForReissue(await response.json()) : null;
      if (current !== sequence.current) return;
      setState(page ? { kind: "ready", rows: page.invitations.filter(row => row.state !== "PENDING_APPROVAL"), next: page.next, observedAt: Date.now() } : { kind: "error" });
    } catch { if (current === sequence.current) setState({ kind: "error" }); }
  }, [generation, cursor, refresh]);
  useEffect(() => {
    const ref = sequence;
    const timer = window.setTimeout(() => void load(), 0);
    return () => { window.clearTimeout(timer); ref.current++; };
  }, [load]);
  async function removeInvitation() {
    if (!selected || submitting.current || !canRemove || !csrfToken || !generation) return;
    const target = selected;
    if (retry.current?.id !== target.id || retry.current.version !== target.version) retry.current = { id: target.id, version: target.version, key: crypto.randomUUID() };
    submitting.current = true; setRemoving(true);
    try {
      const response = await fetch(`/api/platform/administrator-invitations/${target.id}`, {
        method: "DELETE", credentials: "include", cache: "no-store",
        headers: { "X-CSRF-Token": csrfToken, "X-Bidora-Platform-Context-Generation": String(generation), "Idempotency-Key": retry.current.key, "If-Match": `"${target.version}"` },
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 204) {
        retry.current = null; setSelected(null);
        toast.success("Invitation removed.", { id: "remove-invitation" });
        if (state.kind === "ready" && state.rows.length === 1 && cursors.length > 1) setCursors(old => old.slice(0, -1));
        else await load();
      } else if (response.status === 409 || response.status === 404) {
        toast.error("This invitation has changed or is no longer available.", { id: "remove-invitation" }); setSelected(null); await load();
      } else if (response.status === 403) {
        const failure = await response.json().catch(() => null);
        toast.error(failure?.error?.code === "PLATFORM_ASSURANCE_REQUIRED"
          ? "Please sign out and sign in again to verify your account, then remove the invitation."
          : "Your current account does not have permission to remove this invitation.", { id: "remove-invitation" });
      } else toast.error("Removal could not be confirmed. Try again with the same invitation.", { id: "remove-invitation" });
    } catch { toast.error("Removal could not be confirmed. Try again with the same invitation.", { id: "remove-invitation" }); }
    finally { submitting.current = false; setRemoving(false); }
  }
  if (state.kind === "denied") return <p role="alert" className="text-sm">You don’t have access to invited users.</p>;
  const rows = state.kind === "ready" ? state.rows : [];
  const date = (value: string | null) => value ? new Date(value).toLocaleString() : "—";
  return <>
    <section aria-label="Invited users" className="overflow-x-auto rounded-lg border bg-card" aria-busy={state.kind === "loading"}>
      <table className="w-full min-w-[680px] text-left text-sm">
        <thead className="bg-muted/40"><tr>{["Name", "Email address", "Role", "Status", "Expires", "Actions"].map(label => <th scope="col" key={label} className="px-4 py-3">{label}</th>)}</tr></thead>
        <tbody className="divide-y">
          {rows.map(row => <tr key={row.id}>
            <th scope="row" className="px-4 py-3 font-medium">{[row.first_name, row.last_name].filter(Boolean).join(" ") || "—"}</th>
            <td className="break-all px-4 py-3">{row.email}</td>
            <td className="px-4 py-3">{platformRoleLabels[row.role_key]}</td>
            <td className="px-4 py-3"><PlatformStatus value={state.kind === "ready" && row.expires_at && new Date(row.expires_at).getTime() <= state.observedAt ? "Expired" : row.state === "ACTIVE" ? "Invited" : row.state} /></td>
            <td className="whitespace-nowrap px-4 py-3">{date(row.expires_at)}</td>
            <td className="px-4 py-3"><div className="flex justify-end"><PlatformRowActions label={row.email} actions={canRemove && row.state === "ACTIVE" ? [{ label: "Remove invitation", destructive: true, onSelect: () => setSelected(row) }] : []} /></div></td>
          </tr>)}
          {!rows.length ? <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">{state.kind === "loading" ? <Skeleton role="status" aria-label="Loading invited users" className="mx-auto h-6 w-48" /> : "No data."}</td></tr> : null}
        </tbody>
      </table>
    </section>
    <nav aria-label="Invited users pages" className="mt-4 flex items-center gap-3">
      <Button type="button" variant="outline" disabled={state.kind === "loading" || cursors.length === 1} onClick={() => setCursors(old => old.slice(0, -1))}>Previous</Button>
      <span className="text-sm text-muted-foreground">Page {cursors.length}</span>
      <Button type="button" variant="outline" disabled={state.kind !== "ready" || !state.next} onClick={() => { if (state.kind === "ready" && state.next) setCursors(old => [...old, state.next]); }}>Next</Button>
    </nav>
    <AlertDialog open={selected !== null} onOpenChange={open => { if (!open && !submitting.current) setSelected(null); }}>
      <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Remove invitation?</AlertDialogTitle><AlertDialogDescription>This cancels the invitation for {selected?.email} and prevents it from being accepted. Existing user accounts are not deleted.</AlertDialogDescription></AlertDialogHeader>
        <AlertDialogFooter><AlertDialogCancel disabled={removing}>Keep invitation</AlertDialogCancel><Button type="button" variant="destructive" disabled={removing || !csrfToken} onClick={() => void removeInvitation()}>{removing ? "Removing…" : "Remove invitation"}</Button></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </>;
}
