import { notFound } from "next/navigation";
import { PlatformSupportPreviewDetail } from "@/components/platform/platform-support-preview-detail";
export const instant = false;
export default async function SupportTicketPage({ params }: { params: Promise<{ ticketId: string }> }) {
  if ((await params).ticketId !== "example-ticket") notFound();
  return <PlatformSupportPreviewDetail />;
}
