import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Verify email" };

export default function VerifyEmailPage() {
  return (
    <AuthenticationExperience
      title="Verify your email"
      description="Enter the code from your email to verify your address."
    >
      <AuthForm mode="verify-email" />
    </AuthenticationExperience>
  );
}
