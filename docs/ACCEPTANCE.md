# Avaliação de recuperação de M1-03

O comando `acceptance` avalia um gabarito privado previamente revisado contra o
índice ativo. Não gera perguntas nem escolhe evidências automaticamente. A suíte
real de 18 consultas ainda precisa ser preparada e revisada; os testes automatizados
usam somente fontes sintéticas. Nenhuma taxa de acerto real foi medida nesta etapa.

## Preparação do gabarito

Registre exatamente duas consultas para cada uma das nove fontes ativas. Para cada
consulta, revise previamente a fonte, revisão de extração, unidade e intervalo
mínimo que identifica a evidência esperada. Os limites são offsets em bytes UTF-8,
começando em zero e com fim exclusivo, no texto canônico da unidade extraída; não
são offsets no PDF. A evidência deve estar integralmente contida em um fragmento.
Não altere gabaritos para acomodar os resultados. Preserve o idioma e o texto da
fonte; a avaliação não usa tradução automática nem LLM.

O arquivo é JSON UTF-8 com `schema_version` 1, `index_digest` retornado por
`store verify`, e uma lista `cases`. Cada caso tem `id` único, `query`, `source_id`,
`source_sha256`, `extraction_id`, `unit_id`, `start_byte` e `end_byte`. Os IDs e
checksums devem vir da fonte e extração realmente revisadas, e o intervalo deve ser
verificado no texto canônico. O formato abaixo mostra apenas a estrutura de um caso;
uma suíte incompleta não é aceita como avaliação de M1:

```json
{
  "schema_version": 1,
  "index_digest": "DIGEST_DO_INDICE",
  "cases": [{
    "id": "CASO_01",
    "query": "consulta revisada",
    "source_id": "ID_DA_FONTE",
    "source_sha256": "SHA256_DA_FONTE",
    "extraction_id": "ID_DA_EXTRACAO",
    "unit_id": "ID_DA_UNIDADE",
    "start_byte": 0,
    "end_byte": 1
  }]
}
```

Guarde o gabarito, resultados e eventuais evidências protegidas em `data/`, fora do
Git. A existência de uma suíte não comprova revisão humana nem qualidade das
perguntas; a revisão precisa ser registrada separadamente.

## Execução

```sh
go run ./cmd/alexandria store verify --db data/alexandria.sqlite
go run ./cmd/alexandria acceptance --db data/alexandria.sqlite --suite data/acceptance-suite.json > data/acceptance-report.json
```

O avaliador valida todos os gabaritos antes da primeira consulta, recusa fonte ou
revisão não ativa, hash divergente, intervalo inválido, ausência de fragmento
contendo a evidência, índice divergente, IDs duplicados e distribuição incorreta.
A avaliação usa uma única transação de leitura, pesquisa em todo o acervo e registra
o primeiro resultado que contém a evidência esperada entre os cinco primeiros.
Todos os resultados retornados são resolvidos e validados contra o canônico.

O relatório registra digest do índice, digest semântico da suíte, versão SQLite,
IDs dos fragmentos por consulta, posições, total de referências verificadas e o
número de acertos. `passed` significa somente pelo menos 15 de 18 acertos com as
validações executadas; não conclui a issue nem o milestone. Uma execução válida
abaixo da meta retorna código 1 e ainda produz o relatório. Entrada inválida ou
erro de execução também retorna 1; argumentos inválidos retornam 2. Execute duas
vezes com a mesma suíte e compare os relatórios antes de registrar evidência.

A auditoria independente de qualidade de #12 e a verificação interna dos segmentos
FTS5 permanecem pendentes no processo de aceite; a checagem atual compara a projeção
armazenada ao canônico, como detalhado em [SQLite](SQLITE.md).
