import axios from 'axios';

/**
 * Pre-configured Axios instance for API communication.
 * Includes base URL, credentials, and timeout settings.
 */
const client = axios.create({
  baseURL: '/api/v1',
  withCredentials: true, // Required for HttpOnly cookie transmission
  timeout: 30000, // 30 second timeout
});

/**
 * Sets or clears the Authorization header for API requests.
 * @param token - JWT token to set, or null to clear authentication
 */
export const setAuthToken = (token: string | null) => {
  if (token) {
    client.defaults.headers.common.Authorization = `Bearer ${token}`;
  } else {
    delete client.defaults.headers.common.Authorization;
  }
};

/**
 * Callback function invoked when a 401 authentication error occurs.
 * Set via setAuthErrorHandler to allow AuthContext to handle session expiry.
 */
let onAuthError: (() => void) | null = null;

/**
 * Registers a callback to handle authentication errors (401 responses).
 * @param handler - Function to call when authentication fails
 */
export const setAuthErrorHandler = (handler: () => void) => {
  onAuthError = handler;
};

// Global 401 error handling - triggers auth error callback for session expiry
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      console.warn('Authentication failed:', error.config?.url);
      // Skip auth error handling for login/auth endpoints to avoid redirect loops
      const url = error.config?.url || '';
      const isAuthEndpoint = url.includes('/auth/login') || url.includes('/auth/me');
      if (onAuthError && !isAuthEndpoint) {
        onAuthError();
      }
    }
    return Promise.reject(error);
  }
);

export default client;
