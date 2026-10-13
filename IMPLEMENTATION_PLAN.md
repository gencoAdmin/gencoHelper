# Genco Product Profitability Web App

## 1. Product and architecture

Build a shared product evaluation app combining both demos: research products, calculate profitability, save evaluations to a searchable list, import products from Excel, and export multiple evaluations together.

**Confirmed decisions**

- No accounts or login. Everyone accessing the app can manage products and settings.
- Include competitor research, screenshots, market price ranges, categories, and all five channels.
- Shared presets and individual calculation rates are editable.
- Support English, Indonesian, and Simplified Chinese; default to English.
- Each product has its own monthly fixed-cost allocation. The list represents independent scenarios and does not sum their profits.
- Imports create new products and skip duplicate SKUs.
- Deploy on one Ubuntu/Debian server without Docker.

**Architecture**

```text
Browser: Vue frontend
        ↓ HTTPS
Nginx: static frontend + /api reverse proxy
        ↓ localhost
Go service: API + calculations + background jobs
        ↓
MySQL                    Local screenshot/report storage
```

**Implementation stack**

| Area | Choice |
|---|---|
| Frontend | Vue 3, TypeScript, Vite, Vue Router, Pinia |
| UI and charts | PrimeVue, Apache ECharts |
| Localization | Vue I18n |
| Backend | Go, standard-library HTTP routing |
| Database access | `database/sql`, MySQL driver, sqlc |
| Migrations | Goose |
| Financial arithmetic | Decimal arithmetic using `shopspring/decimal` |
| Excel processing | Excelize on the backend |
| API contract | OpenAPI with generated TypeScript types |
| Testing | Go tests, Vitest, Playwright |
| Production | systemd, Nginx, MySQL 8 |

Use a monorepo with `backend/`, `frontend/`, and `deploy/`. Preserve the original demo as a reference.

## 2. Application features and workflows

### Product list and saved evaluations

Make the saved list the home screen.

- Search by product name or SKU.
- Filter by category, recommended channel, viability, and archived status.
- Sort by margin, monthly profit, creation date, and last update.
- Show SKU/name, category, evaluated price, cost, expected monthly orders, recommended channel, margin, profit, and viability.
- Support row selection, detailed viewing, editing, duplication, archiving, restoring, and exporting.
- Keep optional SKU unique across active and archived products; allow duplicate product names and multiple products without SKUs.

A product record contains its commercial details, research, and evaluation history. Editing calculation inputs creates a new evaluation revision; earlier revisions remain available.

### Calculator

Provide an unsaved calculator that can become a saved product.

Inputs:

- Product name, optional SKU, category, and notes.
- Product cost and selling price.
- Expected monthly orders.
- Allocated monthly fixed costs.
- Packaging cost per order.
- Target margin, defaulting to 15%.
- Channel rates and per-product overrides.

Results:

- Contribution and operating profit per order.
- Monthly operating profit and margin.
- Expected successful sales.
- Break-even order count and selling price.
- Maximum affordable supplier cost.
- Price buffer.
- Maximum advertising percentage and break-even ROAS.
- Target selling price and supplier cost for the desired margin.
- All five channels compared side by side.
- Cost waterfall and profit-versus-volume chart.
- Plain-language verdict with the reasons and possible improvements.

Recalculate through a debounced backend request. Saving always recalculates on the server.

### Product research

Provide the Team Edition’s guided workflow: **basic information → market research → results and save**.

- Categories: general, fashion, beauty, electronics, and home.
- Optional market minimum and maximum prices.
- Up to eight competitors: platform, shop, price, monthly sales, rating, and optional URL.
- Show competitor price range, average price, total reported sales, and highest-volume shop.
- Offer an explicit action to use competitor prices as the market range.
- Up to five screenshots per product, with thumbnails and full-size preview.
- Accept JPEG, PNG, and WebP, up to 5 MB each; generate normalized images and thumbnails server-side.

For research, recommend the channel with the highest modeled profit at the market minimum price. Show the maximum-price scenario for that same channel. If no market range exists, evaluate the entered selling price.

### Shared settings

