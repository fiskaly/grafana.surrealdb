
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
	rm -f ${ASSETS}
	rm -rf ./dist

clean-all: clean
	rm -rf ./node_modules

ASSETS := src/img/logo-small.png
ASSETS += src/img/logo-large.png
ASSETS += src/img/screenshot-config-editor.png
ASSETS += src/img/screenshot-query-editor-raw.png
ASSETS += src/img/screenshot-query-editor-log.png
ASSETS += src/img/screenshot-query-editor-metric.png
ASSETS += src/img/screenshot-query-editor-metric-rate.png
ASSETS += src/img/screenshot-query-editor-metric-group.png

assets: ${ASSETS}

src/img/logo-small.png:
	curl -s -J -L -o $@ \
	https://avatars.githubusercontent.com/u/10982346?s=64&v=4

src/img/logo-large.png:
	curl -s -J -L -o $@ \
	https://avatars.githubusercontent.com/u/10982346?s=196&v=4

src/img/screenshot-config-editor.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/8f4b0e53-e5b9-4abb-80a4-612c1291c3c5

src/img/screenshot-query-editor-raw.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/23e374ed-f94e-413e-9d6b-5ff600fb11e3

src/img/screenshot-query-editor-log.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/d3f90a5a-b905-46f6-b52c-5ab1bdca360b

src/img/screenshot-query-editor-metric.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/ff435b1f-4e58-43bb-acd2-b7bd9ea3a392

src/img/screenshot-query-editor-metric-rate.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/bbb8def8-be10-42c2-9779-9cd747ff5620

src/img/screenshot-query-editor-metric-group.png:
	curl -s -J -L -o $@ \
	https://github.com/fiskaly/grafana.surrealdb/assets/6830431/9e819cec-cd70-4530-b3ef-f7bca156c3c1
