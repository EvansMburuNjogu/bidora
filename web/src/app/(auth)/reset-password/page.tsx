import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Reset password" };

export default function ResetPasswordPage() {
  return (
    <AuthenticationExperience
      title="Reset your password"
      description="Enter the recovery code, then choose a new password."
    >
      <AuthForm mode="password-reset" />
    </AuthenticationExperience>
  );
}
