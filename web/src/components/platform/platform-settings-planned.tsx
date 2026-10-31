"use client";

import Link from "next/link";
import { ClockIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSession } from "@/components/session/session-provider";
import { plannedSettings, type PlannedSettingsKind } from "@/lib/platform-settings-planned-contract";

export function PlatformSettingsPlanned({ section }: { section: PlannedSettingsKind }) {
  const details = plannedSettings[section];
  const { resolution } = useSession();
  const related = details.related;
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">{details.title}</h1><p className="type-lead text-muted-foreground">{details.introduction}</p></div>
    <div className="max-w-2xl space-y-5">
      <section className="rounded-lg border bg-card p-5" aria-labelledby={`${section}-current`}>
        <div className="flex flex-wrap items-start justify-between gap-2"><h2 id={`${section}-current`} className="font-medium">What is available</h2><span className="rounded-md border px-2 py-1 text-xs text-muted-foreground">View only</span></div>
        <dl className="mt-3 divide-y">{details.fields.map((field) => <div key={field.label} className="grid gap-1 py-3 text-sm sm:grid-cols-[minmax(9rem,1fr)_minmax(0,2fr)] sm:gap-4"><dt className="font-medium">{field.label}</dt><dd className="text-muted-foreground">{field.value}</dd></div>)}</dl>
        {related && (!related.capability || resolution.capabilities.includes(related.capability)) ? <Link href={related.href} className="mt-3 inline-block text-sm font-medium text-primary underline-offset-2 hover:underline focus-visible:underline">{related.label} →</Link> : null}
      </section>
      <section className="rounded-lg border bg-card p-5" aria-labelledby={`${section}-changes`}>
        <h2 id={`${section}-changes`} className="font-medium">Changes</h2>
        <p className="mt-2 text-sm text-muted-foreground">{details.pendingOperation}</p>
        <Button type="button" disabled className="mt-4"><ClockIcon aria-hidden="true" />{details.pendingAction} · Unavailable</Button>
        <p className="mt-2 text-xs text-muted-foreground">No changes can be saved from this page.</p>
      </section>
    </div>
  </main>;
}
