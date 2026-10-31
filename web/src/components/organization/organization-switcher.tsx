"use client";

import { Building2Icon, CheckIcon, ChevronsUpDownIcon } from "lucide-react";
import { useOrganization } from "@/components/organization/organization-provider";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar";

export function OrganizationSwitcher() {
  const { activeOrganization, memberships, selectOrganization } = useOrganization();

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <SidebarMenuButton size="lg" tooltip="Switch organization">
                <span className="grid size-8 shrink-0 place-items-center bg-sidebar-primary text-sidebar-primary-foreground">
                  <Building2Icon aria-hidden="true" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{activeOrganization.tenant_name}</span>
                  <span className="type-meta block truncate text-sidebar-foreground/65">Tenderpreneur</span>
                </span>
                <ChevronsUpDownIcon className="ml-auto" aria-hidden="true" />
              </SidebarMenuButton>
            }
          />
          <DropdownMenuContent side="right" align="start" className="min-w-56">
            <DropdownMenuGroup>
              <DropdownMenuLabel>Organizations</DropdownMenuLabel>
              {memberships.map((membership) => (
                <DropdownMenuItem
                  key={membership.membership_id}
                  onClick={() => selectOrganization(membership.membership_id)}
                >
                  <Building2Icon aria-hidden="true" />
                  <span className="min-w-0 flex-1 truncate">{membership.tenant_name}</span>
                  {membership.membership_id === activeOrganization.membership_id && <CheckIcon aria-hidden="true" />}
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
