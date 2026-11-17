# What is in this folder

The Roger concept page, unpacked from its Claude Design export into plain files so it can be served by any static host. Nothing here is fetched from a CDN.

| File | What it is | Licence |
|---|---|---|
| `index.html` | The page: markup, styles and the small script that drives the headline and the waitlist form | Yours |
| `model.html` | The 3D radio shown in the hero, loaded in a frame | Yours |
| `assets/dc-runtime.js` | Claude Design's page runtime. It turns the markup in `index.html` into the live page. | Part of the Claude Design export; no licence stated in the file |
| `assets/three-d-stage.js` | Claude Design's 3D viewer element | Same |
| `assets/react.production.min.js`, `assets/react-dom.production.min.js` | React 18.3.1 | MIT |
| `assets/three/` | three.js 0.184.0 (core, OrbitControls, OBJ and GLTF exporters) | MIT, see `assets/three/LICENSE` |
| `assets/fonts/` | Bricolage Grotesque and Instrument Sans, as served by Google Fonts | SIL Open Font License 1.1, see `assets/fonts/OFL-1.1.txt` |

The two Claude Design files are why this folder should sit outside the open-source licences that cover the rest of the repository.

## Changes from the export

- Asset references point at files in `assets/` instead of data embedded in the page.
- React and three.js are served from `assets/` instead of unpkg. The three.js files are byte-identical to the versions the export pinned.
- A `<title>`, a meta description and `lang="en"` were added to `index.html`.

## Known limits

- The waitlist form does not send the email anywhere. Submitting it only shows the thank-you line.
- The page is rendered in the browser by the runtime, so it needs JavaScript.
