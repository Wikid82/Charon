/**
 * Splits the trusted-proxy field into one entry per non-blank line. The server validates and canonicalises the entries.
 * @param text - The raw field value
 * @returns The trimmed, non-empty lines
 */
export function parseTrustedProxyLines(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== '')
}
