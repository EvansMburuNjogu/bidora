import type { Metadata } from "next";
import { AdministratorDetail } from "@/components/platform/administrator-detail";

export const metadata: Metadata = { title: "Administrator details" };
export const instant = false;

export default async function PlatformAdministratorPage({ params }: { params: Promise<{ userId: string }> }) {
  const { userId } = await params;
  return <AdministratorDetail key={userId} userId={userId} />;
}
