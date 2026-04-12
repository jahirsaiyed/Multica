#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo "==> Generating repomix packs..."
cd "$REPO_ROOT"
pnpm repomix:all
echo "==> Done."
ls -lh "$REPO_ROOT/kb/"*.xml 2>/dev/null || echo "  (no XML packs — check repomix output)"
