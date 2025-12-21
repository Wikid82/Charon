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
   * Fetches all access lists.
   * @returns Promise resolving to array of AccessList objects
   * @throws {AxiosError} If the request fails
   */
  async list(): Promise<AccessList[]> {
    const response = await client.get<AccessList[]>('/access-lists');
    return response.data;
  },

  /**
   * Gets a single access list by ID.
   * @param id - The access list ID
   * @returns Promise resolving to the AccessList object
   * @throws {AxiosError} If the request fails or access list not found
   */
  async get(id: number): Promise<AccessList> {
    const response = await client.get<AccessList>(`/access-lists/${id}`);
    return response.data;
  },

  /**
   * Creates a new access list.
   * @param data - CreateAccessListRequest with access list configuration
   * @returns Promise resolving to the created AccessList
   * @throws {AxiosError} If creation fails or validation errors occur
   */
  async create(data: CreateAccessListRequest): Promise<AccessList> {
    const response = await client.post<AccessList>('/access-lists', data);
    return response.data;
  },

  /**
   * Updates an existing access list.
   * @param id - The access list ID to update
   * @param data - Partial CreateAccessListRequest with fields to update
   * @returns Promise resolving to the updated AccessList
   * @throws {AxiosError} If update fails or access list not found
   */
  async update(id: number, data: Partial<CreateAccessListRequest>): Promise<AccessList> {
    const response = await client.put<AccessList>(`/access-lists/${id}`, data);
    return response.data;
  },

  /**
   * Deletes an access list.
   * @param id - The access list ID to delete
   * @throws {AxiosError} If deletion fails or access list not found
   */
  async delete(id: number): Promise<void> {
    await client.delete(`/access-lists/${id}`);
  },

  /**
   * Tests if an IP address would be allowed or blocked by an access list.
   * @param id - The access list ID to test against
   * @param ipAddress - The IP address to test
   * @returns Promise resolving to TestIPResponse with allowed status and reason
   * @throws {AxiosError} If test fails or access list not found
   */
  async testIP(id: number, ipAddress: string): Promise<TestIPResponse> {
    const response = await client.post<TestIPResponse>(`/access-lists/${id}/test`, {
      ip_address: ipAddress,
    });
    return response.data;
  },

  /**
   * Gets predefined access list templates.
   * @returns Promise resolving to array of AccessListTemplate objects
   * @throws {AxiosError} If the request fails
   */
  async getTemplates(): Promise<AccessListTemplate[]> {
    const response = await client.get<AccessListTemplate[]>('/access-lists/templates');
    return response.data;
  },
};
