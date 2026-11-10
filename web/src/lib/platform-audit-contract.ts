export const auditActions = [
  "UserLoggedIn", "EmailLoginCodeVerified", "MFALoginCompleted",
  "PlatformTenantSuspended", "PlatformTenantReactivated",
  "PlatformInformationUpdated", "PlatformSessionPolicyUpdated", "CountryConfigurationUpdated", "OperationalPolicyUpdated", "RestrictedRequestMetadataRead",
 "PlatformGoogleConfigurationUpdated", "PlatformGoogleConnectionTested", "NotificationTemplateUpdated", "NotificationTemplateTested",
  "PlatformOwnerBootstrapped", "PlatformAdministratorSessionsRevoked",
  "PlatformAdministratorInvitationIssued", "PlatformAdministratorInvitationCancelled", "PlatformAdministratorInvitationAccepted", "PlatformOwnerInvitationActivated", "PlatformOwnerInvitationRejected",
  "PlatformRoleAssignmentChanged", "PlatformRoleRevocationChanged", "PlatformAdministratorDisabled", "PlatformAdministratorReactivated", "PlatformAdministratorRemoved", "PlatformAdministratorDeleted",
  "PlatformIntegrationVersionCreated", "PlatformIntegrationVersionTested", "PlatformIntegrationActivationProposed", "PlatformControlAuditExport",
 "PlatformPlansRead", "PlatformPlanDraftCreated", "PlatformPlanPublished", "PlatformSubscriptionRead", "PlatformSubscriptionCancellationScheduled", "PlatformSubscriptionCancellationWithdrawn", "PlatformSubscriptionInventoryRead",
 "PlatformSupportCasesRead", "PlatformSupportCaseRead", "PlatformSupportCaseCreate", "PlatformSupportCaseAssign", "PlatformSupportCaseReply", "PlatformSupportCaseResolve", "PlatformSupportCaseReopen",
] as const;
export const auditModules = ["ADMINISTRATION", "INTEGRATIONS"] as const;
export const auditResults = ["SUCCESSFUL", "FAILED", "PENDING"] as const;
export const auditUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export type AuditChange = { field: string; operation: "ADDED" | "DELETED" | "MODIFIED"; before: string | null; after: string | null };
export type AuditRecord = { id: string; administrator_id: string | null; action: typeof auditActions[number]; module: typeof auditModules[number]; affected_record_id: string | null; result: typeof auditResults[number]; occurred_at: string; administrator_name?: string | null; affected_record_name?: string | null; ip_address?: string | null; browser?: string | null; changes?: AuditChange[] };
const safeText = (value: unknown, max: number) => value === null || typeof value === "string" && value.length > 0 && value.length <= max && !/[\u0000-\u001f\u007f]/.test(value);
const changeFields = ["platform_name", "contact_phone", "support_email", "website", "enabled", "allow_signup", "client_id", "allowed_domains", "client_secret", "role", "idle_seconds", "absolute_seconds", "country_code", "currency", "language", "timezone", "locale", "is_default", "organization_registration_enabled", "ai_features_enabled", "document_uploads_enabled", "max_document_bytes", "maintenance_enabled", "maintenance_message", "maintenance_starts_at", "maintenance_ends_at", "restricted_metadata_retention_days", "template_name", "template_subject", "template_body"];
function validChange(value: unknown): value is AuditChange {
  if (!value || typeof value !== "object" || !exact(value, ["field", "operation", "before", "after"])) return false;
  const change = value as AuditChange;
  return changeFields.includes(change.field) && ["ADDED", "DELETED", "MODIFIED"].includes(change.operation) && safeText(change.before, 512) && safeText(change.after, 512) && (change.field !== "client_secret" || change.before === null && change.after === "[redacted]");
}
export function auditFieldLabel(field: string): string { return ({ platform_name: "Platform name", contact_phone: "Contact phone", support_email: "Support email", website: "Website", enabled: "Enabled", allow_signup: "Google signup", client_id: "Client ID", allowed_domains: "Email domain restrictions", client_secret: "Client secret", role: "Role", idle_seconds: "Idle session duration", absolute_seconds: "Maximum session duration", country_code: "Country", currency: "Currency", language: "Language", timezone: "Time zone", locale: "Locale", is_default: "Default country", organization_registration_enabled: "Organisation registration", ai_features_enabled: "AI features", document_uploads_enabled: "Document uploads", max_document_bytes: "Upload size limit", maintenance_enabled: "Maintenance mode", maintenance_message: "Maintenance message", maintenance_starts_at: "Maintenance start", maintenance_ends_at: "Maintenance end", restricted_metadata_retention_days: "Metadata retention", template_name: "Template name", template_subject: "Template subject", template_body: "Template body" } as Record<string, string>)[field] ?? "Setting"; }
export function auditChangeSummary(row: AuditRecord): string {
  return row.changes?.length ? row.changes.map(change => `${({ ADDED: "Added", DELETED: "Deleted", MODIFIED: "Modified" })[change.operation]} ${auditFieldLabel(change.field).toLowerCase()}`).join("; ") : "No field details recorded";
}
export type AuditPage = { data: AuditRecord[]; meta: { trace_id: string; next_cursor: string | null } };
const exact = (value: object, keys: string[]) => Object.keys(value).sort().join(",") === [...keys].sort().join(",");
export function parseAuditRecord(value: unknown): AuditRecord | null {
  const required = ["id", "administrator_id", "action", "module", "affected_record_id", "result", "occurred_at"];
  const optional = ["administrator_name", "affected_record_name", "ip_address", "browser", "changes"];
  if (!value || typeof value !== "object" || !required.every(key => Object.hasOwn(value, key)) || Object.keys(value).some(key => !required.includes(key) && !optional.includes(key))) return null;
  const row = value as AuditRecord;
  if (!auditUUID.test(row.id) || row.administrator_id !== null && !auditUUID.test(row.administrator_id) || row.affected_record_id !== null && !auditUUID.test(row.affected_record_id) ||
    !auditActions.includes(row.action) || !auditModules.includes(row.module) || !auditResults.includes(row.result) ||
    typeof row.occurred_at !== "string" || !/Z$/.test(row.occurred_at) || !Number.isFinite(Date.parse(row.occurred_at))) return null;
  for (const [field, max] of [["administrator_name", 254], ["affected_record_name", 254], ["ip_address", 45], ["browser", 40]] as const) if (Object.hasOwn(row, field) && !safeText(row[field], max)) return null;
  if (row.ip_address && !/^[0-9a-fA-F:.]+$/.test(row.ip_address)) return null;
  if (row.changes !== undefined && (!Array.isArray(row.changes) || row.changes.length > 30 || !row.changes.every(validChange))) return null;
  return row;
}
export function parseAuditPage(value: unknown): AuditPage | null {
  if (!value || typeof value !== "object" || !exact(value, ["data", "meta"])) return null;
  const page = value as AuditPage;
  if (!Array.isArray(page.data) || page.data.length > 25 || !page.meta || typeof page.meta !== "object" || !exact(page.meta, ["trace_id", "next_cursor"]) || typeof page.meta.trace_id !== "string" ||
    !(page.meta.next_cursor === null || typeof page.meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(page.meta.next_cursor)) || page.data.some((row) => !parseAuditRecord(row))) return null;
  return page;
}
export function parseAuditDetail(value: unknown): AuditRecord | null {
  if (!value || typeof value !== "object" || !exact(value, ["data", "meta"])) return null;
  const response = value as { data: unknown; meta: unknown };
  if (!response.meta || typeof response.meta !== "object" || !exact(response.meta, ["trace_id"]) || typeof (response.meta as { trace_id?: unknown }).trace_id !== "string") return null;
  return parseAuditRecord(response.data);
}
