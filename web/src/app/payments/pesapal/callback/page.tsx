import { Suspense } from "react";
import { Brand } from "@/components/brand";
import { Card,CardContent,CardHeader,CardTitle } from "@/components/ui/card";
import { PesaPalReturn } from "@/components/organization/pesapal-return";
export default function PesaPalCallbackPage(){return <main className="min-h-svh bg-muted/30"><header className="border-b bg-background px-6 py-4"><Brand/></header><div className="app-page max-w-2xl"><Card><CardHeader><CardTitle>Payment verification</CardTitle></CardHeader><CardContent><Suspense fallback={<p role="status" className="text-sm text-muted-foreground">Reading provider return…</p>}><PesaPalReturn/></Suspense></CardContent></Card></div></main>;}
