"use client";

import { useEffect } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { GoogleIcon } from "@/components/auth/google-icon";
import { Button } from "@/components/ui/button";

export function GoogleFailureActions() {
  useEffect(() => {
    toast.error("Google sign-in failed", {
      id: "google-sign-in-error",
      description: "Try again or use your email and password.",
    });
  }, []);

  return (
    <div className="flex flex-col gap-3">
      <form action="/api/auth/google-start" method="post">
        <Button type="submit" variant="outline" className="w-full">
          <GoogleIcon />
          Continue with Google
        </Button>
      </form>
      <Button nativeButton={false} render={<Link href="/login" />} variant="outline" className="w-full">
        Use email and password
      </Button>
    </div>
  );
}
