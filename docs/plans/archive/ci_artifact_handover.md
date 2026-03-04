---
title: "CI Docker Image Artifact Handover"
status: "draft"
scope: "ci/integration, docker/build"
---

## 1. Introduction

This plan updates the CI pipeline to pass the Docker image between jobs
as a build artifact instead of pulling from a registry. The goal is to
eliminate the integration-stage "invalid reference format" error by
loading a known-good local image tag (`charon:local`) in each
integration job.

Objectives:

- Export a Docker image tarball from the build stage, tagged
  `charon:local`.
- Upload the tarball as a CI artifact and consume it in integration
  jobs.
- Remove Docker Hub login and image pull steps from integration jobs.
- Ensure the export step reuses build cache to avoid double build time.

## 2. Research Findings

- The integration jobs in
  `.github/workflows/ci-pipeline.yml` currently log into Docker Hub and
  run `docker pull` using `needs.build-image.outputs.image_ref_dockerhub`.
- The reported "invalid reference format" error is consistent with an
  empty or malformed image reference at the pull step.
- The build job already uses `docker/build-push-action` for the main
  build/push step but does not export a local image tarball artifact.

## 3. Technical Specifications

### 3.1 Build Image Job: Export Artifact

Location: `.github/workflows/ci-pipeline.yml` job `build-image`.

Add a dedicated export step after the main build step:

- Use `docker/build-push-action` with:
  - `platforms: linux/amd64`
  - `outputs: type=docker,dest=/tmp/charon.tar`
  - `tags: charon:local`
- Ensure cache reuse to avoid a full rebuild:
  - Main build step: add `cache-to: type=gha,mode=max`
  - Export step: add `cache-from: type=gha`

Update the `run_integration` output in `build-image`:

- Set `run_integration` to `true` when the artifact build succeeds.
- Do not depend on `steps.image-policy.outputs.push == true` for
  integration gating.

Add an artifact upload step:

- Use `actions/upload-artifact`.
- `name: docker-image`
- `path: /tmp/charon.tar`
- `retention-days: 1`

### 3.2 Integration Jobs: Consume Artifact

Jobs:
- `integration-cerberus`
- `integration-crowdsec`
- `integration-waf`
- `integration-ratelimit`

Required changes:

- Remove steps:
  - "Log in to Docker Hub"
  - "Pull shared image"
- Add steps:
  - Download artifact via `actions/download-artifact` with
    `name: docker-image` and `path: /tmp`.
  - Load image via `docker load --input /tmp/charon.tar`.
  - Verify image availability with `docker image inspect charon:local`.

### 3.3 Image Tag Validation

- Ensure the export step sets `tags: charon:local` so integration tests
  continue to use the existing `charon:local` tag without changes to
  test scripts.

## 4. Implementation Plan

### Phase 1: CI Build Artifact Export

1. Update `build-image` job to add cache settings to the existing
   `docker/build-push-action` step.
2. Add a new export step using `docker/build-push-action` with
  `platforms: linux/amd64`, `outputs: type=docker,dest=/tmp/charon.tar`,
  and `tags: charon:local`.
3. Upload `/tmp/charon.tar` as `docker-image` artifact.
4. Update `run_integration` output to depend on artifact build success
  only.

### Phase 2: Integration Job Artifact Handover

1. Remove Docker Hub login and pull steps from each integration job.
2. Add `actions/download-artifact` for `docker-image`.
3. Add a `docker load` step and verify `charon:local`.

### Phase 3: Validation

1. Confirm integration jobs no longer call `docker pull`.
2. Confirm `docker image inspect charon:local` succeeds before each
   integration test script.
3. Confirm build logs show cache reuse for the export step.

## 5. Acceptance Criteria (EARS)

- WHEN the `build-image` job completes, THE SYSTEM SHALL export a Docker
  tarball at `/tmp/charon.tar` tagged `charon:local`.
- WHEN the `build-image` job completes, THE SYSTEM SHALL upload the
  tarball as an artifact named `docker-image`.
- WHEN an integration job starts, THE SYSTEM SHALL download the
  `docker-image` artifact and load it with `docker load`.
- WHEN the image is loaded in an integration job, THE SYSTEM SHALL
  verify `charon:local` exists before running integration scripts.
- WHEN integration jobs run, THE SYSTEM SHALL NOT log into Docker Hub or
  pull from the registry.
- WHEN the export step runs, THE SYSTEM SHALL reuse the build cache from
  the main build step to avoid a full rebuild.
- WHEN the artifact build step succeeds, THE SYSTEM SHALL set
  `run_integration` to `true` regardless of registry push settings.

## 6. Risks and Mitigations

- Risk: Export step rebuilds the image, increasing CI time.
  Mitigation: Use `cache-to` on the main build step and `cache-from` on
  the export step to reuse the Buildx cache.

- Risk: Artifact download or load fails due to path issues.
  Mitigation: Use consistent `/tmp/charon.tar` path and add
  `docker image inspect charon:local` to fail fast.

## 7. Confidence Score

Confidence: 86 percent

Rationale: The workflow changes are limited to CI steps and standard
Docker Buildx artifact handoff. The only uncertainty is whether the
integration gate should also be adjusted to align with artifact-based
handoff, which can be validated after implementation.
