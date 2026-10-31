import { PlatformCommercialDetail } from "@/components/platform/platform-commercial-detail";
export const instant = false;
export default async function CommercialDetailPage({ params }: { params: Promise<{ queue: string; recordId: string }> }) {
  const { queue, recordId } = await params;
  return <PlatformCommercialDetail queue={queue} recordID={recordId} />;
}
