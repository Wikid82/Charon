import client from './client'

/** CrowdSec Console enrollment status. */
export interface ConsoleEnrollmentStatus {
  status: string
  tenant?: string
  agent_name?: string
  last_error?: string
  last_attempt_at?: string
  enrolled_at?: string
  last_heartbeat_at?: string
  key_present: boolean
  correlation_id?: string
}

/** Payload for enrolling with CrowdSec Console. */
export interface ConsoleEnrollPayload {
  enrollment_key: string
  tenant?: string
  agent_name: string
  force?: boolean
}

/**
 * Gets the current CrowdSec Console enrollment status.
 * @returns Promise resolving to ConsoleEnrollmentStatus
 * @throws {AxiosError} If status check fails
 */
export async function getConsoleStatus(): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.get<ConsoleEnrollmentStatus>('/admin/crowdsec/console/status')
  return resp.data
}

/**
 * Enrolls the instance with CrowdSec Console.
 * @param payload - Enrollment configuration including key and agent name
 * @returns Promise resolving to the new enrollment status
 * @throws {AxiosError} If enrollment fails
 */
export async function enrollConsole(payload: ConsoleEnrollPayload): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.post<ConsoleEnrollmentStatus>('/admin/crowdsec/console/enroll', payload)
  return resp.data
}

/**
 * Clears the current CrowdSec Console enrollment.
 * @throws {AxiosError} If clearing enrollment fails
 */
export async function clearConsoleEnrollment(): Promise<void> {
  await client.delete('/admin/crowdsec/console/enrollment')
}

export default {
  getConsoleStatus,
  enrollConsole,
  clearConsoleEnrollment,
}
