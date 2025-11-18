# CaddyProxyManager+

CaddyProxyManager+ is a modern web UI and management layer that brings Nginx Proxy Manager-style simplicity to Caddy, with extra security add-ons (CrowdSec, WAF, SSO, etc.).

This repository is the project scaffold and planning workspace.

Quick links
- Project board: https://github.com/users/Wikid82/projects/7
- Issues: https://github.com/Wikid82/CaddyProxyManagerPlus/issues

Getting started
1. Pick a stack (Go / Python / Node). This scaffold uses Python examples; adapt as needed.
2. Install development dependencies:

```bash
python -m pip install --upgrade pip
pip install -r requirements.dev.txt
```

3. Install pre-commit hooks:

```bash
pip install pre-commit
pre-commit install
pre-commit run --all-files
```

	The `pre-commit` configuration now includes a `python compile check` hook (backed by `python -m compileall`) so syntax errors are caught locally before hitting CI.

Development notes
- Branching model: `development` is the main working branch; create `feature/**` branches from `development`.
- CI enforces lint and coverage (75% fail-under) in `.github/workflows/ci.yml`.

Contributing
- See `CONTRIBUTING.md` (coming soon) for contribution guidelines.

License
- This project is released under the MIT License - see `LICENSE`.
