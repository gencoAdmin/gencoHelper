GO ?= go
NPM ?= npm

.PHONY: install build test run dev dev-backend dev-frontend migrate contract e2e
install:
	cd frontend && $(NPM) ci
build:
	cd frontend && $(NPM) run build
	cd backend && $(GO) build -buildvcs=false -trimpath -o ../bin/genco-finance ./cmd/server
	cd backend && $(GO) build -buildvcs=false -trimpath -o ../bin/genco-migrate ./cmd/migrate
test:
	cd backend && $(GO) test -race ./...
	cd frontend && $(NPM) run test:render
	cd frontend && $(NPM) run build
run: build
	GENCO_DATA_DIR=$(CURDIR)/data GENCO_FRONTEND_DIR=$(CURDIR)/frontend/dist ./bin/genco-finance
dev-backend:
	cd backend && $(GO) run ./cmd/server
dev-frontend:
	cd frontend && $(NPM) run dev
dev: dev-frontend
migrate:
	cd backend && $(GO) run ./cmd/migrate
contract:
	python3 scripts/build-contract.py
e2e:
	cd frontend && $(NPM) run test:e2e
