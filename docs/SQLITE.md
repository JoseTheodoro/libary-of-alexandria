# SQLite — início de M1-03

Esta etapa inicia a [issue #13](https://github.com/JoseTheodoro/libary-of-alexandria/issues/13).
Implementa inicialização explícita de um banco local, schema versionado e testes de
capacidade/integridade. Importação dos artefatos de #12, chunking, reconstrução do
índice, pesquisa e consultas de aceite ainda serão implementados.

## Decisão de driver

Foi escolhido [modernc.org/sqlite v1.61.0](https://pkg.go.dev/modernc.org/sqlite),
driver puro Go, para usar `database/sql` sem exigir cgo no build de produção.
Sua licença é BSD-3-Clause; a distribuição também contém código SQLite e
dependências com licenças próprias, que devem ser consideradas ao distribuir o
binário. O driver e suas dependências estão fixados em `go.mod` e `go.sum`.

O teste local criou, inseriu e consultou uma tabela FTS5, conferiu foreign keys
e reabriu o banco em macOS/arm64 com Go 1.27.3. O SQLite carregado nesse ambiente
reportou **3.54.1**. Também foi feito cross-build Linux/amd64 sem cgo; isso não
substitui a execução dos testes no runner Linux da CI. Se FTS5 não estiver
disponível, a migration falha explicitamente; nenhuma extensão é baixada em runtime.

A URI do driver habilita `foreign_keys` e `busy_timeout` em cada conexão, e a
inicialização verifica a configuração antes da transação. O teste força uma
reconexão para confirmar que foreign keys permanecem ativas. A aplicação limita
o pool inicial a uma conexão. A [documentação SQLite sobre foreign keys](https://www.sqlite.org/foreignkeys.html#fk_enable)
descreve a necessidade de habilitação por conexão.

## Inicialização explícita

```sh
go run ./cmd/alexandria store init --db .cache/alexandria.sqlite
```

O comando cria ou abre o banco e executa migrations, retornando versão do schema e
versão SQLite em JSON. O diretório pai deve existir. `data/`, `.cache/` e arquivos
SQLite são ignorados pelo Git; escolha uma localização privada para o acervo real.
O comando não importa livros nem artefatos. Reexecutá-lo confere as migrations,
sem recriar as tabelas ou apagar conteúdo.

A migration SQL é incorporada ao binário e aplicada em transação. A tabela
`schema_migrations` registra versão e SHA-256 do SQL aplicado. Versões futuras ou
checksums incompatíveis são recusados; falhas de migration revertem a transação e
preservam o banco existente. Não há migração reversa automática nesta etapa.

O schema inicial contém fontes e revisões, extrações, unidades de documento,
seções, diagnósticos, perfis e fragmentos, seleção ativa e estado do índice. O
conteúdo extraído pertence às tabelas canônicas; `fragment_fts` é uma projeção
FTS5 derivada e sua chave inteira corresponde à de `fragments`. O teste confirma
que excluir entradas do índice não altera o texto canônico. A manutenção e
reconstrução consistente dessa projeção ainda serão implementadas em #13.

## Verificação

```sh
go vet ./...
go test -race -count=1 ./...
```

Os testes usam bancos temporários em disco e verificam FTS5, persistência após
reabertura, foreign keys após reconexão, rejeição de fonte incorreta na seleção
ativa, preservação do texto após exclusão da projeção, checksum/versão de
`schema_migrations` e rollback sem perda de dados existentes. Não há medição de
retrieval nem importação do acervo real nesta etapa.
