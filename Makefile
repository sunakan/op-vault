################################################################################
# Main
################################################################################
CLANG_TIDY ?= $(shell brew --prefix llvm 2>/dev/null)/bin/clang-tidy
CLANG_TIDY_CHECKS ?= -checks=-*,clang-analyzer-*,bugprone-*,-bugprone-easily-swappable-parameters,-bugprone-multi-level-implicit-pointer-conversion

.PHONY: build
build: ## バイナリビルド
	@CGO_ENABLED=1 go build -o op-vault ./cmd/op-vault

.PHONY: clean
clean: ## バイナリ削除
	@rm -rf op-vault

.PHONY: up
up: ## docker compose up -d
	@docker compose up -d

.PHONY: down
down: ## docker compose down
	@docker compose down

.PHONY: open
open: ## Open Jaeger UI in browser
	@open http://localhost:16686

.PHONY: test
test: ## 単体テスト
	@go test ./...

.PHONY: e2e-test
e2e-test: ## e2e-test
	@./scripts/e2e-test.sh

.PHONY: e2e-test-integration
e2e-test-integration: ## e2e-testで、実際に1Passwordから読み込むテスト(OP_ACCOUNT=xxxが必須)
	@OP_TEST_INTEGRATION=1 ./scripts/e2e-test.sh

.PHONY: fmt
fmt: ## go fmt
	@mise exec -- golangci-lint fmt
	@$(MAKE) sh.fmt
	@$(MAKE) c.fmt

.PHONY: lint
lint: ## lint
	@mise exec -- golangci-lint run
	@$(MAKE) sh.lint
	@$(MAKE) c.lint

################################################################################
# Tool
################################################################################
.PHONY: sh.fmt
sh.fmt: ## scripts/以下をformat
	@mise exec -- shfmt -i 2 -w scripts/

.PHONY: sh.lint
sh.lint: ## scripts/以下をlint
	@mise exec -- shellcheck scripts/*.sh

.PHONY: c.fmt
c.fmt: ## C言語ファイルをformat
	@xcrun clang-format -i internal/keychain/keychain_darwin.c internal/keychain/keychain.h

.PHONY: c.lint
c.lint: ## C言語ファイルをlint(cppcheck + clang analyzer + clang-tidy)
	@cppcheck --enable=all --error-exitcode=1 --suppress=missingIncludeSystem --suppress=unusedFunction --suppress=staticFunction --suppress=normalCheckLevelMaxBranches --inline-suppr internal/keychain/keychain_darwin.c
	@xcrun clang --analyze --analyzer-output text -Xanalyzer -analyzer-werror -Iinternal/keychain internal/keychain/keychain_darwin.c
	@test -x "$(CLANG_TIDY)" || { echo "clang-tidy not found: install Homebrew llvm or set CLANG_TIDY=/path/to/clang-tidy"; exit 1; }
	@$(CLANG_TIDY) --quiet '$(CLANG_TIDY_CHECKS)' --warnings-as-errors='*' internal/keychain/keychain_darwin.c -- -Iinternal/keychain

################################################################################
# Utility-Command help
################################################################################
.DEFAULT_GOAL := help

################################################################################
# マクロ
################################################################################
# Makefileの中身を抽出してhelpとして1行で出す
# $(1): Makefile名
# 使い方例: $(call help,{included-makefile})
define help
  grep -E '^[\.a-zA-Z0-9_-]+:.*?## .*$$' $(1) \
  | grep --invert-match "## non-help" \
  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
endef

################################################################################
# タスク
################################################################################
.PHONY: help
help: ## Make タスク一覧
	@echo '######################################################################'
	@echo '# Makeタスク一覧'
	@echo '# $$ make XXX'
	@echo '# or'
	@echo '# $$ make XXX --dry-run'
	@echo '######################################################################'
	@echo $(MAKEFILE_LIST) \
	| tr ' ' '\n' \
	| xargs -I {included-makefile} $(call help,{included-makefile})
