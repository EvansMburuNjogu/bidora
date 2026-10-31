import type { ReactNode } from "react";
import { PlatformAdminShell } from "@/components/platform/platform-admin-shell";

export default function PlatformLayout({ children }: { children: ReactNode }) {
  return <PlatformAdminShell>{children}</PlatformAdminShell>;
}
