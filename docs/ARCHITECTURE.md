# Decisões de arquitetura

Estado: requisitos definidos pelo usuário em 2026-10-31, durante o bootstrap.
Este documento complementa a [North Star](NORTH_STAR.md), o [roadmap](ROADMAP.md)
e o [escopo de M1](milestones/M1.md). Registra decisões para implementação futura;
não há código, schema ou extratores implementados. As issues remotas não foram
alteradas nesta sessão.

## Linguagens e fronteiras

O [ADR-0001 — Arquitetura poliglota](adr/0001-polyglot-architecture.md), com status
**Proposto**, registra a orientação solicitada pelo usuário: Go é a linguagem
principal e padrão do projeto, responsável pelo domínio, modelo canônico,
validação, chunking, persistência, retrieval, métricas e orquestração. Integrações
LLM, geração de Skills/Policy as Code e Harness também pertencem a Go, quando seus
milestones forem executados.

Python é permitido em extração avançada de documentos, OCR/análise de layout quando
necessários e experimentação de embeddings/reranking no escopo autorizado. Os
adaptadores recebem entradas explícitas e emitem saídas estruturadas, validadas e
versionadas. No M1, preferir execução local por subprocesso com JSONL, sem serviços
de rede. Go valida os resultados antes de incorporá-los ao conteúdo canônico.
Extratores não contêm lógica de domínio, decisões de chunking ou regras de negócio.

Rust é permitido somente com justificativa técnica e benchmark, para componentes
intensivos, infraestrutura local ou workers/controladores especializados do Harness.
Não antecipar Rust, servidores Python ou abstrações genéricas. Qualidade da extração,
proveniência, capacidade de teste, recuperação e simplicidade operacional orientam
a escolha; os contratos concretos serão definidos quando houver necessidade real.

## Idiomas e fidelidade das fontes

Fontes técnicas podem estar em inglês, português ou conter trechos mistos, inclusive
no mesmo documento. A linguagem preferencial de consulta e resposta é português
brasileiro (pt-BR); essa preferência não determina o idioma da fonte nem autoriza
traduzir ou substituir evidências.

O sistema deve preservar integralmente o idioma e o texto original das fontes.
O artefato de origem deve permanecer preservado localmente com checksum e direitos
de uso registrados. O conteúdo canônico deve manter o texto original extraído e
sua relação verificável com esse artefato. Falhas, lacunas ou ambiguidades de
extração devem ser registradas; não se deve alegar preservação integral quando
houver perdas nem preencher essas perdas por inferência.

Blocos de código e identificadores técnicos não devem ser traduzidos ou normalizados
de forma destrutiva. Isso inclui preservar capitalização, símbolos e indentação
significativa. Citações permanecem no idioma original mesmo quando a explicação
estiver em pt-BR. A preferência por pt-BR não exige implementar geração de respostas
no M1.

Qualquer representação transformada para busca deve ser derivada e separada do
original, mantendo a ligação com conteúdo e localizadores canônicos. O processamento
do índice nunca deve sobrescrever o texto usado para apresentar evidências.

## Modelo canônico independente do formato

O modelo canônico de documentos não deve depender de PDF, EPUB ou Markdown.
Fonte, conteúdo, estrutura disponível e proveniência são responsabilidades comuns;
o formato de entrada determina como extrair esses elementos e quais localizadores
podem ser obtidos. O conteúdo canônico não deve exigir página de todo documento
nem usar o formato interno de um extrator como contrato geral.

| Localizador | Informação a preservar quando disponível |
| --- | --- |
| Página | Referência à página na fonte; distinguir posição no arquivo de numeração impressa quando ambas existirem. |
| Heading | Título e contexto estrutural disponíveis, distinguindo ocorrências repetidas quando necessário. |
| Linha | Intervalo de linhas e identificação da representação à qual ele se refere. |
| Offset | Intervalo, unidade e representação de referência, permitindo localizar o trecho sem ambiguidade. |

Localizadores podem ser combinados conforme o formato. Dados ausentes devem permanecer
ausentes, com a limitação registrada: não inventar páginas para EPUB/Markdown,
headings, linhas ou offsets. Um offset no texto extraído não deve ser apresentado
como posição no arquivo original. A representação referenciada deve estar vinculada
à fonte e à sua versão para permitir verificação.

O schema concreto e as convenções de intervalos serão definidos nas histórias
pertinentes de M1. Essas decisões não exigem agora uma hierarquia de classes,
framework de plugins ou interfaces para formatos ainda não usados.

## Extração, persistência e recuperação

Extratores devem converter os formatos presentes no acervo autorizado de M1 em
conteúdo e proveniência compatíveis com o modelo canônico. A arquitetura deve
permitir extratores futuros, principalmente Markdown, sem impor conceitos de PDF
às demais fontes. Isso não autoriza implementar formatos fora do escopo de M1.

SQLite armazenará o conteúdo canônico; FTS5 será somente o primeiro mecanismo de
recuperação. O índice será derivado e reconstruível a partir do conteúdo canônico,
sem perda do texto ou da proveniência. Resultados de busca devem remeter ao conteúdo
canônico e aos localizadores, permitindo substituir ou complementar o mecanismo
de recuperação sem reescrever as fontes.

M1 deve funcionar localmente, sem embeddings, tradução por LLM ou serviços externos
como dependências obrigatórias. Não há seleção de modelo, serviço de tradução ou
nova dependência neste documento.

## Avaliação no M2 e verificações futuras

M2 avaliará explicitamente recuperação cross-lingual: consulta em pt-BR e evidência
em inglês. A busca lexical inicial será medida como baseline, sem pressupor que
ela atenda à recuperação entre idiomas. Resultados desse recorte devem ser
identificáveis na avaliação, para não ocultar limitações em métricas agregadas.

A seleção dos casos, sua distribuição no benchmark e os limiares específicos
permanecem em aberto para M2. Não são alterados aqui o total de 120 perguntas,
o split 72/48 ou os gates propostos na issue de M2; tampouco são declarados ganhos
ou escolhidas técnicas antes da avaliação.

Quando houver código funcional, os testes deverão verificar fidelidade de texto em
inglês, português e trechos mistos; preservação de código e identificadores;
localizadores presentes, ausentes e ambíguos; e reconstrução do índice sem alteração
do conteúdo canônico. As fixtures serão sintéticas ou devidamente licenciadas e
limitadas aos formatos implementados no escopo autorizado.
