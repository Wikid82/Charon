/**
 * Shared test constants
 *
 * This file contains constants used across test files.
 * Extracted to avoid circular imports with test setup files.
 */

import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Path to the authentication storage state file.
 * Created by auth.setup.ts during the setup project phase.
 * Used by browser contexts and API request contexts to inherit authentication.
 */
export const STORAGE_STATE = join(__dirname, '../playwright/.auth/user.json');

/**
 * Password of the primary E2E admin account created by auth.setup.ts.
 * Single source of truth: auth.setup.ts imports it, and specs that must
 * re-enter the account password (e.g. certificate private key export) reuse it.
 */
export const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';
