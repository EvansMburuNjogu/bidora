import type { Metadata } from "next";
import { Manrope } from "next/font/google";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import "./globals.css";

const manrope = Manrope({
  variable: "--font-manrope",
  subsets: ["latin"],
  display: "swap",
  fallback: ["system-ui", "sans-serif"],
});

export const metadata: Metadata = {
  title: { default: "Bidora", template: "%s · Bidora" },
  description: "AI-powered tender preparation with human review.",
  icons: {
    icon: [{ url: "/icon.png?v=aperture-1", type: "image/png", sizes: "512x512" }],
    shortcut: "/favicon.ico?v=aperture-1",
    apple: [{ url: "/apple-icon.png?v=aperture-1", type: "image/png", sizes: "180x180" }],
  },
  robots: { index: false, follow: false },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={manrope.variable}>
      <body>
        <TooltipProvider>{children}</TooltipProvider>
        <Toaster position="top-right" closeButton duration={5000} visibleToasts={3} gap={12} mobileOffset={{ top: "max(16px, env(safe-area-inset-top))", left: 16, right: 16, bottom: 16 }} />
      </body>
    </html>
  );
}
