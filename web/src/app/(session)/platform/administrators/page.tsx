import type { Metadata } from "next";
import { AdministratorDirectory } from "@/components/platform/administrator-directory";

export const metadata: Metadata = { title: "Administrators" };

export default function PlatformAdministratorsPage() {
  return <AdministratorDirectory />;
}
