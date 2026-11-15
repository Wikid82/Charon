/**
 * Close code the server sends when the signed-in user already holds the maximum number of live streams.
 * The server accepts the upgrade first and then closes with this code (RFC 6455 1013, "Try Again Later"), because a
 * browser cannot read an HTTP status from a failed WebSocket handshake.
 */
export const STREAM_LIMIT_CLOSE_CODE = 1013

/** Close reason the server sends with {@link STREAM_LIMIT_CLOSE_CODE}. */
export const STREAM_LIMIT_CLOSE_REASON = 'stream_limit_reached'

/**
 * True when a live stream closed because of the per-user cap. Such a close is a refusal, not a dropped connection,
 * so the caller must show the limit message and must not reconnect.
 */
export function isStreamLimitClose(event: Pick<CloseEvent, 'code' | 'reason'>): boolean {
  return event.code === STREAM_LIMIT_CLOSE_CODE || event.reason === STREAM_LIMIT_CLOSE_REASON
}
