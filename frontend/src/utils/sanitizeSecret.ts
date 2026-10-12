const TOKEN_CANDIDATE = /\b[A-Za-z0-9]{10,64}\b/g
// Letters-only strings are ordinary words unless they are implausibly long.
const MIN_LETTERS_ONLY_SECRET_LENGTH = 32

/**
 * Masks token-like substrings (enrollment keys, API tokens) in a message.
 * A candidate is masked when it mixes letters and digits, or is a very long
 * run of letters. Ordinary words such as "enrollment" are left intact.
 */
export function sanitizeSecret(message: string): string {
  return message.replace(TOKEN_CANDIDATE, (candidate) => {
    const hasDigit = /\d/.test(candidate)
    const hasLetter = /[A-Za-z]/.test(candidate)
    if (hasDigit && hasLetter) return '***'
    if (!hasDigit && candidate.length >= MIN_LETTERS_ONLY_SECRET_LENGTH) return '***'
    return candidate
  })
}
