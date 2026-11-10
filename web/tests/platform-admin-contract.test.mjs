import assert from "node:assert/strict";
import test from "node:test";
import { parsePlatformAdministratorChange, parsePlatformChangePage, parsePlatformAssignmentHistory, parsePlatformInvitationReceipt } from "../src/lib/platform-contract.ts";
import { parsePlatformTenantList, parsePlatformTenantDetail } from "../src/lib/platform-tenant-contract.ts";

const target = "11111111-1111-4111-8111-111111111111";
const changeID = "22222222-2222-4222-8222-222222222222";
const maker = "33333333-3333-4333-8333-333333333333";
const meta = { trace_id: "safe-trace", next_cursor: null };
const pending = {
  id: changeID, action: "ASSIGN_ROLE", target_user_id: target, invitation_id: null,
  role_key: "PLATFORM_OWNER", assignment_id: null, expected_user_version: 2,
  expected_target_context_generation: 3, expected_assignment_version: null,
  reason: "Transfer responsibility", decision_reason: null, state: "PENDING_APPROVAL",
  version: 1, maker_id: maker, checker_id: null, created_at: "2026-10-18T12:00:00Z",
  expires_at: "2026-10-18T12:30:00Z", decided_at: null, sessions_revoked: false,
};
const assignment = {
  id: changeID, role_key: "PLATFORM_OWNER", state: "REVOKED", version: 2,
  assigned_by: maker, assigned_at: "2026-10-18T12:00:00Z", revoked_by: maker,
  revoked_at: "2026-10-19T12:00:00Z", reason: "Initial assignment", revocation_reason: "Transferred",
};

test("invitation receipt distinguishes delivery acceptance from recipient delivery and rejects extra data", () => {
  const receipt = { id: target, email: "invited@example.test", role_key: "PLATFORM_OWNER", state: "ACTIVE", version: 1, created_at: "2026-10-18T12:00:00Z", expires_at: "2026-10-21T12:00:00Z", delivery_state: "ACCEPTED_FOR_DELIVERY", governance_change_id: null };
  assert.deepEqual(parsePlatformInvitationReceipt({ data: receipt, meta }), { id: target, state: "ACTIVE", delivery_state: "ACCEPTED_FOR_DELIVERY" });
  assert.deepEqual(parsePlatformInvitationReceipt({ data: { ...receipt, first_name: "Amina", last_name: "Njoroge" }, meta }), { id: target, state: "ACTIVE", delivery_state: "ACCEPTED_FOR_DELIVERY" });
  assert.equal(parsePlatformInvitationReceipt({ data: { ...receipt, first_name: "Amina" }, meta }), null);
  assert.equal(parsePlatformInvitationReceipt({ data: { ...receipt, first_name: "A".repeat(101), last_name: "Njoroge" }, meta }), null);
  assert.equal(parsePlatformInvitationReceipt({ data: { ...receipt, delivery_state: "DELIVERED" }, meta }), null);
  assert.equal(parsePlatformInvitationReceipt({ data: { ...receipt, token: "must-not-be-returned" }, meta }), null);
});

test("administrator change parser distinguishes pending from executed and rejects unsafe shapes", () => {
  assert.equal(parsePlatformAdministratorChange({ data: pending, meta })?.state, "PENDING_APPROVAL");
  assert.equal(parsePlatformAdministratorChange({ data: { ...pending, state: "EXECUTED" }, meta })?.state, "EXECUTED");
  assert.equal(parsePlatformAdministratorChange({ data: { ...pending, action: "DISABLE_IDENTITY", role_key: null, state: "PENDING_APPROVAL" }, meta })?.action, "DISABLE_IDENTITY");
  assert.equal(parsePlatformAdministratorChange({ data: { ...pending, action: "REACTIVATE_IDENTITY", role_key: null, state: "EXECUTED", expires_at: null }, meta })?.state, "EXECUTED");
  assert.equal(parsePlatformAdministratorChange({ data: { ...pending, state: "SATISFIED" }, meta }), null);
  assert.equal(parsePlatformAdministratorChange({ data: { ...pending, secret: "not-allowed" }, meta }), null);
  assert.equal(parsePlatformChangePage({ data: { changes: [pending, pending] }, meta }), null);
});

test("tenant registry parsing keeps placement metadata bounded and rejects private locators", () => {
  const tenant = { id: target, name: "Synthetic organization", tenant_type: "TENDERPRENEUR", state: "ACTIVE", country: "KE", timezone: "Africa/Nairobi", locale: "en-KE", assignment_generation: 2, version: 3, assignment_present: true, common_schema: { current_version: 9, generation: 2, state: "COMPATIBLE", updated_at: "2026-10-18T12:00:00Z" }, tenant_schema: null, created_at: "2026-10-18T12:00:00Z", updated_at: "2026-10-19T12:00:00Z" };
  assert.equal(parsePlatformTenantList({ data: { tenants: [tenant] }, meta })?.tenants[0].id, target);
  assert.equal(parsePlatformTenantDetail({ data: tenant, meta }, target)?.name, tenant.name);
  assert.equal(parsePlatformTenantDetail({ data: tenant, meta }, maker), null);
  assert.equal(parsePlatformTenantList({ data: { tenants: [tenant, tenant] }, meta }), null);
  assert.equal(parsePlatformTenantList({ data: { tenants: [{ ...tenant, database_name: "private" }] }, meta }), null);
  assert.equal(parsePlatformTenantDetail({ data: { ...tenant, assignment_generation: -1 }, meta }, target), null);
});

test("assignment history requires the requested administrator and complete role evidence", () => {
  const response = { data: { user_id: target, assignments: [assignment] }, meta };
  assert.equal(parsePlatformAssignmentHistory(response, target)?.assignments[0].revocation_reason, "Transferred");
  assert.equal(parsePlatformAssignmentHistory(response, maker), null);
  assert.equal(parsePlatformAssignmentHistory({ ...response, data: { ...response.data, assignments: [{ ...assignment, version: 0 }] } }, target), null);
});
