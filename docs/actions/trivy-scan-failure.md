**Status**: ✅ RESOLVED (January 30, 2026)

https://github.com/Wikid82/Charon/actions/runs/21503634925/job/61955008214

Run # Normalize image name for reference
🔍 Extracting binary from: ghcr.io/wikid82/charon:feature/beta-release
invalid reference format
Error: Process completed with exit code 1.

The failure is caused by an invalid Docker image reference format when trying to extract the charon binary. In the logs:

invalid reference format

This happens during the construction of the IMAGE_REF environment variable in this step:

bash
if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
  IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ github.event.workflow_run.head_branch }}"
else
  IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
fi
If the PR number is missing or blank, IMAGE_REF becomes invalid (e.g., ghcr.io/wikid82/charon:pr-), which is not a valid tag. The extraction then fails.

Solution:

Add a check to ensure steps.pr-info.outputs.pr_number is set before constructing IMAGE_REF for PRs. If it’s missing/empty, exit or skip with a clear message.
Suggested code improvement for the “Extract charon binary from container” step:

YAML
- name: Extract charon binary from container
  if: steps.check-artifact.outputs.artifact_exists == 'true'
  id: extract
  run: |
    IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
    if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
      IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ github.event.workflow_run.head_branch }}"
    else
      if [[ -z "${{ steps.pr-info.outputs.pr_number }}" ]]; then
        echo "❌ PR number missing, cannot form Docker image reference."
        exit 1
      fi
      IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
    fi
    echo "🔍 Extracting binary from: ${IMAGE_REF}"
    ...
This ensures the workflow does not attempt to use an invalid image tag when the PR number is missing. Adjust similar logic throughout the workflow to handle missing variables gracefully.
## Resolution

Fixed by adding proper validation for PR number before constructing Docker image reference, ensuring IMAGE_REF is never constructed with empty/missing variables. Branch name sanitization also implemented to handle slashes in feature branch names.
