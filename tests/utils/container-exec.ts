/**
 * Helpers for running commands inside the E2E container and for sending HTTP requests from inside it.
 *
 * Some checks need a request that comes from the container's own loopback address, which the test runner cannot
 * reach. The container ships busybox `nc` but no curl, and busybox `wget` prints only the status line for a non-2xx
 * answer (no headers, no body). Requests are therefore written to `nc` as HTTP/1.0 with `Connection: close`, and the
 * raw answer is parsed here. HTTP/1.0 answers from Caddy are not chunked and the connection closes after the answer,
 * so stdout is the whole response.
 */

import { execFileSync, spawn } from 'node:child_process';

const CONTAINER_CANDIDATES = ['charon-e2e', 'charon-playwright'] as const;
const MAX_BUFFER_BYTES = 16 * 1024 * 1024;
const PREVIEW_BYTES = 200;
/** Idle limit passed to busybox `nc`. */
const NC_IDLE_TIMEOUT_SECONDS = 10;
/** Hard deadline for one request, so a stuck proxy fails the check instead of hanging the run. */
const REQUEST_DEADLINE_MS = 20_000;

/**
 * Returns the name of the running E2E container. PLAYWRIGHT_CONTAINER_NAME wins when set; otherwise the first of the
 * local and CI container names that `docker inspect` finds.
 */
export function findE2EContainer(): string {
  const candidates = [process.env.PLAYWRIGHT_CONTAINER_NAME, ...CONTAINER_CANDIDATES].filter((name): name is string =>
    Boolean(name)
  );
  for (const name of candidates) {
    try {
      execFileSync('docker', ['inspect', '--format', '{{.Id}}', name], { stdio: 'pipe' });
      return name;
    } catch {
      // Not running under this name; try the next candidate.
    }
  }
  throw new Error(`No E2E container found among: ${candidates.join(', ')}`);
}

export interface ExecOptions {
  /** User to run the command as (`docker exec -u`). */
  user?: string;
  /** Data written to the command's stdin. */
  input?: string;
}

/**
 * Runs a command inside the container and returns its stdout. Arguments are passed as an array and never go through a
 * shell. A non-zero exit throws with the command's stderr in the message.
 */
export function execInContainer(container: string, args: readonly string[], options: ExecOptions = {}): string {
  const dockerArgs = ['exec'];
  if (options.input !== undefined) {
    dockerArgs.push('-i');
  }
  if (options.user) {
    dockerArgs.push('-u', options.user);
  }
  dockerArgs.push(container, ...args);

  try {
    return execFileSync('docker', dockerArgs, {
      stdio: 'pipe',
      encoding: 'utf8',
      input: options.input,
      maxBuffer: MAX_BUFFER_BYTES,
    });
  } catch (error) {
    const stderr = String((error as { stderr?: Buffer | string }).stderr ?? '').trim();
    const reason = stderr || (error as Error).message;
    throw new Error(`docker exec ${container} ${args.join(' ')} failed: ${reason}`, { cause: error });
  }
}

export type ContainerHttpMethod = 'GET' | 'POST' | 'DELETE';

export interface ContainerHttpRequest {
  /** Host header. Must not contain CR, LF or spaces. */
  host: string;
  /** Request target. Must not contain CR, LF or spaces. */
  path: string;
  method?: ContainerHttpMethod;
  /** Port inside the container. Caddy listens on 80 there. */
  port?: number;
}

export interface ContainerHttpResponse {
  status: number;
  /** Lower-cased header names. Repeated headers are joined with ', '. */
  headers: Record<string, string>;
  body: string;
}

const ALLOWED_METHODS: readonly ContainerHttpMethod[] = ['GET', 'POST', 'DELETE'];
const FORBIDDEN_LINE_CHARACTERS = /[\r\n ]/;

function preview(raw: string): string {
  return JSON.stringify(raw.slice(0, PREVIEW_BYTES));
}

/**
 * Sends one HTTP/1.0 request from inside the container and resolves with the parsed answer.
 *
 * The answer is parsed from the raw bytes, so a refused response (403 with its body) is returned like any other.
 * Rejects when the status line is missing or the answer is chunked, rather than guessing at a malformed response.
 */
export async function fetchFromContainer(
  container: string,
  options: ContainerHttpRequest
): Promise<ContainerHttpResponse> {
  const { host, path, method = 'GET', port = 80 } = options;

  if (!ALLOWED_METHODS.includes(method)) {
    throw new Error(`Unsupported method for container request: ${method}`);
  }
  if (FORBIDDEN_LINE_CHARACTERS.test(host) || FORBIDDEN_LINE_CHARACTERS.test(path)) {
    throw new Error('Container request host and path must not contain CR, LF or spaces');
  }

  const requestLines = [`${method} ${path} HTTP/1.0`, `Host: ${host}`, 'Connection: close'];
  if (method !== 'GET') {
    requestLines.push('Content-Length: 0');
  }
  const rawRequest = `${requestLines.map((line) => `${line}\r\n`).join('')}\r\n`;

  const stdout = await sendRawRequest(container, port, rawRequest);
  return parseHttp10Response(stdout);
}

