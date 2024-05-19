MIGRATION_NAME?=$(shell bash -c 'read -p "Migration name: " migration_name; echo $$migration_name')

.PHONY: lint
lint:
	golangci-lint run .

.PHONY: create-migration
create-migration:
	migrate create -dir migrations -ext sql ${MIGRATION_NAME}

.PHONY: apply-migrations
apply-migrations:
	migrate -database "${DSN}" -path migrations up
