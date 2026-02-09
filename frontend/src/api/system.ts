import client from './client';

/** Update availability information. */
export interface UpdateInfo {
  available: boolean;
  latest_version: string;
  changelog_url: string;
}

/** System notification entry. */
export interface Notification {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error';
  title: string;
  message: string;
  read: boolean;
  created_at: string;
}

/**
 * Checks for available application updates.
 * @returns Promise resolving to UpdateInfo
 * @throws {AxiosError} If the request fails
 */
export const checkUpdates = async (): Promise<UpdateInfo> => {
  const response = await client.get('/system/updates');
  return response.data;
};

/**
 * Fetches system notifications.
 * @param unreadOnly - If true, only returns unread notifications
 * @returns Promise resolving to array of Notification objects
 * @throws {AxiosError} If the request fails
 */
export const getNotifications = async (unreadOnly = false): Promise<Notification[]> => {
  const response = await client.get('/notifications', { params: { unread: unreadOnly } });
  return response.data;
};

/**
 * Marks a notification as read.
 * @param id - The notification ID to mark as read
 * @throws {AxiosError} If marking fails or notification not found
 */
export const markNotificationRead = async (id: string): Promise<void> => {
  await client.post(`/notifications/${id}/read`);
};

/**
 * Marks all notifications as read.
 * @throws {AxiosError} If the request fails
 */
export const markAllNotificationsRead = async (): Promise<void> => {
  await client.post('/notifications/read-all');
};

/** Response containing the client's public IP address. */
export interface MyIPResponse {
  ip: string;
  source: string;
}

/**
 * Gets the client's public IP address as seen by the server.
 * @returns Promise resolving to MyIPResponse with IP address
 * @throws {AxiosError} If the request fails
 */
export const getMyIP = async (): Promise<MyIPResponse> => {
  const response = await client.get<MyIPResponse>('/system/my-ip');
  return response.data;
};
