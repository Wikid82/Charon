# Debugging the Local Docker Image

Use the `cpmp:local` image as the source of truth and attach VS Code debuggers directly to the running container.

## 1. Enable the debugger
The image now ships with the Delve debugger. When you start the container, set `CPMP_DEBUG=1` (and optionally `CPMP_DEBUG_PORT`) so CPM+ runs under Delve.

```bash
docker run --rm -it \
  --name cpmp-debug \
  -p 8080:8080 \
  -p 2345:2345 \
  -e CPM_ENV=development \
  -e CPM_DEBUG=1 \
  cpmp:local
```

Delve will listen on `localhost:2345`, while the UI remains available at `http://localhost:8080`.

## 2. Attach VS Code
- Use the **Attach to CPMP backend** configuration in `.vscode/launch.json` to connect the Go debugger to Delve.
- Use the **Open CPMP frontend** configuration to launch Chrome against the management UI.

These launch configurations assume the ports above are exposed. If you need a different port, set `CPMP_DEBUG_PORT` when running the container and update the Go configuration's `port` field accordingly.
