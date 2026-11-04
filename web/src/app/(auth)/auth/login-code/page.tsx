import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { LoginCodeDescription } from "@/components/auth/login-code-description";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Verify sign-in" };

export default function MfaPage() {
  return (
    <AuthenticationExperience
      title="Verify sign-in"
      description={<LoginCodeDescription />}
    >
      <AuthForm mode="login-code" />
    </AuthenticationExperience>
  );
}
