################################################################################
# Main
################################################################################
.PHONY: build
build: ## バイナリビルド
	@CGO_ENABLED=1 go build -o op-keychain ./cmd/op-keychain

.PHONY: clean
clean: ## バイナリ削除
	@rm -rf op-keychain

.PHONY: e2e-test
e2e-test: ## e2e-test
	@./scripts/e2e-test.sh

.PHONY: lint
lint: ## lint
	@golangci-lint run

################################################################################
# Tool
################################################################################
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)

# docker: mvdan/shfmtは作者製
# https://hub.docker.com/r/mvdan/shfmt/dockerfile
SHFMT_VERSION := v3.13.1
.PHONY: sh.fmt
sh.fmt: ## scripts/以下をformat
	@docker run --rm -i -u "${USER_ID}:${GROUP_ID}" --mount type=bind,source=${PWD}/scripts/,target=/scripts/ -w /scripts mvdan/shfmt:${SHFMT_VERSION} -i 2 -w .

# docker: koalaman/shellcheck は作者製
# https://hub.docker.com/r/koalaman/shellcheck
SHELLCHECK_VERSION := v0.11.0
.PHONY: sh.lint
sh.lint: ## format
	$(eval SH_FILES := $(shell ls -1 scripts/*.sh | xargs basename))
	@docker run --rm -i -u "${USER_ID}:${GROUP_ID}" --mount type=bind,source=${PWD}/scripts/,target=/scripts/ -w /scripts koalaman/shellcheck:${SHELLCHECK_VERSION} ${SH_FILES}

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
