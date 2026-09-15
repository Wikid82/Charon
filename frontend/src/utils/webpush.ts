/**
 * Converts a base64url-encoded string (as returned by the Web Push
 * VAPID public key API) into a Uint8Array.
 *
 * The browser `PushManager.subscribe({ applicationServerKey })` API
 * requires the raw key bytes, not the base64url string Charon stores and
 * serves — this is standard boilerplate for every Web Push frontend
 * integration (not Charon-specific).
 *
 * @param base64String - A base64url-encoded string (RFC 4648 §5), optionally
 *   missing its trailing `=` padding.
 * @returns The decoded bytes as a Uint8Array.
 */
export const urlBase64ToUint8Array = (base64String: string): Uint8Array => {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');

  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i++) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
};
