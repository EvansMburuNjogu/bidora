// Fictional, deterministic UI-development fixtures. Never merge these into API responses.
export const platformDemoEnabled = process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_BIDORA_DEMO_MODE === "1";

const names = ["Luma", "Kivio", "Nuruva", "Fikira", "Taviko", "Senara", "Zelumi", "Moriva", "Vekara", "Taluno", "Ravani", "Mekura", "Sovela", "Kezari", "Lumiko", "Valora", "Neruvo", "Kifaro", "Domeri", "Zanova"];
const suffixes = ["Works", "Studio", "Supply", "Partners"];
const day = (index: number) => new Date(Date.UTC(2026, 8, 1 + index)).toISOString().slice(0, 10);
export const demoOrganisations = Array.from({ length: 40 }, (_, index) => ({
  id: `demo-org-${String(index + 1).padStart(2, "0")}`,
  name: `${names[index % names.length]} ${suffixes[Math.floor(index / names.length)]}`,
  country: ["Kenya", "Uganda", "Tanzania", "Rwanda"][index % 4],
  status: ["Active", "Pending", "Suspended", "Active"][index % 4],
  createdAt: day(index),
}));
export const demoAdministrators = Array.from({ length: 20 }, (_, index) => ({
  id: `demo-admin-${String(index + 1).padStart(2, "0")}`,
  name: `${names[index]} Administrator`,
  email: `${names[index].toLowerCase()}@example.test`,
  role: ["Platform Owner", "Platform Administrator", "Billing Administrator", "Support Administrator"][index % 4],
  status: index % 7 === 0 ? "Disabled" : "Active",
}));
export const demoPlans = [
  { id: "demo-plan-starter", name: "Starter", price: 4900, limit: 10, status: "Active" },
  { id: "demo-plan-team", name: "Team", price: 14900, limit: 50, status: "Active" },
  { id: "demo-plan-scale", name: "Scale", price: 39900, limit: 200, status: "Inactive" },
];
export const demoSubscriptions = demoOrganisations.map((organisation, index) => ({ id: `demo-sub-${index + 1}`, organisationId: organisation.id, planId: demoPlans[index % demoPlans.length].id, status: index % 6 === 0 ? "Past due" : "Active", date: day(index) }));
export const demoInvoices = demoSubscriptions.map((subscription, index) => ({ id: `DEMO-INV-${String(index + 1).padStart(3, "0")}`, organisationId: subscription.organisationId, amount: demoPlans[index % demoPlans.length].price, status: index % 6 === 0 ? "Outstanding" : "Paid", date: day(index) }));
export const demoPayments = demoInvoices.filter((invoice) => invoice.status === "Paid").map((invoice) => ({ id: `DEMO-PAY-${invoice.id.slice(-3)}`, invoiceId: invoice.id, organisationId: invoice.organisationId, amount: invoice.amount, status: "Paid", date: invoice.date }));
export const demoAdjustments = demoInvoices.filter((_, index) => index % 9 === 0).map((invoice) => ({ id: `DEMO-ADJ-${invoice.id.slice(-3)}`, invoiceId: invoice.id, amount: -500, date: invoice.date }));
export const demoAIUsage = demoOrganisations.slice(0, 24).map((organisation, index) => ({ id: `demo-ai-${index + 1}`, organisationId: organisation.id, operation: index % 2 ? "Document analysis" : "Requirement extraction", units: 40 + index * 3, cost: (40 + index * 3) * 2, status: "Recorded", date: day(index) }));
export const demoNotifications = demoOrganisations.slice(0, 24).map((organisation, index) => ({ id: `demo-notice-${index + 1}`, organisationId: organisation.id, recipient: `team${index + 1}@example.test`, type: index % 2 ? "Account update" : "Subscription reminder", status: ["Delivered", "Sent", "Failed", "Pending"][index % 4], date: day(index) }));
export const demoTickets = demoOrganisations.slice(0, 16).map((organisation, index) => ({ id: `DEMO-TKT-${String(index + 1).padStart(3, "0")}`, organisationId: organisation.id, subject: ["Access to workspace", "Invoice question", "Plan change request", "Notification help"][index % 4], assigneeId: demoAdministrators[index % demoAdministrators.length].id, status: ["Open", "In progress", "Resolved", "Open"][index % 4], date: day(index), message: "Please review this request when available." }));
export const demoReports = ["Subscriptions", "Payments", "AI usage", "Notifications", "Support"].map((name, index) => ({ id: `demo-report-${index + 1}`, name, date: day(index + 25), status: "Preview" }));
export const demoAudit = demoOrganisations.slice(0, 18).map((organisation, index) => ({ id: `demo-event-${index + 1}`, organisationId: organisation.id, actorId: demoAdministrators[index % demoAdministrators.length].id, action: index % 2 ? "Viewed organisation" : "Reviewed subscription", status: "Recorded", date: day(index) }));
export const demoSettings = { supportEmail: "support@example.test", website: "https://bidora.example.test", country: "Kenya", currency: "KES", timeZone: "Africa/Nairobi", smtpHost: "smtp.example.test", model: "Example model (not connected)" } as const;

export type DemoModule = "plans" | "ai" | "communications" | "support" | "reports" | "audit";
export type DemoRow = { id: string; cells: string[]; status: string };
const orgName = (id: string) => demoOrganisations.find((organisation) => organisation.id === id)?.name ?? "Example organisation";
const adminName = (id: string) => demoAdministrators.find((administrator) => administrator.id === id)?.name ?? "Example administrator";
export function demoRows(module: DemoModule, tab?: string): DemoRow[] {
  switch (module) {
    case "plans": return tab === "Subscriptions" ? demoSubscriptions.map((item) => ({ id: item.id, cells: [orgName(item.organisationId), demoPlans.find((plan) => plan.id === item.planId)?.name ?? "—", "Monthly", item.date, item.status], status: item.status })) : demoPlans.map((plan) => ({ id: plan.id, cells: [plan.name, "Monthly", `KES ${plan.price.toLocaleString("en-KE")}`, String(plan.limit), plan.status], status: plan.status }));
    case "ai": return demoAIUsage.map((item) => ({ id: item.id, cells: [orgName(item.organisationId), item.operation, String(item.units), `KES ${item.cost}`, item.date], status: item.status }));
    case "communications": return demoNotifications.map((item) => ({ id: item.id, cells: [item.recipient, item.type, "Email", item.status, item.date], status: item.status }));
    case "support": return demoTickets.filter((item) => tab !== "Open" || item.status !== "Resolved").map((item) => ({ id: item.id, cells: [item.id, item.subject, orgName(item.organisationId), "Normal", adminName(item.assigneeId), item.status, item.date], status: item.status }));
    case "reports": return demoReports.map((item) => ({ id: item.id, cells: [item.name, item.date, "All organisations", item.status], status: item.status }));
    case "audit": return demoAudit.map((item) => ({ id: item.id, cells: [item.date, adminName(item.actorId), item.action, "Subscriptions", orgName(item.organisationId), item.status], status: item.status }));
  }
}
