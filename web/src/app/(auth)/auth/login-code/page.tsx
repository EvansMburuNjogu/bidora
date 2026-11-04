import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Verify sign-in" };

export default function MfaPage() {
  return (
    <AuthenticationExperience
      title="Verify sign-in"
      description="Enter your six-digit code."
    >
      <AuthForm mode="login-code" />
    </AuthenticationExperience>
  );
}
