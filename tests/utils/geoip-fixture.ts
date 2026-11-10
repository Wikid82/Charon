/**
 * Upload fixtures for the GeoIP database specs.
 *
 * - `countryDatabase`: the GeoLite2-Country database the image bundles, with its build epoch set to the current time.
 *   It is a real, valid Country database, so the server accepts it as a replacement.
 * - `textFile`: plain text under an `.mmdb` name, for the "not a GeoIP database" refusal. The name is deliberate: the
 *   server must judge the content, not the extension.
 *
 * Deviations from a generated fixture:
 * - The Country database is copied from the container, not generated. Writing an MMDB search tree with records in
 *   TypeScript would be more code than these checks need. Only the 4-byte `build_epoch` value is rewritten, in place
 *   and at the same width, so the file stays a valid MMDB (the epoch is metadata; Verify() does not check it).
 * - The copy is read with `docker exec ... base64` rather than `docker cp`: `docker cp` fails on the rootless Docker
 *   daemon used for local runs. base64 keeps the bytes exact.
 *
 * The bundled copy must be present when the fixtures are loaded. Load them before a spec hides the database.
 */

import { BUNDLED_DATABASE_PATH } from './geoip-container';
import { execInContainer } from './container-exec';

/** Shape Playwright accepts for `setInputFiles` and `FileChooser.setFiles`, and for multipart API uploads. */
export interface UploadFile {
  name: string;
  mimeType: string;
  buffer: Buffer;
}

export interface GeoIPUploadFixtures {
  countryDatabase: UploadFile;
  textFile: UploadFile;
}

/** Metadata section start marker (0xAB 0xCD 0xEF followed by "MaxMind.com"), per the MaxMind DB format. */
const METADATA_START_MARKER = Buffer.concat([Buffer.from([0xab, 0xcd, 0xef]), Buffer.from('MaxMind.com')]);
const BUILD_EPOCH_KEY = Buffer.from('build_epoch');
/** MMDB control byte type 0 means the type is in the next byte, offset by 7. */
const EXTENDED_TYPE_MARKER = 0;
const EXTENDED_TYPE_OFFSET = 7;
/** MMDB unsigned integer types: uint16, uint32, uint64. */
const UINT_TYPES: ReadonlySet<number> = new Set([5, 6, 9]);
const MAX_UINT_BYTES = 8;

function currentEpochSeconds(): number {
  return Math.floor(Date.now() / 1000);
}

/**
 * Returns a copy of an MMDB file whose metadata `build_epoch` is `epochSeconds`. The value is rewritten in place at its
 * existing width; the function throws when the new value does not fit that width.
 */
export function withBuildEpoch(database: Buffer, epochSeconds: number): Buffer {
  const out = Buffer.from(database);

  const metadataStart = out.lastIndexOf(METADATA_START_MARKER);
  if (metadataStart < 0) {
    throw new Error('GeoIP fixture: database has no metadata section');
  }
  const keyStart = out.indexOf(BUILD_EPOCH_KEY, metadataStart);
  if (keyStart < 0) {
    throw new Error('GeoIP fixture: database metadata has no build_epoch');
  }

  let pos = keyStart + BUILD_EPOCH_KEY.length;
  const control = out[pos];
  pos += 1;
  let type = control >> 5;
  const size = control & 0x1f;
  if (type === EXTENDED_TYPE_MARKER) {
    type = out[pos] + EXTENDED_TYPE_OFFSET;
    pos += 1;
  }
  if (!UINT_TYPES.has(type) || size === 0 || size > MAX_UINT_BYTES) {
    throw new Error(`GeoIP fixture: unexpected build_epoch encoding (type ${type}, size ${size})`);
  }
  if (epochSeconds >= 2 ** (8 * size)) {
    throw new Error('GeoIP fixture: build_epoch does not fit the stored width');
  }

  let remaining = epochSeconds;
  for (let i = size - 1; i >= 0; i -= 1) {
    out[pos + i] = remaining % 256;
    remaining = Math.floor(remaining / 256);
  }
  return out;
}

/**
 * Loads the upload fixtures. The Country database is the bundled copy from the container with its build epoch set to
 * `nowSeconds`, which is after the bundled copy's own epoch, so the server accepts it as newer.
 */
export function loadGeoIPUploadFixtures(container: string, nowSeconds = currentEpochSeconds()): GeoIPUploadFixtures {
  const encoded = execInContainer(container, ['base64', BUNDLED_DATABASE_PATH]);
  const bundled = Buffer.from(encoded.replace(/\s+/g, ''), 'base64');

  return {
    countryDatabase: {
      name: 'GeoLite2-Country.mmdb',
      mimeType: 'application/octet-stream',
      buffer: withBuildEpoch(bundled, nowSeconds),
    },
    textFile: {
      name: 'not-a-database.mmdb',
      mimeType: 'text/plain',
      buffer: Buffer.from('This file is plain text, not a GeoIP database.\n'),
    },
  };
}
