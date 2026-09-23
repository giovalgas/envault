# envault

Cofre local e criptografado de variáveis de ambiente. Guarda credenciais em **envs** nomeadas (por exemplo `postgres-local`, `stripe-test`, `aws-dev`), combina uma ou mais envs e gera o `.env` de um projeto, sem que os valores passem pelo chat do Claude.

Três interfaces sobre o mesmo núcleo:

- **TUI** (Bubble Tea): para uso interativo, visualizar, criar, editar e carregar várias envs de uma vez.
- **CLI** (Cobra): para scripts e para o Claude, com saída previsível, `--json` e códigos de saída estáveis, nunca interativa quando `--json` está ligado.
- **Skill do Claude Code**: ensina o Claude a descobrir, sugerir e carregar envs sempre pedindo permissão e sem nunca ver os valores.

O cofre é global: as envs são reaproveitadas entre projetos. Sincronização entre máquinas e compartilhamento com time estão fora do escopo atual.

## Instalação

### Binário da release

Baixe o binário para o seu sistema operacional na página de releases do repositório e coloque-o em algum diretório do `PATH`.

### `go install`

```
go install github.com/giovalgas/envault/cmd/envault@latest
```

Exige Go 1.23 ou mais novo.

### Homebrew

Ainda não publicado. Quando existir, será um `tap` opcional, sem substituir os dois métodos acima.

## Uso rápido

```
envault init
envault new postgres-local --description "Postgres local via docker-compose" --tags db,local
envault list
envault plan postgres-local
envault load postgres-local --out .env
```

Rodar `envault` sem argumentos, com o terminal em modo interativo, abre a TUI. Sem terminal interativo, mostra o help.

## TUI

### Telas

1. **Lista** (tela inicial): duas colunas. À esquerda, as envs com nome, número de chaves, última edição e marcador de seleção. À direita, descrição, tags e chaves da env focada, com valores sempre mascarados.
2. **Detalhe**: tabela de chaves e valores mascarados, em tela cheia.
3. **Montagem** (envs marcadas): lista ordenável, prévia do resultado com origem de cada chave e indicador de conflito, checklist de chaves do `.env.example` quando existir, escolha de destino e, se ele já existir, escolha entre sobrescrever, mesclar ou cancelar.
4. **Confirmações**: modal reutilizável para apagar, sobrescrever e revisar o diff pós-edição.

### Atalhos

| Tecla | Ação |
|---|---|
| `↑/↓`, `k/j` | Navegar |
| `space` | Marcar ou desmarcar env |
| `l` | Carregar as envs marcadas (montagem) |
| `enter` | Abrir detalhe |
| `v` | Revelar ou ocultar o valor da linha focada |
| `y` | Copiar o valor para a área de transferência |
| `n` | Criar nova env (abre o editor) |
| `e` | Editar env (abre o editor) |
| `c` | Duplicar env |
| `r` | Renomear env |
| `i` | Importar um `.env` |
| `d` | Apagar, com confirmação digitando o nome |
| `K`/`J` | Reordenar na tela de montagem |
| `/` | Filtrar por nome, descrição, tag ou nome de chave |
| `?` | Ajuda com todos os atalhos |
| `q`, `ctrl+c` | Sair ou voltar |

Nenhuma tela da TUI mostra um valor sem confirmação explícita de `v`, e copiar com `y` nunca imprime o valor na tela.

## Referência da CLI

Regras gerais: mensagens para humanos vão em stderr, dados em stdout. Com `--json`, erros também saem em stdout como `{"schema_version":1,"error":{"code":...,"message":"..."}}`. Nenhum comando de leitura (`list`, `show`, `plan`) imprime valores de variáveis.

