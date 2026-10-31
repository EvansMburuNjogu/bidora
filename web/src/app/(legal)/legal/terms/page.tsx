import type { Metadata } from "next";
import { LegalDocument } from "@/components/legal/legal-document";

export const metadata: Metadata = {
  title: "Terms and Conditions — Draft",
  robots: { index: false, follow: false },
};

export default function TermsPage() {
  return (
    <LegalDocument title="Terms and Conditions" summary="Proposed terms for using Bidora’s tender-management and bid-preparation services.">
      <section><h2>1. Provider and effective date</h2><p>The contracting Bidora legal entity, registered address, contact details, governing jurisdiction, effective date, and prior-policy transition rules must be inserted and approved before publication.</p></section>
      <section><h2>2. The service</h2><p>Bidora provides tools for tender discovery, preparation, collaboration, document management, evidence organization, and related account administration. Features may differ by account, organization, subscription, deployment, or release stage.</p><p>Unless an expressly identified integration states otherwise, Bidora does not submit a bid to an external portal, guarantee that a submission was received, or make procurement, evaluation, award, legal, financial, or professional decisions for a user.</p></section>
      <section><h2>3. Accounts and authority</h2><ul><li>You must provide accurate account information and protect your authentication methods.</li><li>You must be legally able to accept these terms and, when acting for an organization, authorized to bind or administer that organization.</li><li>Account type, organization name, email domain, or possession of a link does not by itself grant access. Bidora may require verified membership and current permissions.</li><li>You must promptly report suspected unauthorized access and keep recovery information current.</li></ul></section>
      <section><h2>4. Acceptable use</h2><p>You must not misuse the service, evade access controls or rate limits, access another organization’s information without authorization, upload unlawful or malicious material, probe or disrupt the service, misrepresent procurement evidence, or use Bidora to violate applicable law or third-party rights.</p></section>
      <section><h2>5. Tender content and human decisions</h2><p>You remain responsible for tender facts, deadlines, eligibility, pricing, approvals, evidence, and submissions. Automated extraction and AI-assisted suggestions are advisory and may be incomplete or incorrect. Authorized people must verify source citations and approve decisions before relying on them.</p></section>
      <section><h2>6. Your content</h2><p>You retain the rights you hold in information and documents you submit. You grant the provider only the rights necessary to host, secure, process, reproduce, transform, and transmit that content to operate the contracted service. The final license wording, confidentiality obligations, export rights, deletion process, and post-termination handling require legal and retention-policy approval.</p></section>
      <section><h2>7. Third-party services</h2><p>Bidora may use or connect to infrastructure, email, identity, storage, security-scanning, payment, AI, and external procurement services. Their availability and separate terms may affect a feature. Approved providers, subprocessors, regional processing, and responsibility boundaries must be disclosed before publication.</p></section>
      <section><h2>8. Fees and subscriptions</h2><p>Any paid plan, taxes, renewal, cancellation, Trial, refund, usage limit, or payment-provider term must be presented in an applicable order or subscription agreement. No commercial term should be inferred from this draft.</p></section>
      <section><h2>9. Suspension and termination</h2><p>The final policy must define when accounts may be restricted or terminated, notice and appeal where applicable, treatment of active procurement work, export windows, legal holds, outstanding charges, and survival of relevant obligations.</p></section>
      <section><h2>10. Service commitments and liability</h2><p>Availability commitments, support levels, warranties, disclaimers, indemnities, liability exclusions and caps must be drafted for the approved jurisdiction and commercial model. This draft intentionally creates none of those terms.</p></section>
      <section><h2>11. Changes and contact</h2><p>The final terms must explain material-change notice, effective dates, re-acceptance where required, archived versions, complaints, notices, and the provider’s legal contact channels.</p></section>
      <section><h2>Items required before approval</h2><ul><li>Legal entity, address, registration details, contact channels and governing law.</li><li>Commercial, suspension, termination, export and retention rules.</li><li>Warranty, liability, indemnity and dispute-resolution language reviewed by qualified counsel.</li><li>Approved provider/subprocessor and data-location disclosures.</li><li>Effective date and immutable publication identifier.</li></ul></section>
    </LegalDocument>
  );
}
