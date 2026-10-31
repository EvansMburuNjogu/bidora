import Link from "next/link";
import type { Metadata } from "next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export const metadata: Metadata = { title: "Google linked" };

export default function GoogleLinkedPage() {
  return <main className="grid min-h-svh place-items-center bg-muted/30 p-4"><Card className="w-full max-w-md"><CardHeader><CardTitle>Google linked</CardTitle><CardDescription>Your Google identity was linked after verification. Your current session was rotated.</CardDescription></CardHeader><CardContent><Button nativeButton={false} render={<Link href="/account/security" />}>Return to account security</Button></CardContent></Card></main>;
}
