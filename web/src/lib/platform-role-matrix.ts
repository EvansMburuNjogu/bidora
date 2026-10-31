import { PLATFORM_ROLES, platformRoleLabels, type PlatformRole } from "@/lib/platform-contract";
import { fixedRoleCapabilities } from "@/lib/platform-fixed-role-bundles.generated";
import type { PlatformRoleCatalogue } from "@/lib/platform-roles-contract";

// Source-defined display only. A capability never proves an operation is mounted.
export const fallbackRoles: PlatformRoleCatalogue["roles"] = PLATFORM_ROLES.map((role_key) => ({ role_key, catalogue_version: 1, capabilities: [...fixedRoleCapabilities[role_key]] }));
export const roleDescriptions: Record<PlatformRole, string> = {
  PLATFORM_OWNER: "Platform administration and protected approvals",
  OPERATIONS_ADMINISTRATOR: "Tenderpreneurs and platform operations",
  BILLING_ADMINISTRATOR: "Plans, subscriptions and billing",
  INTEGRATION_ADMINISTRATOR: "Connection settings and provider operations",
  SUPPORT_ADMINISTRATOR: "Support cases and approved support access",
  SECURITY_AUDIT_ADMINISTRATOR: "Security, audit and access review",
};
export const shortRoleLabels: Record<PlatformRole, string> = {
  PLATFORM_OWNER: "Owner", OPERATIONS_ADMINISTRATOR: "Operations", BILLING_ADMINISTRATOR: "Billing",
  INTEGRATION_ADMINISTRATOR: "Integrations", SUPPORT_ADMINISTRATOR: "Support", SECURITY_AUDIT_ADMINISTRATOR: "Security & Audit",
};
export type MatrixRow = { label: string; keys: readonly string[] };
export type MatrixGroup = { name: string; rows: readonly MatrixRow[] };
const row = (label: string, key: string): MatrixRow => ({ label, keys: [key] });
export const permissionMatrix: readonly MatrixGroup[] = [
  { name: "Overview", rows: [{ label: "Financial overview", keys: ["platform.invoice.read", "platform.payment.read"] }, row("Service health", "platform.operations.health.read")] },
  { name: "Tenderpreneurs", rows: [row("View organisations", "platform.tenant.read"), row("Register organisations", "platform.tenant.create"), row("Retry setup", "platform.tenant.provision.retry"), row("Suspend organisations", "platform.tenant.suspend"), row("Reactivate organisations", "platform.tenant.reactivate")] },
  { name: "Users", rows: [row("View administrators", "platform.admin.read"), row("Invite administrators", "platform.admin.invite"), row("Assign roles", "platform.admin.role.assign"), row("Revoke roles", "platform.admin.role.revoke"), row("Disable accounts", "platform.admin.identity.disable"), row("Revoke sessions", "platform.admin.session.revoke"), row("Review changes", "platform.governance.review"), row("Approve changes", "platform.governance.approve")] },
  { name: "Plans & Subscriptions", rows: [row("View plans", "platform.plan.read"), row("Manage plans", "platform.plan.manage"), row("Publish plans", "platform.plan.publish"), row("View subscriptions", "platform.subscription.read"), row("Manage subscriptions", "platform.subscription.manage")] },
  { name: "Billing", rows: [row("View invoices", "platform.invoice.read"), row("View payments", "platform.payment.read"), row("Reconcile payments", "platform.payment.reconcile"), row("Request adjustments", "platform.billing.adjustment.request")] },
  { name: "AI Usage", rows: [{ label: "Usage history", keys: [] }] },
  { name: "Communications", rows: [row("Manage announcements", "platform.announcement.manage")] },
  { name: "Support", rows: [row("View cases", "platform.support.case.read"), row("Manage cases", "platform.support.case.manage"), row("Request access", "platform.support.access.request")] },
  { name: "Reports", rows: [row("Export billing records", "platform.billing.export"), row("Export audit records", "platform.audit.export")] },
  { name: "Audit Trails", rows: [row("View audit records", "platform.audit.read"), row("View security events", "platform.security.event.read")] },
  { name: "Settings", rows: [row("View connections", "platform.integration.read"), row("Manage email", "platform.integration.smtp.manage"), row("Manage AI connection", "platform.integration.ai.manage"), row("Manage payment connection", "platform.integration.pesapal.manage"), row("View reference settings", "platform.reference.read"), row("Manage reference settings", "platform.reference.manage"), row("View retention policy", "platform.retention.policy.read")] },
];
export function roleName(role: PlatformRole): string { return platformRoleLabels[role]; }
export function granted(capabilities: readonly string[], keys: readonly string[]): boolean { return keys.length > 0 && keys.every((key) => capabilities.includes(key)); }
