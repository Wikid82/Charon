/**
 * Parses a JSON-encoded array prop (e.g. a CSP or Permissions-Policy list stored as a string).
 * Returns an empty array for an empty value or malformed JSON.
 */
export function parseJsonArray<T>(value: string): T[] {
  if (!value) return [];
  try {
    return JSON.parse(value) as T[];
  } catch {
    return [];
  }
}
