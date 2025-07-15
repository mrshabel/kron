DOCKER_COMPOSE_FILE := docker-compose.yaml

.PHONY: build start stop clean help compile

build: # bootstrap the application containers
	docker compose -f $(DOCKER_COMPOSE_FILE) build

start: # start the application instances
	docker compose -f $(DOCKER_COMPOSE_FILE) up

stop: # stop all instances of the application
	docker compose -f $(DOCKER_COMPOSE_FILE) stop


clean: # remove unused docker images
	docker image prune --filter dangling=true -y

compile: # build all consumer and producer instances into 'bin' directory
	@echo "Building all application instances"
	go build -o ./bin/producer ./cmd/producer/main.go



help: # Show help for each of the Makefile recipes.
	@grep -E '^[a-zA-Z0-9 -]+:.*#'  Makefile | sort | while read -r l; do printf "\033[1;32m$$(echo $$l | cut -f 1 -d':')\033[00m:$$(echo $$l | cut -f 2- -d'#')\n"; done