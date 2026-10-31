import type { Metadata } from "next";
import { PlatformTenantDetail } from "@/components/platform/platform-tenant-detail";

export const metadata: Metadata = { title: "Organization details" };
export const instant = false;
export default async function PlatformOrganizationPage({ params }: { params: Promise<{ tenantId: string }> }) {
  const { tenantId } = await params;
  return <PlatformTenantDetail key={tenantId} tenantId={tenantId} />;
}
