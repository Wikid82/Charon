import client from './client'

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

export interface ConsoleEnrollPayload {
  enrollment_key: string
  tenant?: string
  agent_name: string
  force?: boolean
}

export async function getConsoleStatus(): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.get<ConsoleEnrollmentStatus>('/admin/crowdsec/console/status')
  return resp.data
}

export async function enrollConsole(payload: ConsoleEnrollPayload): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.post<ConsoleEnrollmentStatus>('/admin/crowdsec/console/enroll', payload)
  return resp.data
}

export default {
  getConsoleStatus,
  enrollConsole,
}
