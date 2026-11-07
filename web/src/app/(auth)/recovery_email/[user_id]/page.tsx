import type { Metadata } from "next";
import { Suspense } from "react";
import { notFound } from "next/navigation";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { AuthForm } from "@/components/auth/auth-form";

export const metadata: Metadata = {
  title: "Reset password",
  robots: { index: false, follow: false },
  referrer: "no-referrer",
};

async function RecoveryForm({ params }: { params: Promise<{ user_id: string }> }) {
  const { user_id } = await params;
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(user_id) || /^0{8}-0{4}-0{4}-0{4}-0{12}$/.test(user_id)) notFound();
  return <AuthForm mode="password-reset" recoveryUserId={user_id} />;
}

export default function RecoveryEmailPage({ params }: { params: Promise<{ user_id: string }> }) {
  return <AuthenticationExperience title="Reset your password" description="Your recovery code is included in the link. Choose and confirm your new password.">
    <Suspense fallback={<p className="text-sm text-muted-foreground">Loading recovery form…</p>}>
      <RecoveryForm params={params} />
    </Suspense>
  </AuthenticationExperience>;
}
