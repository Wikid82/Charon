const DEFAULT_OUTPUT = ".github/badges/ghcr-downloads.json";
const GH_API_BASE = "https://api.github.com";

const owner = process.env.GHCR_OWNER || process.env.GITHUB_REPOSITORY_OWNER;
const packageName = process.env.GHCR_PACKAGE || "charon";
const outputPath = process.env.BADGE_OUTPUT || DEFAULT_OUTPUT;
const token = process.env.GITHUB_TOKEN || "";

if (!owner) {
  throw new Error("GHCR owner is required. Set GHCR_OWNER or GITHUB_REPOSITORY_OWNER.");
}

const headers = {
  Accept: "application/vnd.github+json",
};

if (token) {
  headers.Authorization = `Bearer ${token}`;
}

const formatCount = (value) => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(1).replace(/\.0$/, "")}B`;
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(1).replace(/\.0$/, "")}k`;
  }
  return String(value);
};

const getNextLink = (linkHeader) => {
  if (!linkHeader) {
    return null;
  }
  const match = linkHeader.match(/<([^>]+)>;\s*rel="next"/);
  return match ? match[1] : null;
};

const fetchPage = async (url) => {
  const response = await fetch(url, { headers });
  if (!response.ok) {
    const detail = await response.text();
    const error = new Error(`Request failed: ${response.status} ${response.statusText}`);
    error.status = response.status;
    error.detail = detail;
    throw error;
  }
  const data = await response.json();
  const link = response.headers.get("link");
  return { data, next: getNextLink(link) };
};

const fetchAllVersions = async (baseUrl) => {
  let url = `${baseUrl}?per_page=100`;
  const versions = [];

  while (url) {
    const { data, next } = await fetchPage(url);
    versions.push(...data);
    url = next;
  }

  return versions;
};

const fetchVersionsWithFallback = async () => {
  const userUrl = `${GH_API_BASE}/users/${owner}/packages/container/${packageName}/versions`;
  try {
    return await fetchAllVersions(userUrl);
  } catch (error) {
    if (error.status !== 404) {
      throw error;
    }
  }

  const orgUrl = `${GH_API_BASE}/orgs/${owner}/packages/container/${packageName}/versions`;
  return fetchAllVersions(orgUrl);
};

const run = async () => {
  const versions = await fetchVersionsWithFallback();
  const totalDownloads = versions.reduce(
    (sum, version) => sum + (version.download_count || 0),
    0
  );

  const badge = {
    schemaVersion: 1,
    label: "GHCR pulls",
    message: formatCount(totalDownloads),
    color: "blue",
    cacheSeconds: 3600,
  };

  const output = `${JSON.stringify(badge, null, 2)}\n`;
  await import("node:fs/promises").then((fs) => fs.writeFile(outputPath, output));

  console.log(`GHCR downloads: ${totalDownloads} -> ${outputPath}`);
};

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
