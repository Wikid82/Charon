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
  preset_type: 'basic' | 'strict' | 'paranoid';
  name: string;
  description: string;
  security_score: number;
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
   * Lists all security header profiles.
   * @returns Promise resolving to array of SecurityHeaderProfile objects
   * @throws {AxiosError} If the request fails
   */
  async listProfiles(): Promise<SecurityHeaderProfile[]> {
    const response = await client.get<{profiles: SecurityHeaderProfile[]}>('/security/headers/profiles');
    return response.data.profiles;
  },

  /**
   * Gets a single security header profile by ID or UUID.
   * @param id - The profile ID (number) or UUID (string)
   * @returns Promise resolving to the SecurityHeaderProfile object
   * @throws {AxiosError} If the request fails or profile not found
   */
  async getProfile(id: number | string): Promise<SecurityHeaderProfile> {
    const response = await client.get<{profile: SecurityHeaderProfile}>(`/security/headers/profiles/${id}`);
    return response.data.profile;
  },

  /**
   * Creates a new security header profile.
   * @param data - CreateProfileRequest with profile configuration
   * @returns Promise resolving to the created SecurityHeaderProfile
   * @throws {AxiosError} If creation fails or validation errors occur
   */
  async createProfile(data: CreateProfileRequest): Promise<SecurityHeaderProfile> {
    const response = await client.post<{profile: SecurityHeaderProfile}>('/security/headers/profiles', data);
    return response.data.profile;
  },

  /**
   * Updates an existing security header profile.
   * @param id - The profile ID to update
   * @param data - Partial CreateProfileRequest with fields to update
   * @returns Promise resolving to the updated SecurityHeaderProfile
   * @throws {AxiosError} If update fails or profile not found
   */
  async updateProfile(id: number, data: Partial<CreateProfileRequest>): Promise<SecurityHeaderProfile> {
    const response = await client.put<{profile: SecurityHeaderProfile}>(`/security/headers/profiles/${id}`, data);
    return response.data.profile;
  },

  /**
   * Deletes a security header profile.
   * @param id - The profile ID to delete (cannot delete preset profiles)
   * @throws {AxiosError} If deletion fails, profile not found, or is a preset
   */
  async deleteProfile(id: number): Promise<void> {
    await client.delete(`/security/headers/profiles/${id}`);
  },

  /**
   * Gets all built-in security header presets.
   * @returns Promise resolving to array of SecurityHeaderPreset objects
   * @throws {AxiosError} If the request fails
   */
  async getPresets(): Promise<SecurityHeaderPreset[]> {
    const response = await client.get<{presets: SecurityHeaderPreset[]}>('/security/headers/presets');
    return response.data.presets;
  },

  /**
   * Applies a preset to create or update a security header profile.
   * @param data - ApplyPresetRequest with preset type and profile name
   * @returns Promise resolving to the created/updated SecurityHeaderProfile
   * @throws {AxiosError} If preset application fails
   */
  async applyPreset(data: ApplyPresetRequest): Promise<SecurityHeaderProfile> {
    const response = await client.post<{profile: SecurityHeaderProfile}>('/security/headers/presets/apply', data);
    return response.data.profile;
  },

  /**
   * Calculates the security score for given header settings.
   * @param config - Partial CreateProfileRequest with settings to evaluate
   * @returns Promise resolving to ScoreBreakdown with score, max, breakdown, and suggestions
   * @throws {AxiosError} If calculation fails
   */
  async calculateScore(config: Partial<CreateProfileRequest>): Promise<ScoreBreakdown> {
    const response = await client.post<ScoreBreakdown>('/security/headers/score', config);
    return response.data;
  },

  /**
   * Validates a Content Security Policy string.
   * @param csp - The CSP string to validate
   * @returns Promise resolving to object with validity status and any errors
   * @throws {AxiosError} If validation request fails
   */
  async validateCSP(csp: string): Promise<{ valid: boolean; errors: string[] }> {
    const response = await client.post<{ valid: boolean; errors: string[] }>('/security/headers/csp/validate', { csp });
    return response.data;
  },

  /**
   * Builds a Content Security Policy string from directives.
   * @param directives - Array of CSPDirective objects to combine
   * @returns Promise resolving to object containing the built CSP string
   * @throws {AxiosError} If build request fails
   */
  async buildCSP(directives: CSPDirective[]): Promise<{ csp: string }> {
    const response = await client.post<{ csp: string }>('/security/headers/csp/build', { directives });
    return response.data;
  },
};
