# Library of Alexandria

Uma fábrica de conhecimento de engenharia local-first: ingerir fontes técnicas
autorizadas com proveniência, recuperar evidências com assertividade mensurável e
transformar conhecimento validado em Skills e propostas aprovadas de Policy as Code.
Agentes capazes de construir software verificável são uma evolução condicionada à
comprovação de valor das etapas anteriores.

## Estado atual

Este repositório contém somente o bootstrap documental e as convenções básicas.
Não há aplicação executável, ingestão, catálogo, banco, índice ou testes funcionais.
M1 e M2 têm escopo aprovado, mas não estão implementados; M3–M8 são hipóteses de
capacidade. Gates propostos não representam resultados obtidos.

A próxima história funcional é
[M1-01 — Catalogar seis fontes técnicas e seus direitos de uso](https://github.com/JoseTheodoro/libary-of-alexandria/issues/11),
fora do escopo deste bootstrap. Nenhum livro ou dado bibliográfico foi cadastrado.

## Objetivos e documentação

- [North Star](docs/NORTH_STAR.md): visão de produto e princípios.
- [Arquitetura](docs/ARCHITECTURE.md): idiomas, formatos, conteúdo canônico e recuperação.
- [ADR-0001 — Arquitetura poliglota](docs/adr/0001-polyglot-architecture.md): proposta de responsabilidades e fronteiras entre linguagens.
- [Roadmap](docs/ROADMAP.md): oito milestones, dependências e gates.
- [M1 — Knowledge Foundation](docs/milestones/M1.md): escopo aprovado e critérios de aceite.
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
| GitHub Actions | Futura automação dos testes e validações que também rodem localmente. |

Adaptadores Python terão entradas explícitas e saídas estruturadas, validadas e
versionadas; no M1, a preferência é subprocesso local com JSONL, sem serviços de
rede. Domínio e decisões de chunking permanecem em Go. O ADR está proposto e não
introduz implementação, servidores Python ou código Rust neste bootstrap.

As fontes podem estar em inglês, português ou conter trechos mistos. Consultas e
respostas têm preferência por pt-BR, preservando o texto original das evidências.
A recuperação cross-lingual com consulta pt-BR e evidência EN será avaliada no M2.

O M1 não depende de embeddings, tradução por LLM, serviços externos ou banco vetorial.
Containers só serão introduzidos quando houver necessidade demonstrada.
Não há dependências instaladas
ou versões de ferramentas definidas neste bootstrap.

## Desenvolvimento local

Para trabalhar neste estágio, bastam Git e um editor de texto; `gh` é opcional para
consultar as issues. Após clonar, leia `AGENTS.md`, a documentação do milestone e a
issue da tarefa. Trabalhe em uma issue por vez.

```sh
git clone https://github.com/JoseTheodoro/libary-of-alexandria.git
cd libary-of-alexandria
gh issue view 11 --repo JoseTheodoro/libary-of-alexandria
git status --short
git diff --check
```

A consulta ao GitHub requer rede; o produto deverá funcionar localmente e offline
sempre que possível. Para alterações documentais, revise links, coerência com as
issues e whitespace. `git diff --check` não verifica arquivos ainda não rastreados:
revise-os também. Os comandos de build e testes da aplicação serão documentados
junto ao primeiro código funcional.

Mantenha livros, conteúdo extraído protegido, dados locais, credenciais e artefatos
fora do Git. O `.gitignore` ajuda a evitar inclusões acidentais, mas não substitui
a revisão dos arquivos e das permissões de uso das fontes.
