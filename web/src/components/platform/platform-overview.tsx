"use client";

import { useSession } from "@/components/session/session-provider";
import { PlatformBillingRecency } from "@/components/platform/platform-billing-recency";
import { PlatformMetricCard } from "@/components/platform/platform-metric-card";
import { PlatformAttention } from "@/components/platform/platform-attention";

const summaries = ["Total Tenderpreneurs", "Active Subscriptions", "Outstanding Payments", "Open Support Tickets"];

export function PlatformOverview() {
  const { resolution } = useSession();
  const capabilities = new Set(resolution.capabilities);
  return <main className="app-page space-y-6">
    <div className="app-page-header"><h1 className="type-title">Overview</h1></div>
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{summaries.map((title) => <PlatformMetricCard key={title} label={title} />)}</div>
    <PlatformAttention />
    <section className="rounded-lg border bg-card"><h2 className="border-b px-4 py-3 text-base font-semibold">Recent activity</h2><div className="overflow-x-auto"><table className="w-full min-w-[400px] text-left text-sm"><thead><tr>{["Date", "Activity", "Area"].map((name) => <th key={name} scope="col" className="px-4 py-2 font-medium text-muted-foreground">{name}</th>)}</tr></thead></table></div><p className="border-t px-4 py-6 text-sm text-muted-foreground">No activity records available.</p></section>
    {capabilities.has("platform.invoice.read") && capabilities.has("platform.payment.read") ? <PlatformBillingRecency /> : null}
  </main>;
}
