"use client";

import { useEffect,useRef, useState } from "react";
import Link from "next/link";
import { Building2Icon, CheckCircle2Icon, CreditCardIcon, SparklesIcon,Loader2Icon } from "lucide-react";
import { toast } from "sonner";
import type { OrganizationCommercialPath, OrganizationCreationRequest, OrganizationCreationResponse } from "@/contracts/auth.generated";
import { Brand } from "@/components/brand";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldSet, FieldLegend } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { parseOrganizationCreation, safeOrganizationCreationCode, submitOrganizationCreation } from "@/lib/organization-creation-client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { parseSharedCountries,type SharedCountries } from "@/lib/platform-regional-contract";

type FormValues = Omit<OrganizationCreationRequest, "currency" | "commercial_path"> & {
  commercial_path: OrganizationCommercialPath | "";
};
type FieldName = keyof FormValues | "organization_type";
type Errors = Partial<Record<FieldName | "form", string>>;

const initialValues: FormValues = {
  name: "",
  country: "",
  timezone: "",
  locale: "",
  business_type: "",
  primary_industry: "",
  preferred_language: "",
  commercial_path: "",
};

const fields: Array<{ name: keyof Omit<FormValues, "commercial_path">; label: string; description?: string; placeholder: string; maxLength: number }> = [
  { name: "name", label: "Organization name", description: "Use the registered or working name your team recognizes.", placeholder: "Example Engineering Limited", maxLength: 300 },
  { name: "business_type", label: "Business type", placeholder: "Limited company", maxLength: 100 },
  { name: "primary_industry", label: "Primary industry", placeholder: "Construction and engineering", maxLength: 160 },
  { name: "country", label: "Country code", description: "Two-letter uppercase code.", placeholder: "KE", maxLength: 2 },
  { name: "timezone", label: "Timezone", description: "Use an IANA timezone.", placeholder: "Africa/Nairobi", maxLength: 128 },
  { name: "locale", label: "Locale", placeholder: "en-KE", maxLength: 35 },
  { name: "preferred_language", label: "Preferred language", placeholder: "en", maxLength: 35 },
];

function validate(values: FormValues, confirmedType: boolean): Errors {
  const errors: Errors = {};
  for (const field of fields) {
    const value = values[field.name];
    if (!value || value.trim() !== value) errors[field.name] = `${field.label} is required without leading or trailing spaces.`;
  }
  if (values.country && !/^[A-Z]{2}$/.test(values.country)) errors.country = "Use a two-letter uppercase country code, such as KE.";
  if (values.locale && !/^[A-Za-z0-9-]{1,35}$/.test(values.locale)) errors.locale = "Use a locale such as en-KE.";
  if (values.preferred_language && !/^[A-Za-z0-9-]{1,35}$/.test(values.preferred_language)) errors.preferred_language = "Use a language code such as en.";
  if (!confirmedType) errors.organization_type = "Confirm the fixed Tenderpreneur organization type.";
  if (!values.commercial_path) errors.commercial_path = "Choose Trial or Paid before continuing.";
  return errors;
}

