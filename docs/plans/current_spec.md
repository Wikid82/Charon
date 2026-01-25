# Docker Hub + GHCR Dual Registry Publishing Plan

**Plan ID**: DOCKER-2026-001
**Status**: 📋 PLANNED
**Priority**: High
**Created**: 2026-01-25
**Branch**: feature/beta-release
**Scope**: Publish Docker images to both Docker Hub and GitHub Container Registry (GHCR)

---

## Executive Summary

This plan details the implementation of dual-registry publishing for the Charon Docker image. Currently, images are published exclusively to GHCR (`ghcr.io/wikid82/charon`). This plan adds Docker Hub (`docker.io/wikid82/charon`) as an additional registry while maintaining full parity in tags, platforms, and supply chain security.

---

## 1. Current State Analysis

### 1.1 Existing Registry Setup (GHCR Only)

| Workflow | Purpose | Tags Generated | Platforms |
|----------|---------|----------------|-----------|
| `docker-build.yml` | Main builds on push/PR | `latest`, `dev`, `sha-*`, `pr-*`, `feature-*` | `linux/amd64`, `linux/arm64` |
| `nightly-build.yml` | Nightly builds from `nightly` branch | `nightly`, `nightly-YYYY-MM-DD`, `nightly-sha-*` | `linux/amd64`, `linux/arm64` |
| `release-goreleaser.yml` | Release builds on tag push | `vX.Y.Z` | N/A (binary releases, not Docker) |

### 1.2 Current Environment Variables

```yaml
# docker-build.yml (Line 25-28)
env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository_owner }}/charon
```

### 1.3 Supply Chain Security Features

| Feature | Status | Implementation |
|---------|--------|----------------|
| **SBOM Generation** | ✅ Active | `anchore/sbom-action` → CycloneDX JSON |
| **SBOM Attestation** | ✅ Active | `actions/attest-sbom` → Push to registry |
| **Trivy Scanning** | ✅ Active | SARIF upload to GitHub Security |
| **Cosign Signing** | 🔶 Partial | Verification exists, signing not in docker-build.yml |
| **SLSA Provenance** | ⚠️ Not Implemented | `provenance: true` in Buildx but not verified |

### 1.4 Current Permissions

```yaml
# docker-build.yml (Lines 31-36)
permissions:
  contents: read
  packages: write
  security-events: write
  id-token: write      # OIDC for signing
  attestations: write  # SBOM attestation
```

---

## 2. Docker Hub Setup

### 2.1 Required GitHub Secrets

| Secret Name | Description | Where to Get |
|-------------|-------------|--------------|
| `DOCKERHUB_USERNAME` | Docker Hub username | hub.docker.com → Account Settings |
| `DOCKERHUB_TOKEN` | Docker Hub Access Token | hub.docker.com → Account Settings → Security → New Access Token |

**Access Token Requirements:**
- Scope: `Read, Write, Delete` for automated pushes
- Name: e.g., `github-actions-charon`

### 2.2 Repository Naming

| Registry | Repository | Full Image Reference |
|----------|------------|---------------------|
| Docker Hub | `wikid82/charon` | `docker.io/wikid82/charon:latest` |
| GHCR | `wikid82/charon` | `ghcr.io/wikid82/charon:latest` |

**Note**: Docker Hub uses lowercase repository names. The GitHub repository owner is `Wikid82` (capital W), so we normalize to `wikid82`.

### 2.3 Docker Hub Repository Setup

