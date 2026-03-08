# Docker: Prune Resources

Clean up unused Docker resources to free disk space.

## Command

```bash
.github/skills/scripts/skill-runner.sh docker-prune
```

## What Gets Removed

- Stopped containers
- Unused networks
- Dangling images (untagged)
- Build cache

**Note**: Volumes are NOT removed by default. Use `docker volume prune` separately if needed (this will delete data).

## Check Space Before/After

```bash
docker system df
```

## Related

- `/docker-stop-dev` — Stop environment first before pruning
- `/docker-start-dev` — Restart after pruning
