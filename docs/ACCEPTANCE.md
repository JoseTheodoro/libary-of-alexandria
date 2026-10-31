# Avaliação de recuperação de M1-03

O comando `acceptance` avalia um gabarito privado previamente revisado contra o
índice ativo. Não gera perguntas nem escolhe evidências automaticamente. A suíte
real foi preparada em caráter preliminar; os testes automatizados usam somente
fontes sintéticas. Os resultados locais e suas limitações estão descritos abaixo.

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

## Medição preliminar local

Foi preparado um gabarito privado com 18 consultas, duas por fonte, e intervalos
identificados no texto canônico. Ele foi congelado antes de executar as buscas.
As consultas foram formuladas após a leitura dos trechos pelo mesmo agente que
selecionou as evidências, sem revisão independente; isso pode favorecer a baseline
lexical e limita a interpretação da medição. Não é um benchmark independente.

Duas execuções produziram relatórios idênticos, com **15/18** referências esperadas
no Top 5 e **78 resultados** no total. Os 18 trechos esperados tiveram sequência de
palavras normalizada coincidente com extração independente via PDFKit. Para as 78
páginas retornadas, a comparação de sequência de palavras da página inteira com o
PDFKit ficou em pelo menos 0,9 em 75 casos; três ficaram abaixo, com mínimo de 0,546.
Essa comparação é indicativa, não prova fidelidade visual ou semântica, e o limite
0,9 é um critério exploratório, não um gate aprovado.

Na inspeção visual das três páginas abaixo do limiar, todas são páginas de índice
remissivo em duas colunas. As divergências estão associadas a essas páginas; a
inspeção não demonstra que o restante dos resultados seja semanticamente adequado.
Os índices remissivos também mostram que uma correspondência lexical pode retornar
uma lista de referências em vez de uma passagem explicativa, o que deve ser
considerado ao revisar a qualidade da recuperação.

O aceite **não está comprovado**: os gabaritos e a relevância dos resultados
precisam de revisão independente, as diferenças de extração em páginas de índice
precisam ser consideradas, e a auditoria de #12 deve ser incorporada. A medição também não
resolve a verificação interna dos segmentos FTS5. Gabarito, relatórios e comparações
permanecem privados em `data/`; não foram publicados trechos nem hashes das fontes.
