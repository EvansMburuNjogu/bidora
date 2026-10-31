import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Recover access" };

export default function ForgotPasswordPage() {
  return (
    <AuthenticationExperience
      title="Recover access"
      description="Enter your email. If recovery is available, we’ll send reset instructions."
    >
      <AuthForm mode="password-recovery" />
    </AuthenticationExperience>
  );
}
