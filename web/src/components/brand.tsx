"use client";

import { PlatformName } from "@/components/session/platform-identity-provider";
import Image from "next/image";
import Link from "next/link";

export function Brand({ href = "/login" }: { href?: string }) {
  return (
    <Link href={href} className="brand-wordmark inline-flex items-center gap-2.5" aria-label="Bidora home">
      <span className="relative size-8 shrink-0" aria-hidden="true">
        <Image src="/bidora-aperture-192.png" alt="" fill sizes="32px" className="object-contain dark:hidden" />
        <Image src="/bidora-aperture-inverse-192.png" alt="" fill sizes="32px" className="hidden object-contain dark:block" />
      </span>
      <span><PlatformName /></span>
    </Link>
  );
}
