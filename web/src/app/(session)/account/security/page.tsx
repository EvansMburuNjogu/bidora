import type { Metadata } from "next";
import { AccountSecurity } from "@/components/auth/account-security";

export const metadata: Metadata = { title: "Account security" };

export default function AccountSecurityPage() {
  return <AccountSecurity />;
}
