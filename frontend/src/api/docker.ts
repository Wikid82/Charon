import client from './client'

export interface DockerPort {
  private_port: number
  public_port: number
  type: string
}

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

export const dockerApi = {
  listContainers: async (host?: string, serverId?: string): Promise<DockerContainer[]> => {
    const params: Record<string, string> = {}
    if (host) params.host = host
    if (serverId) params.server_id = serverId

    const response = await client.get<DockerContainer[]>('/docker/containers', { params })
    return response.data
  },
}
