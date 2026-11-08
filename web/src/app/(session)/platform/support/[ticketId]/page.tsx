import { notFound } from "next/navigation";
import { PlatformSupportCaseDetail } from "@/components/platform/platform-support-case-detail";
import { PlatformSupportPreviewDetail } from "@/components/platform/platform-support-preview-detail";
import { auditUUID } from "@/lib/platform-audit-contract";

export const instant = false;
export default async function SupportTicketPage({ params }: { params: Promise<{ ticketId: string }> }) {
  const { ticketId } = await params;
  if (ticketId === "example-ticket") return <PlatformSupportPreviewDetail />;
  if (!auditUUID.test(ticketId)) notFound();
  return <PlatformSupportCaseDetail caseID={ticketId} />;
}
