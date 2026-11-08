// Display names only. Authority always comes from the current server catalogue.
const permissionNames: Record<string, string> = {
  "platform.admin.read": "View administrator accounts",
  "platform.admin.invite": "Invite administrators",
  "platform.admin.role.assign": "Request role assignments",
  "platform.admin.role.revoke": "Request role removals",
  "platform.admin.identity.disable": "Request account disabling",
  "platform.admin.identity.reactivate": "Request account reactivation",
  "platform.admin.session.revoke": "Revoke administrator sessions",
  "platform.governance.approve": "Approve access changes",
  "platform.governance.review": "Review access changes",
  "platform.tenant.read": "View organisations",
  "platform.tenant.create": "Register organisations",
  "platform.tenant.provision.retry": "Retry organisation setup",
  "platform.tenant.suspend": "Suspend organisations",
  "platform.tenant.reactivate": "Reactivate organisations",
  "platform.migration.read": "View platform updates",
  "platform.migration.manage": "Manage platform updates",
  "platform.reference.read": "View reference settings",
  "platform.reference.manage": "Manage reference settings",
  "platform.maintenance.read": "View maintenance",
  "platform.maintenance.manage": "Manage maintenance",
  "platform.announcement.manage": "Manage announcements",
  "platform.operations.health.read": "View service health",
  "platform.plan.read": "View plans",
  "platform.plan.manage": "Manage plans",
  "platform.plan.publish": "Publish plans",
  "platform.subscription.read": "View subscriptions",
  "platform.subscription.manage": "Manage subscriptions",
  "platform.invoice.read": "View invoices",
  "platform.payment.read": "View payments",
  "platform.payment.reconcile": "Request payment reconciliation",
  "platform.billing.adjustment.request": "Request billing adjustments",
  "platform.billing.export": "Export billing records",
  "platform.integration.read": "View integration settings",
  "platform.integration.smtp.manage": "Manage email settings",
  "platform.integration.smtp.test": "Test email connection",
  "platform.integration.smtp.activate": "Apply email settings",
  "platform.integration.ai.manage": "Manage AI settings",
  "platform.integration.ai.test": "Test AI connection",
  "platform.integration.ai.activate": "Request AI activation",
  "platform.integration.pesapal.manage": "Manage payment provider settings",
  "platform.integration.pesapal.test": "Test payment provider connection",
  "platform.integration.pesapal.activate": "Request payment provider activation",
  "platform.integration.pause": "Pause integrations",
  "platform.integration.rollback": "Restore integration settings",
  "platform.integration.failure.read": "View integration failures",
  "platform.support.case.read": "View support cases",
  "platform.support.case.manage": "Manage support cases",
  "platform.support.access.request": "Request support access",
  "platform.support.session.use": "Use approved support session",
  "platform.support.session.revoke": "Revoke support sessions",
  "platform.support.session.read": "View support sessions",
  "platform.security.event.read": "View security events",
  "platform.audit.read": "View audit records",
  "platform.audit.export": "Export audit records",
  "platform.retention.policy.read": "View retention policy",
  "platform.retention.policy.draft": "Draft retention policy",
};

export function platformPermissionLabel(key: string): string {
  return permissionNames[key] ?? "Other platform permission";
}

export function platformPermissionArea(key: string): string {
  if (/^platform\.(admin|governance)\./.test(key)) return "Users & approvals";
  if (/^platform\.(tenant|migration|reference|maintenance|announcement|operations)\./.test(key)) return "Organisations & operations";
  if (/^platform\.(plan|subscription|invoice|payment|billing)\./.test(key)) return "Plans & billing";
  if (key.startsWith("platform.integration.")) return "Integrations";
  if (key.startsWith("platform.support.")) return "Support";
  return "Security & audit";
}
