import type { Metadata } from "next";
import { AdministratorApprovals } from "@/components/platform/administrator-approvals";

export const metadata: Metadata = { title: "Approvals" };
export default function PlatformApprovalsPage() { return <AdministratorApprovals />; }
