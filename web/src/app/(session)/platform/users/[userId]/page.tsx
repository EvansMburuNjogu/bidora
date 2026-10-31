import { AdministratorDetail } from "@/components/platform/administrator-detail";
export const instant = false;
export default async function UserPage({ params }: { params: Promise<{ userId: string }> }) { const { userId } = await params; return <AdministratorDetail key={userId} userId={userId} />; }
