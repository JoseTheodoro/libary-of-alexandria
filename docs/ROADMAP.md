# Roadmap de capacidades

Fonte: [North Star #1](https://github.com/JoseTheodoro/libary-of-alexandria/issues/1)
e as oito issues abaixo, consultadas em 2026-10-31. Os milestones de produto são
rastreados como issues; não são objetos GitHub Milestone. O
[Project #2](https://github.com/users/JoseTheodoro/projects/2/views/1) contém os itens
de M1 e suas histórias, com associação confirmada durante a preparação da execução.

## Estado e governança

M1 está em execução; #11 foi aprovada na revisão técnica local e #12 é a próxima
história. M2 tem escopo aprovado e aguarda o gate de M1.
M3–M8 são hipóteses de
capacidade, a redefinir conforme as evidências das etapas anteriores.
**Todos os exit gates abaixo são propostas a revisar no início do respectivo
milestone**, não resultados nem aprovações de conclusão.

Para fechar qualquer milestone, publicar scorecard com testes, evidências
reproduzíveis, limitações e decisão GO / CONDITIONAL / NO-GO. Uma demo ou a aprovação
do escopo não bastam para iniciar o próximo. Condições e pendências devem ficar
explícitas antes de avançar. Infraestrutura futura exige necessidade medida.

## Milestones e gates

| Milestone / issue | Capacidade e dependência | Exit gate proposto |
| --- | --- | --- |
| [M1 — Knowledge Foundation (#3)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/3) | Em execução; #11 aprovada localmente, #12 e #13 pendentes. Catalogar e ingerir nove livros autorizados; conteúdo canônico em SQLite, FTS5 reconstruível e CLI de busca. | Nove livros com hash e proveniência; ingestão idempotente; fragmentos com localizadores corretos; testes de ingestão, persistência e consulta passam. |
| [M2 — Retrieval Quality & Evaluation (#4)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/4) | Escopo aprovado, depende de M1. Benchmark de 120 perguntas: 30 literais, 42 semânticas, 30 multifonte, 18 sem resposta; split 72/48; avaliador determinístico, baseline FTS5/BM25 e experimentos controlados. | No holdout: Hit@5 ≥ 90% para respondíveis e nDCG@10 ≥ 0,80 (limiares propostos); 100% de integridade dos localizadores; medir ausência de resposta, latência e reprodutibilidade; decisão GO / CONDITIONAL / NO-GO. |
| [M3 — Knowledge Intelligence (#5)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/5) | Visão futura; redefinir após M2. Conceitos tipados, condições, contraexemplos, acordos/conflitos entre fontes e propostas rastreáveis. | Amostra revisada por humanos com precisão e proveniência medidas; divergências representadas; abstenção quando não houver fonte. |
| [M4 — Skill Factory (#6)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/6) | Visão futura; redefinir após M3. Gerar, validar, versionar e distribuir Skills baseadas em conhecimento aprovado; separar comportamento de enforcement. | Comparar tarefas com/sem Skill; melhorias reproduzíveis; custo, latência e riscos registrados; Skills com testes e revisão. |
| [M5 — Policy as Code Factory (#7)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/7) | Visão futura; redefinir após M4. Converter propostas aprovadas em analisadores Go, Rego para Terraform/Kubernetes, fixtures e gates; separar compilação assistida de verificação determinística. | Testes positivos/negativos; falsos positivos medidos; diagnósticos rastreáveis; CI em modo audit-only; aprovação das regras antes de enforcement. |
| [M6 — Knowledge at Scale (#8)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/8) | Visão futura; redefinir após M2–M5. Lotes de aproximadamente 1/5/20 GB, checkpoints, deduplicação, reconstrução de índices, controle de memória e benchmark ampliado. | Throughput, RSS, armazenamento e latência medidos; integridade das fontes e qualidade de recuperação não degradam além dos limites aprovados. |
| [M7 — Single-Agent Engineering Harness (#9)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/9) | Visão futura; redefinir após M4–M6. Um agente recebe tarefa verificável, consulta Alexandria, aplica Skills e implementa em sandbox, com testes, policies e aprovação humana. | Taxa de tarefas aceitas com testes, policies e evidências rastreáveis; custo, tempo e retomada de falhas medidos. |
| [M8 — Multi-Agent Execution Harness (#10)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/10) | Visão futura; redefinir após M7. Comparar 1, 3 e 10 agentes com coordenação durável e isolamento. | Benchmarks comparativos comprovam ganho de qualidade ou tempo, com custos e confiabilidade mensurados; logs, artefatos e aprovação humana. |

As referências a Terraform/Kubernetes em M5 são alvos futuros de políticas, não
infraestrutura exigida pelo projeto. A issue de M8 cita Firecracker e a restrição
Linux/KVM, sem execução nativa em macOS; isso não é uma escolha de implementação
para o bootstrap nem para M1.

## Próximo trabalho funcional

Como requisito complementar definido pelo usuário nesta sessão, M2 avaliará
recuperação cross-lingual com consulta pt-BR e evidência EN, preservando as fontes
originais. A composição desses casos no benchmark, os limiares específicos e os
experimentos serão definidos no M2, sem inventar resultados ou alterar aqui os
números propostos na issue #4. Essa decisão está registrada localmente em
[Arquitetura](ARCHITECTURE.md) e no plano publicado na issue #3.

O [escopo detalhado de M1](milestones/M1.md) reúne as histórias #11, #12 e #13.
A primeira é [M1-01 (#11)](https://github.com/JoseTheodoro/libary-of-alexandria/issues/11),
aprovada na revisão técnica local dos nove livros reais. M2 possui histórias #14–#21, conforme a North Star; seu
detalhamento deve ser consultado antes da execução, após o gate de M1.
