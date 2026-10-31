import { PlatformTenantDetail } from "@/components/platform/platform-tenant-detail";
export const instant = false;
export default async function TenderpreneurPage({ params }: { params: Promise<{ tenantId: string }> }) { const { tenantId } = await params; return <PlatformTenantDetail key={tenantId} tenantId={tenantId} />; }
