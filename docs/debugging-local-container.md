# Debugging the Local Docker Image

Use the `charon:local` image as the source of truth and attach VS Code debuggers directly to the running container. Backwards-compatibility: `cpmp:local` still works (fallback).

## 1. Enable the debugger
The image now ships with the Delve debugger. When you start the container, set `CHARON_DEBUG=1` (and optionally `CHARON_DEBUG_PORT`) to enable Delve. For backward compatibility you may still use `CPMP_DEBUG`/`CPMP_DEBUG_PORT`.

```bash
docker run --rm -it \
  --name charon-debug \
  -p 8080:8080 \
  -p 2345:2345 \
  -e CHARON_ENV=development \
  -e CHARON_DEBUG=1 \
  charon:local
```

Delve will listen on `localhost:2345`, while the UI remains available at `http://localhost:8080`.

## 2. Attach VS Code
 - Use the **Attach to Charon backend** configuration in `.vscode/launch.json` to connect the Go debugger to Delve.
 - Use the **Open Charon frontend** configuration to launch Chrome against the management UI.

These launch configurations assume the ports above are exposed. If you need a different port, set `CHARON_DEBUG_PORT` (or `CPMP_DEBUG_PORT` for backward compatibility) when running the container and update the Go configuration's `port` field accordingly.
