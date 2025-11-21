import client from './client';

export interface UpdateInfo {
  available: boolean;
  latest_version: string;
  changelog_url: string;
}

export interface Notification {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error';
  title: string;
  message: string;
  read: boolean;
  created_at: string;
}

export const checkUpdates = async (): Promise<UpdateInfo> => {
  const response = await client.get('/system/updates');
  return response.data;
};

export const getNotifications = async (unreadOnly = false): Promise<Notification[]> => {
  const response = await client.get('/notifications', { params: { unread: unreadOnly } });
  return response.data;
};

export const markNotificationRead = async (id: string): Promise<void> => {
  await client.post(`/notifications/${id}/read`);
};

export const markAllNotificationsRead = async (): Promise<void> => {
  await client.post('/notifications/read-all');
};
