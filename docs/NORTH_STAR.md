# North Star

Referência de produto: [issue #1](https://github.com/JoseTheodoro/libary-of-alexandria/issues/1).
Este documento registra a visão; não declara capacidades já implementadas.

## Visão de produto

Construir uma fábrica de conhecimento de engenharia local-first que permita:

1. Ingerir livros técnicos autorizados preservando evidências e proveniência.
2. Recuperar conhecimento com assertividade mensurável e localizadores verificáveis.
3. Transformar conhecimento validado em Skills revisadas e avaliadas.
4. Compilar propostas aprovadas em Policy as Code com verificações determinísticas.
5. Permitir que agentes usem esse conhecimento para construir software verificável.
6. Evoluir para um Harness isolado e multiagente somente após comprovação de valor.

## Princípios

- **Assertividade antes de escala:** volume de livros, Skills ou agentes não é sucesso.
- **Evidência antes de recomendação:** fonte, versão e localizador devem ser verificáveis;
  lacunas são explícitas, não preenchidas com informação inventada.
- **Conhecimento não implica obrigação:** recomendações exigem aprovação humana antes
  de se tornarem políticas corporativas.
- **Local-first e offline-first:** o fluxo essencial deve funcionar localmente sempre
  que possível, sem dependência obrigatória de serviços de LLM no M1.
- **Direitos de uso:** respeitar direitos autorais e licenças; não publicar textos
  protegidos. Autorização de processamento não implica autorização de redistribuição.
- **Conteúdo canônico independente da busca:** índices são derivados e reconstruíveis;
  a proveniência não pode depender apenas do índice.
- **Fidelidade linguística e independência de formato:** fontes em inglês, português
  ou com trechos mistos mantêm seu idioma e texto original, inclusive código e
  identificadores. Consultas e respostas preferem pt-BR. O modelo canônico independe
  de PDF, EPUB ou Markdown, conforme as [decisões de arquitetura](ARCHITECTURE.md).
- **Simplicidade e modularidade:** código idiomático e testável, interfaces somente nas
  fronteiras que precisam variar, sem microsserviços ou abstrações prematuras.
- **Gates reproduzíveis:** métricas objetivas não dependem do julgamento de um LLM;
  metas propostas e resultados medidos devem aparecer separados.
- **Complexidade progressiva:** infraestrutura e isolamento adicionais exigem evidência
  de necessidade. Não antecipar Temporal, Firecracker, Kubernetes, MCP ou distribuição.

## Como comprovar valor

Cada etapa do [roadmap](ROADMAP.md) depende de testes, evidências reproduzíveis e
decisão GO / CONDITIONAL / NO-GO registrada em scorecard. M1 e M2 têm escopo aprovado;
M3–M8 são hipóteses a detalhar após os gates anteriores. Nenhum milestone está
concluído neste bootstrap.

A partir de M7, a **Verified Knowledge-to-Software Success Rate** será:

> tarefas aceitas com testes/policies aprovados e evidências rastreáveis /
> tarefas elegíveis tentadas.

Essa é uma métrica futura, sem resultado medido neste repositório. Antes disso, M1
estabelece a base de fontes e M2 mede qualidade de recuperação; ganhos de escala ou
quantidade de agentes não substituem esses critérios.
