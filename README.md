# il-supermarket-compare

Compare the price of a shopping list across the supermarkets nearest an Israeli
address, using the chains' own published price files (חוק קידום התחרות בענף המזון).

- **Backend:** Go 1.23 + SQLite (pure Go driver, no CGO), single static binary.
- **Frontend:** TypeScript + Vite, no framework, RTL Hebrew UI.
- **Data:** fetched live from the chains' transparency portals, then cached in
  SQLite. A daily job refreshes the store list and coordinates.

The whole app is one process: the Go server serves the API and the built
frontend. It also runs on its own as a static site with a remote API.

## Why SQLite (and not "no database")

The store list and the geocode cache are small (a few thousand rows), read-mostly
and fully regenerable, so a single SQLite file is the right amount of database:
no server, no migrations, no hosting cost, and it survives restarts on a
persistent volume. A managed Postgres/MySQL would add an external dependency and
cost for no benefit at this size. If you deploy to a host with no persistent
disk, set `DB_PATH` to a path on a mounted volume (Fly, Render and Docker Compose
configs below all do this).

## Quick start (local)

```bash
# 1. backend + data
go run ./cmd/ingest --db data/prices.db        # first run: a few minutes (geocoding)
go run ./cmd/server                            # http://localhost:8787

# 2. frontend (dev, with hot reload and API proxy)
cd web && npm install && npm run dev           # http://localhost:5173
```

For a production-style run, build the frontend once and let the Go server serve
it:

```bash
cd web && npm ci && npm run build && cd ..
go run ./cmd/server --db data/prices.db --static web/dist
```

The first `/api/compare` for a new address downloads that day's price files for
the nearby branches (a few seconds to about a minute) and caches them; repeat
queries for the same day are instant.

## How it works

1. `GET /api/compare` geocodes the address with OpenStreetMap Nominatim,
   falling back to Photon (komoot) when Nominatim rate-limits or misses. Results
   are cached permanently in SQLite; both providers are rate-limited to about
   1 req/s.
2. Branches within the radius are selected: every discount branch (priced per
   branch) plus the nearest branch of each other banner.
3. For each selected branch, the day's full price file is downloaded, decoded
   (the publishedprices portal serves UTF-16LE with a BOM), matched against the
   catalogue, and cached for the day.
4. The basket is priced per branch. Three numbers are reported:
   - **cheapest basket** — the lowest full total;
   - **like-for-like** — the total over only the items every compared branch has;
   - **cherry-pick** — the sum of each item's lowest price anywhere.

### Price portals

| Chain(s) | Portal |
| --- | --- |
| שופרסל (Yesh Hesed, Universe, Deal, Express, Sheli) | `prices.shufersal.co.il` |
| קרפור | `prices.carrefour.co.il` |
| רמי לוי, אושר עד, יוחננוף, טיב טעם, קשת, דור אלון, פרשמרקט, סאלח דבאח, סטופ מרקט, פוליצר | `url.publishedprices.co.il` (public per-chain login, empty password) |

### Matching

Products are matched to the catalogue by Hebrew-name regex (`internal/catalog`).
This is the fuzzy part: chain names are abbreviated (`תפו"א`, `ג.צהובה`) and the
same item is spelled differently per chain. The catalogue keeps patterns loose
and excludes obvious non-comparable variants (snacks, sauces, pet food). Weighted
items are compared per kilogram; pack items prefer a standard pack size.

## API

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/health` | liveness and store count |
| `GET` | `/api/meta` | catalogue, chains, data freshness |
| `GET` | `/api/stores?lat=&lon=&radius=` | branches in a radius |
| `POST` | `/api/compare` | `{address, radiusKm, items:[{itemId, qty}]}` |

## Deploy

The app is a normal container. Any container host runs the whole thing:

```bash
docker compose up --build        # http://localhost:8080
```

Included configs: `Dockerfile`, `docker-compose.yml`, `fly.toml` (Fly.io),
`render.yaml` (Render), and `deploy/cloud-run.sh` (Google Cloud Run).

### Vercel, Cloudflare and GitHub Pages

These hosts run static frontends and serverless functions, not a long-lived Go
process with a SQLite file. Two supported ways to use them:

1. **Static UI + hosted API (recommended).** Deploy `web/` as a static site and
   point it at the Go API running on any container host:

   ```bash
   cd web
   VITE_API_BASE="https://your-api.example.com" npm run build
   ```

   - Vercel: set the project root to `web/`, output `dist`, env `VITE_API_BASE`.
   - Cloudflare Pages: build command `npm ci && npm run build`, output `web/dist`,
     env `VITE_API_BASE`.
   - GitHub Pages: the **Data refresh** workflow builds and deploys `web/dist`
     with `VITE_API_BASE` taken from the repository variable `API_BASE`.

   The API sends `Access-Control-Allow-Origin: *`, so cross-origin use works.

2. **GitHub Pages with data baked in.** The **Data refresh** workflow runs
   `cmd/ingest --snapshot web/public/data`, which writes `stores.json` and
   `meta.json` into the built site. That mode has no live prices; it is useful as
   a store map/directory and for the daily data update itself.

Cloudflare Workers/D1 cannot run this Go binary, so on Cloudflare use option 1
(static UI on Pages, API elsewhere).

## Scheduled data update

`.github/workflows/data-refresh.yml` runs daily (and on demand):

- ingests the store list and geocodes it,
- writes the static snapshot,
- builds and deploys the UI to GitHub Pages.

`cmd/server` also self-refreshes daily in-process (`SELF_REFRESH=1`, the
default), so a container host needs no external scheduler.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `DB_PATH` | `data/prices.db` | SQLite file |
| `ADDR` | `:8787` | listen address |
| `STATIC_DIR` | `web/dist` | frontend build directory |
| `SELF_REFRESH` | `1` | daily in-process store refresh |
| `VITE_API_BASE` | *(empty)* | frontend build-time API base URL |

## Tests

```bash
go test ./...            # decoding, XML parsing, matching, basket maths, geo
cd web && npm run build  # type-check + bundle
```

## Limitations

- Product matching is name-based and will occasionally pick the wrong variant.
  The per-store line detail shows the exact product used, so it is auditable.
- Weighted items are compared at list price per kg; promotions and club prices
  are not modelled.
- Nominatim is rate-limited and occasionally misses a street; those branches fall
  back to the city centre and are flagged "approximate" in the UI.
