import type { Metadata } from "next";
import { redirect } from "next/navigation";

export const metadata: Metadata = { title: "Approvals" };
export default function PlatformApprovalsPage() { redirect("/platform/users"); }