- Edit the five channel presets, including the Team Edition’s consignment preset.
- Edit category return-rate defaults and viability thresholds.
- Show where each effective assumption comes from.
- Rate precedence: channel preset → category return-rate override → explicit product/channel override.
- Saving a preset creates a new preset version.
- Existing evaluations retain their original assumptions.
- “Recalculate with current defaults” explicitly creates a new revision; it preserves explicit product overrides.

### Excel import and combined export

**Import flow:** download template → upload `.xlsx` → select worksheet → map columns → preview → select valid rows → confirm → view results.

- Required columns: product name, cost, selling price, and monthly orders.
- Optional columns: SKU, category, fixed costs, packaging, market minimum/maximum price, target margin, and notes.
- Default missing category to general, fixed costs and packaging to zero, and target margin to the shared default.
- Auto-map template headers and recognized translations; allow manual mapping.
- Preserve SKU strings, including leading zeros.
- Validate each row and show its source row number and errors.
- Flag existing SKUs and repeated SKUs within the file; keep the first valid occurrence and skip subsequent duplicates.
- Recheck uniqueness when committing.
- Import selected valid rows and calculate all channels automatically.
- Limit uploads to 10 MB and 5,000 data rows; impose worksheet cell and decompression limits.
- Reject formula cells in mapped input columns rather than relying on cached values.
- Competitors and screenshots are entered through product research in the first release.

Import preview writes no products. Committing is idempotent, runs as a background job, and reports created, skipped, and failed rows.

**Export flow:** choose selected products or all matching filters → choose Excel or summary CSV → download.

The Excel workbook contains:

1. **Summary:** one row per product with headline results and verdict.
2. **Inputs and Rates:** saved assumptions and calculation/preset versions.
3. **Channel Results:** all five channels and their financial breakdowns.
4. **Competitors:** product-linked competitor records.
5. **Images:** product-linked screenshot hyperlinks.

Export each product’s latest saved evaluation; do not silently recalculate. Use numeric spreadsheet cells for financial values and localized labels. Treat user text as text, including protection against CSV formula execution.

Screenshot links point to the app and remain usable while the corresponding product/image exists. Images are not embedded.

## 3. Calculation model and data/API design

### One authoritative calculation engine

All workflows use the same Go calculation package: calculator, research, saving, importing, and exporting.

Keep the demos’ tax-exclusive model. Exclude PPN/PPh calculations from this release and label results as **estimated operating profit before tax**.

Let:

- `Q` = monthly orders; one item per order.
- `s` = successful-sales fraction after returns.
- `P` = selling price.
- `F` = allocated monthly fixed costs.

Calculate:

```text
Successful quantity = Q × s
Product revenue = Q × s × P
Shipping income = Q × s × Customer shipping charge

Contribution = Revenue + Shipping income − Variable costs
Monthly operating profit = Contribution − F
Profit per order = Monthly operating profit ÷ Q
Margin = Monthly operating profit ÷ (Q × P)
```

Apply product costs and platform/affiliate/promotion fees to successful sales. Apply payment fees to successful product revenue plus shipping collected. Apply advertising, logistics, settlement charges, and packaging to all orders.

Returned products are assumed recoverable into inventory without product-value loss.

Derive break-even price, supplier-cost limits, target-margin prices, and advertising limits from this exact equation, including returns and packaging. Use the affine form `contribution per order = A × price + B` to solve thresholds consistently.

- Round required order counts and minimum prices upward.
- Round affordable supplier-cost limits downward.
- Preserve unrounded decimal values internally.
- Return explicit statuses for unattainable or undefined metrics instead of infinity or fabricated zero values.
- Handle zero contribution, zero fixed cost, 100% returns, and excessive combined fees.
- Include shipping income and returned revenue explicitly in the waterfall so it reconciles exactly.
- ROAS uses gross product GMV divided by advertising spend, matching the advertising input basis.

Use four verdicts: healthy, viable, thin margin, and unprofitable. Default thresholds are 15% margin plus a 10% price buffer for healthy, and 8% margin for viable. Explain whether losses result from negative contribution or insufficient volume; identify exact break-even separately in the explanation.

### Database entities

