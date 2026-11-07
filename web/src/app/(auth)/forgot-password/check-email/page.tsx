import type { Metadata } from "next";
import Link from "next/link";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";

export const metadata: Metadata = { title: "Check your email" };

export default function RecoveryEmailPage() {
  return <AuthenticationExperience title="Check your email" description="If your account is eligible, you’ll receive a recovery email.">
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">Open the secure link in that email to choose and confirm your new password. Your recovery code will be filled in automatically. Check your spam folder too.</p>
      <Button nativeButton={false} render={<Link href="/forgot-password" />} variant="outline">Request another recovery email</Button>
      <Link href="/login" className="text-sm text-muted-foreground underline underline-offset-4">Back to sign in</Link>
    </div>
  </AuthenticationExperience>;
}
