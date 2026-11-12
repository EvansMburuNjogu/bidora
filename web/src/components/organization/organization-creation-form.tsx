"use client";

import { useCallback,useEffect,useRef, useState } from "react";
import Link from "next/link";
import { Building2Icon, CheckCircle2Icon, CreditCardIcon, SparklesIcon,Loader2Icon } from "lucide-react";
import { toast } from "sonner";
import type { OrganizationCommercialPath, OrganizationCreationRequest, OrganizationCreationResponse } from "@/contracts/auth.generated";
import { Brand } from "@/components/brand";
import { useSession } from "@/components/session/session-provider";
import { selectAccountType } from "@/lib/account-type-client";
import { useSignOut } from "@/hooks/use-sign-out";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldSet, FieldLegend } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { parseOrganizationCreation, parseOrganizationCreationStatus, safeOrganizationCreationCode, submitOrganizationCreation, type OrganizationCreationStatus } from "@/lib/organization-creation-client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { parseSharedCountries,type SharedCountries } from "@/lib/platform-regional-contract";
import { parseOrganizationReferences, type OrganizationReference, type OrganizationReferenceKind } from "@/lib/organization-reference-contract";

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
  const { csrfToken, resolution, refresh } = useSession();
  const needsAccountType = resolution.account_type === "UNASSIGNED";
  const [values, setValues] = useState(initialValues);
  const [confirmedType, setConfirmedType] = useState(false);
  const [errors, setErrors] = useState<Errors>({});
  const [submitting, setSubmitting] = useState(false);
  const [pending, setPending] = useState<OrganizationCreationResponse | null>(null);
  const [step, setStep] = useState(0);
  const [uncertain, setUncertain] = useState(false);
  const [existing, setExisting] = useState<OrganizationCreationStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(true), [statusError, setStatusError] = useState(""), [recoveryRequired, setRecoveryRequired] = useState(false);
  const loadStatus = useCallback(async () => {
    try {
      const response = await fetch("/api/tenderpreneur/organizations", { credentials: "same-origin", cache: "no-store", redirect: "error" });
      const parsed = response.ok ? parseOrganizationCreationStatus(await response.json()) : null;
      if (!parsed) throw new Error("status_unavailable");
      const unresolved = sessionStorage.getItem("bidora_organization_request_unconfirmed") === "1";
      setExisting(parsed.status); setRecoveryRequired(!parsed.status && unresolved); setStatusError("");
      if (parsed.status) sessionStorage.removeItem("bidora_organization_request_unconfirmed");
    } catch { setStatusError("Request status could not be checked. Please retry before starting an organization request."); }
    finally { setStatusLoading(false); }
  }, []);
  useEffect(() => { void Promise.resolve().then(loadStatus); }, [loadStatus]);
  const idempotencyKey = useRef<string | null>(null);
  const accountTypeKey = useRef<string | null>(null), accountTypeConfirmed = useRef(false);
 const lock=useRef(false),idempotencyBody=useRef<string|null>(null);
 const [regional,setRegional]=useState<SharedCountries|null>(null),[regionalError,setRegionalError]=useState(""),[reload,setReload]=useState(0);
 const [references,setReferences]=useState<Record<OrganizationReferenceKind,OrganizationReference[]>|null>(null),[referenceError,setReferenceError]=useState("");
 useEffect(()=>{let active=true;const read=async(kind:OrganizationReferenceKind)=>{const response=await fetch(`/api/session/organization-references/${kind}`,{cache:"no-store",credentials:"same-origin",redirect:"error"});const value=response.ok?parseOrganizationReferences(await response.json(),kind,true):null;if(!value)throw Error();return value;};void Promise.all([read("business-types"),read("industries")]).then(([business,industry])=>{if(active){setReferences({"business-types":business,industries:industry});setReferenceError("");}}).catch(()=>{if(active){setReferences(null);setReferenceError("Business types and industries could not be loaded. Organization requests remain unavailable.");}});return()=>{active=false;};},[reload]);
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
 if(!regional||regionalError||!references||referenceError){toast.error("Organization options are temporarily unavailable.",{id:"organization-create"});return;}
    const nextErrors = validate(values, confirmedType);
    if (!references["business-types"].some(row => row.code === values.business_type)) nextErrors.business_type = "Choose an available business type.";
    if (!references.industries.some(row => row.code === values.primary_industry)) nextErrors.primary_industry = "Choose an available industry.";
    const groups: FieldName[][] = [["name", "country", "timezone", "locale", "preferred_language"], ["business_type", "primary_industry", "organization_type"], ["commercial_path"]];
    const failed = groups.findIndex(group => group.some(field => nextErrors[field]));
    if (step < 3) {
      const currentErrors = Object.fromEntries(Object.entries(nextErrors).filter(([field]) => groups[step].includes(field as FieldName))) as Errors;
      if (Object.keys(currentErrors).length) { setErrors(currentErrors); return; }
      setErrors({}); setStep(step + 1); return;
    }
    if (failed >= 0) { setErrors(nextErrors); setStep(failed); return; }
    if (!regional.organization_registration_enabled) { toast.error("Organization requests are temporarily unavailable.", { id: "organization-create" }); return; }
    const fingerprint=JSON.stringify(values);if(idempotencyBody.current!==fingerprint)idempotencyKey.current=null;idempotencyBody.current=fingerprint;
    const key = idempotencyKey.current ?? crypto.randomUUID();
    idempotencyKey.current = key;
    lock.current=true;
    setSubmitting(true);
    setErrors({});
    try {
      if (needsAccountType && !accountTypeConfirmed.current) {
        accountTypeKey.current ??= crypto.randomUUID();
        let selection: Response;
        try { selection = await selectAccountType({ account_type: "TENDERPRENEUR" }, csrfToken, accountTypeKey.current); }
        catch { toast.error("Account type could not be confirmed", { id: "organization-create", description: "Retry the same request; no organization request has been sent." }); return; }
        if (selection.status === 409 || selection.status === 401) { toast.info("Refreshing account details", { id: "organization-create" }); refresh(); return; }
        if (selection.status !== 204) { toast.error("Account type could not be confirmed", { id: "organization-create", description: "Retry the same request; no organization request has been sent." }); return; }
        accountTypeConfirmed.current = true;
      }
      try { sessionStorage.setItem("bidora_organization_request_unconfirmed", "1"); }
      catch { toast.error("Request cannot be started", { id: "organization-create", description: "Browser session storage is unavailable. Enable it before submitting." }); return; }
      const response = await submitOrganizationCreation(
        { ...values, commercial_path: values.commercial_path as OrganizationCommercialPath, currency: regional.billing_currency },
        csrfToken,
        key,
      );
      if (response.status === 202) {
        const result = parseOrganizationCreation(await response.json());
        if (!result) throw new Error("invalid_response");
        try { sessionStorage.removeItem("bidora_organization_request_unconfirmed"); } catch { /* The status endpoint remains authoritative. */ }
        setPending(result);
        toast.info("Organization request received", { id:"organization-create",description: "Pending provisioning; no active workspace is confirmed." });
        return;
      }
      if (response.status >= 500) { setUncertain(true); toast.error("Request could not be confirmed", { id: "organization-create", description: "Retry this same request without changing its details, or contact support." }); return; }
      try { sessionStorage.removeItem("bidora_organization_request_unconfirmed"); } catch { /* Preserve safe recovery if storage is unavailable. */ }
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
      setUncertain(true);
      toast.error("Request could not be confirmed", { id:"organization-create",description: "Retry this same request without changing its details, or contact support." });
    } finally {
      lock.current=false;
      setSubmitting(false);
    }
  }

  if (statusLoading || statusError || recoveryRequired || existing) {
    return <div className="flex flex-col gap-4">
      {statusLoading ? <p role="status">Checking organization request status…</p> : null}
      {statusError ? <Alert variant="destructive" role="alert"><AlertTitle>Status unavailable</AlertTitle><AlertDescription>{statusError}</AlertDescription></Alert> : null}
      {recoveryRequired && !statusError ? <Alert variant="info" role="status"><AlertTitle>Request outcome not confirmed</AlertTitle><AlertDescription>A previous request may still be processing. Check again before starting another request, or contact support.</AlertDescription></Alert> : null}
      {existing && !statusError ? <Alert variant="info" role="status"><AlertTitle>Organization request: {existing.name}</AlertTitle><AlertDescription>Recorded state: {existing.state.replaceAll("_", " ")}. Processing state: {existing.job_state.replaceAll("_", " ")}. This does not by itself confirm workspace access or payment.</AlertDescription></Alert> : null}
      {!statusLoading ? <div className="flex flex-wrap gap-3"><Button type="button" variant="outline" onClick={() => { setStatusLoading(true); void loadStatus(); }}>Check request status</Button>{existing ? <Button nativeButton={false} render={<Link href="/auth/resolve" />}>Check workspace access</Button> : null}</div> : null}
    </div>;
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
          <Button type="button" variant="outline" onClick={() => { setStatusLoading(true); void loadStatus(); }}>Check request status</Button>
          <Button nativeButton={false} render={<Link href="/auth/resolve" />}>Check workspace access</Button>
        </div>
      </div>
    );
  }

  return (
    <form className="flex flex-col gap-6" onSubmit={(event) => void submit(event)} noValidate>
      <ol aria-label="Setup progress" className="flex flex-wrap gap-3 text-sm">{["Organization", "Business profile", "Get started", "Review"].map((label,index)=><li key={label} aria-current={step===index?"step":undefined} className={step===index?"font-semibold text-foreground":"text-muted-foreground"}>{index+1}. {label}</li>)}</ol>
      {step>=2 && regional && !regional.organization_registration_enabled ? <Alert variant="info" role="status"><AlertTitle>Organization requests are unavailable</AlertTitle><AlertDescription>You can review your details, but submission is not available yet.</AlertDescription></Alert> : null}
      {uncertain ? <Alert variant="info"><AlertTitle>Request outcome unknown</AlertTitle><AlertDescription>Do not change your details or start a new request. Retry the same request or contact support.</AlertDescription></Alert> : null}
      <fieldset disabled={submitting||uncertain} className="flex min-w-0 flex-col gap-6">
      {(step===0||step===1) && <FieldSet>
        <FieldLegend>Organization details</FieldLegend>
        <FieldDescription>Your verified account email becomes the initial work contact.</FieldDescription>
        {regionalError||referenceError?<p role="alert" className="text-sm text-destructive">{regionalError||referenceError} <Button type="button" variant="outline" size="sm" onClick={()=>setReload(v=>v+1)}>Retry</Button></p>:!regional||!references?<p role="status" className="text-sm text-muted-foreground">Loading available organization options…</p>:!regional.organization_registration_enabled?<p role="status" className="text-sm text-muted-foreground">New organization requests are temporarily unavailable.</p>:null}
        <FieldGroup className="grid gap-5 sm:grid-cols-2">
         {step===0 && <Field data-invalid={Boolean(errors.country)}><FieldLabel htmlFor="organization-country">Country</FieldLabel><PlatformSelect id="organization-country" value={values.country} onChange={e=>selectCountry(e.target.value)} disabled={!regional||submitting||!!regionalError}><option value="">Choose an enabled country</option>{regional?.countries.map(c=><option key={c.country_code} value={c.country_code}>{c.name}</option>)}</PlatformSelect><FieldDescription>Regional defaults come from the current enabled country configuration.</FieldDescription><FieldError>{errors.country}</FieldError></Field>}
          {fields.filter(field=>step===0 ? field.name==="name" : field.name==="business_type"||field.name==="primary_industry").map((field) => (
            <Field key={field.name} data-invalid={Boolean(errors[field.name])} className={field.name === "name" ? "sm:col-span-2" : undefined}>
              <FieldLabel htmlFor={`organization-${field.name}`}>{field.label}</FieldLabel>
              {field.name === "business_type" || field.name === "primary_industry" ? <PlatformSelect id={`organization-${field.name}`} name={field.name} value={values[field.name]} disabled={submitting||!references} aria-invalid={Boolean(errors[field.name])} onChange={event => update(field.name, event.target.value)}><option value="">Choose {field.name === "business_type" ? "a business type" : "an industry"}</option>{references?.[field.name === "business_type" ? "business-types" : "industries"].map(row => <option key={row.code} value={row.code}>{row.label}</option>)}</PlatformSelect> : <Input
                id={`organization-${field.name}`}
                name={field.name}
                value={values[field.name]}
                placeholder={field.placeholder}
                maxLength={field.maxLength}
                autoComplete="off"
                readOnly={["timezone","locale","preferred_language"].includes(field.name)}
                disabled={submitting}
                aria-invalid={Boolean(errors[field.name])}
                onChange={(event) => update(field.name, event.target.value)}
              />}
              {field.description ? <FieldDescription>{field.description}</FieldDescription> : null}
              <FieldError>{errors[field.name]}</FieldError>
            </Field>
          ))}
          {step===0 && <Field>
            <FieldLabel htmlFor="organization-currency">Currency</FieldLabel>
            <Input id="organization-currency" value={regional?.billing_currency??""} readOnly aria-readonly="true" />
            <FieldDescription>Billing currency is set by the platform.</FieldDescription>
          </Field>}
        </FieldGroup>
      </FieldSet>}

      {step===1 && <FieldSet>
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
              <span className="text-sm font-normal text-muted-foreground">This permanently selects the Tenderpreneur account type and requests a private tender-preparation organization. It does not create a procurement organization.</span>
            </span>
          </FieldLabel>
          <FieldError>{errors.organization_type}</FieldError>
        </Field>
      </FieldSet>}

      {step===2 && <FieldSet>
        <FieldLegend>Commercial path</FieldLegend>
        <FieldDescription>Nothing is charged or activated by submitting this form.</FieldDescription>
        <div className="grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label="Commercial path">
          {([
            { value: "TRIAL" as const, title: "Trial", copy: "Provisioning must complete before the trial can begin. Eligibility and terms are confirmed by the platform.", icon: SparklesIcon },
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
      </FieldSet>}
      {step===3 && <section aria-labelledby="review-heading" className="space-y-3"><h2 id="review-heading" className="font-semibold">Review your request</h2><dl className="grid gap-3 text-sm sm:grid-cols-2"><div><dt className="text-muted-foreground">Organization</dt><dd>{values.name}</dd></div><div><dt className="text-muted-foreground">Country</dt><dd>{regional?.countries.find(row=>row.country_code===values.country)?.name}</dd></div><div><dt className="text-muted-foreground">Business type</dt><dd>{references?.["business-types"].find(row=>row.code===values.business_type)?.label}</dd></div><div><dt className="text-muted-foreground">Industry</dt><dd>{references?.industries.find(row=>row.code===values.primary_industry)?.label}</dd></div><div><dt className="text-muted-foreground">Commercial path</dt><dd>{values.commercial_path==="TRIAL"?"Trial":"Paid plan"}</dd></div><div><dt className="text-muted-foreground">Billing currency</dt><dd>{regional?.billing_currency}</dd></div></dl></section>}
      </fieldset>

      {errors.form ? (
        <Alert variant="destructive" role="alert">
          <AlertTitle>Request not submitted</AlertTitle>
          <AlertDescription>{errors.form}</AlertDescription>
        </Alert>
      ) : null}

      {step===3 && <Alert variant="info">
        <Building2Icon aria-hidden="true" />
        <AlertTitle>Pending means pending</AlertTitle>
        <AlertDescription>A successful submission records a request only. Bidora will not claim your organization is ready until provisioning, compatibility, membership, and commercial checks all complete.</AlertDescription>
      </Alert>}

      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
        {!uncertain && (step===0 ? <Button nativeButton={false} render={<Link href="/auth/resolve" />} variant="outline" className="max-sm:w-full">Cancel</Button> : <Button type="button" variant="outline" disabled={submitting} onClick={()=>setStep(step-1)} className="max-sm:w-full">Back</Button>)}
        <Button type="submit" disabled={submitting||!regional||!!regionalError||!references||!!referenceError||(step===3&&!regional.organization_registration_enabled)} className="max-sm:w-full">
          {submitting?<Loader2Icon aria-hidden="true" className="animate-spin"/>:null}{submitting ? "Submitting request…" : uncertain ? "Retry same request" : step===3 ? "Submit organization request" : "Continue"}
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