- **Products:** identity, SKU, category, notes, research prices, archive state, timestamps, and edit version.
- **Evaluation revisions:** inputs, effective rates, preset/calculation versions, channel results, recommendation, and timestamp.
- **Competitors:** product-linked research records.
- **Product images:** storage identifiers, metadata, and thumbnail references.
- **Preset versions:** channel/category defaults and grading settings.
- **Jobs:** import/export state, progress, errors, and artifact references.

Use MySQL decimal columns for monetary inputs and rates, foreign keys, and indexes supporting list filters. Store decimal API values as strings to avoid JavaScript precision loss.

Use optimistic concurrency for products and settings: reject stale edits with HTTP 409 and prompt the user to reload.

### API capabilities

Expose versioned `/api/v1` endpoints for:

- Calculation and research evaluation.
- Product listing, creation, editing, duplication, archiving, and restoration.
- Evaluation history and explicit recalculation.
- Competitor and image management.
- Shared settings and preset versions.
- Import template, preview, commit, and job progress.
- Export creation, progress, and download.

Define shared `CalculationInput`, `EffectiveRates`, `ChannelResult`, `Product`, `EvaluationRevision`, and `JobStatus` types in OpenAPI. Standardize field-validation errors.

Use a MySQL-backed job worker inside the Go service, with leases and restart recovery. No Redis or separate worker service is required. Freeze export selections and revision IDs when the export starts.

## 4. Deployment and operations

- Build Vue into static assets and compile Go into a Linux binary.
- Serve Vue through Nginx with SPA route fallback.
- Proxy `/api/` to Go on `127.0.0.1:8080`; serve product images through the API.
- Run Go as a dedicated non-root user under systemd with automatic restart and graceful shutdown.
- Store configuration under `/etc/genco-finance/`, writable data under `/var/lib/genco-finance/`, and release artifacts under `/opt/genco-finance/`.
- Keep MySQL local, using separate runtime and migration credentials.
- Provide a systemd unit, Nginx configuration, environment example, migration command, and deployment/rollback runbook.
- Apply migrations as an explicit deployment step. Use backward-compatible migrations so binary rollback remains practical.
- Configure HTTPS once the production domain is available.
- Add liveness/readiness endpoints, structured logs, request IDs, job failures, and request/job timing.
- Remove the saved demo’s third-party analytics scripts.
- Back up MySQL and image storage daily; document and verify restoration.
- Expire temporary uploads, import previews, and generated reports after 24 hours.

The first release is one shared installation: no tenancy, user administration, marketplace scraping, portfolio accounting, PDF reports, or historical browser-storage migration.

## 5. Delivery order and acceptance tests

| Phase | Deliverable |
|---|---|
| 1 | Project skeleton, migrations, OpenAPI, calculation engine, and financial fixtures |
| 2 | Shared settings, calculator, charts, product persistence, and evaluation history |
| 3 | Research workflow, competitors, screenshots, and three-language interface |
| 4 | Excel template/mapping/preview, background imports, combined Excel/CSV exports |
| 5 | End-to-end verification, systemd/Nginx deployment, backups, and operational documentation |

**Financial acceptance**

- Original marketplace example produces Rp8,000 contribution/order, Rp2,700,000 monthly profit, 5.4% margin, and 663 break-even orders.
- Original TikTok Live example produces −Rp4,125,000 monthly profit and a corrected minimum whole-Rupiah break-even price of Rp56,951.
- Prices and costs calculated for break-even or target margin satisfy the same profit engine.
- Cover all channels, category returns, packaging, shipping income, zero advertising, impossible targets, and boundary conditions.
- Chart breakdowns reconcile with reported profit.

**Workflow acceptance**

- Manual and Excel-created products with identical inputs produce identical results.
- Saved products and screenshots are visible from another browser.
- Preset changes leave saved results unchanged until explicit recalculation.
- Stale edits cannot overwrite newer changes.
- Imports handle invalid rows, duplicate SKUs, leading-zero SKUs, Unicode, formulas, repeated confirmation, and service restart.
- Exports respect selection/filter scope and reproduce the saved evaluations across every workbook sheet.
- All core flows work in three languages and on desktop/mobile.
- A clean server can be deployed from the runbook, survive reboot, and restore products plus screenshots from backup.
