"use client";
import { OperationalNotice } from "@/components/session/operational-notice";

import { PlatformContact } from "@/components/session/platform-identity-provider";

import Image from "next/image";
import Link from "next/link";
import { HomeIcon, KeyRoundIcon, LogOutIcon, ShieldCheckIcon } from "lucide-react";
import { Brand } from "@/components/brand";
import { OrganizationSwitcher } from "@/components/organization/organization-switcher";
import { useOrganization } from "@/components/organization/organization-provider";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarSeparator,
  SidebarTrigger,
} from "@/components/ui/sidebar";

export function TenderpreneurShell({ children }: { children: React.ReactNode }) {
  const { activeOrganization } = useOrganization();
  const { resolution } = useSession();
  const { isSigningOut, signOut } = useSignOut();

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon" className="dark">
        <SidebarHeader>
          <div className="flex h-10 items-center px-2 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0">
            <span className="group-data-[collapsible=icon]:hidden"><Brand href="/tenderpreneur" /></span>
            <Link
              href="/tenderpreneur"
              aria-label="Bidora home"
              className="hidden size-8 place-items-center text-sidebar-primary group-data-[collapsible=icon]:grid"
            >
              <Image src="/bidora-aperture-inverse-192.png" alt="" width={28} height={28} className="size-7" />
            </Link>
          </div>
          <OrganizationSwitcher />
        </SidebarHeader>
        <SidebarSeparator />
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel>Workspace</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    render={<Link href="/tenderpreneur" />}
                    isActive
                    tooltip="Overview"
                  >
                    <HomeIcon aria-hidden="true" />
                    <span>Overview</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    render={<Link href="/account/security" />}
                    tooltip="Account security"
                  >
                    <KeyRoundIcon aria-hidden="true" />
                    <span>Account security</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarSeparator />
        <SidebarFooter>
          <div className="type-meta flex h-8 items-center gap-2 px-2 text-sidebar-foreground/70 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0">
            <ShieldCheckIcon className="size-4 shrink-0" aria-hidden="true" />
            <span className="group-data-[collapsible=icon]:hidden">
              {resolution.session.assurance_level === "AAL2" ? "Two-step verified" : "Signed in"}
            </span>
          </div>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                type="button"
                tooltip="Sign out"
                disabled={isSigningOut}
                onClick={() => void signOut()}
              >
                <LogOutIcon aria-hidden="true" />
                <span>{isSigningOut ? "Signing out…" : "Sign out"}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
          <PlatformContact />
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>

      <SidebarInset>
        <header className="flex h-14 shrink-0 items-center gap-3 border-b px-4 sm:px-6">
          <SidebarTrigger />
          <div className="h-4 w-px bg-border" aria-hidden="true" />
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbPage>{activeOrganization.tenant_name}</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </header>
        <OperationalNotice/>{children}
      </SidebarInset>
    </SidebarProvider>
  );
}
