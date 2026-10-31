import type { LucideIcon } from "lucide-react";
import {
  BadgeDollarSignIcon, BellIcon, BookOpenIcon, Building2Icon, ChartNoAxesCombinedIcon,
  CircleHelpIcon, LayoutDashboardIcon, ScrollTextIcon, SettingsIcon,
  SparklesIcon, UsersIcon,
} from "lucide-react";

export type PlatformNavigationItem = {
  label: string;
  icon: LucideIcon;
  capabilities: readonly string[];
  href: string;
  available: boolean;
};
export type PlatformNavigationGroup = { label: string; items: readonly PlatformNavigationItem[] };

// Incomplete modules stay in the official menu but cannot imply an unmounted operation.
export const platformNavigation: readonly PlatformNavigationGroup[] = [
  { label: "Workspace", items: [
    { label: "Overview", icon: LayoutDashboardIcon, capabilities: [], href: "/platform", available: true },
  ] },
  { label: "Directory", items: [
    { label: "Tenderpreneurs", icon: Building2Icon, capabilities: ["platform.tenant.read"], href: "/platform/tenderpreneurs", available: true },
    { label: "Users", icon: UsersIcon, capabilities: ["platform.admin.read"], href: "/platform/users", available: true },
  ] },
  { label: "Operations", items: [
    { label: "Plans & Subscriptions", icon: BookOpenIcon, capabilities: [], href: "/platform/plans", available: true },
    { label: "Billing", icon: BadgeDollarSignIcon, capabilities: ["platform.invoice.read", "platform.payment.read"], href: "/platform/billing", available: true },
    { label: "AI Usage", icon: SparklesIcon, capabilities: [], href: "/platform/ai-usage", available: true },
    { label: "Communications", icon: BellIcon, capabilities: [], href: "/platform/communications", available: true },
    { label: "Support", icon: CircleHelpIcon, capabilities: [], href: "/platform/support", available: true },
  ] },
  { label: "Insights", items: [
    { label: "Reports", icon: ChartNoAxesCombinedIcon, capabilities: [], href: "/platform/reports", available: true },
    { label: "Audit Trails", icon: ScrollTextIcon, capabilities: [], href: "/platform/audit", available: true },
  ] },
  { label: "Administration", items: [
    { label: "Settings", icon: SettingsIcon, capabilities: ["platform.admin.read", "platform.integration.read", "platform.reference.read"], href: "/platform/settings", available: true },
  ] },
];
export function hasPlatformNavigationAccess(item: PlatformNavigationItem, capabilities: ReadonlySet<string>): boolean {
  return item.available && (item.capabilities.length === 0 || item.capabilities.some((capability) => capabilities.has(capability)));
}
