.PHONY: compose-build
compose-build:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose --profile release build

.PHONY: compose-build-debug
compose-build-debug:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose --profile debug build

.PHONY: compose-mongo
compose-mongo:
	docker-compose start mongo

.PHONY: compose
compose:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose up --detach --build app

.PHONY: compose-debug
compose-debug:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose up --detach --build app-debug

.PHONY: compose-all
compose-all:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose --profile release up --detach --build

.PHONY: compose-all-debug
compose-all-debug:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose --profile debug up --detach --build

.PHONY: test
test:
	docker-compose -f docker-compose.test.yml up --build app-test

.PHONY: build
build:
	DOCKER_BUILDKIT=1 docker build --target yandexdating --tag yandexdating .

.PHONY: build-debug
build-debug:
	DOCKER_BUILDKIT=1 docker build --target yandexdating-debug --tag yandexdating-debug .

.PHONY: run
run: build
	docker run yandexdating

.PHONY: debug
debug: build-debug
	 docker run -p 40000:40000 --cap-add SYS_PTRACE --security-opt apparmor=unconfined yandexdating-debug
