import type { Metadata } from "next";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { GoogleFailureActions } from "@/components/auth/google-failure-actions";

export const metadata: Metadata = { title: "Google sign-in unavailable" };

export default function GoogleFailedPage() {
  return (
    <AuthenticationExperience
      title="Google sign-in didn’t work"
      description="Choose how you’d like to continue."
    >
      <GoogleFailureActions />
    </AuthenticationExperience>
  );
}
