import type { Metadata } from "next";
import Link from "next/link";
import { MFAEnrollment } from "@/components/auth/mfa-enrollment";

export const metadata: Metadata = { title: "Administrator verification" };

export default function AdministratorVerificationPage() {
  return <main className="app-page min-h-svh max-w-2xl space-y-6">
    <Link href="/account/security" className="text-sm underline underline-offset-4">Back to account security</Link>
    <div>
      <h1 className="type-title">Administrator verification</h1>
      <p className="mt-2 text-sm text-muted-foreground">Required administrator assurance is separate from email sign-in codes.</p>
    </div>
    <MFAEnrollment />
  </main>;
}
