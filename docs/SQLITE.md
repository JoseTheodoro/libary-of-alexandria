# SQLite — início de M1-03

Esta etapa inicia a [issue #13](https://github.com/JoseTheodoro/libary-of-alexandria/issues/13).
Implementa inicialização explícita de um banco local, schema versionado e
importação transacional dos artefatos de #12 e geração inicial de fragmentos. Ativação de revisões,
reconstrução do índice, pesquisa e consultas de aceite ainda serão implementados.

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
O comando `init` não importa livros nem artefatos. Reexecutá-lo confere as migrations,
sem recriar as tabelas ou apagar conteúdo.

As migrations SQL são incorporadas ao binário e aplicadas em transação. A tabela
`schema_migrations` registra versão e SHA-256 do SQL aplicado. Versões futuras ou
checksums incompatíveis são recusados; falhas de migration revertem a transação e
preservam o banco existente. Não há migração reversa automática nesta etapa.

O schema contém fontes e revisões, extrações, unidades de documento,
seções, diagnósticos, perfis e fragmentos, seleção ativa e estado do índice. O
conteúdo extraído pertence às tabelas canônicas; `fragment_fts` é uma projeção
FTS5 derivada e sua chave inteira corresponde à de `fragments`. O teste confirma
que excluir entradas do índice não altera o texto canônico. A manutenção e
reconstrução consistente dessa projeção ainda serão implementadas em #13.

## Importação canônica

```sh
go run ./cmd/alexandria store import --db data/alexandria.sqlite --catalog data/catalog.json --source ID --artifact /caminho/do/artefato
```

O banco deve ter sido criado por `store init`. O importador verifica permissões,
checksum da fonte, identidade e versão do manifesto, digest e protocolo JSONL da
extração completa. Ele grava fonte, revisão, unidades, seções e diagnósticos em
uma transação; a leitura por streaming confere novamente o digest antes do commit.
O checksum da fonte é revalidado antes de concluir a transação.

Reimportar o mesmo artefato valida os registros existentes e retorna
`already_imported: true`; diferenças ou corrupção são recusadas com rollback.
Metadados existentes divergentes não são sobrescritos silenciosamente. A migration
2 registra o SHA-256 do adaptador e preserva os dados existentes da versão 1.

Nove extrações reais foram importadas e reimportadas em um banco privado de
validação: nove fontes, 4.214 unidades, 2.430 seções e 69 diagnósticos. SQLite
`integrity_check` e `foreign_key_check` passaram. O banco, as fontes e os relatórios
estão em `data/`, ignorado pelo Git; nenhum trecho do acervo é necessário na CI.

Os diagnósticos do fluxo de extração são importados. A auditoria independente de
qualidade de #12 permanece em arquivo privado separado e sua incorporação ao fluxo de
busca precisa ser definida antes do aceite final. A importação não ativa revisões,
não cria fragmentos e não atualiza o índice FTS5.

## Fragmentação inicial

```sh
go run ./cmd/alexandria store chunk --db data/alexandria.sqlite --extraction ID
```

O perfil inicial versionado usa uma unidade canônica inteira por fragmento não
vazio, sem overlap. A meta de 4 KiB é indicativa: como ainda não há limites de
estrutura suficientemente confiáveis para separar código e tabelas, unidades
maiores permanecem inteiras. O comando registra o número de fragmentos acima da
meta. Cada fragmento possui ID derivado da extração, perfil, unidade, intervalo
em bytes UTF-8 e digest do trecho. A repetição valida os dados existentes em
transação; texto divergente ou fragmentos extras causam erro. O conteúdo canônico
não é modificado e unidades vazias não produzem fragmento.

No banco privado de validação, as nove extrações produziram 4.154 fragmentos,
dos quais 99 excedem a meta; a repetição preservou as contagens. A integridade
e as foreign keys passaram. Esses números descrevem a execução local, não são
uma avaliação de qualidade de recuperação. A configuração ainda deve ser avaliada
antes do aceite. Fragmentação não ativa revisões nem atualiza a projeção FTS5;
ativação, reconstrução, pesquisa e avaliação continuam pendentes.

## Verificação

```sh
go vet ./...
go test -race -count=1 ./...
```

Os testes usam bancos temporários em disco e verificam FTS5, persistência após
reabertura, foreign keys após reconexão, rejeição de fonte incorreta na seleção
ativa, preservação do texto após exclusão da projeção, checksum/versão de
`schema_migrations`, atualização da versão 1 para 2, importação/reimportação,
artefatos manipulados, preservação de texto e rollback sem perda de dados existentes.
Não há medição de retrieval nesta etapa.
