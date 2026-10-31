# Library of Alexandria — atalhos de desenvolvimento local.
# Execute `make help` para ver os alvos e as variáveis disponíveis.
#
# O Makefile não substitui a documentação nem os comandos da CLI: ele apenas
# reúne os passos usados em README, docs/ e no CI. Nenhum alvo executa commit,
# push ou abertura de PR.

SHELL := /bin/sh

BIN_DIR ?= bin
BIN     ?= $(BIN_DIR)/alexandria
GO      ?= go
PYTHON  ?= python3

# Ferramentas de análise fixadas para reprodutibilidade. Não entram em go.mod:
# são instaladas por `make tools` e executadas a partir de GOBIN ou GOPATH/bin.
STATICCHECK_VERSION  ?= v0.8.2
GOVULNCHECK_VERSION  ?= v1.9.0
TOOLS_BIN := $(shell go env GOBIN)
ifeq ($(TOOLS_BIN),)
TOOLS_BIN := $(shell go env GOPATH)/bin
endif
STATICCHECK ?= $(TOOLS_BIN)/staticcheck
GOVULNCHECK ?= $(TOOLS_BIN)/govulncheck

# Catálogo e extração (M1-01 e M1-02).
CATALOG       ?= testdata/catalog/valid.json
REQUIRE_COUNT ?= 0
MODE          ?= layout
EXTRACTED     ?= extracted
FILE          ?=

# Banco SQLite local (M1-03). O padrão é um caminho privado, ignorado pelo Git.
DB ?= data/alexandria.sqlite

# Identificadores exigidos pela importação, fragmentação e ativação.
SOURCE     ?=
EXTRACTION ?=
PROFILE    ?=
ARTIFACT   ?=

# Busca.
QUERY ?=
LIMIT ?= 5

# Avaliação de recuperação: gabarito e auditoria privados.
SUITE  ?= data/acceptance-suite.json
AUDIT  ?= data/extraction-acceptance-audit.json
REPORT ?= data/acceptance-report.json

GO_SOURCES := $(shell find cmd internal -type f -name '*.go')

.DEFAULT_GOAL := help
.PHONY: help build test test-race vet staticcheck vulncheck tools fmt fmt-check adapter-deps adapter-test ci \
	catalog-validate checksum extract store-init store-import store-chunk \
	store-activate store-rebuild store-verify store-verify-fts search search-json \
	acceptance acceptance-evidence clean

##@ Ajuda
help: ## Mostra esta ajuda e os valores atuais das variáveis
	@awk 'BEGIN {FS = ":.*##"; print ""; print "Uso: make <alvo> [VAR=valor]"; print ""} /^##@/ {printf "\n%s\n", substr($$0, 5); next} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\nVariáveis principais (valores atuais):\n'
	@printf '  %-14s %s\n' BIN '$(BIN)'
	@printf '  %-14s %s\n' STATICCHECK '$(STATICCHECK_VERSION)'
	@printf '  %-14s %s\n' GOVULNCHECK '$(GOVULNCHECK_VERSION)'
	@printf '  %-14s %s\n' CATALOG '$(CATALOG)'
	@printf '  %-14s %s\n' DB '$(DB)'
	@printf '  %-14s %s\n' SOURCE '$(SOURCE)'
	@printf '  %-14s %s\n' EXTRACTION '$(EXTRACTION)'
	@printf '  %-14s %s\n' PROFILE '$(PROFILE)'
	@printf '  %-14s %s\n' ARTIFACT '$(ARTIFACT)'
	@printf '  %-14s %s\n' QUERY '$(QUERY)'
	@printf '  %-14s %s\n' LIMIT '$(LIMIT)'
	@printf '  %-14s %s\n' SUITE '$(SUITE)'
	@printf '  %-14s %s\n' AUDIT '$(AUDIT)'
	@printf '\nExemplo: make search DB=data/alexandria.sqlite QUERY="architecture" LIMIT=3\n\n'

##@ Construção e qualidade
build: $(BIN) ## Compila a CLI em bin/alexandria

$(BIN): $(GO_SOURCES) go.mod go.sum
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/alexandria

test: ## Executa os testes Go sem detector de corridas
	$(GO) test -count=1 ./...

test-race: ## Executa os testes Go com -race (requer cgo)
	$(GO) test -race -count=1 ./...

vet: ## Executa go vet
	$(GO) vet ./...

