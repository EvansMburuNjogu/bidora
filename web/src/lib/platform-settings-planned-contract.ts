// Frontend-only connection inventory. These are not mounted API routes or editable settings.
// A section can become writable only after a reviewed operation and its typed HTTP contract exist.
export type PlannedSettingsKind = "google_sso" | "pesapal" | "countries" | "notifications" | "policies";
export type PlannedSettingsField = { label: string; value: string; source: "approved_rule" | "not_connected" };
export type PlannedSettingsConnection = {
  kind: PlannedSettingsKind;
  title: string;
  introduction: string;
  fields: readonly PlannedSettingsField[];
  pendingAction: string;
  pendingOperation: string;
  related?: { label: string; href: string; capability?: string };
};
export const plannedSettings: Record<PlannedSettingsKind, PlannedSettingsConnection> = {
  google_sso: {
    kind: "google_sso", title: "Google Login & SSO", introduction: "Google account access is handled separately from provider administration.",
    fields: [
      { label: "Your Google account", value: "Manage your own connection from Account security.", source: "approved_rule" },
      { label: "Platform sign-in provider", value: "Managed configuration is not connected to this page.", source: "not_connected" },
    ], pendingAction: "Manage sign-in provider", pendingOperation: "Provider configuration, connection checks and activation need a reviewed backend operation.",
    related: { label: "Manage your account security", href: "/account/security" },
  },
  pesapal: {
    kind: "pesapal", title: "PesaPal", introduction: "Payment-provider configuration is separate from invoice and payment records.",
    fields: [
      { label: "Provider connection", value: "No managed connection status is available here.", source: "not_connected" },
      { label: "Merchant configuration", value: "Not connected to a provider-settings API.", source: "not_connected" },
    ], pendingAction: "Configure PesaPal", pendingOperation: "Credential handling, callback validation, testing and governed activation require a backend contract and provider integration.",
    related: { label: "Review available billing records", href: "/platform/billing", capability: "platform.invoice.read" },
  },
  countries: {
    kind: "countries", title: "Countries & Localisation", introduction: "Supported market information is distinct from a changeable country catalogue.",
    fields: [
      { label: "Pilot market", value: "Kenya", source: "approved_rule" },
      { label: "Pilot currency", value: "Kenyan shilling (KES)", source: "approved_rule" },
      { label: "Additional markets", value: "No managed country catalogue is available.", source: "not_connected" },
    ], pendingAction: "Manage markets", pendingOperation: "Market enablement, currencies and local rules need approved policy and an enforcing backend operation.",
  },
  notifications: {
    kind: "notifications", title: "Notifications", introduction: "Delivery connection and notification rules are different settings.",
    fields: [
      { label: "Delivery connection", value: "SMTP has its own managed settings page; it does not define notification rules.", source: "approved_rule" },
      { label: "Rules and templates", value: "No managed notification policy is available here.", source: "not_connected" },
    ], pendingAction: "Edit notification rules", pendingOperation: "Notification categories, templates, channels and audited activation need a backend contract.",
    related: { label: "Open email settings", href: "/platform/settings/smtp", capability: "platform.integration.read" },
  },
  policies: {
    kind: "policies", title: "Operational Policies", introduction: "Core protections are enforced by the application, not editable as general-purpose switches.",
    fields: [
      { label: "Security protections", value: "Review the current source-defined account and session safeguards.", source: "approved_rule" },
      { label: "Operational controls", value: "No general policy configuration service is connected.", source: "not_connected" },
    ], pendingAction: "Change operational policies", pendingOperation: "Each adjustable control needs an approved range, owner, audit trail and runtime enforcement before it can be edited.",
    related: { label: "Review security safeguards", href: "/platform/settings/security", capability: "platform.admin.read" },
  },
};
