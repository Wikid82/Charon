import client from './client'

/** Status of a manual DNS challenge */
export type ChallengeStatus = 'created' | 'pending' | 'verifying' | 'verified' | 'expired' | 'failed'

/** Manual DNS challenge response from API */
export interface ManualChallenge {
  id: string
  status: ChallengeStatus
  fqdn: string
  value: string
  ttl: number
  created_at: string
  expires_at: string
  last_check_at?: string
  dns_propagated: boolean
  error_message?: string
}

/** Polling response for challenge status */
export interface ChallengePollResponse {
  status: ChallengeStatus
  dns_propagated: boolean
  time_remaining_seconds: number
  last_check_at: string
  error_message?: string
}

/** Challenge verification result */
export interface ChallengeVerifyResponse {
  success: boolean
  dns_found: boolean
  message: string
}

/** Request to create a new manual challenge */
export interface CreateChallengeRequest {
  domain: string
}

/**
 * Fetches a manual challenge by ID.
 * @param providerId - The DNS provider ID
 * @param challengeId - The challenge UUID
 * @returns Promise resolving to the challenge details
 * @throws {AxiosError} If not found or request fails
 */
export async function getChallenge(providerId: number, challengeId: string): Promise<ManualChallenge> {
  const response = await client.get<ManualChallenge>(
    `/dns-providers/${providerId}/manual-challenge/${challengeId}`
  )
  return response.data
}

/**
 * Creates a new manual DNS challenge.
 * @param providerId - The DNS provider ID
 * @param data - Challenge creation data
 * @returns Promise resolving to the created challenge
 * @throws {AxiosError} If validation fails or request fails
 */
export async function createChallenge(
  providerId: number,
  data: CreateChallengeRequest
): Promise<ManualChallenge> {
  const response = await client.post<ManualChallenge>(
    `/dns-providers/${providerId}/manual-challenge`,
    data
  )
  return response.data
}

/**
 * Triggers verification of a manual challenge.
 * @param providerId - The DNS provider ID
 * @param challengeId - The challenge UUID
 * @returns Promise resolving to verification result
 * @throws {AxiosError} If not found or request fails
 */
export async function verifyChallenge(
  providerId: number,
  challengeId: string
): Promise<ChallengeVerifyResponse> {
  const response = await client.post<ChallengeVerifyResponse>(
    `/dns-providers/${providerId}/manual-challenge/${challengeId}/verify`
  )
  return response.data
}

/**
 * Polls for challenge status updates.
 * @param providerId - The DNS provider ID
 * @param challengeId - The challenge UUID
 * @returns Promise resolving to poll response
 * @throws {AxiosError} If not found or request fails
 */
export async function pollChallenge(
  providerId: number,
  challengeId: string
): Promise<ChallengePollResponse> {
  const response = await client.get<ChallengePollResponse>(
    `/dns-providers/${providerId}/manual-challenge/${challengeId}/poll`
  )
  return response.data
}

/**
 * Deletes/cancels a manual challenge.
 * @param providerId - The DNS provider ID
 * @param challengeId - The challenge UUID
 * @throws {AxiosError} If not found or request fails
 */
export async function deleteChallenge(providerId: number, challengeId: string): Promise<void> {
  await client.delete(`/dns-providers/${providerId}/manual-challenge/${challengeId}`)
}
