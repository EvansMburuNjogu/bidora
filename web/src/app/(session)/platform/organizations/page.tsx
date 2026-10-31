import type { Metadata } from "next";
import { PlatformTenantDirectory } from "@/components/platform/platform-tenant-directory";

export const metadata: Metadata = { title: "Organizations" };
export default function PlatformOrganizationsPage() { return <PlatformTenantDirectory />; }
