import client from './client'

/** Docker port mapping information. */
export interface DockerPort {
  private_port: number
  public_port: number
  type: string
}

/** Docker container information. */
export interface DockerContainer {
  id: string
  names: string[]
  image: string
  state: string
  status: string
  network: string
  ip: string
  ports: DockerPort[]
}

/** Docker API client for container operations. */
export const dockerApi = {
  /**
   * Lists Docker containers from a local or remote host.
   * @param host - Optional Docker host address
   * @param serverId - Optional remote server ID
   * @returns Promise resolving to array of DockerContainer objects
   * @throws {AxiosError} If listing fails or host unreachable
   */
  listContainers: async (host?: string, serverId?: string): Promise<DockerContainer[]> => {
    const params: Record<string, string> = {}
    if (host) params.host = host
    if (serverId) params.server_id = serverId

    const response = await client.get<DockerContainer[]>('/docker/containers', { params })
    return response.data
  },
}
