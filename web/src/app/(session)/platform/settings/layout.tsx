import { PlatformSettingsNavigation } from "@/components/platform/platform-settings-navigation";

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  return <PlatformSettingsNavigation>{children}</PlatformSettingsNavigation>;
}
