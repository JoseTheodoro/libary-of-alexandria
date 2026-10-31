# Extração local de PDF — M1-02

O comando `alexandria extract` valida as fontes do catálogo, executa um adaptador
Python local e grava eventos JSONL e manifesto privados. Go valida o contrato,
localizadores e contagens antes da publicação atômica. Não há banco nem busca nesta
etapa. O pypdf 6.20.1 é uma dependência especializada, com licença BSD-3-Clause;
Python 3.12.3 foi utilizado na execução local.

```sh
python3 -m pip install -r adapters/pdf/requirements.txt
go build -o bin/alexandria ./cmd/alexandria
bin/alexandria extract --catalog data/catalog.json --source ID --mode layout --output extracted
bin/alexandria extract --catalog data/catalog.json --all --mode plain --output extracted
python3 -m unittest discover -s adapters/pdf -p 'test_*.py'
go test -race -count=1 ./...
```

Selecione `--source` ou `--all`; `--python`, `--adapter`, `--timeout` e
`--max-record-bytes` são configuráveis. Saída JSON informa estado por fonte;
o código 0 indica execução completa, 1 indica erro ou parcialidade e 2 uso inválido.
Não publique as saídas ou caminhos do acervo privado.

O adaptador recebe JSON explícito em stdin e emite JSONL UTF-8 em stdout. Os eventos
`document_start`, `section`, `diagnostic`, `unit` e `document_end` são versionados
e vinculados ao ID e SHA-256 da fonte. Unidades representam páginas físicas base 1;
offsets são bytes UTF-8 do texto extraído, início inclusivo e fim exclusivo.
Seções vêm dos bookmarks do PDF quando disponíveis; lacunas e avisos são eventos.
Página sem texto não implica página em branco. Não há inferência de OCR ou de
numeração impressa. `complete` indica processamento sem erro de página, não prova
fidelidade integral visual ou recuperação de texto em imagens. Um documento sem
texto em todas as páginas é marcado como parcial com `no_text_extracted`.

Cada execução é identificada pela fonte/revisão, versão do contrato, hash do
adaptador, versões da biblioteca/runtime e modo. Resultados completos são validados
e reutilizados; artefatos corrompidos, resultados parciais e locks existentes são
recusados sem sobrescrita. Inspecione e preserve a tentativa antes de removê-la ou
movê-la manualmente para recuperação. Cada tentativa com falha conserva seu próprio
diagnóstico local, incluindo causa e indicação de truncamento dos logs quando houver.

Um piloto selecionou 27 páginas e comparou modos `plain`/`layout`, além de usar
PDFKit como referência independente de texto. A seleção, os resultados e as
limitações ficam em `data/extraction-pilot.json` e `data/extraction-quality-audit.json`.
Essas comparações são indicativas, não demonstram fidelidade integral.

Uma página com diagramas apresentou ambiguidade de ordem de leitura. Outro livro
fragmentava palavras em `plain` e passou a usar `layout` após comparação local.
Em um terceiro, `layout` falhou na primeira página enquanto `plain` perdeu
indentação numa amostra de código. O adaptador usa `layout` e, somente quando
uma página falha nesse modo, tenta `plain` para aquela página e registra
`layout_fallback` com o localizador. Falha em ambos resulta em estado parcial.
As tentativas anteriores são preservadas; nenhum conteúdo é corrigido por inferência.

Uma busca adicional identificou páginas candidatas de código em sete livros;
duas não tiveram candidato nos primeiros 250 páginas inspecionadas. Os resultados
de comparação da versão final estão em `data/extraction-acceptance-audit.json`;
candidatos e métricas
não substituem avaliação visual de todas as páginas nem comprovam que os demais
livros não contenham código. Ambiguidades e páginas sem texto exigem atenção na
revisão de qualidade e no uso posterior dos artefatos por #13.

## Evidências da execução final

Os nove PDFs foram processados em 4.214 unidades de página, com nove resultados
tecnicamente completos e sem erros de página. Foram registrados 60 eventos de
página sem texto, sete avisos do parser, um fallback de `layout` para `plain` e um
aviso de estrutura indisponível. A segunda execução verificou e reutilizou os nove
artefatos sem criar novas cópias completas.

A auditoria privada vinculada aos hashes finais compara 34 amostras com PDFKit:
27 iniciais e sete candidatas adicionais de código. Uma amostra apresentou
divergência de ordem acima do limiar indicativo usado na comparação; a página
contém diagramas e exige cuidado na interpretação. A análise de tokens não prova
fidelidade visual ou semântica de todo o acervo. Amostras de código dos três livros
verificados visualmente preservaram identificadores e indentação observáveis.

`data/extraction-acceptance-run.json`, `data/extraction-acceptance-repeat.json` e
`data/extraction-acceptance-audit.json` registram execução, repetição e auditoria.
Os eventos e manifestos privados são a entrada candidata para #13; o importador
deverá considerar também a auditoria de qualidade e seus avisos, sem tratar
`complete` como garantia de fidelidade integral.

Arquivos em `data/` e `extracted/` são privados e ignorados pelo Git. O uso informado
pelo responsável é não comercial, apenas estudo e pesquisa. Não envie PDFs, textos
extraídos, relatórios completos ou imagens de páginas ao GitHub/CI.
