# Twigs: make check before a pull request runs what CI runs on it.
# Run every target from the repository root. Needs Go (go.mod names the
# version); lint also needs yamllint (pip install yamllint==1.37.1).

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO     ?= go
TWIGS  := $(GO) run ./cmd/twigs
# Pinned linters, fetched on first use.
ACTIONLINT := $(GO) run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
YAMLLINT   ?= yamllint

.PHONY: help check index new test lint

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

check: test ## Check every Twig: the schema, names, requires, and the Hub's rules (BASE=origin/main also checks version bumps)
	$(TWIGS) check $(if $(BASE),-base $(BASE),)

index: ## Rewrite catalog.json and the README's table of Twigs (CI does it after a merge)
	$(TWIGS) index

new: ## Start twigs/$(NAME)/twig.yaml from a template: make new NAME=mytool
	@test -n "$(NAME)" || { echo "usage: make new NAME=mytool" >&2; exit 2; }
	$(TWIGS) new $(NAME)

test: ## The checker's own tests
	$(GO) vet ./...
	$(GO) test ./...

lint: ## yamllint on every YAML file, actionlint on the workflows
	$(YAMLLINT) --strict .
	$(ACTIONLINT)
