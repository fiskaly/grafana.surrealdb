
TARGET := fiskaly-surrealdb-datasource

default: all

all: build
all: local

.attic:
	mkdir -p $@

build: .attic
build: assets
	mage -v build:linux
ifndef DEVELOPMENT
	npm run build
endif

test:
	npm run test:ci

dev:
	npm run dev

.attic/surrealdb:
	mkdir -p $@
	chmod -R 777 $@

local: .attic/surrealdb
local: build
	docker-compose up

local-clean:
	docker-compose down --remove-orphans

sign:
	npx @grafana/sign-plugin@latest

check:
	npx @grafana/levitate@latest is-compatible --path src/module.ts --target @grafana/data,@grafana/ui,@grafana/runtime

clean:
	rm -rf ./dist

clean-all: clean
	rm -rf ./node_modules

ASSETS := src/img/logo-small.png
ASSETS += src/img/logo-large.png
ASSETS += src/img/screenshot-config-editor.png
ASSETS += src/img/screenshot-query-editor-raw.png
ASSETS += src/img/screenshot-query-editor-log.png
ASSETS += src/img/screenshot-query-editor-metric.png

assets: ${ASSETS}

src/img/logo-small.png:
	curl -s -J -L -o $@ \
	https://avatars.githubusercontent.com/u/10982346?s=64&v=4

src/img/logo-large.png:
	curl -s -J -L -o $@ \
	https://avatars.githubusercontent.com/u/10982346?s=196&v=4

src/img/screenshot-config-editor.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/01590b63-c23e-4915-b690-093fb796e8dd

src/img/screenshot-query-editor-raw.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/2ee98536-636a-4690-8856-65ea0d60b96c

src/img/screenshot-query-editor-log.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/e32f9ff9-bfac-4ae6-b037-fbf44c6ae9ec

src/img/screenshot-query-editor-metric.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/4d12d31c-4033-4c6d-b6d1-0096a7cabc37
