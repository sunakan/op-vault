################################################################################
# Main
################################################################################
.PHONY: test
test: ## キャッシュ読み取りをテスト (op read のキャッシュ動作確認)
	@./op-keychain.sh read 'op://Test/test02/password'
	@./op-keychain.sh read 'op://Test/i4rypq4trhjhki7sebzt3zhxwy/password'
	@./op-keychain.sh read 'op://Test/d37fevxavo5ddlqh62hbyecfc4/password'
	@./op-keychain.sh read 'op://Test/ulqdvp7ovsk4rt4xhv3wu2ydum/password'

.PHONY: e2e
e2e: ## Go 版 Step 1-6 の自動化 E2E テスト (Touch ID 不要なコマンドのみ)
	@bash e2e_test.sh

.PHONY: unit
unit: ## Go 版ユニットテスト (Step 9)
	@go test -short ./...

.PHONY: build
build: ## Go 版バイナリビルド (Step 10)
	@CGO_ENABLED=1 go build ./cmd/op-keychain

.PHONY: lint
lint: ## Go 版 lint (Step 10)
	@golangci-lint run

.PHONY: list
list: ## キャッシュ一覧を表示
	@./op-keychain.sh list

.PHONY: clear
clear: ## キャッシュを全削除 (op-keychain clear)
	@./op-keychain.sh clear

.PHONY: refresh
refresh: ## 全キャッシュを再取得 (op-keychain refresh)
	@./op-keychain.sh refresh

.PHONY: status
status: ## キーチェーンの状態を表示 (IDLE_TIMEOUT・ロック状態・エントリ数)
	@./op-keychain.sh status

.PHONY: update-idle-timeout
update-idle-timeout: ## 自動ロックまでの時間を変更 (例: make update-idle-timeout SECONDS=1800)
	@./op-keychain.sh update-idle-timeout '$(SECONDS)'

################################################################################
# Tool
################################################################################
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)

# docker: mvdan/shfmtは作者製
# https://hub.docker.com/r/mvdan/shfmt/dockerfile
SHFMT_VERSION := v3.12.0
.PHONY: fmt-sh
fmt-sh: ## op-keychain.sh を shfmt でフォーマット
	@docker run --rm -it -u "${USER_ID}:${GROUP_ID}" --mount type=bind,source=${PWD},target=/work -w /work mvdan/shfmt:${SHFMT_VERSION} -i 2 -w op-keychain.sh

# docker: koalaman/shellcheck は作者製
# https://hub.docker.com/r/koalaman/shellcheck
SHELLCHECK_VERSION := v0.11.0
.PHONY: lint-sh
lint-sh: ## op-keychain.sh を shellcheck で lint
	@docker run --rm -it -u "${USER_ID}:${GROUP_ID}" --mount type=bind,source=${PWD},target=/work -w /work koalaman/shellcheck:${SHELLCHECK_VERSION} op-keychain.sh

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
