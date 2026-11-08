"use client";
import { OperationalNotice } from "@/components/session/operational-notice";

import { PlatformName, PlatformContact } from "@/components/session/platform-identity-provider";

import { useEffect, useRef, type CSSProperties } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronsUpDownIcon, KeyRoundIcon, Loader2Icon, LogOutIcon, XIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";
import { PlatformDemoWorkspace } from "@/components/platform/platform-demo-workspace";
import { platformDemoEnabled } from "@/lib/platform-demo-data";
import { Button } from "@/components/ui/button";
import { hasPlatformNavigationAccess, platformNavigation } from "@/components/platform/platform-navigation";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent, SidebarGroupLabel,
  SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarProvider,
  SidebarRail, SidebarTrigger, useSidebar,
} from "@/components/ui/sidebar";

const shellStyle = {
  "--sidebar-width": "250px",
  "--sidebar": "#18181b",
  "--sidebar-foreground": "#f4f4f5",
  "--sidebar-border": "color-mix(in oklab, #3f3f46 60%, transparent)",
  "--sidebar-accent": "#1a3f42",
  "--sidebar-accent-foreground": "#d5f5ef",
  "--sidebar-ring": "#71717a",
} as CSSProperties;

export function PlatformAdminShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const contentRef = useRef<HTMLDivElement>(null);
  useEffect(() => { if (contentRef.current) contentRef.current.scrollTop = 0; }, [pathname]);
  const { resolution } = useSession();
  const grantedCapabilities = new Set(resolution.capabilities);
  const navigationItems = platformNavigation.flatMap((group) => group.items);
  const activeHref = pathname === "/platform/audit-trails" ? "/platform/audit" :
    pathname === "/platform/approvals" || pathname.startsWith("/platform/administrators") ? "/platform/users" :
    pathname.startsWith("/platform/organizations") ? "/platform/tenderpreneurs" :
    pathname.startsWith("/platform/commercial") ? "/platform/billing" :
    pathname.startsWith("/platform/integrations") ? "/platform/settings" :
    navigationItems.find((item) => pathname === item.href || item.href !== "/platform" && pathname.startsWith(`${item.href}/`))?.href;
  const pageTitle = pathname === "/platform/settings/smtp" ? "SMTP" : pathname.includes("/openai") ? "OpenAI" :
    pathname === "/platform/users/approvals" || pathname === "/platform/approvals" ? "Approvals" :
    pathname.startsWith("/platform/users/") || pathname.startsWith("/platform/administrators/") ? "User details" :
    pathname === "/platform/settings/roles" || pathname === "/platform/settings/access" ? "Roles & Permissions" : pathname === "/platform/settings/information" || pathname === "/platform/settings/platform" ? "Platform Information" :
    pathname === "/platform/settings/sso" ? "Google Login & SSO" : pathname === "/platform/settings/security" ? "Security & Sessions" :
    pathname === "/platform/settings/pesapal" ? "PesaPal" : pathname === "/platform/settings/countries" ? "Countries & Localisation" :
    pathname === "/platform/settings/notifications" ? "Notifications" : pathname === "/platform/settings/policies" ? "Operational Policies" :
    pathname.startsWith("/platform/organizations/") || pathname.startsWith("/platform/tenderpreneurs/") ? "Tenderpreneur details" :
    navigationItems.find((item) => item.href === activeHref)?.label ?? "Platform Administration";
  const { isSigningOut, signOut } = useSignOut();

  return (
    <SidebarProvider style={shellStyle} className="platform-admin-shell">
      <a href="#platform-main" className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-background focus:px-4 focus:py-2 focus:text-foreground focus:ring-2 focus:ring-ring">Skip to main content</a>
      <Sidebar collapsible="icon" className="dark [--sidebar:#18181b] [--sidebar-accent:#1a3f42] [--sidebar-accent-foreground:#d5f5ef] [--sidebar-border:#3f3f46] [--sidebar-foreground:#f4f4f5] max-md:[&_[data-slot=sidebar-menu-button]]:min-h-11">
        <SidebarHeader className="flex min-h-14 flex-row items-center justify-between px-2 py-1.5 group-data-[collapsible=icon]:flex-col group-data-[collapsible=icon]:justify-center">
          <PlatformSidebarLink href="/platform" aria-label="Platform Administration home" className="flex min-w-0 items-center gap-2 px-1 text-zinc-100">
            <Image src="/bidora-aperture-inverse-192.png" alt="" width={28} height={28} className="size-7 shrink-0" />
            <span className="brand-wordmark truncate text-sm group-data-[collapsible=icon]:hidden"><PlatformName /></span>
          </PlatformSidebarLink>
          <PlatformMobileClose />
        </SidebarHeader>

        <SidebarContent>
          <nav aria-label="Platform Administration" className="flex flex-col">
            {platformNavigation.map((group) => (
              <SidebarGroup key={group.label}>
                <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {group.items.map((item) => {
                      const Icon = item.icon;
                      const active = item.href === activeHref;
                      const usable = hasPlatformNavigationAccess(item, grantedCapabilities);
                      return <SidebarMenuItem key={item.label}>
                        {usable ? <SidebarMenuButton render={<PlatformSidebarLink href={item.href} aria-current={active ? "page" : undefined} />} isActive={active} tooltip={item.label} className="text-zinc-400 hover:bg-sidebar-accent! hover:text-sidebar-accent-foreground data-active:bg-sidebar-accent! data-active:text-sidebar-accent-foreground data-active:hover:bg-sidebar-accent! [&>svg]:opacity-60 data-active:[&>svg]:opacity-100"><Icon aria-hidden="true" /><span>{item.label}</span></SidebarMenuButton> :
                          <SidebarMenuButton type="button" disabled title={item.available ? "No access" : "Not available yet"} className="text-zinc-500 disabled:opacity-100"><Icon aria-hidden="true" /><span>{item.label}</span></SidebarMenuButton>}
                      </SidebarMenuItem>;
                    })}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            ))}
          </nav>
        </SidebarContent>

        <SidebarFooter className="pb-[max(0.5rem,env(safe-area-inset-bottom))]">
          <PlatformAccountMenu isSigningOut={isSigningOut} onSignOut={() => void signOut()} />
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>

      <SidebarInset className="h-dvh min-w-0 overflow-hidden bg-card">
        <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-3 border-b border-border/70 bg-card/95 px-[18px] backdrop-blur supports-[backdrop-filter]:bg-card/85">
          <SidebarTrigger className="-ml-1 max-md:size-10" aria-label="Collapse or expand navigation" />
          <nav aria-label="Breadcrumb" className="ml-auto flex min-w-0 items-center gap-2 text-sm">
            <Link href="/platform" className="hidden truncate text-muted-foreground hover:text-foreground sm:inline">Platform Administration</Link>
            <span className="hidden text-muted-foreground sm:inline" aria-hidden="true">/</span>
            <span className="truncate font-medium text-foreground" aria-current="page">{pageTitle}</span>
          </nav>
        </header>
        <div ref={contentRef} id="platform-main" tabIndex={-1} className="isolate flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto overscroll-contain px-[18px] pb-[18px] [&>.app-page]:p-0 [&>.app-page]:pt-[18px]">{platformDemoEnabled && <div role="status" className="mt-3 rounded-md border border-primary/30 bg-primary/5 px-3 py-2 text-xs font-medium">Demo mode · Fictional records · No changes saved to Bidora</div>}<OperationalNotice/><PlatformDemoWorkspace key={pathname} />{children}</div>
        <footer className="flex min-h-12 shrink-0 items-center justify-center border-t border-border/70 bg-card px-[18px] pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] text-center text-xs text-muted-foreground">
          <span className="mr-3"><PlatformName /> © {new Date().getFullYear()}</span><PlatformContact />
        </footer>
      </SidebarInset>
    </SidebarProvider>
  );
}

