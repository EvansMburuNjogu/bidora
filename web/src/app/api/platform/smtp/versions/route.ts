import { proxySMTPRead } from "@/lib/server/platform-smtp-read-proxy";
export async function GET(request: Request) { return proxySMTPRead(request); }
