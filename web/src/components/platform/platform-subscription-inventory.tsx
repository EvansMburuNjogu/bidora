"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { PlatformSelect } from "@/components/platform/platform-select";
import { useSession } from "@/components/session/session-provider";
import { parsePlatformSubscriptionInventory, platformSubscriptionStates, type PlatformSubscriptionInventoryRow } from "@/lib/platform-subscription-contract";

type Page = { generation: number; filter: string; rows: PlatformSubscriptionInventoryRow[]; next: string | null };
export function PlatformSubscriptionInventory({ onSelect, revision }: { onSelect: (tenantID: string) => void; revision: number }) {
 const { resolution, refresh } = useSession();
 const generation = resolution.platform_context_generation, allowed = resolution.capabilities.includes("platform.subscription.read");
 const [filter, setFilter] = useState(""), [page, setPage] = useState<Page | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState("");
 const sequence = useRef(0);
 const load = useCallback(async (cursor?: string) => {
  if (!generation || !allowed) return;
  const current = ++sequence.current;setBusy(true);setError("");
  const params = new URLSearchParams({ limit: "20" });if (filter) params.set("state", filter);if (cursor) params.set("cursor", cursor);
  try {
   const response = await fetch(`/api/platform/subscriptions?${params}`, { credentials: "include", cache: "no-store", headers: { "X-Bidora-Platform-Context-Generation": String(generation) } });
   if (current !== sequence.current) return;
   if (response.status === 401 || response.status === 409) { setPage(null); refresh(); return; }
   const data = response.ok ? parsePlatformSubscriptionInventory(await response.json()) : null;
   if (!data || data.subscriptions.length > 20 || cursor && data.subscriptions.some((row) => row.subscription_id.toLowerCase() <= cursor.toLowerCase())) { setPage(null); setError(response.status === 403 ? "Verified Billing access is required." : "Subscription inventory could not be confirmed."); return; }
   setPage((old) => ({ generation, filter, rows: cursor && old?.generation === generation && old.filter === filter && old.next === cursor ? [...old.rows, ...data.subscriptions] : data.subscriptions, next: data.next_cursor }));
  } catch { if (current === sequence.current) { setPage(null); setError("Subscription inventory is unavailable."); } }
  finally { if (current === sequence.current) setBusy(false); }
 }, [generation, allowed, filter, refresh]);
 useEffect(() => { const timer = window.setTimeout(() => { void load(); }, 0); const ref = sequence; return () => { window.clearTimeout(timer); ref.current++; }; }, [load, revision]);
 const visible = page?.generation === generation && page.filter === filter && allowed ? page : null;
 if (!allowed || !generation) return <p role="status" className="text-sm text-muted-foreground">Platform subscription read permission is required.</p>;
 return <section aria-label="Subscription inventory" className="space-y-3">
  <div className="flex flex-wrap items-end gap-3"><div><h3 className="font-semibold">Subscription inventory</h3><p className="text-xs text-muted-foreground">Customer plans, current terms and scheduled cancellations.</p></div><label className="grid gap-1 text-sm">State<PlatformSelect value={filter} disabled={busy} onChange={(event) => { sequence.current++; setPage(null); setFilter(event.target.value); }}><option value="">All states</option>{platformSubscriptionStates.map((state) => <option key={state} value={state}>{state.replaceAll("_", " ")}</option>)}</PlatformSelect></label><Button type="button" variant="outline" disabled={busy} onClick={() => void load()}>Refresh</Button></div>
  {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
  {!visible && !error && <p role="status" className="text-sm text-muted-foreground">{busy ? "Loading subscriptions…" : "No inventory confirmed."}</p>}
  {visible && <><div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-muted/40"><tr><th scope="col" className="px-4 py-3">Organisation</th><th scope="col" className="px-4 py-3">Plan</th><th scope="col" className="px-4 py-3">State</th><th scope="col" className="px-4 py-3">Revision</th><th scope="col" className="px-4 py-3">Term end (UTC)</th><th scope="col" className="px-4 py-3">Action</th></tr></thead><tbody>{visible.rows.map((row) => <tr key={row.subscription_id} className="border-t"><td className="px-4 py-3">{row.tenant_name || "Organisation"}</td><td className="px-4 py-3">{row.plan_key || "Not yet assigned"}</td><td className="px-4 py-3">{row.state.replaceAll("_", " ")}</td><td className="px-4 py-3">{row.version}</td><td className="px-4 py-3">{row.commercial_period_end ?? "No current term"}</td><td className="px-4 py-3"><Button type="button" size="sm" variant="outline" disabled={!row.has_current_term} onClick={() => onSelect(row.tenant_id)}>Inspect</Button></td></tr>)}</tbody></table>{!visible.rows.length && <p className="border-t p-4 text-sm text-muted-foreground">No subscriptions found for this state.</p>}</div><div className="flex items-center gap-3">{visible.next && <Button type="button" variant="outline" disabled={busy} onClick={() => void load(visible.next ?? undefined)}>Load more</Button>}<span className="text-xs text-muted-foreground">{visible.rows.length} subscriptions shown</span></div></>}
 </section>;
}
