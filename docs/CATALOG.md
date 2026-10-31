# Catálogo de fontes — M1-01

Implementação inicial da [issue #11](https://github.com/JoseTheodoro/libary-of-alexandria/issues/11).
O catálogo registra fontes explicitamente fornecidas pelo operador; não busca
livros, não extrai conteúdo e não altera os arquivos. É uma fronteira de entrada
para as histórias posteriores, independente do modelo canônico de conteúdo.

## Contrato JSON versão 1

O objeto raiz contém `schema_version: 1` e uma lista não vazia `sources`.
Cada fonte possui:

| Campo | Regra |
| --- | --- |
| `id` | Identificador estável e único escolhido pelo operador. Letras minúsculas ASCII, números, ponto, hífen ou sublinhado; iniciar com letra ou número. |
| `title` | Título original não vazio, sem tradução ou normalização. |
| `authors` | Lista não vazia de nomes não vazios, preservados como registrados. |
| `edition` | Texto original não vazio quando conhecido; `null` quando desconhecido. Omissão também significa desconhecido. |
| `format` | Identificador declarado, em minúsculas, começando por letra e contendo letras ASCII, números ou hífen; por exemplo `pdf`, `epub` ou `txt`. Não seleciona um extrator nesta etapa. |
| `sha256` | 64 dígitos hexadecimais; a comparação aceita letras maiúsculas ou minúsculas. Identifica a revisão exata do arquivo. |
| `path` | Caminho local absoluto ou relativo ao diretório do catálogo, independentemente do diretório atual da CLI. Não expande `~`, variáveis ou URLs. |
| `permission.status` | `authorized` para validar a fonte. Ausência, `unknown`, `denied` ou outros valores impedem sua liberação. |
| `permission.basis` | Descrição não vazia da licença ou autorização para processamento. |
| `permission.reference` | Referência não vazia à licença ou evidência de autorização: URI, caminho ou descrição verificável pelo responsável. |

Não invente título, autores ou edição para satisfazer a validação. Edição desconhecida
pode ficar `null`; título e autores ausentes impedem validar a fonte. Permissões
não comprovadas devem permanecer sem autorização até revisão humana. A CLI verifica
o registro da declaração, não a autenticidade da evidência ou sua validade jurídica;
não acessa automaticamente URIs nem comprovantes de permissão.

IDs duplicados e checksums duplicados são erros: duas cópias do mesmo arquivo não
contam como duas fontes do acervo. O ID representa a fonte; o checksum representa
uma revisão. Alterações no arquivo são reportadas, nunca corrigidas automaticamente.
Preserve os registros e evidências da revisão anterior antes de atualizar o catálogo.
Esta etapa não implementa histórico persistente de revisões.

O JSON deve usar UTF-8 válido e conter um único objeto. Campos desconhecidos e
chaves duplicadas, inclusive com variação de maiúsculas/minúsculas, são rejeitados.
Escapes Unicode com surrogates isolados ou pares inválidos também são rejeitados,
evitando substituição silenciosa por caracteres de reposição nos metadados.
A validação mantém os metadados originais; índices normalizados pertencem a uma
etapa futura.

## Uso local

Com Go 1.27.3 instalado, sem dependências de módulos externos:

```sh
go run ./cmd/alexandria catalog validate testdata/catalog/valid.json
go run ./cmd/alexandria catalog checksum testdata/catalog/source.txt
```

O [exemplo versionado](../testdata/catalog/valid.json) contém somente uma
[fonte sintética](../testdata/catalog/source.txt), com permissão de uso para testes
declarada no próprio arquivo. Não representa um livro real nem demonstra o aceite
dos nove livros. O checksum corresponde aos bytes efetivos da fixture.

Para o acervo real, crie um catálogo privado, por exemplo `data/catalog.json`,
com os metadados confirmados e caminhos dos arquivos autorizados. A pasta `data/`
é ignorada pelo Git. Calcule o hash de cada arquivo informado explicitamente:

```sh
go run ./cmd/alexandria catalog checksum /caminho/autorizado/livro.pdf
go run ./cmd/alexandria catalog validate --require-count 9 data/catalog.json
```

Use apenas caminhos reais autorizados. Flags vêm antes do caminho do catálogo.
O comando `checksum` calcula o hash do arquivo solicitado sem ler um catálogo;
cabe ao operador fornecer uma fonte cujo acesso esteja autorizado.

`validate` retorna um relatório JSON em stdout: versão do relatório, `valid`,
`source_count`, `verified_sources` e `issues`. Cada problema contém `field`, `code`
e mensagem em pt-BR. Índices em `sources[N]` começam em zero. `verified_sources`
conta entradas cujos metadados e checksum passaram nesta execução; um catálogo com
qualquer problema permanece inválido, mesmo havendo fontes verificadas.

| Código de saída | Significado |
| --- | --- |
| `0` | Validação sem erros, checksum calculado ou ajuda exibida. |
| `1` | Catálogo inválido, erro de leitura/JSON, checksum divergente ou falha ao escrever resultado. |
| `2` | Uso incorreto da CLI, comando/flag desconhecido ou quantidade negativa. |

Erros de uso, abertura ou decodificação são escritos em stderr. Problemas de domínio
e de arquivos de fontes aparecem no relatório JSON. Sem `--require-count`, aceita-se
qualquer quantidade não vazia; **o aceite de M1 exige explicitamente nove fontes**.
`go run` pode encapsular o código de saída do programa; para automações que precisam
dos códigos exatos, compile com `go build -o bin/alexandria ./cmd/alexandria`.

## Verificação e limites

```sh
go vet ./...
go test -race -count=1 ./...
```

Os testes geram fontes sintéticas em diretórios temporários e cobrem metadados,
permissões ausentes, JSON ambíguo, duplicidades, arquivos ausentes/vazios, checksums,
caminhos, repetibilidade, preservação e códigos de saída. O workflow
[Go validation](../.github/workflows/ci.yml) executa os testes e valida o exemplo;
o acervo privado nunca é necessário na CI.

A validação calcula SHA-256 por streaming sobre arquivos regulares não vazios.
Fontes com metadados ou autorização inválidos não são abertas. O formato é uma
declaração: não há inspeção de assinatura, parser de PDF/EPUB ou extração nesta
história. Validar um arquivo `txt` de teste não implementa um extrator de texto ou
Markdown. Mantenha os arquivos estáveis durante a verificação; o catálogo não cria
snapshots nem bloqueia alterações concorrentes no acervo.

Relatórios de erro podem conter caminhos locais. Antes de publicar evidências,
revise caminhos, permissões e conteúdo; PDFs, EPUBs e documentos protegidos devem
continuar fora do Git. A validação sintética não conclui #11: ainda são necessários
os nove livros reais, metadados e permissões revisadas pelo responsável.

## Acervo privado preparado nesta execução

Os nove PDFs informados pelo usuário foram cadastrados localmente em
`data/catalog.json`, com IDs estáveis, SHA-256 e metadados bibliográficos conferidos.
`data/catalog-provenance.json` registra os localizadores usados na conferência,
divergências com nomes de arquivo e limitações da inspeção. Esses arquivos são
privados e ignorados pelo Git; não contêm o texto integral dos livros.

Os hashes foram calculados pela CLI Go e conferidos independentemente. A leitura
pontual de metadados e páginas preliminares utilizou o pypdf já instalado, sem
adicioná-lo como dependência do projeto ou implementar a extração de M1-02.

`data/permissions-declaration.json` registra a declaração do responsável pelo
acervo: "livre para uso não comercial. Apenas estudo e pesquisa." As nove fontes
referenciam esse registro para processamento local, sem atribuir uma licença
padronizada ou autorização de redistribuição.

Após o registro, `data/catalog-validation.json` retorna `valid: true`,
`source_count: 9`, `verified_sources: 9`, `issues: []` e código 0 para
`--require-count 9`. Duas execuções produziram resultado idêntico. O relatório
anterior à declaração foi preservado em `data/catalog-validation-before-declaration.json`.
A validação confirma o registro dos direitos declarados e a integridade dos
arquivos; não constitui análise jurídica. A entrega de M1-01 foi aprovada na
[revisão técnica local](reviews/M1-01.md), sem conclusão do restante de M1.