| Comando | Descrição |
|---|---|
| `envault init` | Cria o diretório, a chave e o cofre vazio. Idempotente. |
| `envault list [--search termo] [--tag t] [--json]` | Lista envs: nome, descrição, tags, nomes das chaves e `updated_at`. Sem valores. `--search` casa com nome, descrição, tags e nomes de chaves, sem diferenciar maiúsculas de minúsculas. |
| `envault show <env> [--json]` | Metadados e nomes das chaves de uma env. Sem valores. |
| `envault get <env> <CHAVE>` | Imprime um valor. Para humanos e scripts. |
| `envault set <env> KEY=VALUE...` | Define uma ou mais chaves. `envault set <env> KEY`, sem `=`, lê o valor do stdin. |
| `envault unset <env> KEY...` | Remove uma ou mais chaves. |
| `envault new <env> [--description d] [--tags a,b] [--from-file f]` | Cria uma env. Sem `--from-file`, abre o editor no formato descrito abaixo. |
| `envault edit <env>` | Abre a env existente no editor. |
| `envault import <env> <arquivo> [--description d]` | Cria ou substitui uma env a partir de um arquivo `.env`. |
| `envault rename <atual> <novo>` | Renomeia uma env. |
| `envault copy <origem> <destino>` | Duplica uma env. |
| `envault delete <env> [--yes]` | Remove uma env. Sem `--yes`, exige terminal interativo e confirmação digitando o nome. |
| `envault plan <env>... [--out .env] [--template f] [--no-template] [--only-template]` | Mostra em JSON o que `load` faria. Sempre JSON, nunca grava nada. |
| `envault load <env>... [--out .env] [--force \| --merge] [--template f] [--no-template] [--only-template] [--json]` | Grava o arquivo de destino. Sem `--force` nem `--merge`, recusa quando o destino já existe. |
| `envault exec -e a,b [--template f] [--no-template] [--only-template] [--] <comando...>` | Roda um comando com as variáveis combinadas injetadas no ambiente, sem criar arquivo. O `--` antes do comando é opcional. |
| `envault shell <env>... [--shell bash\|zsh\|fish]` | Imprime uma linha `export` por variável, para uso com `eval`. Sem `--shell`, detecta o dialeto pela variável `$SHELL`; se não reconhecer, usa `bash`. |
| `envault skill install [--dir d]` | Instala a Skill em `~/.claude/skills/envault/SKILL.md`, ou no diretório passado em `--dir`. |
| `envault key migrate` | Move a chave do arquivo para o keychain do sistema operacional. |
| `envault completion`, `envault --version` | Padrão do Cobra; a versão é definida em tempo de build. |

### Combinação de envs e template

`plan`, `load`, `exec` e `shell` combinam envs na ordem dada na linha de comando: a última env vence em caso de chave repetida. As flags de template são compartilhadas por esses comandos:

- `--template <arquivo>`: usa o arquivo indicado como template de chaves esperadas, em vez de detectar automaticamente.
- Sem `--template` e sem `--no-template`, o comando procura `.env.example` no diretório atual.
- `--no-template`: ignora qualquer template, inclusive o `.env.example` detectado automaticamente.
- `--only-template`: grava só as chaves listadas no template; exige que exista um template, seja explícito ou detectado.

Com template, a saída segue a ordem do template. Chaves com valor padrão no template e sem valor em nenhuma env mantêm o padrão. Chaves sem valor em lugar nenhum entram em `missing`. Chaves das envs que não estão no template entram no final da lista, a menos que `--only-template` esteja ligado.

`--merge` em `load` preserva a ordem do arquivo de destino existente: mantém chaves que só existem nele, atualiza as que vêm das envs e acrescenta as novas no final. `--force` e `--merge` juntos são erro de uso. Sem nenhuma das duas flags e com o destino já existindo, `load` sai com o código 5.

Depois de gravar, `load` verifica se o destino está coberto pelo `.gitignore` do repositório atual (via `git check-ignore`) e avisa em stderr, e no JSON quando `--json` está ligado, sem nunca alterar o `.gitignore` sozinho.

### JSON de `list`

```json
{
  "schema_version": 1,
  "envs": [
    {
      "name": "postgres-local",
      "description": "Postgres local via docker-compose, porta 5432",
      "tags": ["db", "local"],
      "keys": ["DATABASE_URL", "DATABASE_POOL"],
      "updated_at": "2026-09-20T18:30:00Z"
    }
  ]
}
```

### JSON de `plan`

```json
{
  "schema_version": 1,
  "envs": ["postgres-local", "stripe-test"],
  "target": { "path": ".env", "exists": true, "gitignored": true },
  "template": { "path": ".env.example", "found": true },
  "keys": [
    { "key": "DATABASE_URL", "from": "postgres-local", "shadows": [] },
    { "key": "APP_URL", "from": "stripe-test", "shadows": ["postgres-local"] },
    { "key": "PORT", "from": null, "default": true }
  ],
  "conflicts": ["APP_URL"],
  "missing": ["SENTRY_DSN"],
  "extra": []
}
```

`load --json` devolve o mesmo formato de `plan`, com o campo adicional `"mode"` (`"created"`, `"overwritten"` ou `"merged"`).

### Códigos de saída

| Código | Significado |
|---|---|
| 0 | Sucesso |
| 1 | Erro genérico |
| 2 | Uso incorreto de flags ou argumentos |
| 3 | Env não encontrada |
| 4 | Cofre não inicializado |
| 5 | Arquivo de destino já existe, use `--force` ou `--merge` |
| 6 | Falha ao decifrar, chave errada ou cofre corrompido |
| 7 | Erro de validação, parse do `.env` ou nome inválido |
| 130 | Cancelado pelo usuário |

`envault exec` propaga o código de saída do processo filho em vez de usar essa tabela; quando o filho é encerrado por sinal, o código é 128 mais o número do sinal.

