import client from './client';

/** Update availability information, mirroring the backend `UpdateInfo` JSON. */
export interface UpdateInfo {
  /** Running Charon version. */
  current_version: string;
  /** Whether `latest_version` is newer than `current_version`. */
  available: boolean;
  latest_version: string;
  /** Validated release notes URL; empty string when none is available. */
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

/** Severity of an active alert, using the same values as a notification. */
export type AlertSeverity = Notification['type'];

/**
 * A condition the server currently reports to the notification bell, mirroring one entry of
 * `GET /notifications/alerts`. Alerts are computed per request and never stored.
 */
export interface ActiveAlert {
  /** Stable identifier; the frontend maps it to text and links. Unknown keys are not rendered. */
  key: string;
  severity: AlertSeverity;
  /** Up to five affected names, sorted by the server. */
  affected_lists?: string[];
  /** Full number of affected names, which can exceed `affected_lists.length`. */
  affected_lists_total?: number;
  /** Whether a usable database is loaded now; selects the message variant only. */
  database_available?: boolean;
}

/** Wire shape of `GET /notifications/alerts`. */
interface ActiveAlertsResponse {
  alerts: ActiveAlert[];
}

/**
 * Fetches the alerts that currently need attention.
 * @returns Promise resolving to ActiveAlert objects (empty when nothing needs attention)
 * @throws {AxiosError} If the request fails
 */
export const getActiveAlerts = async (): Promise<ActiveAlert[]> => {
  const response = await client.get<ActiveAlertsResponse>('/notifications/alerts');
  return response.data.alerts;
};

/**
 * Checks for available application updates.
 * @returns Promise resolving to UpdateInfo
 * @throws {AxiosError} If the request fails
 */
export const checkUpdates = async (): Promise<UpdateInfo> => {
  const response = await client.get<UpdateInfo>('/system/updates');
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
