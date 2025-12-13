import axios from 'axios';

const client = axios.create({
  baseURL: '/api/v1',
  withCredentials: true, // Required for HttpOnly cookie transmission
  timeout: 30000, // 30 second timeout
});

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
