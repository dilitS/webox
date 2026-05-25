# `assets/screenshots/` — static cockpit screenshots

## What lives here

| File | Frame | Render command | Used by |
|---|---|---|---|
| `dashboard.png` | `demo.cast` @ t=8s (cockpit at rest, Bento Ultra) | `bash scripts/capture-screenshot.sh` | README.md hero, GitHub social preview |
| `wizard.png` | Manual capture — new-project wizard, step 3 (subdomain + Node version) | Manual screenshot — see `capture-screenshot.sh` | docs/UX.md, landing/, twitter cards |
| `.gitkeep` | Bootstrap | n/a | Folder lifeline before first capture |

## How to (re)capture `dashboard.png`

```bash
bash scripts/capture-screenshot.sh
```

The script prefers [`agg`](https://github.com/asciinema/agg) (best fidelity) and falls back to printed manual instructions if `agg` is not installed.

## How to capture `wizard.png`

1. Resize terminal to **120×35**.
2. Run `./bin/webox --mock`.
3. Press `Ctrl+N` to open the new-project wizard.
4. Step through to the **subdomain + Node version** page.
5. Take a screenshot of the entire terminal window (macOS: ⌘⇧4 · GNOME: `gnome-screenshot -w`).
6. Save as `assets/screenshots/wizard.png` (≥ 1280×800).

## Git policy

- PNGs are committed directly. They should stay under 500 kB (use `pngquant --quality 80-95 in.png` if larger).
- If a screenshot ever exceeds 1 MB, switch to Git LFS:

  ```bash
  git lfs track "assets/screenshots/*.png"
  git add .gitattributes
  ```