/**
 * True once the raw answer holds its full header block and the Content-Length bytes of body after it.
 * The text is latin1-decoded, so string length equals byte length.
 */
function isCompleteAnswer(raw: string): boolean {
  const headerEnd = raw.indexOf('\r\n\r\n');
  if (headerEnd === -1) {
    return false;
  }
  const declared = /^content-length:\s*(\d+)/im.exec(raw.slice(0, headerEnd));
  return declared !== null && raw.length - (headerEnd + 4) >= Number(declared[1]);
}

/**
 * Writes the raw request to `nc` and resolves with the answer as soon as it is complete.
 *
 * busybox `nc` does not exit when the server closes its side; it only exits after stdin ends. Ending stdin before the
 * answer is complete would make `nc` drop the connection, and Caddy would then cancel a proxied request before its
 * upstream answers (a 499 instead of the 502). So stdin is ended only after `isCompleteAnswer` holds. Answers without a
 * Content-Length never complete and fail at the deadline; Caddy and Go set one for the small bodies used here.
 */
function sendRawRequest(container: string, port: number, rawRequest: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(
      'docker',
      ['exec', '-i', container, 'nc', '-w', String(NC_IDLE_TIMEOUT_SECONDS), '127.0.0.1', String(port)],
      { stdio: ['pipe', 'pipe', 'pipe'] }
    );
    const received: Buffer[] = [];
    const stderr: Buffer[] = [];
    let settled = false;

    const settle = (outcome: () => void): void => {
      if (!settled) {
        settled = true;
        clearTimeout(deadline);
        outcome();
      }
    };

    const deadline = setTimeout(() => {
      settle(() => {
        child.kill();
        reject(
          new Error(
            `Container request to port ${port} did not complete within ${REQUEST_DEADLINE_MS} ms ` +
              '(the answer may lack a Content-Length header)'
          )
        );
      });
    }, REQUEST_DEADLINE_MS);

    child.stdout.on('data', (chunk: Buffer) => {
      received.push(chunk);
      const text = Buffer.concat(received).toString('latin1');
      if (isCompleteAnswer(text)) {
        settle(() => {
          child.stdin.end();
          resolve(text);
        });
      }
    });
    child.stderr.on('data', (chunk: Buffer) => stderr.push(chunk));
    child.on('error', (error) => settle(() => reject(error)));
    child.on('close', () => {
      settle(() => {
        const output = Buffer.concat(received).toString('latin1');
        if (output === '' && stderr.length > 0) {
          reject(new Error(`docker exec ${container} nc failed: ${Buffer.concat(stderr).toString('utf8').trim()}`));
          return;
        }
        resolve(output);
      });
    });
    // nc may exit before the request is fully written; the close handler reports the outcome.
    child.stdin.on('error', () => undefined);
    child.stdin.write(rawRequest);
  });
}

/** Parses one raw HTTP/1.0 answer: status line, headers, then the body after the blank line. */
function parseHttp10Response(stdout: string): ContainerHttpResponse {
  const headerEnd = stdout.indexOf('\r\n\r\n');
  if (headerEnd === -1) {
    throw new Error(`Malformed container response (no header terminator): ${preview(stdout)}`);
  }
  const head = stdout.slice(0, headerEnd);
  // The raw text is latin1-decoded (one char per byte), so the body is decoded back to UTF-8 here.
  const body = Buffer.from(stdout.slice(headerEnd + 4), 'latin1').toString('utf8');

  const statusMatch = /^HTTP\/1\.[01] (\d{3})/.exec(head);
  if (!statusMatch) {
    throw new Error(`Malformed container response (no status line): ${preview(stdout)}`);
  }

  const headers = new Map<string, string>();
  for (const line of head.split('\r\n').slice(1)) {
    const colon = line.indexOf(':');
    if (colon <= 0) {
      continue;
    }
    const name = line.slice(0, colon).trim().toLowerCase();
    const value = line.slice(colon + 1).trim();
    const previous = headers.get(name);
    headers.set(name, previous === undefined ? value : `${previous}, ${value}`);
  }

  if ((headers.get('transfer-encoding') ?? '').toLowerCase().includes('chunked')) {
    throw new Error('Container response is chunked; HTTP/1.0 answers must not be chunked');
  }

  return {
    status: Number(statusMatch[1]),
    headers: Object.fromEntries(headers),
    body,
  };
}
