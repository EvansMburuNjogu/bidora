import "server-only";
import { open } from "node:fs/promises";
import { X509Certificate } from "node:crypto";

// The Go contract still requires explicit trust roots. Use deployment-owned
// trust configuration, never browser input, network discovery or a TLS bypass.
export async function integrationTrustRoots(maxBytes = 24_576): Promise<string> {
  const path = process.env.NODE_EXTRA_CA_CERTS || process.env.SSL_CERT_FILE;
  if (!path) throw new Error("integration_trust_unavailable");
  const file = await open(path, "r");
  try {
    if (!(await file.stat()).isFile()) throw new Error("integration_trust_unavailable");
    const bytes = Buffer.alloc(maxBytes + 1);
    const { bytesRead } = await file.read(bytes, 0, bytes.length, 0);
    if (!bytesRead || bytesRead > maxBytes) throw new Error("integration_trust_unavailable");
    const pem = bytes.subarray(0, bytesRead).toString("utf8");
    const certificates = pem.match(/-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/g);
    if (!certificates?.length || certificates.some(value => !new X509Certificate(value).ca)) throw new Error("integration_trust_unavailable");
    return pem;
  } finally { await file.close(); }
}