## Formato do arquivo no editor

`new`, `edit` e a TUI abrem o editor do usuário (`$VISUAL`, depois `$EDITOR`, depois o padrão do sistema operacional) com um arquivo neste formato:

```
# @description: Postgres local via docker-compose, porta 5432
# @tags: db, local
#
# Formato: KEY=VALUE, uma por linha. Linhas com # são comentários.
# Salve e feche o editor para aplicar. Deixe o arquivo vazio para cancelar.

DATABASE_URL=postgres://user:pass@localhost:5432/app
DATABASE_POOL=10
```

Regras do parser:

- As linhas `# @description:` e `# @tags:` viram metadados da env; outros comentários são ignorados.
- Aceita o prefixo `export `, aspas simples (valor literal), aspas duplas (com `\n`, `\t`, `\"` e `\\`) e comentário inline com ` #` em valores sem aspas.
- Erros de parse sempre citam o número da linha.

Ao salvar e fechar o editor:

- Arquivo vazio, ignorando comentários, numa env nova: a criação é cancelada.
- Conteúdo com o mesmo hash do original: nada é gravado.
- Erro de parse: o editor reabre com o conteúdo digitado e o erro como comentário no topo, por exemplo `# ERRO linha 4: chave inválida "1KEY"`. Salvar vazio nesse momento desiste da edição.
- Conteúdo válido: é calculado um diff por chave (adicionadas, removidas, alteradas, nunca os valores), pedida confirmação e só então gravado.

## Modelo de ameaça

O envault protege contra:

- Vazamento do arquivo do cofre isolado: sem a chave, `vault.enc` é inútil.
- Commit acidental de segredos: `load` avisa quando o destino não está no `.gitignore`.
- Segredos aparecendo na conversa do Claude: a Skill e as regras de permissão bloqueiam leitura de valores.

O envault não protege contra:

- Alguém com acesso à conta de usuário da máquina: enquanto a chave estiver em arquivo (antes da migração para o keychain), quem lê a home lê a chave.
- Malware local rodando com os privilégios do usuário.
- Undo ou swap de arquivo criados por alguns editores durante a edição.
- Zeração de memória: a linguagem Go não garante isso, então valores decifrados podem permanecer na memória do processo por um tempo.

## Skill do Claude Code

A Skill ensina o Claude a montar o `.env` de um projeto sem nunca ver valores de variáveis: ele trabalha só com nomes de envs, descrições e nomes de chaves, e sempre pede confirmação explícita antes de gravar qualquer arquivo.

Instalação:

```
envault skill install
```

Instala em `~/.claude/skills/envault/SKILL.md` por padrão. Use `--dir <diretório>` para instalar em outro lugar.

A Skill só funciona onde o Claude executa comandos na máquina do usuário (Claude Code ou Cowork), não no claude.ai web.

### Regras de permissão recomendadas

Adicione ao arquivo de permissões do Claude Code (confira o caminho e a sintaxe atuais na documentação do Claude Code):

```json
{
  "permissions": {
    "allow": [
      "Bash(envault --version)",
      "Bash(envault list:*)",
      "Bash(envault show:*)",
      "Bash(envault plan:*)"
    ],
    "ask": [
      "Bash(envault load:*)",
      "Bash(envault exec:*)"
    ],
    "deny": [
      "Bash(envault get:*)",
      "Bash(envault shell:*)",
      "Read(./.env)",
      "Read(./.env.local)"
    ]
  }
}
```

O bloco acima lista dois arquivos específicos em vez de um padrão com curinga, porque um curinga sobre `.env` também bloquearia a leitura de `.env.example`, e o Claude precisa ler esse arquivo para descobrir quais chaves um projeto espera. Se o seu projeto tiver outras variações de nome de arquivo de segredo, como um ambiente de produção ou de homologação, adicione uma entrada de negação específica para cada uma.

## Migração da chave para o keychain

Por padrão, a chave de criptografia do cofre fica num arquivo `0600` na home do usuário. Para movê-la para o keychain do sistema operacional:

```
envault key migrate
```

O comando só remove o arquivo da chave depois de confirmar que consegue lê-la de volta do keychain.

## Variáveis de ambiente

| Variável | Efeito |
|---|---|
| `ENVAULT_HOME` | Sobrescreve o diretório do cofre (`vault.enc`, `key` e `vault.lock`). Sem ela, usa `os.UserConfigDir()/envault`. |
| `ENVAULT_KEY` | Chave de criptografia em base64, sobrescrevendo qualquer implementação de armazenamento de chave. |
| `ENVAULT_DEBUG` | Com valor `1`, `true`, `yes` ou `on`, liga o log de depuração da TUI em arquivo. |
