"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useSession } from "@/components/session/session-provider";
import { settingsSections, settingsSectionVisible } from "@/components/platform/platform-settings-sections";

const legacyPaths: Record<string, string> = {
  "/platform/settings/information": "/platform/settings/platform",
  "/platform/settings/roles": "/platform/settings/access",
};

export function PlatformSettingsNavigation({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { resolution } = useSession();
  const capabilities = new Set(resolution.capabilities);
  const current = legacyPaths[pathname] ?? pathname;
  const visible = settingsSections.filter((section) => settingsSectionVisible(section, capabilities));
  const selected = visible.some((section) => section.href === current) ? current : "/platform/settings/platform";

  return <div className="mx-auto grid w-full max-w-7xl min-w-0 gap-6 pt-[18px] lg:grid-cols-[minmax(12rem,14rem)_minmax(0,1fr)] lg:items-start [&_.app-page]:p-0">
    <div className="min-w-0 lg:sticky lg:top-4">
      <label htmlFor="settings-section" className="mb-2 block text-sm font-medium lg:hidden">Settings section</label>
      <PlatformSelect className="lg:hidden" id="settings-section" value={selected} onChange={(event) => router.push(event.target.value)}>
        {visible.map((section) => <option key={section.href} value={section.href} disabled={!section.ready}>{section.title}{section.ready ? "" : " — Not available yet"}</option>)}
      </PlatformSelect>
      <nav aria-label="Settings sections" className="hidden rounded-lg border bg-card p-2 lg:block lg:max-h-[calc(100dvh-8rem)] lg:overflow-y-auto">
        <ul className="space-y-0.5">{visible.map((section) => <li key={section.href}>
          {section.ready ? <Link href={section.href} aria-current={selected === section.href ? "page" : undefined} className={`block rounded-md px-3 py-2 text-sm hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring ${selected === section.href ? "bg-muted font-medium text-foreground" : "text-muted-foreground"}`}>{section.title}</Link> : <span className="block px-3 py-2 text-sm text-muted-foreground" title="Not available yet">{section.title}<span className="block text-xs">Not available yet</span></span>}
        </li>)}</ul>
      </nav>
    </div>
    <div className="min-w-0 [&_input[data-slot=input]]:h-11 sm:[&_input[data-slot=input]]:h-10 [&_.app-page-header]:mb-6 [&_form]:min-w-0">{children}</div>
  </div>;
}
