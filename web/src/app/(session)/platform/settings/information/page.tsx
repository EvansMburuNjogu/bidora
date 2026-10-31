import { redirect } from "next/navigation";

export default function LegacyPlatformInformationPage() {
  redirect("/platform/settings/platform");
}
