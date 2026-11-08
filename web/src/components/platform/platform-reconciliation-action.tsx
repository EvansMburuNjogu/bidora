"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useSession } from "@/components/session/session-provider";
import { parseReconciliationReceipt } from "@/lib/platform-commercial-contract";

export function PlatformReconciliationAction({ paymentID, version, onScheduled }: { paymentID: string; version: number; onScheduled: () => void }) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const pendingKey = useRef<string | null>(null);
  const submitting = useRef(false);
  if (!generation || !resolution.capabilities.includes("platform.payment.reconcile")) return null;
  async function schedule() {
    if (submitting.current || !generation || !csrfToken || !reason.trim()) return;
    submitting.current = true; setBusy(true); setMessage("");
    const key = pendingKey.current ?? crypto.randomUUID(); pendingKey.current = key;
    try {
      const response = await fetch(`/api/platform/commercial/payments/${paymentID}/reconciliation`, { method: "POST", credentials: "include", cache: "no-store", headers: { Accept: "application/json", "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": String(generation), "X-CSRF-Token": csrfToken, "Idempotency-Key": key }, body: JSON.stringify({ expected_version: version, reason: reason.trim() }) });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 202) {
        if (parseReconciliationReceipt(await response.json(), paymentID, version)) {
          setOpen(false); pendingKey.current = null; setReason(""); setMessage("Reconciliation requested. Provider outcome and any reversal are not confirmed."); onScheduled(); return;
        }
        setMessage("The request outcome could not be confirmed. Retry with the same reason before issuing another request."); return;
      }
      setMessage(response.status === 403 ? "A recent verified Billing Administrator session is required." : response.status === 409 ? "Payment state changed. Reload the queue before requesting reconciliation." : response.status === 503 ? "The outcome may be uncertain. Retry with the same reason and request key." : "Reconciliation could not be requested.");
    } catch { setMessage("The outcome could not be confirmed. Retry with the same reason before issuing another request."); }
    finally { submitting.current = false; setBusy(false); }
  }
  return <><Button type="button" size="sm" variant="outline" onClick={() => setOpen(true)}>Request reconciliation</Button>
    {message && !open && <p role="status" className="mt-2 text-xs text-muted-foreground">{message}</p>}
    <AlertDialog open={open} onOpenChange={(next) => { if (!busy) setOpen(next); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Recheck settled payment</AlertDialogTitle><AlertDialogDescription>This schedules an audited provider-status recheck. It does not refund, settle, or reverse a payment.</AlertDialogDescription></AlertDialogHeader>
      <label className="grid gap-1 text-sm">Reason<Input value={reason} maxLength={2000} disabled={busy} onChange={(event) => { setReason(event.target.value); pendingKey.current = null; setMessage(""); }} /></label>{message && <p role="alert" className="text-sm text-destructive">{message}</p>}
      <AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="button" disabled={busy || !reason.trim() || !csrfToken} onClick={() => void schedule()}>{busy ? "Requesting…" : "Confirm request"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </>;
}
