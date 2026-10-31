"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import Link from "next/link";
import { useState } from "react";
import { SendIcon, PlusIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { demoOrganisations, demoTickets, platformDemoEnabled } from "@/lib/platform-demo-data";

export function PlatformSupportPreviewDetail() {
  const [tab, setTab] = useState<"Conversation" | "Internal Notes">("Conversation");
  const [draft, setDraft] = useState("");
  if (!platformDemoEnabled) return <main className="app-page"><h1 className="type-title">Support</h1><p className="mt-3 text-sm text-muted-foreground">This example is available in demo mode only.</p></main>;
  const ticket = demoTickets[0];
  return <main className="app-page space-y-5">
    <Link href="/platform/support" className="text-sm text-primary underline underline-offset-4">Back to support</Link>
    <div className="app-page-header"><p className="text-xs text-muted-foreground">{ticket.id} · Demo mode</p><h1 className="type-title">{ticket.subject}</h1><p className="text-sm text-muted-foreground">{demoOrganisations.find((item) => item.id === ticket.organisationId)?.name ?? "Example organisation"}</p></div>
    <div className="grid gap-3 sm:grid-cols-3"><label className="grid gap-1 text-sm">Assigned To<PlatformSelect disabled><option>Unassigned</option></PlatformSelect></label><label className="grid gap-1 text-sm">Status<PlatformSelect disabled><option>{ticket.status}</option></PlatformSelect></label><label className="grid gap-1 text-sm">Priority<PlatformSelect disabled><option>Normal</option></PlatformSelect></label></div>
    <div role="tablist" aria-label="Ticket details" className="flex gap-4 border-b">{(["Conversation", "Internal Notes"] as const).map((item) => <button key={item} type="button" role="tab" aria-selected={tab === item} onClick={() => setTab(item)} className={`py-2 text-sm ${tab === item ? "border-b-2 border-primary font-medium" : "text-muted-foreground"}`}>{item}</button>)}</div>
    <section aria-label={tab} className="min-h-28 rounded-lg border bg-card p-4 text-sm text-muted-foreground">{tab === "Conversation" ? ticket.message : "No internal notes have been recorded."}</section>
    <label className="grid gap-1 text-sm">{tab === "Conversation" ? "Reply" : "Note"}<textarea value={draft} onChange={(event) => setDraft(event.target.value)} maxLength={4000} placeholder={tab === "Conversation" ? "Write a reply" : "Add an internal note"} className="min-h-28 rounded-md border bg-background p-3" /></label>
    <div className="flex justify-end gap-2"><Button type="button" variant="outline" onClick={() => setDraft("")}>Cancel</Button><Button type="button" disabled>{tab === "Conversation" ? <SendIcon aria-hidden="true" /> : <PlusIcon aria-hidden="true" />}{tab === "Conversation" ? "Send Reply" : "Add Note"}</Button><Button type="button" variant="outline" disabled>Resolve</Button></div>
  </main>;
}
