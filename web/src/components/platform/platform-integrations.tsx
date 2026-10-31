"use client";

import Link from "next/link";
import { MailIcon, SparklesIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";

export function PlatformIntegrations() {
  const { resolution } = useSession();
  if (!resolution.capabilities.includes("platform.integration.read")) return <main className="app-page"><Alert role="alert" variant="destructive"><AlertTitle>Integrations unavailable</AlertTitle><AlertDescription>Access to integration settings is not available in this context.</AlertDescription></Alert></main>;
  return <main className="app-page">
    <div className="app-page-header"><h1 className="type-title">Settings</h1><p className="type-lead text-muted-foreground">Review integration settings through their governed workflows. Provider secrets are never shown here.</p></div>
    <section aria-label="Available integrations" className="grid gap-3 sm:grid-cols-2">
      <Link href="/platform/settings/smtp" className="flex min-h-28 items-start gap-3 rounded-lg border bg-card p-4 text-primary underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"><MailIcon aria-hidden="true" className="size-5 shrink-0"/><span><span className="block font-medium">Email delivery (SMTP)</span><span className="mt-1 block text-sm text-muted-foreground">Review redacted configuration and version history; manage or approve only with current authority.</span></span></Link>
      <Link href="/platform/settings/openai" className="flex min-h-28 items-start gap-3 rounded-lg border bg-card p-4 text-primary underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"><SparklesIcon aria-hidden="true" className="size-5 shrink-0"/><span><span className="block font-medium">OpenAI setting status</span><span className="mt-1 block text-sm text-muted-foreground">View redacted managed versions. Provider configuration and acceptance are separate.</span></span></Link>
    </section>
  </main>;
}
