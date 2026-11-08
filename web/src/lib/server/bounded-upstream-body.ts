import "server-only";

// Bound memory even when an upstream omits Content-Length or streams chunked JSON.
export async function boundedUpstreamBody(response: Pick<Response, "headers" | "body">, limit = 65_536): Promise<ArrayBuffer | null> {
  const declared = response.headers.get("content-length");
  if (declared !== null && Number.isFinite(Number(declared)) && Number(declared) > limit) {
    await response.body?.cancel();
    return null;
  }
  if (!response.body) return new ArrayBuffer(0);
  const reader = response.body.getReader();
  const output = new Uint8Array(limit);
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return output.buffer.slice(0, size);
      if (value.byteLength > limit - size) {
        await reader.cancel();
        return null;
      }
      output.set(value, size);
      size += value.byteLength;
    }
  } catch {
    await reader.cancel().catch(() => undefined);
    return null;
  } finally {
    reader.releaseLock();
  }
}
