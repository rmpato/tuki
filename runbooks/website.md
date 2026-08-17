# Change the website

The site is <https://rmpato.github.io/tuki/>. It's four static files in
`docs/`, served by GitHub Pages straight off `main`. There is no build step
and no dependencies.

| file | what it is |
| --- | --- |
| `docs/index.html` | all the content |
| `docs/style.css` | all the styling |
| `docs/demo.js` | the interactive demo, the nav bar, tabs, copy button |
| `docs/screenshot.png` | see [screenshot.md](screenshot.md) |

## Edit and preview

```sh
cd docs
python3 -m http.server 8000
open http://localhost:8000
```

Pages is configured as branch `main`, folder `/docs`. Pushing to `main`
redeploys; it takes under a minute.

```sh
git add docs && git commit -m "..." && git push origin main
gh api repos/rmpato/tuki/pages/builds/latest --jq '{status, error: .error.message}'
```

## Check before pushing

- **Both themes.** The palette is CSS variables at the top of `style.css`,
  redefined under `@media (prefers-color-scheme: light)`. Change one, change
  the other.
- **Narrow widths.** Nothing may overflow horizontally. Quickest check, pasted
  into the browser console:

  ```js
  document.documentElement.scrollWidth <= document.documentElement.clientWidth
  ```

  Wide things (`<pre>`, the screenshot) must scroll inside their own box
  rather than widening the page.
- **The demo still runs.** Click it, press `↓` and `space`; a task should
  cross out and the counter should move.
- **The nav.** Every link in `.topbar-links` needs a matching section `id`.

## Gotchas

**`.wrap` sets `padding-inline`, not `padding`.** Several sections *are*
`.wrap` elements, so a `padding` shorthand there silently flattens the
vertical rhythm of every section at once.

**Grid children need `min-width: 0`.** Otherwise a wide `<pre>` pushes out of
its column instead of scrolling (`.two-up` does this already).

**Animations don't run in a background tab.** If you're testing through
automation and smooth scrolling or the celebration seems dead, check
`document.visibilityState` before assuming the code is broken.

**No external requests.** Fonts are system stacks and there's no CDN. Keep it
that way — the page should load instantly and work offline.

## Rollback

```sh
git revert <commit> && git push origin main
```
