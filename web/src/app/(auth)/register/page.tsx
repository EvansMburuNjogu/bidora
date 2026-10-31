import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export const metadata: Metadata = { title: "Create account" };

export default function RegisterPage() {
  return (
    <AuthenticationExperience
      title="Create your account"
      description="Create your Bidora account."
    >
      <AuthForm mode="register" />
    </AuthenticationExperience>
  );
}
