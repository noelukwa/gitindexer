GOOSE := $(shell command -v goose 2> /dev/null)
SQLC := $(shell command -v sqlc 2> /dev/null)

.PHONY: manager-migration manager-store-queries check-goose check-sqlc

manager-migration: check-goose
	@read -p "enter migration name: " name; \
	goose -dir internal/manager/repository/postgres/migrations create $${name} sql && goose -dir internal/manager/repository/postgres/migrations fix

manager-store-queries: check-sqlc
	@echo "running sqlc..."
	@sqlc generate -f configs/manager.sqlc.yaml

check-goose:
ifndef GOOSE
	@echo "goose is not installed. Installing..."
	@go install github.com/pressly/goose/v3/cmd/goose@latest
else
	@echo "goose is installed."
endif

check-sqlc:
ifndef SQLC
	@echo "sqlc is not installed. Installing..."
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
else
	@echo "sqlc is installed"
endif


install_swag:
	@command -v swag >/dev/null 2>&1 || { echo >&2 "swagger is not installed. Installing..."; go install github.com/swaggo/swag/cmd/swag@latest; }

manager-docs: install_swag
	swag init -g  cmd/manager/main.go -o docs/swagger -ot yaml	