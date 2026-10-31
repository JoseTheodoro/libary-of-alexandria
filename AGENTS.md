# Instruções para agentes

Estas instruções se aplicam a todo este repositório e às sessões de Codex, OpenCode
e outros agentes que nele trabalhem.

## Antes de implementar

- Leia [README.md](README.md), [North Star](docs/NORTH_STAR.md),
  [arquitetura](docs/ARCHITECTURE.md), [roadmap](docs/ROADMAP.md),
  a documentação do milestone e a issue da tarefa,
  incluindo seus critérios de aceite e dependências.
- Consulte os ADRs pertinentes, incluindo o
  [ADR-0001 — Arquitetura poliglota](docs/adr/0001-polyglot-architecture.md),
  preservando a distinção entre proposta, decisão aceita e implementação.
- Consulte as issues com `gh issue view <número> --repo JoseTheodoro/libary-of-alexandria`
  quando disponível. Se o contexto remoto estiver indisponível, reporte a limitação;
  não invente requisitos nem trate documentação local desatualizada como aprovação.
- Trabalhe em uma issue por vez. Verifique o estado do Git e preserve alterações
  existentes do usuário. Reconcilie divergências entre issue e documentação antes
  de implementar requisitos afetados.
- Este bootstrap não autoriza implementar
  [M1-01](https://github.com/JoseTheodoro/libary-of-alexandria/issues/11).
  A implementação funcional depende de uma tarefa posterior explicitamente solicitada.

## Engenharia e escopo

- Priorize local-first, offline-first, simplicidade operacional, qualidade da
  extração, proveniência, capacidade de teste e recuperação de conhecimento.
- Go é a linguagem principal e padrão: CLI e orquestração; catálogo, metadados e
  proveniência; modelo canônico; validação, chunking e SQLite/FTS5; retrieval,
  benchmarks e métricas; integração com APIs LLM; geração de Skills e Policy as Code;
  Harness e sua orquestração futura. Isso não antecipa o escopo dos milestones.
- Python é permitido nas fronteiras especializadas: extração avançada de PDF, EPUB
  e documentos complexos; OCR e análise de layout quando necessários; experimentação
  de embeddings e reranking no escopo autorizado, sem torná-los obrigatórios no M1.
- Adaptadores Python devem receber entradas explícitas e emitir saídas estruturadas,
  validadas e versionadas. Prefira subprocessos locais com JSONL no M1, sem serviços
  de rede. Go valida o contrato antes de incorporar os resultados ao domínio.
- Não misture lógica de domínio, decisões de chunking ou regras de negócio aos
  extratores. Python fornece conteúdo, estrutura e localizadores observados; Go
  mantém as decisões de domínio, a persistência canônica e os índices derivados.
- Rust exige justificativa técnica e benchmark reproduzível: somente para indexação
  ou processamento intensivo, infraestrutura local, workers ou controladores
  especializados do Harness. Não antecipe código Rust, servidores Python ou
  abstrações genéricas sem necessidade concreta.
- Mantenha conteúdo canônico independente do índice; o índice deve ser reconstruível.
- Crie interfaces apenas em fronteiras que realmente precisem variar. Não adicione
  dependências sem necessidade concreta e justificativa registrada.
- Não antecipe microsserviços, Temporal, Firecracker, Kubernetes, MCP ou sistemas
  distribuídos. Containers exigem necessidade demonstrada; LLM remoto é opcional
  e nunca requisito do M1.
- Não crie diretórios vazios, serviços ou abstrações para representar etapas futuras.
- Não modifique outros projetos nem arquivos fora deste repositório.

## Evidências, fontes e políticas

- Aceite fontes em inglês, português ou com trechos mistos. Prefira português
  brasileiro (pt-BR) nas consultas e respostas, preservando integralmente o idioma
  e o texto original das fontes e das evidências citadas.
- Não traduza nem normalize destrutivamente blocos de código ou identificadores
  técnicos. Transformações para busca devem ser derivadas, sem sobrescrever o original.
- Mantenha o modelo canônico independente de PDF, EPUB ou Markdown. Localizadores
  devem representar páginas, headings, linhas ou offsets conforme a fonte e o
  formato, sem inventar informações ausentes; explicite lacunas e ambiguidades.
- FTS5 é apenas o primeiro mecanismo de recuperação, nunca a fonte canônica.
  Preserve a possibilidade de novos extratores, principalmente Markdown, sem criar
  abstrações antecipadas nem implementar formatos fora do escopo do M1.
- Avalie recuperação cross-lingual (consulta pt-BR, evidência EN) no M2. Não presuma
  que a busca lexical satisfaz esse requisito sem medição. Não introduza embeddings,
  tradução por LLM ou serviços externos como dependências obrigatórias do M1.
- Priorize qualidade de recuperação acima de volume ou performance.
- Preserve fonte, versão/edição, checksum e localizadores verificáveis. Registre
  lacunas, ambiguidades e falhas sem inventar informações bibliográficas ou referências.
- Respeite licenças e permissões; não versione livros nem conteúdo extraído protegido.
  Use fontes sintéticas ou com licença adequada nos testes e revise o que será incluído.
- Nunca invente resultados de benchmarks; diferencie metas, medições e hipóteses.
- Recomendações de livros não são políticas corporativas automaticamente. Propostas
  exigem revisão e aprovação humana antes de se tornarem regras obrigatórias.
- Separe geração assistida por IA de validação determinística e rastreável.

## Validação e entrega

- Crie testes para código funcional, cobrindo critérios de aceite e falhas relevantes;
  execute os testes e validações pertinentes à mudança.
- Nunca considere um milestone concluído sem testes e evidências reproduzíveis.
  Escopo aprovado não é conclusão técnica. Registre scorecard e decisão
  GO / CONDITIONAL / NO-GO; uma demo não autoriza avançar ao próximo milestone.
- Revise a integridade dos arquivos, os links e `git diff --check`. Para documentação
  sem código, não invente uma suíte funcional nem declare testes inexistentes como aprovados.
- Reporte o que mudou, verificações executadas, limitações, decisões em aberto e o que
  ficou fora do escopo. Atualize a documentação quando o comportamento mudar.
- Não execute `git commit`, `git push` nem abra PR sem autorização explícita do usuário.
