import {Suspense,type ReactNode} from "react";
import { LoginChallengeProvider } from "@/components/auth/login-challenge-provider";

export default function AuthenticationLayout({ children }: { children: ReactNode }) {
  return <LoginChallengeProvider><Suspense fallback={<p role="status">Loading…</p>}>{children}</Suspense></LoginChallengeProvider>;
}
