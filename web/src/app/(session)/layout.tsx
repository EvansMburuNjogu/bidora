import type { ReactNode } from "react";
import { PlatformIdentityProvider } from "@/components/session/platform-identity-provider";
import { SessionProvider } from "@/components/session/session-provider";

export default function SessionLayout({ children }: { children: ReactNode }) {
  return <SessionProvider><PlatformIdentityProvider>{children}</PlatformIdentityProvider></SessionProvider>;
}
