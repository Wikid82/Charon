import { isAxiosError } from 'axios'

/**
 * Reads the machine-readable `error_code` from a failed request.
 * @param err - The value thrown by the request
 * @param codes - The codes the caller knows how to handle
 * @returns The code, or undefined when it is missing or not one of `codes`
 */
export function getApiErrorCode<C extends string>(err: unknown, codes: readonly C[]): C | undefined {
  if (!isAxiosError<{ error_code?: string }>(err)) return undefined
  const code = err.response?.data?.error_code
  return codes.find((known) => known === code)
}

/** True when the server refused a CrowdSec change because a background preset install is running. */
export function isCrowdsecBusyError(err: unknown): boolean {
  return getApiErrorCode(err, ['crowdsec_busy'] as const) !== undefined
}
