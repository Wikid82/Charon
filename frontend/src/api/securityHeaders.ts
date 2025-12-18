import client from './client';

// Types
export interface SecurityHeaderProfile {
  id: number;
  uuid: string;
  name: string;
  hsts_enabled: boolean;
  hsts_max_age: number;
  hsts_include_subdomains: boolean;
  hsts_preload: boolean;
  csp_enabled: boolean;
  csp_directives: string;
  csp_report_only: boolean;
  csp_report_uri: string;
  x_frame_options: string;
  x_content_type_options: boolean;
  referrer_policy: string;
  permissions_policy: string;
  cross_origin_opener_policy: string;
  cross_origin_resource_policy: string;
  cross_origin_embedder_policy: string;
  xss_protection: boolean;
  cache_control_no_store: boolean;
  security_score: number;
  is_preset: boolean;
  preset_type: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface SecurityHeaderPreset {
  type: 'basic' | 'strict' | 'paranoid';
  name: string;
  description: string;
  score: number;
  config: Partial<SecurityHeaderProfile>;
}

export interface ScoreBreakdown {
  score: number;
  max_score: number;
  breakdown: Record<string, number>;
  suggestions: string[];
}

export interface CSPDirective {
  directive: string;
  values: string[];
}

export interface CreateProfileRequest {
  name: string;
  description?: string;
  hsts_enabled?: boolean;
  hsts_max_age?: number;
  hsts_include_subdomains?: boolean;
  hsts_preload?: boolean;
  csp_enabled?: boolean;
  csp_directives?: string;
  csp_report_only?: boolean;
  csp_report_uri?: string;
  x_frame_options?: string;
  x_content_type_options?: boolean;
  referrer_policy?: string;
  permissions_policy?: string;
  cross_origin_opener_policy?: string;
  cross_origin_resource_policy?: string;
  cross_origin_embedder_policy?: string;
  xss_protection?: boolean;
  cache_control_no_store?: boolean;
}

export interface ApplyPresetRequest {
  preset_type: string;
  name: string;
}

// API Functions
export const securityHeadersApi = {
  /**
   * List all security header profiles
   */
  async listProfiles(): Promise<SecurityHeaderProfile[]> {
    const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
    return response.data;
  },

  /**
   * Get a single profile by ID or UUID
   */
  async getProfile(id: number | string): Promise<SecurityHeaderProfile> {
    const response = await client.get<SecurityHeaderProfile>(`/security/headers/profiles/${id}`);
    return response.data;
  },

  /**
   * Create a new security header profile
   */
  async createProfile(data: CreateProfileRequest): Promise<SecurityHeaderProfile> {
    const response = await client.post<SecurityHeaderProfile>('/security/headers/profiles', data);
    return response.data;
  },

  /**
   * Update an existing profile
   */
  async updateProfile(id: number, data: Partial<CreateProfileRequest>): Promise<SecurityHeaderProfile> {
    const response = await client.put<SecurityHeaderProfile>(`/security/headers/profiles/${id}`, data);
    return response.data;
  },

  /**
   * Delete a profile (not presets)
   */
  async deleteProfile(id: number): Promise<void> {
    await client.delete(`/security/headers/profiles/${id}`);
  },

  /**
   * Get built-in presets
   */
  async getPresets(): Promise<SecurityHeaderPreset[]> {
    const response = await client.get<SecurityHeaderPreset[]>('/security/headers/presets');
    return response.data;
  },

  /**
   * Apply a preset to create/update a profile
   */
  async applyPreset(data: ApplyPresetRequest): Promise<SecurityHeaderProfile> {
    const response = await client.post<SecurityHeaderProfile>('/security/headers/presets/apply', data);
    return response.data;
  },

  /**
   * Calculate security score for given settings
   */
  async calculateScore(config: Partial<CreateProfileRequest>): Promise<ScoreBreakdown> {
    const response = await client.post<ScoreBreakdown>('/security/headers/score', config);
    return response.data;
  },

  /**
   * Validate a CSP string
   */
  async validateCSP(csp: string): Promise<{ valid: boolean; errors: string[] }> {
    const response = await client.post<{ valid: boolean; errors: string[] }>('/security/headers/csp/validate', { csp });
    return response.data;
  },

  /**
   * Build a CSP string from directives
   */
  async buildCSP(directives: CSPDirective[]): Promise<{ csp: string }> {
    const response = await client.post<{ csp: string }>('/security/headers/csp/build', { directives });
    return response.data;
  },
};
