import { AuthForm } from "@/components/auth/auth-form";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";

export function LoginExperience() {
  return (
    <AuthenticationExperience title="Sign in" description="Use your work email and password to continue.">
      <AuthForm mode="login" />
    </AuthenticationExperience>
  );
}
