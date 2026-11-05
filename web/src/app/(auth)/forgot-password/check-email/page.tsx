import type { Metadata } from "next";
import Link from "next/link";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";

export const metadata: Metadata = { title: "Check your email" };

export default function RecoveryEmailPage() {
  return <AuthenticationExperience title="Check your email" description="If your account is eligible, you’ll receive a recovery email.">
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">Open the recovery link in that email. On the reset page, enter its one-time recovery code and choose a new password. Check your spam folder too.</p>
      <Button nativeButton={false} render={<Link href="/reset-password" />}>Enter recovery code</Button>
      <Button nativeButton={false} render={<Link href="/forgot-password" />} variant="outline">Request another recovery email</Button>
      <Link href="/login" className="text-sm text-muted-foreground underline underline-offset-4">Back to sign in</Link>
    </div>
  </AuthenticationExperience>;
}
