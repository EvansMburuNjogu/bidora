import type { Metadata } from "next";
import { SMTPConfiguration } from "@/components/platform/smtp-configuration";
export const metadata: Metadata = { title: "SMTP Configuration" };
export default function SMTPConfigurationPage() { return <SMTPConfiguration/>; }