export function OrganizationCreationForm() {
  const { csrfToken } = useSession();
  const [values, setValues] = useState(initialValues);
  const [confirmedType, setConfirmedType] = useState(false);
  const [errors, setErrors] = useState<Errors>({});
  const [submitting, setSubmitting] = useState(false);
  const [pending, setPending] = useState<OrganizationCreationResponse | null>(null);
  const idempotencyKey = useRef<string | null>(null);
 const lock=useRef(false),idempotencyBody=useRef<string|null>(null);
 const [regional,setRegional]=useState<SharedCountries|null>(null),[regionalError,setRegionalError]=useState(""),[reload,setReload]=useState(0);
 useEffect(()=>{let active=true;void fetch("/api/session/countries",{cache:"no-store",credentials:"same-origin",redirect:"error"}).then(async r=>{const data=r.ok?parseSharedCountries(await r.json()):null;if(!data)throw Error();if(!active)return;setRegional(data);setRegionalError("");setValues(old=>{const country=data.countries.find(c=>c.country_code===(old.country||data.default_country));return country?{...old,country:country.country_code,timezone:country.timezone,locale:country.locale,preferred_language:country.language}:old;});}).catch(()=>{if(active)setRegionalError("Available countries could not be loaded. Organization requests remain unavailable.");});return()=>{active=false;};},[reload]);
 function selectCountry(code:string){const country=regional?.countries.find(c=>c.country_code===code);if(!country)return;setValues(old=>({...old,country:country.country_code,timezone:country.timezone,locale:country.locale,preferred_language:country.language}));setErrors({});idempotencyKey.current=null;}

  function update<Name extends keyof FormValues>(name: Name, value: FormValues[Name]) {
    setValues((current) => ({ ...current, [name]: value }));
    setErrors((current) => ({ ...current, [name]: undefined, form: undefined }));
    idempotencyKey.current = null;
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
 if(lock.current)return;
 if(!regional||regionalError||!regional.organization_registration_enabled){toast.error("Organization requests are temporarily unavailable.",{id:"organization-create"});return;}
    const nextErrors = validate(values, confirmedType);
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }
    const fingerprint=JSON.stringify(values);if(idempotencyBody.current!==fingerprint)idempotencyKey.current=null;idempotencyBody.current=fingerprint;
    const key = idempotencyKey.current ?? crypto.randomUUID();
    idempotencyKey.current = key;
    lock.current=true;
    setSubmitting(true);
    setErrors({});
    try {
      const response = await submitOrganizationCreation(
        { ...values, commercial_path: values.commercial_path as OrganizationCommercialPath, currency: regional.billing_currency },
        csrfToken,
        key,
      );
      if (response.status === 202) {
        const result = parseOrganizationCreation(await response.json());
        if (!result) throw new Error("invalid_response");
        setPending(result);
        toast.info("Organization request received", { id:"organization-create",description: "Pending provisioning; no active workspace is confirmed." });
        return;
      }
      const code = await safeOrganizationCreationCode(response);
      const message = code === "ASSURANCE_REQUIRED"
        ? "Sign in again or complete two-step verification before creating an organization."
        : code === "ORGANIZATION_CREATION_RATE_LIMITED"
          ? "Too many creation requests. Wait 15 minutes before trying again."
          : code === "ORGANIZATION_CREATION_CONFLICT" || code === "ORGANIZATION_CREATION_INELIGIBLE"
            ? "This account cannot start another organization request."
            : "Organization creation is not available yet. Your details remain on this page.";
      toast.error("Request not submitted", { id:"organization-create",description: message });
    } catch {
      const message = "Organization creation is temporarily unavailable. Your details remain on this page.";
      toast.error("Request not submitted", { id:"organization-create",description: message });
    } finally {
      lock.current=false;
      setSubmitting(false);
    }
  }

  if (pending) {
    return (
      <div className="flex flex-col gap-5">
        <Alert variant="success" role="status">
          <CheckCircle2Icon aria-hidden="true" />
          <AlertTitle>Request received</AlertTitle>
          <AlertDescription>
            <p>Your organization is still pending. This does not mean a database, subscription, or workspace has been activated.</p>
          </AlertDescription>
        </Alert>
        <div className="flex flex-wrap gap-3">
          <Button nativeButton={false} render={<Link href="/auth/resolve" />}>Check access status</Button>
          <Button type="button" variant="outline" onClick={() => setPending(null)}>View submitted details</Button>
        </div>
      </div>
    );
  }

  return (
    <form className="flex flex-col gap-6" onSubmit={(event) => void submit(event)} noValidate>
      <FieldSet>
        <FieldLegend>Organization details</FieldLegend>
        <FieldDescription>Your verified account email becomes the initial work contact.</FieldDescription>
        {regionalError?<p role="alert" className="text-sm text-destructive">{regionalError} <Button type="button" variant="outline" size="sm" onClick={()=>setReload(v=>v+1)}>Retry</Button></p>:!regional?<p role="status" className="text-sm text-muted-foreground">Loading available countries…</p>:!regional.organization_registration_enabled?<p role="status" className="text-sm text-muted-foreground">New organization requests are temporarily unavailable.</p>:null}
        <FieldGroup className="grid gap-5 sm:grid-cols-2">
         <Field data-invalid={Boolean(errors.country)}><FieldLabel htmlFor="organization-country">Country</FieldLabel><PlatformSelect id="organization-country" value={values.country} onChange={e=>selectCountry(e.target.value)} disabled={!regional||submitting||!!regionalError}><option value="">Choose an enabled country</option>{regional?.countries.map(c=><option key={c.country_code} value={c.country_code}>{c.name}</option>)}</PlatformSelect><FieldDescription>Regional defaults come from the current enabled country configuration.</FieldDescription><FieldError>{errors.country}</FieldError></Field>
          {fields.filter(field=>field.name!=="country").map((field) => (
            <Field key={field.name} data-invalid={Boolean(errors[field.name])} className={field.name === "name" ? "sm:col-span-2" : undefined}>
              <FieldLabel htmlFor={`organization-${field.name}`}>{field.label}</FieldLabel>
              <Input
                id={`organization-${field.name}`}
                name={field.name}
                value={values[field.name]}
                placeholder={field.placeholder}
                maxLength={field.maxLength}
                autoComplete="off"
                readOnly={["timezone","locale","preferred_language"].includes(field.name)}
                disabled={submitting}
                aria-invalid={Boolean(errors[field.name])}
                onChange={(event) => update(field.name, field.name === "country" ? event.target.value.toUpperCase() : event.target.value)}
              />
              {field.description ? <FieldDescription>{field.description}</FieldDescription> : null}
              <FieldError>{errors[field.name]}</FieldError>
            </Field>
          ))}
          <Field>
            <FieldLabel htmlFor="organization-currency">Currency</FieldLabel>
            <Input id="organization-currency" value={regional?.billing_currency??""} readOnly aria-readonly="true" />
            <FieldDescription>Phase 1 plans and limits use Kenyan shillings.</FieldDescription>
          </Field>
        </FieldGroup>
      </FieldSet>

      <FieldSet>
        <FieldLegend>Organization type</FieldLegend>
        <Field data-invalid={Boolean(errors.organization_type)}>
          <FieldLabel className="w-full rounded-lg border bg-card p-4">
            <Checkbox
              checked={confirmedType}
              onCheckedChange={(checked) => {
                setConfirmedType(checked);
                setErrors((current) => ({ ...current, organization_type: undefined, form: undefined }));
                idempotencyKey.current = null;
              }}
              aria-invalid={Boolean(errors.organization_type)}
            />
            <span className="flex flex-col gap-1">
              <span className="font-medium">Create a Tenderpreneur organization</span>
              <span className="text-sm font-normal text-muted-foreground">This creates a private tender-preparation organization request. It does not create a procurement organization.</span>
            </span>
          </FieldLabel>
          <FieldError>{errors.organization_type}</FieldError>
        </Field>
      </FieldSet>

      <FieldSet>
        <FieldLegend>Commercial path</FieldLegend>
        <FieldDescription>Nothing is charged or activated by submitting this form.</FieldDescription>
        <div className="grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label="Commercial path">
          {([
            { value: "TRIAL" as const, title: "14-day Trial", copy: "Provisioning must complete before the one-time trial clock can begin.", icon: SparklesIcon },
            { value: "PAID" as const, title: "Paid plan", copy: "Provisioning waits for a separate exact plan choice and reconciled payment.", icon: CreditCardIcon },
          ]).map(({ value, title, copy, icon: Icon }) => (
            <label key={value} className="relative flex min-h-28 cursor-pointer gap-3 rounded-xl border bg-card p-4 transition-colors has-[:checked]:border-primary has-[:checked]:bg-primary/5 has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/40">
              <input
                className="sr-only"
                type="radio"
                name="commercial_path"
                value={value}
                checked={values.commercial_path === value}
                onChange={() => update("commercial_path", value)}
              />
              <Icon className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />
              <span className="flex flex-col gap-1">
                <span className="font-medium">{title}</span>
                <span className="text-sm text-muted-foreground">{copy}</span>
              </span>
            </label>
          ))}
        </div>
        <FieldError>{errors.commercial_path}</FieldError>
      </FieldSet>

      {errors.form ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Request not submitted</AlertTitle>
          <AlertDescription>{errors.form}</AlertDescription>
        </Alert>
      ) : null}

      <Alert variant="info">
        <Building2Icon aria-hidden="true" />
        <AlertTitle>Pending means pending</AlertTitle>
        <AlertDescription>A successful submission records a request only. Bidora will not claim your organization is ready until provisioning, compatibility, membership, and commercial checks all complete.</AlertDescription>
      </Alert>

      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
        <Button nativeButton={false} render={<Link href="/auth/resolve" />} variant="outline" className="max-sm:w-full">Cancel</Button>
        <Button type="submit" disabled={submitting||!regional||!!regionalError||!regional.organization_registration_enabled} className="max-sm:w-full">
          {submitting?<Loader2Icon aria-hidden="true" className="animate-spin"/>:null}{submitting ? "Submitting request…" : "Submit organization request"}
        </Button>
      </div>
    </form>
  );
}

