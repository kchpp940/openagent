.PHONY: check-config build lint

check-config:
	@bash scripts/check_config_drift.sh

build:
	@go build ./...

lint:
	@cd web && npx eslint src/ConfigForm.js src/SiteEditPage.js src/backend/SystemInfo.js
