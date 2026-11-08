import assert from "node:assert/strict";
import test from "node:test";
import { parseInvitationsForReissue } from "../src/lib/platform-invitation-reissue-contract.ts";
const id = "11111111-1111-4111-8111-111111111111";
const invitation = { id, email: "recipient@example.test", role_key: "SUPPORT_ADMINISTRATOR", state: "ACTIVE", version: 1, created_at: "2026-05-01T12:00:00Z", expires_at: "2026-05-04T12:00:00Z", delivery_state: "DELIVERY_UNCONFIRMED", governance_change_id: null };
test("reissue reads only structurally valid invitation records", () => {
 const response = { data: { invitations: [invitation] }, meta: { trace_id: "synthetic", next_cursor: null } };
 assert.equal(parseInvitationsForReissue(response)?.invitations[0].id, id);
 assert.equal(parseInvitationsForReissue({ ...response, data: { invitations: [invitation, invitation] } }), null);
 assert.equal(parseInvitationsForReissue({ ...response, data: { invitations: [{ ...invitation, role_key: "UNKNOWN" }] } }), null);
});
