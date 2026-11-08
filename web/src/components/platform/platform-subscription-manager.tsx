"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformSubscription, parsePlatformSubscriptionReceipt, type PlatformSubscription } from "@/lib/platform-subscription-contract";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { PlatformSubscriptionInventory } from "@/components/platform/platform-subscription-inventory";
const uuid = /^[a-f\d]{8}-[a-f\d]{4}-[1-8][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
export function PlatformSubscriptionManager({ tenantID }: { tenantID?: string } = {}) {
  const { resolution, csrfToken, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const canRead = resolution.capabilities.includes("platform.subscription.read"), canManage = resolution.capabilities.includes("platform.subscription.manage");
  const [tenant, setTenant] = useState(tenantID ?? ""), [view, setView] = useState<{ generation: number; subscription: PlatformSubscription } | null>(null);
  const [busy, setBusy] = useState(false), [message, setMessage] = useState(""), [inventoryRevision, setInventoryRevision] = useState(0);
  const sequence = useRef(0), key = useRef<{fingerprint:string;value:string} | null>(null), locked = useRef(false);
 const [confirm,setConfirm]=useState<"schedule"|"withdraw"|null>(null);
  const load = useCallback(async (id: string): Promise<PlatformSubscription | null> => {
    if (!generation || !canRead || !uuid.test(id)) return null;
    const current = ++sequence.current; setBusy(true); setMessage(""); setView(null);
    try {
      const response = await fetch(`/api/platform/subscriptions/${id}`, { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
      if (current !== sequence.current) return null;
      if (response.status === 401 || response.status === 409) { refresh(); return null; }
      const value = response.ok ? parsePlatformSubscription(await response.json(), id) : null;
      if (!value) { setView(null); setMessage(response.status === 404 ? "No current subscription was found for this organisation." : response.status === 403 ? "Verified Billing access is required." : "Subscription status could not be confirmed."); return null; }
      setView({ generation, subscription: value }); return value;
    } catch { if (current === sequence.current) setMessage("Subscription status could not be confirmed."); return null; }
    finally { if (current === sequence.current) setBusy(false); }
  }, [generation, canRead, refresh]);
  useEffect(() => { const timer = window.setTimeout(() => { if (tenantID) void load(tenantID); }, 0); const active = sequence; return () => { window.clearTimeout(timer); active.current++; }; }, [tenantID, load]);
  async function change(action: "schedule" | "withdraw") {
    const currentView = view?.generation === generation && view.subscription.tenant_id === tenant ? view.subscription : null;
    if (!currentView || locked.current || !csrfToken || !generation || !canManage) return;
    locked.current = true; setBusy(true); setMessage(""); const current = ++sequence.current;
    const fingerprint = `${action}|${tenant}|${currentView.version}|${generation}`;
    if (key.current?.fingerprint !== fingerprint) key.current = {fingerprint,value:crypto.randomUUID()};
    const idempotency = key.current.value;
    try {
      const route = `/api/platform/subscriptions/${tenant}/cancellation${action === "withdraw" ? "/withdraw" : ""}`;
      const response = await fetch(route, { method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": idempotency, "X-Bidora-Platform-Context-Generation": String(generation) }, body: JSON.stringify({ expected_version: currentView.version }) });
      if (current !== sequence.current) return;
      if (response.status === 401) { refresh(); return; }
      const receipt = response.ok ? parsePlatformSubscriptionReceipt(await response.json(), tenant) : null;
      if (!receipt) { setMessage(response.status === 409 ? "Subscription changed. Reload it before trying again." : response.status === 403 ? "Verified Billing management access is required." : "The outcome could not be confirmed. Retry without changing the request."); return; }
      key.current = null; setConfirm(null);
      setInventoryRevision((current) => current + 1);
      // A historical idempotent receipt is not a current subscription read.
      const latest = await load(tenant);
      setMessage(latest ? latest.cancel_at ? `Cancellation is scheduled for ${new Date(latest.cancel_at).toLocaleString()}. No payment or refund was executed.` : "No cancellation is currently scheduled. No payment or refund was executed." : "Your request was recorded, but current subscription status could not be reloaded. Refresh before another change.");
    } catch { if (current === sequence.current) setMessage("The outcome could not be confirmed. Retry without changing the request."); }
    finally { locked.current = false; setBusy(false); }
  }
  const visible = view?.generation === generation && view.subscription.tenant_id === tenant && canRead ? view.subscription : null;
  return <section className="space-y-3" aria-label="Platform subscriptions"><div><h2 className="font-semibold">Subscription oversight</h2><p className="text-xs text-muted-foreground">Review a customer’s current plan and schedule or withdraw cancellation at the end of their term. These actions do not issue payments or refunds.</p></div>
    {!tenantID && <PlatformSubscriptionInventory revision={inventoryRevision} onSelect={(id) => { sequence.current++; key.current = null; setView(null); setTenant(id); void load(id); }} />}
    {!tenantID && <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => { event.preventDefault(); void load(tenant); }}><PlatformReferencePicker id="subscription-organisation" kind="organizations" label="Organisation" value={tenant} disabled={busy} onChange={(id) => { sequence.current++; key.current = null; setView(null); setMessage(""); setTenant(id); }} /><Button type="submit" variant="outline" disabled={!canRead || !uuid.test(tenant) || busy}>Read subscription</Button></form>}
    {visible && <div className="space-y-2 rounded-lg border bg-card p-4 text-sm"><p className="font-medium">{visible.plan_key} · {visible.state.replaceAll("_", " ")} · Version {visible.version}</p><p className="text-muted-foreground">Catalogue {visible.catalogue_version} · {visible.cadence} · Access {visible.access_mode.replaceAll("_", " ")}</p><p className="text-muted-foreground">Term ends {visible.commercial_period_end} UTC · Cancellation {visible.cancel_at ? `${visible.cancel_at} UTC` : "not scheduled"}</p>
      {canManage && <div className="flex flex-wrap gap-2">{!visible.cancel_at && ["TRIAL_ACTIVE", "ACTIVE"].includes(visible.state) && <Button type="button" disabled={busy || !csrfToken} onClick={() => setConfirm("schedule")}>Schedule boundary cancellation</Button>}{visible.cancel_at && <Button type="button" variant="outline" disabled={busy || !csrfToken} onClick={() => setConfirm("withdraw")}>Withdraw cancellation</Button>}</div>}
    </div>}
    <AlertDialog open={confirm!==null} onOpenChange={open=>{if(!open&&!busy)setConfirm(null)}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{confirm==="schedule"?"Cancel at the end of this term?":"Keep this subscription active?"}</AlertDialogTitle><AlertDialogDescription>{confirm==="schedule"?"The organisation keeps access until its current term ends. No refund or immediate cancellation is performed.":"The scheduled cancellation will be removed. Existing plan and payment terms remain unchanged."}</AlertDialogDescription></AlertDialogHeader>{message && <p role="alert" className="text-sm text-destructive">{message}</p>}<AlertDialogFooter><AlertDialogCancel disabled={busy}>Back</AlertDialogCancel><Button disabled={busy||!csrfToken||!confirm} onClick={()=>{if(confirm)void change(confirm)}}>{busy?"Saving…":"Confirm"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
    <Button variant="outline" disabled={busy || !tenant} onClick={() => void load(tenant)}>Refresh subscription</Button>
    {message && confirm === null && <p role="status" className="text-sm text-muted-foreground">{message}</p>}
  </section>;
}
