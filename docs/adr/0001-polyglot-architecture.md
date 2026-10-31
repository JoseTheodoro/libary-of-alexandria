# ADR-0001 — Arquitetura poliglota com Go como linguagem principal

- Status: **Proposto**.
- Data: 2026-10-31.
- Origem: requisitos de arquitetura poliglota fornecidos pelo usuário nesta sessão.
- Contexto: [arquitetura](../ARCHITECTURE.md), [M1](../milestones/M1.md) e
  [roadmap](../ROADMAP.md).

## Contexto

O projeto precisa combinar qualidade de extração de documentos complexos com um
núcleo local-first simples, testável e responsável pela proveniência. Especializar
ferramentas não deve fragmentar o domínio entre linguagens nem exigir serviços de
rede para o fluxo local. Qualidade de recuperação tem prioridade sobre volume ou
ganhos de performance sem evidência.

Este ADR registra a proposta solicitada, sem declarar aceitação formal ou
implementação. As responsabilidades futuras abaixo não autorizam antecipar
milestones, instalar dependências ou iniciar M1-01 neste bootstrap.

## Decisão proposta

### Go como principal e padrão

Go concentra as responsabilidades da aplicação:

- CLI e orquestração.
- Catálogo, metadados e proveniência.
- Modelo canônico de documentos, independente do formato de entrada.
- Validação, decisões de chunking e armazenamento SQLite/FTS5.
- Retrieval, benchmarks e métricas.
- Integração com APIs LLM, quando aplicável e opcional no M1.
- Geração de Skills e Policy as Code nos milestones correspondentes.
- Harness e sua orquestração futura.

O conteúdo canônico permanece separado do índice FTS5, que é derivado e
reconstruível. Regras de negócio e decisões de domínio permanecem em Go, inclusive
quando o trabalho especializado for delegado a outro processo.

### Python nas fronteiras especializadas

Python é permitido para extração avançada de PDFs, EPUBs e documentos complexos;
OCR e análise de layout quando necessários; e experimentação de embeddings e
reranking dentro de uma tarefa autorizada. Essa permissão não torna embeddings,
tradução por LLM ou serviços externos dependências obrigatórias do M1.

Extratores retornam conteúdo, estrutura observada, localizadores e limitações de
extração. A identificação de blocos ou seções da fonte não deve incorporar decisões
de chunking para retrieval. Extratores não contêm regras de negócio, não definem o
modelo de domínio e não assumem a persistência canônica ou a gestão do índice.
Texto original, idiomas mistos, blocos de código e identificadores devem ser
preservados conforme os requisitos de arquitetura.

### Contrato dos adaptadores

Adaptadores Python devem receber entradas explícitas e emitir saídas estruturadas,
validadas e versionadas. No M1, preferir execução local por subprocesso com JSONL,
sem servidores Python ou serviços de rede.

O contrato concreto será definido quando o primeiro adaptador for necessário,
observando estes requisitos:

- Entradas identificam a fonte local e os parâmetros necessários à operação, sem
  depender de estado implícito ou descoberta automática de arquivos não solicitados.
- Saídas identificam a versão do contrato e do extrator e vinculam o conteúdo à
  fonte processada, preservando localizadores, lacunas e falhas verificáveis.
- Go valida estrutura, versão suportada e consistência da proveniência antes de
  incorporar resultados ao modelo canônico. Validação do contrato no adaptador não
  transfere para Python a validação de domínio.
- O canal de dados estruturados deve permanecer separado de logs de diagnóstico;
  na execução por subprocesso, usar stdout para JSONL e stderr para diagnósticos.
- Saídas inválidas, versões incompatíveis e falhas do processo devem ser reportadas
  explicitamente; resultados parciais não podem ser apresentados como extração
  completa bem-sucedida.

O schema, a política de compatibilidade e o tratamento concreto de timeout,
cancelamento e reprocessamento permanecem em aberto para a história do adaptador.
Não criar agora protocolo genérico, framework de plugins ou servidor de integração.

### Rust condicionado a evidência

Rust é permitido somente com justificativa técnica e benchmark reproduzível, para
indexação ou processamento intensivo, componentes de infraestrutura local, workers
ou controladores especializados do Harness. A responsabilidade geral pelo domínio
e pela orquestração continua em Go.

Antes de introduzi-lo, documentar o problema concreto, a alternativa em Go ou solução
existente, o ambiente e a carga de trabalho, as métricas pertinentes e os resultados
comparativos. Avaliar também custos de manutenção e operação, correção e preservação
da proveniência. A escolha exige benefício demonstrado para a necessidade identificada;
não há benchmark realizado, limiar inventado ou código Rust autorizado por este ADR.

## Alternativas e consequências

Usar apenas Go mantém uma única cadeia de ferramentas e continua sendo o padrão
quando atender à necessidade. Python permite recorrer a ferramentas especializadas
de extração ou experimentação, ao custo de um runtime adicional e de um contrato
entre processos que precisa ser validado e testado.

Servidores Python introduziriam operação de rede sem necessidade demonstrada no M1.
Rust antecipado adicionaria outra cadeia de build sem benefício medido. Frameworks
genéricos de adaptadores criariam abstrações antes de existirem casos concretos.
Essas opções não são introduzidas por esta proposta.

Qualidade da extração, proveniência, capacidade de teste, qualidade da recuperação
e simplicidade operacional devem guiar as escolhas, antes de uniformidade de
linguagem ou performance isolada.

## Verificação e acompanhamento

Quando houver implementação, testar os contratos com fontes sintéticas ou
devidamente licenciadas, incluindo saídas inválidas, versões incompatíveis, falhas
de processo, lacunas de extração e preservação de texto e localizadores. Verificar
que as decisões de chunking e as regras de domínio continuam em Go.

Os testes de integração devem comprovar execução local e incorporação validada dos
resultados, sem exigir serviços externos no M1. Benchmarks serão necessários para
justificar qualquer adoção de Rust; nenhum resultado é declarado neste bootstrap.
Revisar o status do ADR quando houver uma decisão explícita de aceitação ou revisão.
