
**Status**: ✅ RESOLVED (January 30, 2026)

## Summary

The nightly build failed during the GoReleaser release step while attempting
to cross-compile for macOS.

## Failure details

Run link:
[GitHub Actions run][nightly-run]

Relevant log excerpt:

```text
release failed after 4m19s
error=
  build failed: exit status 1: go: downloading github.com/gin-gonic/gin v1.11.0
  info: zig can provide libc for related target x86_64-macos.11-none
target=darwin_amd64_v1
The process '/opt/hostedtoolcache/goreleaser-action/2.13.3/x64/goreleaser'
failed with exit code 1
```

## Root cause

GoReleaser failed while cross-compiling the darwin_amd64_v1 target using Zig
to provide libc. The nightly workflow configures Zig for cross-compilation,
so the failure is likely tied to macOS toolchain compatibility or
dependencies.

## Recommended fixes

- Ensure go.mod includes all platform-specific dependencies needed for macOS.
- Confirm Zig is installed and available in the runner environment.
- Update .goreleaser.yml to explicitly enable Zig for darwin builds.
- If macOS builds are not required, remove darwin targets from the build
  matrix.
- Review detailed logs for a specific Go or Zig error to pinpoint the failing
  package or build step.

## Resolution

Fixed by updating `.goreleaser.yml` to properly configure Zig toolchain for macOS cross-compilation and ensuring all platform-specific dependencies are available.

## References

- .github/workflows/nightly-build.yml
- .goreleaser.yml

[nightly-run]:
  https://github.com/Wikid82/Charon/actions/runs/21503512215/job/61955865462
