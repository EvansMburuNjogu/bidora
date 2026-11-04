import type { ReactNode } from "react";
import { LoginChallengeProvider } from "@/components/auth/login-challenge-provider";

export default function AuthenticationLayout({ children }: { children: ReactNode }) {
  return <LoginChallengeProvider>{children}</LoginChallengeProvider>;
}
