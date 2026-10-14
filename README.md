# Genco Finance

A shared product research and profitability web app based on [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md). It has no accounts: people accessing an installation share products, images, and settings.

## Run locally

Requires Go 1.24+ and Node 22+. The development setup is two ordinary processes: the Go API and the Vite frontend. No database is needed; the API writes `data/state.json` by default.

Install frontend dependencies once:

```sh
cd frontend && npm ci
```

Copy the environment examples:

```sh
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env
```

Start the API in one terminal from `backend/`:

```sh
cd backend
go run ./cmd/server
```

Start the frontend in a second terminal from `frontend/`:

```sh
cd frontend
npm run dev
```

Open the Vite URL printed in the frontend terminal, usually **http://127.0.0.1:5173**. Vite proxies `/api` requests to the Go API at `http://127.0.0.1:8080`.

To use MySQL, set `GENCO_MYSQL_DSN` in `backend/.env` and apply migrations once with `GENCO_MIGRATION_DSN`:

```sh
cd backend
go run ./cmd/migrate
```

The existing `make run` command remains available for a production-like build and serves the compiled frontend from port 8080. `make dev-backend`, `make dev-frontend`, and `make migrate` are shortcuts for the commands above.

This workspace already includes installed dependencies and built release artifacts. With the tools installed on this machine:

```sh
GENCO_DATA_DIR="$PWD/data" GENCO_FRONTEND_DIR="$PWD/frontend/dist" ./bin/genco-finance
```

The agent execution environment prevents network sockets and Chromium from starting. Run the command above from a normal terminal. There is no running preview server in the restricted environment.

## Included workflows

- Search, filter, sort, select, duplicate, archive, restore, and export saved evaluations.
- One editable research form for manual and imported products. Add products or import Excel rows, switch product/SKU in the dropdown, edit independent financial inputs and market evidence, and choose which evaluations to save.
- Five editable channel presets, category returns, individual overrides, saved assumptions, revision history, optimistic concurrency, and explicit recalculation.
- Up to eight competitors, market summaries and explicit price-range adoption; up to five shared normalized screenshots with thumbnails and full previews.
- English, Indonesian, and Simplified Chinese, with English as the initial language.
- Excel template/upload/worksheet mapping/preview/selection loads selected rows as editable research drafts. The legacy import URL opens this workflow in Product research. The import API also supports idempotent commits, durable jobs, and restart recovery.
- Combined Excel reports with all five planned sheets and protected summary CSV exports. Selections and saved revisions are frozen at export creation. Temporary workbooks, previews, and reports expire after 24 hours.
- MySQL migrations, runtime/migration/backup credentials, systemd/Nginx configuration, and backup/rollback/restore instructions.

## Financial model

The Go calculation package is authoritative. Monetary values and percentages use **exact rational arithmetic** internally and decimal strings over the API. Required prices/order counts round upward; affordable cost ceilings round downward. This implements the plan's decimal-precision requirement without depending on an unavailable decimal package.

Returns reduce successful-sale revenue, product costs, platform/affiliate/promotion fees, and collected shipping. Advertising, logistics, settlement, and packaging apply to every order. Returned inventory is recoverable. Margin is monthly operating profit divided by gross product GMV. The waterfall includes returned revenue and shipping income and reconciles with profit.

All results are estimated operating profit **before tax**; PPN/PPh are excluded. Each product represents an independent allocation of fixed costs. Product profits are never summed into a portfolio total.

The supplied demo contains four channels. Its original presets are retained. The consignment preset starts at **25% commission and zero other costs**, because the Team Edition source was not supplied; confirm or replace this editable assumption before using it. Category defaults start at general = channel preset, fashion = 10%, beauty = 3%, electronics = 5%, home = 3%; these are shared, editable starting assumptions.

## Structure and contracts

```text
backend/       Go API, calculation engine, Excel processing, persistence and jobs
frontend/      Vue 3, TypeScript, Vite, Pinia, Vue Router, Vue I18n and ECharts
api/           OpenAPI 3.0 contract
deploy/        systemd, Nginx, credentials examples, backup and server runbook
scripts/       local run and API/type generation tools
demo/          supplied reference calculator; never loaded by the application
```

`make contract` regenerates OpenAPI and frontend API types from `scripts/build-contract.py`. Types are consumed by the interface. `backend/sqlc.yaml` and its query source allow regenerating the state query wrappers with sqlc. Migrations use Goose's format and tracking table; the included `genco-migrate` binary applies them without a separately installed Goose command.

The interface uses native controls and custom CSS instead of PrimeVue. The build uses the locally available ECharts 4.9 package. Go query wrappers use `database/sql`; sqlc regeneration is optional. These choices keep this offline workspace buildable. The original HTML is preserved for reference, including its original markup, but none of its third-party scripts are used by the app.

The current MySQL store commits a durable installation snapshot and normalized product/revision/preset/job/image/competitor tables together. A single service process owns mutations and leases; run one instance. The snapshot makes local mode and job restart recovery consistent. This is suitable for the planned single installation; large datasets would benefit from incremental SQL writes and server-side pagination rather than loading the shared state into memory.

## Verification

```sh
make test
```

Tests cover the original financial examples, all channels and categories, threshold substitution back into the profit engine, returns/shipping/packaging, boundaries, waterfall reconciliation, validation, saved versions, stale updates, archived SKU uniqueness, durable restart, selected imports, duplicate/formula/invalid rows, frozen exports, numeric cells, CSV formula protection, and shared image access.

Browser tests require a normal environment with Chromium and a separate service using disposable data:

```sh
GENCO_ADDR=127.0.0.1:8081 GENCO_DATA_DIR=/tmp/genco-browser-test \
  GENCO_FRONTEND_DIR="$PWD/frontend/dist" GENCO_PUBLIC_URL=http://127.0.0.1:8081 \
  ./bin/genco-finance
# In a second terminal:
cd frontend
npx playwright install chromium
npm run test:e2e
```

The suite exercises the calculator, saved product, guided research, revisions, screenshots from a second browser context, exports, search, all three languages, mobile width, and Excel upload/mapping/preview. Screenshots are written into `artifacts/`. API and calculation tests run without network listeners. Browser execution, MySQL integration, and an actual deployment/restore drill require an unrestricted host; they have not been verified inside this workspace.

## Deploy

Follow [deploy/RUNBOOK.md](deploy/RUNBOOK.md) for a clean Ubuntu/Debian server, HTTPS, explicit migration, release rollback, daily backup, and restoration. No production service or external database has been changed by this implementation.
