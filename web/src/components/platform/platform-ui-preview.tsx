"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useMemo, useState } from "react";
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon, BanIcon, CheckIcon, ChevronLeftIcon, ChevronRightIcon, DownloadIcon, FileChartColumnIcon, PencilIcon, PlusIcon, RotateCcwIcon, SaveIcon, SearchIcon, XIcon } from "lucide-react";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformMetricCard } from "@/components/platform/platform-metric-card";
import { DemoTrendChart } from "@/components/platform/platform-demo-workspace";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { normalizeNumericInput } from "@/lib/numeric-input";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { demoAIUsage, demoNotifications, demoRows, demoTickets, platformDemoEnabled, type DemoModule, type DemoRow } from "@/lib/platform-demo-data";

type Module = DemoModule;
type SampleRow = DemoRow;
const demos: Record<Module, { title: string; description: string; tabs: string[]; columns: string[]; action?: string; rows: SampleRow[]; measures?: string[] }> = {
  plans: { title: "Plans & Subscriptions", description: "Plan and subscription workspace", tabs: ["Plans", "Subscriptions"], columns: ["Plan", "Billing period", "Price", "Usage limit", "Status"], action: "Create Plan", rows: [] },
  ai: { title: "AI Usage", description: "Usage across organisations", tabs: ["Usage"], columns: ["Organisation", "AI Operation", "Usage", "Estimated Cost", "Date"], action: "Export", rows: [], measures: ["Total Usage", "Estimated Cost", "Active Organisations"] },
  communications: { title: "Communications", description: "Notification delivery history", tabs: ["Notifications"], columns: ["Recipient", "Notification Type", "Channel", "Status", "Date"], rows: [], measures: ["Sent", "Delivered", "Failed", "Pending"] },
  support: { title: "Support", description: "Manage support requests", tabs: ["Open", "All tickets"], columns: ["Ticket Number", "Subject", "Organisation", "Priority", "Assigned To", "Status", "Updated Date"], action: "New Ticket", rows: [], measures: ["Open Tickets", "Overdue Tickets"] },
  reports: { title: "Reports", description: "Choose filters to prepare a report", tabs: ["Preview"], columns: ["Category", "Period", "Organisation", "Status"], action: "Generate Report", rows: [] },
  audit: { title: "Audit Trails", description: "Historical platform events", tabs: ["Events"], columns: ["Date & Time", "User", "Action", "Module", "Affected Record", "Result"], rows: [] },
};
const categories = ["Tenderpreneur Registrations", "Subscription Activity", "Revenue & Payments", "AI Usage", "Communications", "Support Performance"];
const inputClass = "h-10 rounded-md border bg-background px-3 text-sm";

