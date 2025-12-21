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

// Global 401 error logging for debugging
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      console.warn('Authentication failed:', error.config?.url);
    }
    return Promise.reject(error);
  }
);

export default client;
