"use client";

import { useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { ChevronLeftIcon, ChevronRightIcon, SearchIcon } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useSession } from "@/components/session/session-provider";
import { PlatformSelect } from "@/components/platform/platform-select";
import { PlatformStatus } from "@/components/platform/platform-status";
import { demoAdministrators, demoAdjustments, demoAIUsage, demoAudit, demoInvoices, demoNotifications, demoOrganisations, demoPayments, demoSettings, demoSubscriptions, demoTickets, platformDemoEnabled, type DemoModule } from "@/lib/platform-demo-data";

type Row = { id: string; name: string; detail: string; status: string; date: string };
const org = (id: string) => demoOrganisations.find((item) => item.id === id)?.name ?? "—";
function rowsFor(area: string): Row[] {
  switch (area) {
    case "tenderpreneurs": case "organizations": return demoOrganisations.map((item) => ({ id: item.id, name: item.name, detail: item.country, status: item.status, date: item.createdAt }));
    case "users": case "administrators": return demoAdministrators.map((item) => ({ id: item.id, name: item.name, detail: item.email, status: item.status, date: "—" }));
    case "billing": case "commercial": return [...demoInvoices.map((item) => ({ id: item.id, name: org(item.organisationId), detail: `Invoice · KES ${item.amount.toLocaleString("en-KE")}`, status: item.status, date: item.date })), ...demoPayments.map((item) => ({ id: item.id, name: org(item.organisationId), detail: `Payment · KES ${item.amount.toLocaleString("en-KE")}`, status: item.status, date: item.date })), ...demoAdjustments.map((item) => ({ id: item.id, name: org(demoInvoices.find((invoice) => invoice.id === item.invoiceId)?.organisationId ?? ""), detail: `Adjustment · KES ${item.amount}`, status: "Recorded", date: item.date }))];
    default: return [];
  }
}
export function DemoTrendChart({ area }: { area: "overview" | DemoModule | "billing" }) {
  if (!platformDemoEnabled) return null;
  const source = area === "ai" ? demoAIUsage.map((item) => item.units) : area === "communications" ? ["Sent", "Delivered", "Failed", "Pending"].map((state) => demoNotifications.filter((item) => item.status === state).length) : area === "support" ? ["Open", "In progress", "Resolved"].map((state) => demoTickets.filter((item) => item.status === state).length) : area === "plans" ? demoSubscriptionsByWeek() : area === "overview" || area === "billing" ? revenueByWeek() : [];
  if (!source.length) return null;
  const max = Math.max(...source, 1);
  const units = area === "overview" || area === "billing" ? "KES " : "";
  return <section aria-label={`${area} demo trend`} className="rounded-lg border bg-card p-4"><p className="text-sm font-medium">{area === "ai" ? "Usage by organisation" : area === "support" ? "Ticket activity" : area === "communications" ? "Delivery status" : area === "plans" ? "Subscription growth" : "Paid revenue by week"}</p>
    <div className="mt-4 flex gap-2"><div className="flex h-36 shrink-0 flex-col justify-between text-right text-[10px] text-muted-foreground"><span>{units}{max.toLocaleString("en-KE")}</span><span>0</span></div><div className="flex h-36 min-w-0 flex-1 items-end gap-1.5 border-b pb-1 sm:gap-2">{source.map((value, index) => <div key={index} className="group relative flex h-full min-w-0 flex-1 items-end" title={`${chartLabel(area, index)}: ${units}${value.toLocaleString("en-KE")}`}><span role="img" aria-label={`${chartLabel(area, index)}: ${units}${value.toLocaleString("en-KE")}`} className="w-full rounded-t-sm bg-primary/65 transition-colors group-hover:bg-primary" style={{ height: `${Math.max(3, value / max * 100)}%` }} /></div>)}</div></div>
    <div className="mt-2 flex justify-between gap-2 text-xs text-muted-foreground"><span>{chartLabel(area, 0)}</span><span>{chartLabel(area, source.length - 1)}</span></div>
  </section>;
}
function chartLabel(area: string, index: number) { return area === "communications" ? ["Sent", "Delivered", "Failed", "Pending"][index] : area === "support" ? ["Open", "In progress", "Resolved"][index] : area === "ai" ? org(demoAIUsage[index]?.organisationId ?? "") : area === "plans" ? `Week ${index + 1}` : `Week ${index + 1}`; }
function revenueByWeek() { return [0, 1, 2, 3, 4, 5].map((week) => demoPayments.filter((item) => Math.floor((Number(item.date.slice(-2)) - 1) / 7) === week).reduce((sum, item) => sum + item.amount, 0)); }
function demoSubscriptionsByWeek() { return [0, 1, 2, 3, 4, 5].map((week) => demoSubscriptions.filter((item) => Math.floor((Number(item.date.slice(-2)) - 1) / 7) <= week).length); }

