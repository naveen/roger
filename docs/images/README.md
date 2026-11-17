# Images

Renders of the concept, used by the top-level README and the design notes. None of these is a photograph: no radio has been built yet.

| File | What |
|---|---|
| `site-hero.png`, `site-how-it-works.png`, `site-heard-on-the-air.png`, `site-for-grown-ups.png` | Sections of the concept page in `public/`, at 2x |
| `site-full.png` | The whole concept page, at 1x |
| `model-front.png`, `model-three-quarter.png`, `model-side.png`, `model-back.png`, `model-top.png` | The concept model in `public/model.html` from fixed camera angles |
| `model-turnaround.png` | Front, three-quarter, side and back in one strip |

They are made by `capture.mjs`, which loads the page from a local server in headless Chrome and screenshots it. To regenerate after a change to the page or the model:

```
(cd public && python3 -m http.server 8765 --bind 127.0.0.1) &
cd docs/images && npm i --no-save playwright && node capture.mjs
```

The pictures are CC-BY-4.0 like the rest of `docs/`.
