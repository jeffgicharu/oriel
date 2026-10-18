# Oriel

The sales website and sales desk for Oriel, eighty-six homes in Riverside, Nairobi.

Visitors explore the building in 3D, find a home, see how the light reaches it through the day, price it on each payment plan, book a viewing and request a reservation. The sales team runs the stock, prices, leads, viewings and reservations from the desk at `/desk`.

## How it is built

- **Go** serves everything from one static binary: pages, the desk, PDFs and the live-update stream.
- **templ** components render the HTML on the server. **htmx** swaps in fragments for filters, forms and desk actions.
- **three.js** draws the building, its floors and the moving sun in the browser. **GSAP** and CSS view transitions carry the motion.
- **SQLite**, through the pure-Go `modernc.org/sqlite` driver, holds the data. A visitor's first change gives their browser its own copy of the database, so every action stays private to them.
- Styles are hand-written CSS with no framework. Fonts are Mona Sans and Martian Mono, self-hosted.

## Layout

```
cmd/oriel           the binary: serve, version, check-release, desk-links
internal/app        routes, middleware and handlers for the site and the desk
internal/web        templ components, drawings and plans in SVG
internal/store      the database: homes, leads, viewings, reservations, prices, the activity log
internal/copies     each visitor's private copy of the database, and clearing them away
internal/sample     the sample sales history every copy starts from, rebuilt every hour
internal/programme  the construction programme, moving on with the calendar
internal/building   the one description of the building: floors, homes, sizes
internal/floorplan  each home type's rooms, doors and windows
internal/pricing    prices from the list in force
internal/plans      payment schedules on each plan
internal/light      direct sun reaching each home through the year
internal/viewings   the sales suite's opening hours and slots
internal/pdf        the letter of offer and the price list
internal/check      the release gate, run before visitors reach a release
internal/assets     the embedded browser build and root files
internal/config     settings from the environment
web/                TypeScript, CSS, fonts and pictures, built by esbuild
deploy/             server setup, the systemd unit, nginx and the activation script
```

## Building

```sh
cd web && pnpm install && pnpm build && cd ..
go tool templ generate
CGO_ENABLED=0 GOAMD64=v1 go build -trimpath -o oriel ./cmd/oriel
```

`GOAMD64=v1` matters: the production processor only has the baseline x86-64 instructions.

## Releasing

A push to `main` runs `.github/workflows/deploy.yml`:

1. Type-checks and builds the browser code, generates the templates, then vets and tests the Go code.
2. Builds the static binary and walks it with `oriel check-release`.
3. Uploads the release over an SSH key that may only write into the releases directory.
4. Calls `oriel-activate` over a second key that may only run that script. It starts the release beside the live one, walks it again there, switches nginx over, checks the live site, and switches back if anything fails.

The server runs two slots, on ports 8400 and 8401, behind nginx. See `deploy/activate.sh`.

## Settings

| Name | Purpose |
|---|---|
| `ORIEL_ADDR` | Local address to listen on |
| `ORIEL_DATA` | Folder for the databases |
| `ORIEL_SITE_URL` | Public address |
| `ORIEL_SECRET` | Signs a visitor's copy cookie, the confirmations shown after a form, and the release gate's check token |
| `ORIEL_DESK_KEY` | Signs one-click links into the desk |