function PlatformMobileClose() {
  const { setOpenMobile } = useSidebar();
  return (
    <Button type="button" variant="ghost" size="icon" className="ml-auto text-sidebar-foreground md:hidden" aria-label="Close navigation" onClick={() => setOpenMobile(false)}>
      <XIcon aria-hidden="true" />
    </Button>
  );
}

function PlatformSidebarLink({ onClick, ...props }: React.ComponentProps<typeof Link>) {
  const { isMobile, setOpenMobile } = useSidebar();
  return (
    <Link
      {...props}
      onClick={(event) => {
        onClick?.(event);
        if (!event.defaultPrevented && isMobile) setOpenMobile(false);
      }}
    />
  );
}

function PlatformAccountMenu({ isSigningOut, onSignOut }: { isSigningOut: boolean; onSignOut: () => void }) {
  const { isMobile } = useSidebar();
  return (
    <DropdownMenu>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenuTrigger render={<SidebarMenuButton size="lg" aria-label="Open administrator account menu" className="h-14 border border-zinc-700 bg-zinc-900/80 text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 data-popup-open:bg-zinc-800 group-data-[collapsible=icon]:border-0 group-data-[collapsible=icon]:bg-transparent" />}>
            <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-zinc-800 text-xs font-semibold text-zinc-100" aria-hidden="true">PA</span>
            <span className="min-w-0 flex-1 text-left group-data-[collapsible=icon]:hidden">
              <span className="block truncate text-sm font-medium text-zinc-100">Platform Administrator</span>
              <span className="block truncate text-xs text-zinc-500">Account and security</span>
            </span>
            <ChevronsUpDownIcon className="ml-auto size-4 opacity-50 group-data-[collapsible=icon]:hidden" aria-hidden="true" />
          </DropdownMenuTrigger>
        </SidebarMenuItem>
      </SidebarMenu>
      <DropdownMenuContent side={isMobile ? "top" : "right"} align="end" sideOffset={8} className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuLabel>Account</DropdownMenuLabel>
          <DropdownMenuItem render={<PlatformSidebarLink href="/account/security" />}>
            <KeyRoundIcon aria-hidden="true" />Account security
          </DropdownMenuItem>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" disabled={isSigningOut} onClick={onSignOut}>
          {isSigningOut ? <Loader2Icon aria-hidden="true" className="animate-spin" /> : <LogOutIcon aria-hidden="true" />}{isSigningOut ? "Signing out…" : "Sign out"}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
