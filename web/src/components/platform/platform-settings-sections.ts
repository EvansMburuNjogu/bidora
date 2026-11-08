export type SettingsSection = {
  title: string;
  description: string;
  href: string;
  capabilities: readonly string[];
  ready: boolean;
};

// Navigation metadata is not authority. Every supported operation checks access again.
export const settingsSections: readonly SettingsSection[] = [
  { title: "Platform Information", description: "Platform identity and contact details", href: "/platform/settings/platform", capabilities: [], ready: true },
  { title: "Roles & Permissions", description: "Fixed roles and administrator access", href: "/platform/settings/access", capabilities: ["platform.admin.read"], ready: true },
  { title: "Google Login & SSO", description: "Sign-in provider configuration", href: "/platform/settings/sso", capabilities: ["platform.integration.read"], ready: true },
  { title: "Security & Sessions", description: "Account and session safeguards", href: "/platform/settings/security", capabilities: ["platform.admin.read"], ready: true },
  { title: "Email (SMTP)", description: "Outbound email configuration", href: "/platform/settings/smtp", capabilities: ["platform.integration.read"], ready: true },
  { title: "OpenAI", description: "AI provider status", href: "/platform/settings/openai", capabilities: ["platform.integration.read"], ready: true },
  { title: "PesaPal", description: "Payment provider configuration", href: "/platform/settings/pesapal", capabilities: ["platform.integration.read"], ready: true },
  { title: "Countries & Localisation", description: "Supported markets and defaults", href: "/platform/settings/countries", capabilities: ["platform.reference.read"], ready: true },
  { title: "Notifications", description: "Delivery rules and templates", href: "/platform/settings/notifications", capabilities: ["platform.announcement.manage"], ready: true },
  { title: "Operational Policies", description: "Approved operational controls", href: "/platform/settings/policies", capabilities: ["platform.reference.read", "platform.retention.policy.read"], ready: true },
];

export function settingsSectionVisible(section: SettingsSection, capabilities: ReadonlySet<string>): boolean {
  return section.capabilities.length === 0 || section.capabilities.some((capability) => capabilities.has(capability));
}
