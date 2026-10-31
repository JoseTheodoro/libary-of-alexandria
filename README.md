# Library of Alexandria

Uma fábrica de conhecimento de engenharia local-first: ingerir fontes técnicas
autorizadas com proveniência, recuperar evidências com assertividade mensurável e
transformar conhecimento validado em Skills e propostas aprovadas de Policy as Code.
Agentes capazes de construir software verificável são uma evolução condicionada à
comprovação de valor das etapas anteriores.

## Estado atual

M1 está em execução. A história
[M1-01 — Catalogar nove fontes técnicas e seus direitos de uso](https://github.com/JoseTheodoro/libary-of-alexandria/issues/11),
com catálogo JSON versionado, CLI Go para validação e SHA-256, testes sintéticos e
workflow de CI. O catálogo privado dos nove PDFs foi validado, com hashes,
proveniência e declaração do responsável para uso não comercial em estudo e pesquisa.
A entrega de M1-01 foi aprovada na [revisão técnica local](docs/reviews/M1-01.md).
M1-02 possui [extração local de PDF](docs/EXTRACTION.md); a auditoria registra
limites de fidelidade. M1-03 possui [persistência SQLite, índice reconstruível e
busca lexical com referências](docs/SQLITE.md); a avaliação preliminar de recuperação
tem limitações e o aceite permanece pendente.
M2 aguarda o gate de M1; M3–M8
são hipóteses de capacidade. Gates propostos não representam resultados obtidos.

## Objetivos e documentação

- [North Star](docs/NORTH_STAR.md): visão de produto e princípios.
- [Arquitetura](docs/ARCHITECTURE.md): idiomas, formatos, conteúdo canônico e recuperação.
- [ADR-0001 — Arquitetura poliglota](docs/adr/0001-polyglot-architecture.md): proposta de responsabilidades e fronteiras entre linguagens.
- [Roadmap](docs/ROADMAP.md): oito milestones, dependências e gates.
- [M1 — Knowledge Foundation](docs/milestones/M1.md): escopo aprovado e critérios de aceite.
- [Catálogo de fontes](docs/CATALOG.md): contrato JSON, comandos e limites de validação.
- [SQLite](docs/SQLITE.md): driver, inicialização, migrations e escopo implementado.
- [Avaliação de M1-03](docs/ACCEPTANCE.md): gabarito privado e medição de recuperação.
- [AGENTS.md](AGENTS.md): instruções persistentes para Codex, OpenCode e futuras sessões.

O contexto de produto parte da
[issue North Star #1](https://github.com/JoseTheodoro/libary-of-alexandria/issues/1).
Mudanças de escopo devem ser reconciliadas com as issues antes da implementação.

## Direção técnica

| Tecnologia prevista | Responsabilidade |
| --- | --- |
| Go, principal e padrão | CLI, orquestração, domínio, modelo canônico, validação, chunking, persistência, retrieval e avaliação; integrações LLM, Skills, Policy as Code e Harness nos milestones correspondentes. |
| Python, nas fronteiras especializadas | Extração avançada de documentos, OCR/análise de layout quando necessários e experimentação de embeddings/reranking no escopo autorizado. |
| Rust, somente com justificativa e benchmark | Componentes intensivos, infraestrutura local e workers/controladores especializados do Harness quando houver necessidade comprovada. |
| SQLite + FTS5 | Persistência local e busca lexical inicial; conteúdo canônico independente do índice reconstruível. |
| GitHub Actions | Testes Go e validação do catálogo sintético, também executáveis localmente. |

Adaptadores Python terão entradas explícitas e saídas estruturadas, validadas e
versionadas; no M1, a preferência é subprocesso local com JSONL, sem serviços de
rede. Domínio e decisões de chunking permanecem em Go. O ADR está proposto e não
introduz implementação, servidores Python ou código Rust neste bootstrap.

As fontes podem estar em inglês, português ou conter trechos mistos. Consultas e
respostas têm preferência por pt-BR, preservando o texto original das evidências.
A recuperação cross-lingual com consulta pt-BR e evidência EN será avaliada no M2.

O M1 não depende de embeddings, tradução por LLM, serviços externos ou banco vetorial.
Containers só serão introduzidos quando houver necessidade demonstrada.
A CLI usa Go 1.27.3 e o driver puro Go `modernc.org/sqlite`, fixados em `go.mod`.
A extração usa Python e a versão de pypdf fixada em `adapters/pdf/requirements.txt`.

## Desenvolvimento local

Use Git, Go 1.27.3 e um editor de texto; `gh` é opcional para consultar as issues.
Após clonar, leia `AGENTS.md`, a documentação do milestone e a issue da tarefa.
Trabalhe em uma issue por vez.

```sh
git clone https://github.com/JoseTheodoro/libary-of-alexandria.git
cd libary-of-alexandria
gh issue view 11 --repo JoseTheodoro/libary-of-alexandria
go vet ./...
go test -race -count=1 ./...
go run ./cmd/alexandria catalog validate testdata/catalog/valid.json
git status --short
git diff --check
```

A consulta ao GitHub, a preparação do runner e o download inicial das dependências
requerem rede. Com toolchain e dependências instalados, a CLI e os testes rodam
offline. `go test -race` requer suporte a cgo e um
compilador C no ambiente; `go test ./...` executa a suíte sem o detector de corridas.
O workflow usa runner Linux e somente fontes sintéticas.

Para produzir um executável local:

```sh
go build -o bin/alexandria ./cmd/alexandria
bin/alexandria --help
```

O exemplo contém uma fonte sintética e não cumpre o aceite dos nove livros.
Consulte o [guia do catálogo](docs/CATALOG.md) para registrar fontes reais e executar
`catalog validate --require-count 9`. Para alterações documentais, revise links,
coerência com as issues e whitespace. `git diff --check` não verifica arquivos ainda
não rastreados: revise-os também.

Mantenha livros, conteúdo extraído protegido, dados locais, credenciais e artefatos
fora do Git. O `.gitignore` ajuda a evitar inclusões acidentais, mas não substitui
a revisão dos arquivos e das permissões de uso das fontes.
