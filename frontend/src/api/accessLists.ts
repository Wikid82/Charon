import client from './client';

export interface AccessListRule {
  cidr: string;
  description: string;
}

export interface AccessList {
  id: number;
  uuid: string;
  name: string;
  description: string;
  type: 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist';
  ip_rules: string; // JSON string of AccessListRule[]
  country_codes: string; // Comma-separated
  local_network_only: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAccessListRequest {
  name: string;
  description?: string;
  type: 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist';
  ip_rules?: string;
  country_codes?: string;
  local_network_only?: boolean;
  enabled?: boolean;
}

export interface TestIPRequest {
  ip_address: string;
}

export interface TestIPResponse {
  allowed: boolean;
  reason: string;
}

export interface AccessListTemplate {
  name: string;
  description: string;
  type: string;
  local_network_only?: boolean;
  country_codes?: string;
}

export const accessListsApi = {
  /**
   * Fetch all access lists
   */
  async list(): Promise<AccessList[]> {
    const response = await client.get<AccessList[]>('/access-lists');
    return response.data;
  },

  /**
   * Get a single access list by ID
   */
  async get(id: number): Promise<AccessList> {
    const response = await client.get<AccessList>(`/access-lists/${id}`);
    return response.data;
  },

  /**
   * Create a new access list
   */
  async create(data: CreateAccessListRequest): Promise<AccessList> {
    const response = await client.post<AccessList>('/access-lists', data);
    return response.data;
  },

  /**
   * Update an existing access list
   */
  async update(id: number, data: Partial<CreateAccessListRequest>): Promise<AccessList> {
    const response = await client.put<AccessList>(`/access-lists/${id}`, data);
    return response.data;
  },

  /**
   * Delete an access list
   */
  async delete(id: number): Promise<void> {
    await client.delete(`/access-lists/${id}`);
  },

  /**
   * Test if an IP address would be allowed/blocked
   */
  async testIP(id: number, ipAddress: string): Promise<TestIPResponse> {
    const response = await client.post<TestIPResponse>(`/access-lists/${id}/test`, {
      ip_address: ipAddress,
    });
    return response.data;
  },

  /**
   * Get predefined ACL templates
   */
  async getTemplates(): Promise<AccessListTemplate[]> {
    const response = await client.get<AccessListTemplate[]>('/access-lists/templates');
    return response.data;
  },
};
