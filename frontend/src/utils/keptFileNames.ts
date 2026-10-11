const MAX_NAME_LENGTH = 80

// Control, format (including direction overrides) and line/paragraph separator characters.
const UNSAFE_CHARS = /[\p{Cc}\p{Cf}\p{Zl}\p{Zp}]/gu

/**
 * Makes an archive-derived file name safe to show as plain text: removes
 * control and direction-override characters and caps the length.
 */
export const sanitizeKeptFileName = (name: string): string => {
  const clean = name.replace(UNSAFE_CHARS, '')
  return clean.length > MAX_NAME_LENGTH ? `${clean.slice(0, MAX_NAME_LENGTH)}…` : clean
}