export function PlatformDemoWorkspace() {
  const pathname = usePathname();
  const { resolution } = useSession();
  const area = pathname.split("/")[2] ?? "overview";
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(0);
  const [sortDescending, setSortDescending] = useState(false);
  const [pageSize, setPageSize] = useState(10);
  const rows = useMemo(() => rowsFor(area), [area]);
  if (!platformDemoEnabled) return null;
  const capabilities = new Set(resolution.capabilities);
  if (["tenderpreneurs", "organizations"].includes(area) && !capabilities.has("platform.tenant.read")) return null;
  if (["users", "administrators"].includes(area) && !capabilities.has("platform.admin.read")) return null;
  if (["billing", "commercial"].includes(area) && !capabilities.has("platform.invoice.read") && !capabilities.has("platform.payment.read")) return null;
  if (["settings", "integrations"].includes(area) && !["platform.admin.read", "platform.integration.read", "platform.reference.read"].some((capability) => capabilities.has(capability))) return null;
  if (["plans", "ai-usage", "communications", "support", "reports", "audit", "audit-trails"].includes(area)) return null; // These modules show the same fixtures in their own workspace.
  if (area === "settings" || area === "integrations") return <details className="mb-4 rounded-lg border bg-card p-4 text-sm"><summary className="cursor-pointer font-medium">Example settings</summary><dl className="mt-3 grid gap-2 sm:grid-cols-2">{Object.entries(demoSettings).map(([name, value]) => <div key={name}><dt className="text-muted-foreground">{name.replace(/[A-Z]/g, (letter) => ` ${letter.toLowerCase()}`)}</dt><dd>{value}</dd></div>)}</dl></details>;
  const statuses = [...new Set(rows.map((row) => row.status))];
  const filtered = rows.filter((row) => (!status || row.status === status) && (!query || `${row.name} ${row.detail} ${row.id}`.toLowerCase().includes(query.toLowerCase()))).sort((a, b) => (sortDescending ? -1 : 1) * a.name.localeCompare(b.name));
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const visible = filtered.slice(Math.min(page, pages - 1) * pageSize, (Math.min(page, pages - 1) + 1) * pageSize);
  return <section aria-label="Demo workspace" className="mt-4 space-y-4 rounded-lg border border-primary/30 bg-primary/5 p-4 text-sm">
    <p className="font-semibold">Example {area === "overview" ? "platform activity" : area === "billing" || area === "commercial" ? "financial records" : area === "users" ? "administrators" : "organisations"}</p>
    {area === "overview" ? <><p>{demoOrganisations.length} organisations · {demoAdministrators.filter((item) => item.status === "Active").length} active administrators · {demoTickets.filter((item) => item.status === "Open").length} open tickets</p><DemoTrendChart area="overview" /><p className="text-muted-foreground">Recent example: {demoAudit.at(-1)?.action} · {demoAudit.at(-1)?.date}</p></> : <>
      <div className="flex flex-wrap items-end gap-3"><label className="min-w-40 flex-1">Search<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" value={query} onChange={(event) => { setQuery(event.target.value); setPage(0); }} placeholder="Search examples" /></div></label><label>Status<PlatformSelect value={status} onChange={(event) => { setStatus(event.target.value); setPage(0); }}><option value="">All statuses</option>{statuses.map((item) => <option key={item}>{item}</option>)}</PlatformSelect></label><label>Rows<PlatformSelect value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setPage(0); }}><option value={10}>10</option><option value={20}>20</option></PlatformSelect></label></div>
      <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[460px] text-left"><thead className="bg-muted/40"><tr><th className="px-3 py-2"><button type="button" onClick={() => { setSortDescending((value) => !value); setPage(0); }} className="font-medium underline-offset-2 hover:underline">Name {sortDescending ? "↓" : "↑"}</button></th><th className="px-3 py-2">Detail</th><th className="px-3 py-2">Status</th><th className="px-3 py-2">Date</th></tr></thead><tbody>{visible.map((row) => <tr key={row.id} className="border-t hover:bg-muted/30"><td className="px-3 py-2 font-medium">{row.name}</td><td className="px-3 py-2">{row.detail}</td><td className="px-3 py-2"><PlatformStatus value={row.status} /></td><td className="px-3 py-2">{row.date}</td></tr>)}</tbody></table>{!visible.length && <p className="p-4 text-muted-foreground">No matching examples.</p>}</div>
      <div className="flex flex-wrap items-center justify-between gap-2 text-muted-foreground"><span>{filtered.length} example records · Page {Math.min(page, pages - 1) + 1} of {pages}</span><div className="flex gap-2"><Button type="button" variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((value) => value - 1)}><ChevronLeftIcon aria-hidden="true" />Previous</Button><Button type="button" variant="outline" size="sm" disabled={page >= pages - 1} onClick={() => setPage((value) => value + 1)}><ChevronRightIcon aria-hidden="true" />Next</Button></div></div>
      {(area === "billing" || area === "commercial") && <DemoTrendChart area="billing" />}
    </>}
  </section>;
}
