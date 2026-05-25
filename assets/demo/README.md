# `assets/demo/` — recorded operator demos

## What lives here

| File | Source | Re-render command | Used by |
|---|---|---|---|
| `demo.cast` | `scripts/record-demo.sh` | `bash scripts/record-demo.sh` | README.md embed, asciinema.org upload, landing/ |
| `demo.sh.log` | Side-effect of `record-demo.sh` | (auto) | Reviewers diffing timing |
| `.gitkeep` | Bootstrap | n/a | Ensures the folder ships even when the cast hasn't been recorded yet |

## How to (re)record

1. Build a fresh binary:

   ```bash
   make build
   ```

2. Resize your terminal to **exactly 120×35** (the script refuses any other size — Bento Ultra needs this framing to stay reproducible).

3. Run the recorder:

   ```bash
   bash scripts/record-demo.sh
   ```

   The script:
   - Validates `asciinema`, `expect`, and `./bin/webox` are present.
   - Drives a deterministic 45-60 s `expect` scenario (Tab tour → project detail → CI/CD modal → Live Logs → Topology Map → quit).
   - Writes `assets/demo/demo.cast` (asciinema 2.x format) and `assets/demo/demo.sh.log` (the literal keystroke script played).

4. Verify locally:

   ```bash
   asciinema play assets/demo/demo.cast
   ```

5. Upload to asciinema.org and update the README badge URL:

   ```bash
   asciinema upload assets/demo/demo.cast
   ```

6. Capture the static PNG fallback for SEO/OG previews:

   ```bash
   bash scripts/capture-screenshot.sh
   ```

## Git policy

- Casts **under 100 kB** are committed directly. The reference 45-second cast typically lands at 30-60 kB.
- Casts **over 100 kB** must be tracked via Git LFS (`git lfs track "assets/demo/*.cast"`).
- Never commit `.cast` files from ad-libbed recordings — always re-run the script so the timings stay scripted.
