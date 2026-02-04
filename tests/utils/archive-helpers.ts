import { promises as fs } from 'fs';
import * as tar from 'tar';
import * as path from 'path';
import { createGzip } from 'zlib';
import { createWriteStream, createReadStream } from 'fs';
import { pipeline } from 'stream/promises';

export interface ArchiveOptions {
  format: 'tar.gz' | 'zip';
  compression?: 'high' | 'normal' | 'none';
  files: Record<string, string>; // filename -> content
}

/**
 * Create a tar.gz archive with specified files
 * @param files - Object mapping filenames to their content
 * @param outputPath - Absolute path where the archive should be created
 * @returns Absolute path to the created archive
 */
export async function createTarGz(
  files: Record<string, string>,
  outputPath: string
): Promise<string> {
  // Ensure parent directory exists
  await fs.mkdir(path.dirname(outputPath), { recursive: true });

  // Create temporary directory for files
  const tempDir = path.join(path.dirname(outputPath), `.temp-${Date.now()}`);
  await fs.mkdir(tempDir, { recursive: true });

  try {
    // Write all files to temp directory
    for (const [filename, content] of Object.entries(files)) {
      const filePath = path.join(tempDir, filename);
      await fs.mkdir(path.dirname(filePath), { recursive: true });
      await fs.writeFile(filePath, content, 'utf-8');
    }

    // Create tar.gz archive
    await tar.create(
      {
        gzip: true,
        file: outputPath,
        cwd: tempDir,
      },
      Object.keys(files)
    );

    return outputPath;
  } finally {
    // Clean up temp directory
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}

/**
 * Create a zip bomb (highly compressed file) for testing compression ratio detection
 * @param outputPath - Absolute path where the archive should be created
 * @param compressionRatio - Target compression ratio (default: 150x)
 * @returns Absolute path to the created archive
 */
export async function createZipBomb(
  outputPath: string,
  compressionRatio: number = 150
): Promise<string> {
  // Ensure parent directory exists
  await fs.mkdir(path.dirname(outputPath), { recursive: true });

  // Create temporary directory
  const tempDir = path.join(path.dirname(outputPath), `.temp-zipbomb-${Date.now()}`);
  await fs.mkdir(tempDir, { recursive: true });

  try {
    // Create a highly compressible file (10MB of zeros)
    // This will compress to a very small size
    const uncompressedSize = 10 * 1024 * 1024; // 10MB
    const compressibleData = Buffer.alloc(uncompressedSize, 0);

    const tempFilePath = path.join(tempDir, 'config.yaml');

    // Add valid YAML header to make it look legitimate
    const yamlHeader = Buffer.from(`api:
  server:
    listen_uri: 0.0.0.0:8080
# Padding data below to create compression ratio anomaly
# `, 'utf-8');

    await fs.writeFile(tempFilePath, Buffer.concat([yamlHeader, compressibleData]));

    // Create tar.gz archive with maximum compression
    await tar.create(
      {
        gzip: {
          level: 9, // Maximum compression
        },
        file: outputPath,
        cwd: tempDir,
      },
      ['config.yaml']
    );

    return outputPath;
  } finally {
    // Clean up temp directory
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}

/**
 * Create a corrupted archive file for testing error handling
 * @param outputPath - Absolute path where the corrupted archive should be created
 * @returns Absolute path to the created corrupted archive
 */
export async function createCorruptedArchive(
  outputPath: string
): Promise<string> {
  // Ensure parent directory exists
  await fs.mkdir(path.dirname(outputPath), { recursive: true });

  // Create a file that starts with gzip magic bytes but has corrupted data
  const gzipMagicBytes = Buffer.from([0x1f, 0x8b]); // gzip signature
  const corruptedData = Buffer.from('this is not valid gzip data after the magic bytes');

  const corruptedArchive = Buffer.concat([gzipMagicBytes, corruptedData]);

  await fs.writeFile(outputPath, corruptedArchive);

  return outputPath;
}

/**
 * Create a ZIP file (unsupported format) for testing format validation
 * @param files - Object mapping filenames to their content
 * @param outputPath - Absolute path where the ZIP should be created
 * @returns Absolute path to the created ZIP file
 */
export async function createZip(
  files: Record<string, string>,
  outputPath: string
): Promise<string> {
  // Ensure parent directory exists
  await fs.mkdir(path.dirname(outputPath), { recursive: true });

  // Create a minimal ZIP file with magic bytes
  // PK\x03\x04 is ZIP magic number
  const zipMagicBytes = Buffer.from([0x50, 0x4b, 0x03, 0x04]);

  // For testing, just create a file with ZIP signature
  // Real ZIP creation would require jszip or archiver library
  await fs.writeFile(outputPath, zipMagicBytes);

  return outputPath;
}

/**
 * Create an oversized archive for testing size limits
 * @param outputPath - Absolute path where the archive should be created
 * @param sizeMB - Size in megabytes (default: 51MB to exceed 50MB limit)
 * @returns Absolute path to the created archive
 */
export async function createOversizedArchive(
  outputPath: string,
  sizeMB: number = 51
): Promise<string> {
  // Ensure parent directory exists
  await fs.mkdir(path.dirname(outputPath), { recursive: true });

  // Create temporary directory
  const tempDir = path.join(path.dirname(outputPath), `.temp-oversized-${Date.now()}`);
  await fs.mkdir(tempDir, { recursive: true });

  try {
    // Create a large file (use random data so it doesn't compress well)
    const sizeBytes = sizeMB * 1024 * 1024;
    const chunkSize = 1024 * 1024; // 1MB chunks
    const tempFilePath = path.join(tempDir, 'large-config.yaml');

    // Write in chunks to avoid memory issues
    const writeStream = createWriteStream(tempFilePath);

    for (let i = 0; i < Math.ceil(sizeBytes / chunkSize); i++) {
      const remainingBytes = Math.min(chunkSize, sizeBytes - (i * chunkSize));
      // Use random data to prevent compression
      const chunk = Buffer.from(
        Array.from({ length: remainingBytes }, () => Math.floor(Math.random() * 256))
      );
      writeStream.write(chunk);
    }

    await new Promise((resolve) => writeStream.end(resolve));

    // Create tar.gz archive
    await tar.create(
      {
        gzip: true,
        file: outputPath,
        cwd: tempDir,
      },
      ['large-config.yaml']
    );

    return outputPath;
  } finally {
    // Clean up temp directory
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}
