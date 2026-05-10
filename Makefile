SHELL := /bin/bash

.PHONY: test backend-test backend-vet user-install admin-install user-build admin-build compose-config build verify

test: backend-test

backend-test:
	cd yuexiang-image-backend && go test ./...

backend-vet:
	cd yuexiang-image-backend && go vet ./...

user-install:
	cd yuexiang-image-user-web && npm ci

admin-install:
	cd yuexiang-image-admin-web && npm ci

user-build:
	cd yuexiang-image-user-web && npm run build

admin-build:
	cd yuexiang-image-admin-web && npm run build

compose-config:
	docker compose -f docker-compose.full.yml config >/tmp/yuexiang-compose-full.out
	docker compose -f yuexiang-image-backend/docker-compose.prod.yml config >/tmp/yuexiang-compose-prod.out

build: user-install user-build admin-install admin-build

verify: backend-test backend-vet user-install user-build admin-install admin-build compose-config
