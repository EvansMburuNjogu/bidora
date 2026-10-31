import { PlatformUnavailable } from "@/components/platform/platform-unavailable";

export function PlatformSettingsUnavailable({ title, detail }: { title: string; detail: string }) {
  return <PlatformUnavailable title={title} detail={detail} backHref={null} />;
}
