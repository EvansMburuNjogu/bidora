import Link from "next/link";
import type { Metadata } from "next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export const metadata: Metadata = { title: "Google linking incomplete" };

export default function GoogleLinkFailedPage() {
  return <main className="grid min-h-svh place-items-center bg-muted/30 p-4"><Card className="w-full max-w-md"><CardHeader><CardTitle>Google wasn’t linked</CardTitle><CardDescription>Your existing sign-in method is unchanged. Return to account security and try again.</CardDescription></CardHeader><CardContent><Button nativeButton={false} render={<Link href="/account/security" />}>Return to account security</Button></CardContent></Card></main>;
}