tools: ## Instala staticcheck e govulncheck nas versões fixadas (requer rede)
	$(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

staticcheck: tools ## Executa o staticcheck (instala a ferramenta se necessário)
	$(STATICCHECK) ./...

vulncheck: tools ## Executa o govulncheck (consulta o banco público de vulnerabilidades)
	$(GOVULNCHECK) ./...

fmt: ## Formata cmd/ e internal/ com gofmt
	gofmt -w cmd internal

fmt-check: ## Falha se algum arquivo estiver fora do gofmt (como no CI)
	@test -z "$$(gofmt -l cmd internal)" || { echo "erro: execute 'make fmt'"; gofmt -l cmd internal; exit 1; }

adapter-deps: ## Instala as dependências do adaptador PDF (requer rede)
	$(PYTHON) -m pip install -r adapters/pdf/requirements.txt

adapter-test: ## Executa os testes do adaptador PDF com fixtures sintéticas
	$(PYTHON) -m unittest discover -s adapters/pdf -p 'test_*.py' -v

ci: fmt-check vet staticcheck vulncheck test-race adapter-test catalog-validate ## Reproduz os passos do CI localmente

##@ Catálogo e extração
catalog-validate: $(BIN) ## Valida o catálogo (CATALOG, REQUIRE_COUNT)
	@$(BIN) catalog validate --require-count $(REQUIRE_COUNT) $(CATALOG)

checksum: $(BIN) ## Calcula o SHA-256 de um arquivo (FILE=/caminho)
	@test -n "$(strip $(FILE))" || { echo "erro: informe FILE=/caminho/do/arquivo"; exit 2; }
	@$(BIN) catalog checksum $(FILE)

extract: $(BIN) ## Extrai PDFs do catálogo (SOURCE=id; sem SOURCE extrai todas)
	@$(BIN) extract --catalog $(CATALOG) $(if $(strip $(SOURCE)),--source $(SOURCE),--all) --mode $(MODE) --output $(EXTRACTED)

##@ Banco canônico (M1-03)
store-init: $(BIN) ## Cria ou migra o banco (DB)
	@mkdir -p $(dir $(DB))
	@$(BIN) store init --db $(DB)

store-import: $(BIN) ## Importa uma extração validada (SOURCE, ARTIFACT)
	@test -n "$(strip $(SOURCE))" || { echo "erro: informe SOURCE=<id>"; exit 2; }
	@test -n "$(strip $(ARTIFACT))" || { echo "erro: informe ARTIFACT=<diretório>"; exit 2; }
	@$(BIN) store import --db $(DB) --catalog $(CATALOG) --source $(SOURCE) --artifact $(ARTIFACT)

store-chunk: $(BIN) ## Gera fragmentos de uma extração (EXTRACTION)
	@test -n "$(strip $(EXTRACTION))" || { echo "erro: informe EXTRACTION=<id>"; exit 2; }
	@$(BIN) store chunk --db $(DB) --extraction $(EXTRACTION)

store-activate: $(BIN) ## Ativa a extração e reconstrói o FTS5 (SOURCE, EXTRACTION, PROFILE)
	@test -n "$(strip $(SOURCE))" || { echo "erro: informe SOURCE=<id>"; exit 2; }
	@test -n "$(strip $(EXTRACTION))" || { echo "erro: informe EXTRACTION=<id>"; exit 2; }
	@test -n "$(strip $(PROFILE))" || { echo "erro: informe PROFILE=<id>"; exit 2; }
	@$(BIN) store activate --db $(DB) --source $(SOURCE) --extraction $(EXTRACTION) --profile $(PROFILE)

store-rebuild: $(BIN) ## Reconstrói o FTS5 a partir do conteúdo canônico
	@$(BIN) store rebuild --db $(DB)

store-verify: $(BIN) ## Verifica o índice sem escrever (somente leitura)
	@$(BIN) store verify --db $(DB)

store-verify-fts: $(BIN) ## Verifica também os segmentos internos do FTS5
	@$(BIN) store verify-fts --db $(DB)

##@ Busca
search: $(BIN) ## Busca lexical (QUERY; opcional SOURCE e LIMIT)
	@test -n "$(strip $(QUERY))" || { echo "erro: informe QUERY='termos da busca'"; exit 2; }
	@$(BIN) search --db $(DB) --query "$(QUERY)" --limit $(LIMIT) $(if $(strip $(SOURCE)),--source $(SOURCE),)

search-json: $(BIN) ## Mesma busca com saída JSON
	@test -n "$(strip $(QUERY))" || { echo "erro: informe QUERY='termos da busca'"; exit 2; }
	@$(BIN) search --db $(DB) --query "$(QUERY)" --limit $(LIMIT) --json $(if $(strip $(SOURCE)),--source $(SOURCE),)

##@ Avaliação
acceptance: $(BIN) ## Avalia o gabarito privado (SUITE, AUDIT)
	@test -f "$(SUITE)" || { echo "erro: gabarito não encontrado: $(SUITE)"; exit 2; }
	@test -f "$(AUDIT)" || { echo "erro: auditoria não encontrada: $(AUDIT)"; exit 2; }
	@$(BIN) acceptance --db $(DB) --suite $(SUITE) --audit $(AUDIT)

acceptance-evidence: $(BIN) ## Gera relatório privado com evidências em REPORT
	@test -f "$(SUITE)" || { echo "erro: gabarito não encontrado: $(SUITE)"; exit 2; }
	@test -f "$(AUDIT)" || { echo "erro: auditoria não encontrada: $(AUDIT)"; exit 2; }
	@mkdir -p $(dir $(REPORT))
	@$(BIN) acceptance --db $(DB) --suite $(SUITE) --audit $(AUDIT) --include-evidence > $(REPORT)
	@echo "relatório privado gravado em $(REPORT)"

##@ Limpeza
clean: ## Remove binários e cobertura locais (não toca em data/)
	rm -rf $(BIN_DIR) coverage.out
