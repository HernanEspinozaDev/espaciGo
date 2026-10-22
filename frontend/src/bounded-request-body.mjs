export async function readBoundedRequestBody(request, maxBytes) {
  const chunks = [];
  let bytes = 0;
  for await (const chunk of request) {
    const value = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    bytes += value.byteLength;
    if (bytes > maxBytes) return null;
    chunks.push(value);
  }
  return Buffer.concat(chunks, bytes);
}
