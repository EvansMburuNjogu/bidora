import { redirect } from "next/navigation";

// Keep existing bookmarks working without offering retired authenticator setup.
export default function LegacyAuthenticatorPage() {
  redirect("/account/security");
}