1. Go to [hub.docker.com](https://hub.docker.com)
2. Click "Create Repository"
3. Name: `charon`
4. Visibility: Public
5. Description: "Web UI for managing Caddy reverse proxy configurations"

---

## 3. Workflow Modifications

### 3.1 Files to Modify

| File | Changes |
|------|---------|
| `.github/workflows/docker-build.yml` | Add Docker Hub login, multi-registry push |
| `.github/workflows/nightly-build.yml` | Add Docker Hub login, multi-registry push |
| `.github/workflows/supply-chain-verify.yml` | Verify Docker Hub signatures |

### 3.2 docker-build.yml Changes

#### 3.2.1 Update Environment Variables

**Location**: Lines 25-28

**Before**:
```yaml
env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository_owner }}/charon
  SYFT_VERSION: v1.17.0
  GRYPE_VERSION: v0.85.0
```

**After**:
```yaml
env:
  # Primary registry (GHCR)
  GHCR_REGISTRY: ghcr.io
  # Secondary registry (Docker Hub)
  DOCKERHUB_REGISTRY: docker.io
  # Image name (lowercase for Docker Hub compatibility)
  IMAGE_NAME: wikid82/charon
  SYFT_VERSION: v1.17.0
  GRYPE_VERSION: v0.85.0
```

#### 3.2.2 Add Docker Hub Login Step

**Location**: After "Log in to Container Registry" step (around line 70)

**Add**:
```yaml
      - name: Log in to Docker Hub
        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true'
        uses: docker/login-action@5e57cd118135c172c3672efd75eb46360885c0ef # v3.6.0
        with:
          registry: docker.io
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
```

#### 3.2.3 Update Metadata Action for Multi-Registry

**Location**: Extract metadata step (around line 78)

**Before**:
```yaml
      - name: Extract metadata (tags, labels)
        id: meta
        uses: docker/metadata-action@c299e40c65443455700f0fdfc63efafe5b349051 # v5.10.0
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=semver,pattern={{version}}
            # ... rest of tags
```

**After**:
```yaml
      - name: Extract metadata (tags, labels)
        id: meta
        uses: docker/metadata-action@c299e40c65443455700f0fdfc63efafe5b349051 # v5.10.0
        with:
          images: |
            ${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}
            ${{ env.DOCKERHUB_REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=dev,enable=${{ github.ref == 'refs/heads/development' }}
            type=ref,event=branch,enable=${{ startsWith(github.ref, 'refs/heads/feature/') }}
            type=raw,value=pr-${{ github.event.pull_request.number }},enable=${{ github.event_name == 'pull_request' }}
            type=sha,format=short,enable=${{ github.event_name != 'pull_request' }}
          flavor: |
            latest=false
```

#### 3.2.4 Add Cosign Signing for Docker Hub

**Location**: After SBOM attestation step (around line 190)

**Add**:
```yaml
      # Sign Docker Hub image with Cosign (keyless, OIDC-based)
      - name: Install Cosign
        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true' && steps.skip.outputs.is_feature_push != 'true'
        uses: sigstore/cosign-installer@3454372f43399081ed03b604cb2d021dabca52bb # v3.8.2

      - name: Sign GHCR Image with Cosign
        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true' && steps.skip.outputs.is_feature_push != 'true'
        env:
          DIGEST: ${{ steps.build-and-push.outputs.digest }}
          COSIGN_EXPERIMENTAL: "true"
        run: |
          echo "Signing GHCR image with Cosign..."
          cosign sign --yes ${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}@${DIGEST}

      - name: Sign Docker Hub Image with Cosign
        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true' && steps.skip.outputs.is_feature_push != 'true'
        env:
          DIGEST: ${{ steps.build-and-push.outputs.digest }}
          COSIGN_EXPERIMENTAL: "true"
        run: |
          echo "Signing Docker Hub image with Cosign..."
          cosign sign --yes ${{ env.DOCKERHUB_REGISTRY }}/${{ env.IMAGE_NAME }}@${DIGEST}
```

#### 3.2.5 Attach SBOM to Docker Hub

**Location**: After existing SBOM attestation (around line 200)

**Add**:
```yaml
      # Attach SBOM to Docker Hub image
      - name: Attach SBOM to Docker Hub
        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true' && steps.skip.outputs.is_feature_push != 'true'
        run: |
          echo "Attaching SBOM to Docker Hub image..."
          cosign attach sbom --sbom sbom.cyclonedx.json \
            ${{ env.DOCKERHUB_REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}
```

### 3.3 nightly-build.yml Changes

Apply similar changes:

1. Add `DOCKERHUB_REGISTRY` environment variable
2. Add Docker Hub login step
3. Update metadata action with multiple images
4. Add Cosign signing for both registries
5. Attach SBOM to Docker Hub image

### 3.4 Complete docker-build.yml Diff Summary

```diff
 env:
-  REGISTRY: ghcr.io
-  IMAGE_NAME: ${{ github.repository_owner }}/charon
+  GHCR_REGISTRY: ghcr.io
+  DOCKERHUB_REGISTRY: docker.io
+  IMAGE_NAME: wikid82/charon
   SYFT_VERSION: v1.17.0
   GRYPE_VERSION: v0.85.0

 # ... in steps ...

       - name: Log in to Container Registry
         if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true'
         uses: docker/login-action@5e57cd118135c172c3672efd75eb46360885c0ef # v3.6.0
         with:
-          registry: ${{ env.REGISTRY }}
+          registry: ${{ env.GHCR_REGISTRY }}
           username: ${{ github.actor }}
           password: ${{ secrets.GITHUB_TOKEN }}

+      - name: Log in to Docker Hub
+        if: github.event_name != 'pull_request' && steps.skip.outputs.skip_build != 'true'
+        uses: docker/login-action@5e57cd118135c172c3672efd75eb46360885c0ef # v3.6.0
+        with:
+          registry: docker.io
+          username: ${{ secrets.DOCKERHUB_USERNAME }}
+          password: ${{ secrets.DOCKERHUB_TOKEN }}

       - name: Extract metadata (tags, labels)
         id: meta
         uses: docker/metadata-action@c299e40c65443455700f0fdfc63efafe5b349051 # v5.10.0
         with:
-          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
+          images: |
+            ${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}
+            ${{ env.DOCKERHUB_REGISTRY }}/${{ env.IMAGE_NAME }}
           tags: |
             # ... tags unchanged ...
```

---

## 4. Supply Chain Security

### 4.1 Parity Matrix

| Feature | GHCR | Docker Hub |
|---------|------|------------|
| Multi-platform | ✅ `linux/amd64`, `linux/arm64` | ✅ Same |
| SBOM | ✅ Attestation | ✅ Attached via Cosign |
| Cosign Signature | ✅ Keyless OIDC | ✅ Keyless OIDC |
| Trivy Scan | ✅ SARIF to GitHub | ✅ Same SARIF |
| SLSA Provenance | 🔶 Buildx `provenance: true` | 🔶 Same |

### 4.2 Cosign Signing Strategy

Both registries will use **keyless signing** via OIDC (OpenID Connect):

- No private keys to manage
- Signatures tied to GitHub Actions identity
- Transparent logging to Sigstore Rekor

### 4.3 SBOM Attachment Strategy

**GHCR**: Uses `actions/attest-sbom` which creates an attestation linked to the image manifest.

**Docker Hub**: Uses `cosign attach sbom` to attach the SBOM as an OCI artifact.

### 4.4 Verification Commands

```bash
# Verify GHCR signature
cosign verify ghcr.io/wikid82/charon:latest \
  --certificate-identity-regexp="https://github.com/Wikid82/Charon" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Verify Docker Hub signature
cosign verify docker.io/wikid82/charon:latest \
  --certificate-identity-regexp="https://github.com/Wikid82/Charon" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Download SBOM from Docker Hub
cosign download sbom docker.io/wikid82/charon:latest > sbom.json
```

---

## 5. Tag Strategy

### 5.1 Tag Parity Matrix

| Trigger | GHCR Tag | Docker Hub Tag |
|---------|----------|----------------|
| Push to `main` | `ghcr.io/wikid82/charon:latest` | `docker.io/wikid82/charon:latest` |
| Push to `development` | `ghcr.io/wikid82/charon:dev` | `docker.io/wikid82/charon:dev` |
| Push to `feature/*` | `ghcr.io/wikid82/charon:feature-*` | `docker.io/wikid82/charon:feature-*` |
| PR | `ghcr.io/wikid82/charon:pr-N` | ❌ Not pushed to Docker Hub |
| Release tag `vX.Y.Z` | `ghcr.io/wikid82/charon:X.Y.Z` | `docker.io/wikid82/charon:X.Y.Z` |
| SHA | `ghcr.io/wikid82/charon:sha-abc1234` | `docker.io/wikid82/charon:sha-abc1234` |
| Nightly | `ghcr.io/wikid82/charon:nightly` | `docker.io/wikid82/charon:nightly` |

### 5.2 PR Images

PR images (`pr-N`) are **not pushed to Docker Hub** to:
- Reduce Docker Hub storage/bandwidth usage
- Keep Docker Hub clean for production images
- PRs are internal development artifacts

---

## 6. Documentation Updates

### 6.1 README.md Changes

**Location**: Badge section (around line 13-22)

**Add Docker Hub badge**:
```markdown
<p align="center">
  <!-- Existing badges -->
  <a href="https://www.repostatus.org/#active"><img src="https://www.repostatus.org/badges/latest/active.svg" alt="Project Status: Active" /></a>
  <a href="https://www.bestpractices.dev/projects/11648"><img src="https://www.bestpractices.dev/projects/11648/badge"></a>
  <br>
  <!-- Add Docker Hub badge -->
  <a href="https://hub.docker.com/r/wikid82/charon"><img src="https://img.shields.io/docker/pulls/wikid82/charon.svg" alt="Docker Pulls"></a>
  <a href="https://hub.docker.com/r/wikid82/charon"><img src="https://img.shields.io/docker/v/wikid82/charon?sort=semver" alt="Docker Version"></a>
  <!-- Existing badges continue -->
  <a href="https://codecov.io/gh/Wikid82/Charon" ><img src="https://codecov.io/gh/Wikid82/Charon/branch/main/graph/badge.svg?token=RXSINLQTGE" alt="Code Coverage"/></a>
  <!-- ... -->
</p>
```

**Add to Installation section**:
```markdown
## Quick Start

### Docker Hub (Recommended)

\`\`\`bash
docker pull wikid82/charon:latest
docker run -d -p 80:80 -p 443:443 -p 8080:8080 \
  -v charon-data:/app/data \
  wikid82/charon:latest
\`\`\`

### GitHub Container Registry

\`\`\`bash
docker pull ghcr.io/wikid82/charon:latest
docker run -d -p 80:80 -p 443:443 -p 8080:8080 \
  -v charon-data:/app/data \
  ghcr.io/wikid82/charon:latest
\`\`\`
```

### 6.2 getting-started.md Changes

**Location**: Step 1 Install section

**Update docker-compose.yml example**:
```yaml
services:
  charon:
    # Docker Hub (recommended for most users)
    image: wikid82/charon:latest
    # Alternative: GitHub Container Registry
    # image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"
    volumes:
      - ./charon-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - CHARON_ENV=production
```

### 6.3 Docker Hub README Sync

Create a workflow or use Docker Hub's "Build Settings" to sync the README:

**Option A**: Manual sync via Docker Hub API (in workflow)
```yaml
      - name: Sync README to Docker Hub
        if: github.ref == 'refs/heads/main'
        uses: peter-evans/dockerhub-description@0e6a7b2f56b498411d884fc55f14e1e2caf38d24 # v4.0.2
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
          repository: wikid82/charon
          readme-filepath: ./README.md
          short-description: "Web UI for managing Caddy reverse proxy configurations"
```

**Option B**: Create a dedicated Docker Hub README at `docs/docker-hub-readme.md`

---

## 7. File Change Review

### 7.1 .gitignore

**No changes required.** Current `.gitignore` is comprehensive.

### 7.2 codecov.yml

**No changes required.** Docker image publishing doesn't affect code coverage.

### 7.3 .dockerignore

**No changes required.** Current `.dockerignore` is comprehensive and well-organized.

### 7.4 Dockerfile

**No changes required.** The Dockerfile is registry-agnostic. Labels are already configured:

```dockerfile
LABEL org.opencontainers.image.source="https://github.com/Wikid82/charon" \
      org.opencontainers.image.url="https://github.com/Wikid82/charon" \
      org.opencontainers.image.vendor="charon" \
```

---

## 8. Implementation Checklist

### Phase 1: Docker Hub Setup (Manual)

- [ ] **1.1** Create Docker Hub account (if not exists)
- [ ] **1.2** Create `wikid82/charon` repository on Docker Hub
- [ ] **1.3** Generate Docker Hub Access Token
- [ ] **1.4** Add `DOCKERHUB_USERNAME` secret to GitHub repository
- [ ] **1.5** Add `DOCKERHUB_TOKEN` secret to GitHub repository

### Phase 2: Workflow Updates

- [ ] **2.1** Update `docker-build.yml` environment variables
- [ ] **2.2** Add Docker Hub login step to `docker-build.yml`
- [ ] **2.3** Update metadata action for multi-registry in `docker-build.yml`
- [ ] **2.4** Add Cosign signing steps to `docker-build.yml`
- [ ] **2.5** Add SBOM attachment step to `docker-build.yml`
- [ ] **2.6** Apply same changes to `nightly-build.yml`
- [ ] **2.7** Add README sync step (optional)

### Phase 3: Documentation

- [ ] **3.1** Add Docker Hub badge to `README.md`
- [ ] **3.2** Update Quick Start section in `README.md`
- [ ] **3.3** Update `docs/getting-started.md` with Docker Hub examples
- [ ] **3.4** Create Docker Hub-specific README (optional)

### Phase 4: Verification

- [ ] **4.1** Push to `development` branch and verify both registries receive image
- [ ] **4.2** Verify tags are identical on both registries
- [ ] **4.3** Verify Cosign signatures on both registries
- [ ] **4.4** Verify SBOM attachment on Docker Hub
- [ ] **4.5** Pull image from Docker Hub and run basic smoke test
- [ ] **4.6** Create test release tag and verify version tags

### Phase 5: Monitoring

- [ ] **5.1** Set up Docker Hub vulnerability scanning (Settings → Vulnerability Scanning)
- [ ] **5.2** Monitor Docker Hub download metrics

---

## 9. Rollback Plan

If issues occur with Docker Hub publishing:

1. **Immediate**: Remove Docker Hub login step from workflow
2. **Revert**: Use `git revert` on the workflow changes
3. **Secrets**: Secrets can remain (they're not exposed)
4. **Docker Hub Repo**: Can remain (no harm in empty repo)

---

## 10. Security Considerations

### 10.1 Secret Management

| Secret | Rotation Policy | Access Level |
|--------|-----------------|--------------|
| `DOCKERHUB_TOKEN` | Every 90 days | Read/Write/Delete |
| `GITHUB_TOKEN` | Auto-rotated | Built-in |

### 10.2 Supply Chain Risks

| Risk | Mitigation |
|------|------------|
| Compromised Docker Hub credentials | Use access tokens (not password), enable 2FA |
| Image tampering | Cosign signatures verify integrity |
| Dependency confusion | SBOM provides transparency |
| Malicious base image | Pin base images by digest in Dockerfile |

---

## 11. Cost Analysis

### Docker Hub Free Tier Limits

| Resource | Limit | Expected Usage |
|----------|-------|----------------|
| Private repos | 1 | 0 (public repo) |
| Pulls | Unlimited for public | N/A |
| Builds | Disabled (we use GitHub Actions) | 0 |
| Teams | 1 | 1 |

**Conclusion**: No cost impact expected for public repository.

---

## 12. References

- [docker/login-action](https://github.com/docker/login-action)
- [docker/metadata-action - Multiple registries](https://github.com/docker/metadata-action#extracting-to-multiple-registries)
- [Cosign keyless signing](https://docs.sigstore.dev/cosign/keyless/)
- [Cosign attach sbom](https://docs.sigstore.dev/cosign/signing/other_types/#sbom)
- [peter-evans/dockerhub-description](https://github.com/peter-evans/dockerhub-description)
- [Docker Hub Access Tokens](https://docs.docker.com/docker-hub/access-tokens/)

---

## 13. Appendix: Full Workflow YAML

### 13.1 Updated docker-build.yml (Complete)

See the detailed diff in Section 3.4. The full updated workflow should be generated during implementation.

### 13.2 Example Multi-Registry Push Output

```
#12 pushing ghcr.io/wikid82/charon:latest with docker
#12 pushing layer sha256:abc123... 0.2s
#12 pushing manifest sha256:xyz789... done
#12 pushing ghcr.io/wikid82/charon:sha-abc1234 with docker
#12 done

#13 pushing docker.io/wikid82/charon:latest with docker
#13 pushing layer sha256:abc123... 0.2s
#13 pushing manifest sha256:xyz789... done
#13 pushing docker.io/wikid82/charon:sha-abc1234 with docker
#13 done
```

---

**End of Plan**