export function PlatformUIPreview({ module }: { module: Module }) {
  const definition = demos[module];
  const [tab, setTab] = useState(definition.tabs[0]);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(5);
  const [sort, setSort] = useState<{ index: number; ascending: boolean } | null>(null);
  const [drawer, setDrawer] = useState<SampleRow | null>(null);
  const [dialog, setDialog] = useState(false);
  const [confirm, setConfirm] = useState<string | null>(null);
  const [category, setCategory] = useState(categories[0]);
  const [period, setPeriod] = useState("");
  const [endDate, setEndDate] = useState("");
  const [draftName, setDraftName] = useState("");
  const [draftDetail, setDraftDetail] = useState("");
  const columns = module === "plans" && tab === "Subscriptions" ? ["Organisation", "Plan", "Billing Period", "Renewal Date", "Status"] : definition.columns;
  const rows = useMemo(() => {
    const source = platformDemoEnabled ? demoRows(module, tab) : []; 
    const filtered = source.filter((row) => (!status || row.status === status) && (!search || row.cells.join(" ").toLowerCase().includes(search.toLowerCase())));
    return sort ? filtered.sort((a, b) => (sort.ascending ? 1 : -1) * a.cells[sort.index].localeCompare(b.cells[sort.index], undefined, { numeric: true })) : filtered;
  }, [module, search, sort, status, tab]);
  const statuses = platformDemoEnabled ? [...new Set(demoRows(module, tab).map((row) => row.status))] : [];
  const measures: Record<string, string> = platformDemoEnabled ? {
    "Total Usage": String(demoAIUsage.reduce((sum, row) => sum + row.units, 0)),
    "Estimated Cost": `KES ${demoAIUsage.reduce((sum, row) => sum + row.cost, 0).toLocaleString("en-KE")}`,
    "Active Organisations": String(new Set(demoAIUsage.map((row) => row.organisationId)).size),
    Sent: String(demoNotifications.length), Delivered: String(demoNotifications.filter((row) => row.status === "Delivered").length),
    Failed: String(demoNotifications.filter((row) => row.status === "Failed").length), Pending: String(demoNotifications.filter((row) => row.status === "Pending").length),
    "Open Tickets": String(demoTickets.filter((row) => row.status === "Open").length),
  } : {};
  const pages = Math.max(1, Math.ceil(rows.length / pageSize));
  const visible = rows.slice((Math.min(page, pages) - 1) * pageSize, Math.min(page, pages) * pageSize);
  const changeTab = (value: string) => { setTab(value); setStatus(""); setPage(1); setSort(null); };
  const clearFilters = () => { setSearch(""); setStatus(""); setPage(1); };

  return <main className="app-page min-w-0 space-y-5">
    <div className="app-page-header flex-row flex-wrap items-start justify-between gap-3">
      <div><h1 className="type-title">{definition.title}</h1><p className="text-sm text-muted-foreground">{definition.description}</p></div>
      {definition.action ? <Button type="button" onClick={() => setDialog(true)}>{module === "plans" || module === "support" ? <PlusIcon aria-hidden="true" className="size-4" /> : module === "reports" ? <FileChartColumnIcon aria-hidden="true" className="size-4" /> : <DownloadIcon aria-hidden="true" className="size-4" />}{definition.action}</Button> : null}
    </div>

    {definition.measures ? <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{definition.measures.map((measure) => <PlatformMetricCard key={measure} label={measure} value={measures[measure]} />)}</div> : null}
    {platformDemoEnabled && ["plans", "ai", "communications", "support"].includes(module) ? <DemoTrendChart area={module} /> : null}
    {module === "ai" && !platformDemoEnabled ? <div className="grid gap-3 lg:grid-cols-2"><section className="rounded-lg border bg-card p-4"><h2 className="text-sm font-medium">Usage trend</h2><p className="mt-3 text-sm text-muted-foreground">No usage data available.</p></section><section className="rounded-lg border bg-card p-4"><h2 className="text-sm font-medium">Usage by organisation</h2><p className="mt-3 text-sm text-muted-foreground">No organisation totals available.</p></section></div> : null}
    {module === "reports" ? <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <label className="grid gap-1 text-sm">Report category<PlatformSelect value={category} onChange={(event) => setCategory(event.target.value)}>{categories.map((item) => <option key={item}>{item}</option>)}</PlatformSelect></label>
      <label className="grid gap-1 text-sm">From<input type="date" className={inputClass} value={period} onChange={(event) => { const next = event.target.value; setPeriod(next); setEndDate((old) => old && old < next ? "" : old); }} /></label>
      <label className="grid gap-1 text-sm">To<input type="date" className={inputClass} value={endDate} min={period || undefined} onChange={(event) => setEndDate(event.target.value)} /></label>
    </div> : null}
    {definition.tabs.length > 1 ? <div role="tablist" aria-label={`${definition.title} views`} className="flex gap-2 border-b">{definition.tabs.map((item) => <button key={item} role="tab" aria-selected={tab === item} type="button" onClick={() => changeTab(item)} className={`px-3 py-2 text-sm ${tab === item ? "border-b-2 border-primary font-medium" : "text-muted-foreground"}`}>{item}</button>)}</div> : null}
    <div className="flex flex-wrap items-end gap-3">
      <label className="grid min-w-48 flex-1 gap-1 text-sm">Search<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" value={search} onChange={(event) => { setSearch(event.target.value); setPage(1); }} placeholder="Search records" /></div></label>
      <label className="grid gap-1 text-sm">Status<PlatformSelect value={status} onChange={(event) => { setStatus(event.target.value); setPage(1); }}><option value="">All statuses</option>{statuses.map((item) => <option key={item} value={item}>{item}</option>)}</PlatformSelect></label>
      {search || status ? <Button type="button" variant="outline" onClick={clearFilters}><XIcon aria-hidden="true" />Clear Filters</Button> : null}
    </div>
    <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-muted/40"><tr>{columns.map((column, index) => <th key={column} scope="col" aria-sort={sort?.index === index ? sort.ascending ? "ascending" : "descending" : undefined} className="whitespace-nowrap px-4 py-3 font-medium text-muted-foreground"><button type="button" aria-label={`Sort by ${column}`} onClick={() => { setSort((old) => ({ index, ascending: old?.index === index ? !old.ascending : true })); setPage(1); }} className="inline-flex items-center gap-1 rounded-sm text-left hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring">{column}{sort?.index === index ? sort.ascending ? <ArrowUpIcon aria-hidden="true" className="size-3" /> : <ArrowDownIcon aria-hidden="true" className="size-3" /> : <ArrowUpDownIcon aria-hidden="true" className="size-3 opacity-50" />}</button></th>)}<th scope="col" className="px-4 py-3 font-medium text-muted-foreground">Actions</th></tr></thead><tbody>{visible.map((row) => <tr key={row.id} className="border-t">{row.cells.map((cell, i) => <td key={`${row.id}-${i}`} className="px-4 py-3">{columns[i] === "Status" || columns[i] === "Result" ? <PlatformStatus value={cell} /> : cell}</td>)}<td className="px-4 py-3"><PlatformRowActions label={row.cells[0]} actions={[{ label: "View details", onSelect: () => setDrawer(row) }]} /></td></tr>)}</tbody></table>{!visible.length ? <div className="border-t px-4 py-8 text-center text-sm text-muted-foreground">No records found.{search || status ? <Button type="button" variant="link" onClick={clearFilters}><XIcon aria-hidden="true" />Clear Filters</Button> : null}</div> : null}</div>
    <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-muted-foreground"><label className="flex items-center gap-2">Rows<PlatformSelect value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setPage(1); }}><option value={5}>5</option><option value={10}>10</option></PlatformSelect></label><div className="flex items-center gap-2"><span>Page {Math.min(page, pages)} of {pages}</span><Button type="button" variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}><ChevronLeftIcon aria-hidden="true" />Previous</Button><Button type="button" variant="outline" size="sm" disabled={page >= pages} onClick={() => setPage((value) => value + 1)}><ChevronRightIcon aria-hidden="true" />Next</Button></div></div>
    {module === "reports" ? <div className="flex gap-2"><Button type="button" variant="outline" disabled><DownloadIcon aria-hidden="true" />Export CSV</Button><Button type="button" variant="outline" disabled><DownloadIcon aria-hidden="true" />Export PDF</Button></div> : null}
    <Sheet open={drawer !== null} onOpenChange={(open) => { if (!open) setDrawer(null); }}><SheetContent className="w-full sm:max-w-md"><SheetHeader><SheetTitle>{drawer?.cells[0] ?? "Details"}</SheetTitle><SheetDescription>Example record</SheetDescription></SheetHeader><dl className="space-y-4 px-4">{drawer?.cells.map((cell, i) => <div key={columns[i]}><dt className="text-xs text-muted-foreground">{columns[i]}</dt><dd className="text-sm">{cell}</dd></div>)}</dl>{module === "communications" ? <p className="px-4 text-sm text-muted-foreground">Delivery attempts · None to show.</p> : null}{module === "plans" && tab === "Subscriptions" ? <div className="flex flex-wrap gap-2 px-4"><Button type="button" variant="outline" onClick={() => { setDrawer(null); setConfirm("Change Plan"); }}><PencilIcon aria-hidden="true" />Change Plan</Button><Button type="button" variant="destructive" onClick={() => { setDrawer(null); setConfirm("Cancel Subscription"); }}><BanIcon aria-hidden="true" />Cancel Subscription</Button></div> : null}{module === "communications" ? <div className="px-4"><Button type="button" variant="outline" onClick={() => { setDrawer(null); setConfirm("Retry Delivery"); }}><RotateCcwIcon aria-hidden="true" />Retry</Button></div> : null}</SheetContent></Sheet>
    <AlertDialog open={confirm !== null} onOpenChange={(open) => { if (!open) setConfirm(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{confirm}?</AlertDialogTitle><AlertDialogDescription>This action isn’t available in demo mode.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><Button type="button" variant={confirm === "Cancel Subscription" ? "destructive" : "default"} disabled><CheckIcon aria-hidden="true" />Confirm</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
    <AlertDialog open={dialog} onOpenChange={(open) => { setDialog(open); if (!open) { setDraftName(""); setDraftDetail(""); } }}><AlertDialogContent className="max-h-[90vh] overflow-y-auto"><AlertDialogHeader><AlertDialogTitle>{definition.action}</AlertDialogTitle><AlertDialogDescription>Changes aren’t saved or sent.</AlertDialogDescription></AlertDialogHeader><div className="space-y-3">
      {module === "reports" || module === "ai" ? <p className="text-sm text-muted-foreground">{module === "reports" ? `${category}${period ? ` · From ${period}` : ""}${endDate ? ` · To ${endDate}` : ""}. No report is available to download.` : "No usage file is available."}</p> : <>
        <label className="grid gap-1 text-sm">{module === "plans" ? "Plan name" : "Subject"}<Input value={draftName} onChange={(event) => setDraftName(event.target.value)} maxLength={100} placeholder={module === "plans" ? "Plan name" : "Ticket subject"} /></label>
        <label className="grid gap-1 text-sm">{module === "plans" ? "Price" : "Description"}<Input type={module === "plans" ? "number" : "text"} inputMode={module === "plans" ? "decimal" : "text"} min={module === "plans" ? 0 : undefined} step={module === "plans" ? "0.01" : undefined} value={draftDetail} onChange={(event) => setDraftDetail(event.target.value)} placeholder={module === "plans" ? "Amount" : "What needs help?"} /></label>
        {module === "plans" ? <><label className="grid gap-1 text-sm">Billing period<PlatformSelect><option>Monthly</option><option>Annual</option></PlatformSelect></label><label className="grid gap-1 text-sm">Usage limit<Input type="number" min="0" placeholder="Optional" onChange={event => { event.currentTarget.value = normalizeNumericInput(event.currentTarget.value); }} /></label><label className="grid gap-1 text-sm">Status<PlatformSelect><option>Inactive</option><option>Active</option></PlatformSelect></label></> : null}
      </>}
    </div><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><Button type="button" disabled>{module === "reports" ? <FileChartColumnIcon aria-hidden="true" /> : module === "ai" ? <DownloadIcon aria-hidden="true" /> : <SaveIcon aria-hidden="true" />}{module === "reports" ? "Generate Report" : module === "ai" ? "Export" : "Save"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </main>;
}
