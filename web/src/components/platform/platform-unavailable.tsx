import Link from "next/link";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";

export function PlatformUnavailable({ title, detail, backHref = "/platform", backLabel = "Back to Overview" }: { title: string; detail: string; backHref?: string | null; backLabel?: string }) {
  return <main className="app-page"><div className="app-page-header"><h1 className="type-title">{title}</h1></div>
    <Alert role="status"><AlertTitle>Not available yet</AlertTitle><AlertDescription>{detail} No records or actions are shown until this module is connected to an authorized service.</AlertDescription></Alert>
    {backHref ? <Link href={backHref} className="mt-5 inline-block text-sm text-primary underline-offset-2 hover:underline">{backLabel}</Link> : null}
  </main>;
}
