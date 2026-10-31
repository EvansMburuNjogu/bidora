import type { Metadata } from "next";
import { LoginExperience } from "@/components/auth/login-experience";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage() {
  return <LoginExperience />;
}
