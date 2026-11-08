"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { PlatformBillingRecency } from "@/components/platform/platform-billing-recency";
import { PlatformMetricCard } from "@/components/platform/platform-metric-card";
import { PlatformAttention } from "@/components/platform/platform-attention";
import { parseAuditPage, type AuditRecord } from "@/lib/platform-audit-contract";

type Summary = { tenderpreneurs: string | null; active_subscriptions: string | null; outstanding_minor_units: string | null; open_support_cases: string | null; observed_at: string };
const fields = ["tenderpreneurs", "active_subscriptions", "outstanding_minor_units", "open_support_cases"] as const;
function parseSummary(value: unknown): Summary | null {
  const row = (value as { data?: Summary } | null)?.data;
  if (!row || typeof row.observed_at !== "string" || !Number.isFinite(Date.parse(row.observed_at))) return null;
  return fields.every((key) => row[key] === null || typeof row[key] === "string" && /^\d{1,40}$/.test(row[key]!)) ? row : null;
}
const actionLabel = (action: string) => action.replace(/([a-z])([A-Z])/g, "$1 $2");
export function PlatformOverview() {
  const { resolution } = useSession();
  return <OverviewWorkspace key={`${resolution.platform_context_generation}|${resolution.capabilities.join("|")}`} />;
}
function OverviewWorkspace() {
  const { resolution, refresh } = useSession();
  const capabilities = new Set(resolution.capabilities);
  const generation = resolution.platform_context_generation;
  const canAudit = capabilities.has("platform.audit.read");
  const [summary, setSummary] = useState<Summary | null>(null);
  const [activity, setActivity] = useState<AuditRecord[] | null>(null);
  const [error, setError] = useState("");
  const [activityError, setActivityError] = useState("");
  const [busy, setBusy] = useState(true);
  const [revision, setRevision] = useState(0);
  const reload = useCallback(() => setRevision((old) => old + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setBusy(true); setError(""); setActivityError("");
      if (!generation) { setSummary(null); setActivity(null); setBusy(false); return; }
      const headers = { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) };
      const options = { credentials: "include" as const, cache: "no-store" as const, headers, signal: controller.signal };
      await Promise.allSettled([
        (async () => {
          try {
            const response = await fetch("/api/platform/overview", options);
            if (controller.signal.aborted) return;
            if (response.status === 401 || response.status === 409) { refresh(); return; }
            const data = response.ok ? parseSummary(await response.json()) : null;
            if (!controller.signal.aborted) { setSummary(data); if (!data) setError(response.status === 403 ? "Verify your account to view the overview." : "The overview could not be loaded."); }
          } catch { if (!controller.signal.aborted) { setSummary(null); setError("The overview could not be loaded."); } }
        })(),
        ...(canAudit ? [(async () => {
          try {
            const response = await fetch("/api/platform/audit/actions?limit=5&sort=DESC", options);
            if (controller.signal.aborted) return;
            if (response.status === 401 || response.status === 409) { refresh(); return; }
            const page = response.ok ? parseAuditPage(await response.json()) : null;
            if (!controller.signal.aborted) { setActivity(page?.data ?? null); if (!page) setActivityError("Recent activity could not be loaded."); }
          } catch { if (!controller.signal.aborted) { setActivity(null); setActivityError("Recent activity could not be loaded."); } }
        })()] : []),
      ]);
      if (!controller.signal.aborted) setBusy(false);
    }
    void load(); return () => controller.abort();
  }, [generation, canAudit, refresh, revision]);
  const count = (value: string | null | undefined) => value == null ? undefined : BigInt(value).toLocaleString();
  const outstanding = summary?.outstanding_minor_units == null ? undefined : (() => { const n = BigInt(summary.outstanding_minor_units); return `KES ${(n / BigInt(100)).toLocaleString()}.${(n % BigInt(100)).toString().padStart(2, "0")}`; })();
  const metrics = [
    { title: "Total Tenderpreneurs", permission: "platform.tenant.read", href: "/platform/tenderpreneurs", value: count(summary?.tenderpreneurs) },
    { title: "Active Subscriptions", permission: "platform.subscription.read", href: "/platform/plans", value: count(summary?.active_subscriptions) },
    { title: "Outstanding Payments", permission: "platform.invoice.read", href: "/platform/billing?queue=invoices", value: outstanding },
    { title: "Open Support Tickets", permission: "platform.support.case.read", href: "/platform/support", value: count(summary?.open_support_cases) },
  ].filter((row) => capabilities.has(row.permission));
  return <main className="app-page space-y-6">
    <div className="app-page-header flex-row items-start justify-between gap-3"><div><h1 className="type-title">Overview</h1><p className="type-lead text-muted-foreground">Monitor organisations, customer accounts and work needing attention.</p></div><Button variant="outline" disabled={busy} onClick={reload}>{busy ? "Loading…" : "Refresh"}</Button></div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{metrics.map((row) => <Link key={row.title} href={row.href}><PlatformMetricCard label={row.title} value={row.value} /></Link>)}</div>
    {summary && <p className="text-xs text-muted-foreground">As of {new Date(summary.observed_at).toLocaleString()}. Active subscriptions include current trials; outstanding payments are unpaid invoice balances.</p>}
    {!metrics.length && <p className="text-sm text-muted-foreground">Use the navigation to manage the areas assigned to your account.</p>}
    <PlatformAttention />
    {canAudit && <section className="rounded-lg border bg-card"><div className="flex items-center justify-between border-b px-4 py-3"><h2 className="text-base font-semibold">Recent activity</h2><Link href="/platform/audit" className="text-sm text-primary hover:underline">View all activity</Link></div>{activityError ? <p role="alert" className="p-4 text-sm">{activityError}</p> : activity ? activity.length ? <ul className="divide-y">{activity.map((row) => <li key={row.id} className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm"><span>{actionLabel(row.action)}{row.administrator_name ? ` · ${row.administrator_name}` : ""}</span><time dateTime={row.occurred_at} className="text-xs text-muted-foreground">{new Date(row.occurred_at).toLocaleString()}</time></li>)}</ul> : <p className="p-4 text-sm text-muted-foreground">No activity has been recorded yet.</p> : <p role="status" className="p-4 text-sm text-muted-foreground">Loading recent activity…</p>}</section>}
    {capabilities.has("platform.invoice.read") && capabilities.has("platform.payment.read") ? <PlatformBillingRecency /> : null}
  </main>;
}
