export const auditActions = [
  "UserLoggedIn", "EmailLoginCodeVerified", "MFALoginCompleted",
  "PlatformInformationUpdated", "PlatformSessionPolicyUpdated",
 "PlatformGoogleConfigurationUpdated", "PlatformGoogleConnectionTested", "NotificationTemplateUpdated", "NotificationTemplateTested",
  "PlatformOwnerBootstrapped", "PlatformAdministratorSessionsRevoked",
  "PlatformAdministratorInvitationIssued", "PlatformAdministratorInvitationCancelled", "PlatformAdministratorInvitationAccepted", "PlatformOwnerInvitationActivated", "PlatformOwnerInvitationRejected",
  "PlatformRoleAssignmentChanged", "PlatformRoleRevocationChanged", "PlatformAdministratorDisabled", "PlatformAdministratorReactivated",
  "PlatformIntegrationVersionCreated", "PlatformIntegrationVersionTested", "PlatformIntegrationActivationProposed",
] as const;
export const auditModules = ["ADMINISTRATION", "INTEGRATIONS"] as const;
export const auditResults = ["SUCCESSFUL", "FAILED", "PENDING"] as const;
export const auditUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export type AuditRecord = { id: string; administrator_id: string | null; action: typeof auditActions[number]; module: typeof auditModules[number]; affected_record_id: string | null; result: typeof auditResults[number]; occurred_at: string };
export type AuditPage = { data: AuditRecord[]; meta: { trace_id: string; next_cursor: string | null } };
const exact = (value: object, keys: string[]) => Object.keys(value).sort().join(",") === [...keys].sort().join(",");
export function parseAuditRecord(value: unknown): AuditRecord | null {
  if (!value || typeof value !== "object" || !exact(value, ["id", "administrator_id", "action", "module", "affected_record_id", "result", "occurred_at"])) return null;
  const row = value as AuditRecord;
  if (!auditUUID.test(row.id) || row.administrator_id !== null && !auditUUID.test(row.administrator_id) || row.affected_record_id !== null && !auditUUID.test(row.affected_record_id) ||
    !auditActions.includes(row.action) || !auditModules.includes(row.module) || !auditResults.includes(row.result) ||
    typeof row.occurred_at !== "string" || !/Z$/.test(row.occurred_at) || !Number.isFinite(Date.parse(row.occurred_at))) return null;
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
