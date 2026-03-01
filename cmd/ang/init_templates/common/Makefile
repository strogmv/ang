.PHONY: bootstrap deps up dev down build validate lint smoke doctor reset

bootstrap:
	bash scripts/bootstrap.sh

deps:
	bash scripts/preflight.sh

up: deps
	ang up

dev:
	bash scripts/dev-up.sh

down:
	bash scripts/dev-down.sh

validate:
	ang validate

lint:
	ang lint

build: deps
	ang build

smoke:
	bash scripts/dev-smoke.sh

doctor:
	bash scripts/doctor.sh

reset:
	bash scripts/dev-reset.sh
