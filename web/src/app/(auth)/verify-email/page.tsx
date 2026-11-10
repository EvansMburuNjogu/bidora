import type { Metadata } from "next";
import {Suspense} from "react";
import {VerificationEmailHint} from "@/components/auth/verification-email-hint";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Verify email" };

export default function VerifyEmailPage() {
  return (
    <AuthenticationExperience
      title="Verify your email"
      description={<Suspense fallback="Loading verification details…"><VerificationEmailHint/></Suspense>}
    >
      <AuthForm mode="verify-email" />
    </AuthenticationExperience>
  );
}
