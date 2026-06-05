SHELL := /bin/bash

# Colors
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
RESET  := \033[0m

# Paths
CMD := ./cmd/gophermart

mock_gen:
	@echo -e "\n$(YELLOW)▶ Mockery:$(RESET)"
	@mockery && \
		echo -e "$(GREEN)✔ Mockery — OK$(RESET)\n" || (echo -e "$(RED)✘ Mockery — FAILED$(RESET)\n" && exit 1)

run_test:
	@echo -e "\n$(YELLOW)▶ Run tests:$(RESET)"
	@go test -count=1 ./... && \
		echo -e "$(GREEN)✔ Tests — OK$(RESET)\n" || (echo -e "$(RED)✘ Tests — FAILED$(RESET)\n" && exit 1)

build:
	@cd $(CMD); echo -e "$(YELLOW)▶ I'm in cmd folder$(RESET)"; \
		go build -o gophermart && \
        echo -e "$(GREEN)✔ Build — OK$(RESET)\n" || (echo -e "$(RED)✘ Build — FAILED$(RESET)\n" && exit 1)

swagger:
	@swag init -g cmd/gophermart/main.go -o docs

check: mock_gen run_test build swagger
