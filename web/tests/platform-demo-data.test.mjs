import test from "node:test";
import assert from "node:assert/strict";
import {
  platformDemoEnabled, demoOrganisations, demoAdministrators, demoPlans, demoSubscriptions,
  demoInvoices, demoPayments, demoAIUsage, demoNotifications, demoTickets, demoAudit,
  demoRows, demoSettings,
} from "../src/lib/platform-demo-data.ts";

test("fictional demo records are coherent and never enable themselves", () => {
  assert.equal(platformDemoEnabled, process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_BIDORA_DEMO_MODE === "1");
  assert.equal(demoOrganisations.length, 40);
  assert.equal(demoAdministrators.length, 20);
  const orgIDs = new Set(demoOrganisations.map((item) => item.id));
  const adminIDs = new Set(demoAdministrators.map((item) => item.id));
  const planIDs = new Set(demoPlans.map((item) => item.id));
  const invoiceIDs = new Set(demoInvoices.map((item) => item.id));
  for (const row of [...demoSubscriptions, ...demoInvoices, ...demoPayments, ...demoAIUsage, ...demoNotifications, ...demoTickets, ...demoAudit]) assert.ok(orgIDs.has(row.organisationId));
  for (const row of demoSubscriptions) assert.ok(planIDs.has(row.planId));
  for (const row of demoPayments) assert.ok(invoiceIDs.has(row.invoiceId));
  for (const row of demoTickets) assert.ok(adminIDs.has(row.assigneeId));
  for (const row of demoAudit) assert.ok(adminIDs.has(row.actorId));
  assert.ok(demoAdministrators.every((item) => item.email.endsWith("@example.test")));
  assert.ok(demoNotifications.every((item) => item.recipient.endsWith("@example.test")));
  assert.equal(demoSettings.supportEmail, "support@example.test");
  assert.ok(demoRows("plans", "Subscriptions").length >= 30);
  assert.ok(demoRows("ai").length >= 20);
  assert.ok(demoRows("communications").length >= 20);
  assert.ok(demoRows("support", "All tickets").length >= 10);
  assert.ok(demoRows("reports").length > 0);
  assert.ok(demoRows("audit").length > 0);
});
