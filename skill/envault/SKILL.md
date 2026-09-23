---
name: envault
description: Carrega variáveis de ambiente do cofre local envault para gerar o .env de um projeto. Use SEMPRE que o usuário for iniciar, configurar ou rodar um projeto que precise de .env, mencionar variáveis de ambiente, segredos, credenciais, chaves de API, .env.example, ou pedir para "carregar"/"usar" uma env pelo nome, mesmo que não cite o envault.
---

# envault

Você monta o `.env` de projetos a partir de envs salvas no cofre `envault`. Você **nunca vê valores**: trabalha só com nomes de envs, descrições e nomes de chaves.

## Regras invioláveis

- Os únicos comandos do envault que você roda sem pedir permissão são `envault --version`, `envault list`, `envault show` e `envault plan`. Qualquer outro comando do envault exige confirmação explícita do usuário na conversa atual.
- NUNCA rode `envault get`, `envault shell` nem use `--show`.
- NUNCA leia arquivos `.env` (cat, Read, grep). Pode ler `.env.example`.
- NUNCA rode `envault new`/`edit` (são interativos). Se faltar uma env, peça ao usuário para criá-la com `envault` (TUI) ou `envault new <nome>`.
- NUNCA rode `envault load` sem confirmação explícita do usuário **para aquele carregamento específico**, na conversa atual.
- Não altere o `.gitignore` sem perguntar.

## Fluxo

### 0. Pré-checagem
Rode `envault --version`. Se falhar, explique como instalar e pare. Se `envault list` sair com código 4, peça ao usuário para rodar `envault init`.

### 1. O usuário disse quais envs usar?
- **Nomes exatos:** confirme que existem com `envault list --json` e vá para o passo 3.
- **Referência vaga** ("a do stripe"): rode `envault list --json --search <termo>`. Uma candidata clara: confirme com o usuário. Várias: passo 2.
- **Nada:** passo 2.

### 2. Descoberta
1. Entenda o que o projeto precisa: `.env.example`, dependências (`package.json`, `go.mod`, `requirements.txt`, `pyproject.toml`...), `docker-compose.yml`.
2. Rode `envault list --json`.
3. Cruze nome, descrição, tags e nomes de chaves com as necessidades do projeto. Para detalhar uma candidata, rode `envault show <nome> --json`, que mostra só os nomes das chaves.
4. Apresente as candidatas (nome, descrição, quais chaves do template cada uma cobre) e peça para o usuário escolher **uma ou mais**, e em que ordem. Use a ferramenta de perguntas com seleção múltipla se disponível; senão, lista numerada.

### 3. Plano
Rode `envault plan <envs...> --out <destino>`. Leia do JSON: chaves e origem, `conflicts`, `missing`, `target.exists`, `target.gitignored`.

### 4. Permissão (sempre)
Mostre um resumo e pergunte se pode continuar:
- envs na ordem de precedência (a última vence);
- total de chaves e conflitos (qual env vence);
- chaves faltando do template;
- se o destino existe: pergunte **sobrescrever** (`--force`) ou **mesclar** (`--merge`).

### 5. Carregar
Só após o "sim": `envault load <envs...> --out <destino> [--force|--merge]`.
Depois:
- se `gitignored` for `false`, pergunte se pode adicionar o arquivo ao `.gitignore`;
- informe os **nomes** das chaves faltantes e sugira criá-las no envault.

## Códigos de saída

| código | significado |
|---|---|
| 3 | env não encontrada |
| 4 | cofre não inicializado |
| 5 | destino existe |
| 6 | falha ao decifrar |
| 7 | validação |

Explique o erro ao usuário; não tente contornar.
