import type { Metadata } from "next";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { GoogleCompletion } from "@/components/auth/google-completion";

export const metadata: Metadata = { title: "Completing Google sign-in" };

export default function GoogleCompletePage() {
  return (
    <AuthenticationExperience
      title="Finishing your sign-in"
      description="This should only take a moment."
    >
      <GoogleCompletion />
    </AuthenticationExperience>
  );
}