export function OrganizationCreationExperience() {
  const { resolution } = useSession();
  const { isSigningOut, signOut } = useSignOut();
  return (
    <main className="min-h-svh bg-muted/30">
      <header className="flex min-h-14 flex-wrap items-center justify-between gap-3 border-b bg-background px-4 py-2 sm:px-6">
        <Brand />
        <div className="flex items-center gap-3">
          <span className="type-meta text-muted-foreground">{resolution.session.assurance_level === "AAL2" ? "Two-step verified" : "Signed in"}</span>
          <Button type="button" variant="ghost" size="sm" disabled={isSigningOut} onClick={() => void signOut()}>{isSigningOut ? "Signing out…" : "Sign out"}</Button>
        </div>
      </header>
      <div className="app-page max-w-5xl">
        <div className="app-page-header">
          <p className="type-kicker text-muted-foreground">Tenderpreneur setup</p>
          <h1 className="type-title">Create your organization</h1>
          <p className="type-lead text-muted-foreground">Provide the minimum details needed to request a private Tenderpreneur workspace.</p>
        </div>
        <Card>
          <CardHeader className="border-b">
            <CardTitle>Organization request</CardTitle>
            <CardDescription>You can review every choice before submitting. No organization type or commercial path is preselected.</CardDescription>
          </CardHeader>
          <CardContent>
            <OrganizationCreationForm />
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
