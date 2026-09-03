# mkgo

`mkgo` creates a Go project the way you would create it by hand — a directory,
a scaffold, `go.mod`, a git repository, a GitHub repository and the first push —
in one command.

```sh
mkgo hello-world
```

```
🚀 mkgo 1.0.0
create a ready-to-push Go project in one command

╭─ plan ──────────────────────────────────────────────────────╮
│ project    hello-world                                      │
│ directory  ./hello-world                                    │
│ module     github.com/octocat/hello-world                   │
│ layout     cli                                              │
│ license    MIT                                              │
│ branch     main                                             │
│ repository octocat/hello-world (public)                     │
│                                                             │
│ steps      files · go.mod · git · commit · github · push    │
╰─────────────────────────────────────────────────────────────╯

✅ authenticated as octocat ($GITHUB_TOKEN)
✅ octocat/hello-world is available
✅ created ./hello-world
✅ wrote 10 files
✅ formatted the code and tidied go.mod
✅ initialized a repository on branch main
✅ committed the initial tree
✅ created octocat/hello-world 🌍
✅ origin ➜ https://github.com/octocat/hello-world.git
✅ pushed main to origin

╭─ ✨ hello-world is ready ────────────────────────╮
│ path       ./hello-world                         │
│ module     github.com/octocat/hello-world        │
│ files      10                                    │
│ repository https://github.com/octocat/hello-world│
│ elapsed    2.4s                                  │
╰──────────────────────────────────────────────────╯
```

## Features

- **One command, whole setup** — scaffold, `go.mod`, `git init`, initial commit,
  GitHub repository, remote and push.
- **Native GitHub API** — repositories are created over the REST API with
  `net/http`. The `gh` CLI is never invoked and is not required.
- **Four layouts** — `cli`, `lib`, `api` and `minimal`, each with tests that
  pass out of the box.
- **Everything is optional** — `--no-git`, `--no-gh`, `--no-mod` and a switch
  for every other step.
- **Readable output** — spinners, colors and emoji, all of which can be turned
  off with `--no-color` and `--no-emoji` (and `$NO_COLOR`).
- **Zero dependencies** — the standard library only.

## Install

```sh
go install github.com/x-chunk/mkgo@latest
```

Or build from source:

```sh
git clone https://github.com/x-chunk/mkgo.git
cd mkgo
make install          # installs into ~/.local/bin
```

Requirements: Go 1.24 or newer to build. `git` is needed for the git steps, and
a GitHub token for the GitHub step; everything else works without them.

## Authentication

`mkgo` looks for a token in this order and stops at the first hit:

1. `--token <value>`
2. `$MKGO_GITHUB_TOKEN`, `$GITHUB_TOKEN`, `$GH_TOKEN`
3. `~/.config/gh/hosts.yml` — the file the GitHub CLI stores its token in. It is
   read as a plain file; no external command is ever executed.

The token needs the `repo` scope (or `Administration: read and write` for a
fine-grained token). Without a token `mkgo` prints a warning, skips the GitHub
step and still creates everything locally.

The token is never written into the git remote URL. Pushes pass it through a
one-shot credential helper, so it does not end up in `.git/config`, in the
process arguments or in any error message.

## Usage

```
mkgo [flags] <project-name>
```

The name can also be a path — `mkgo tools/scanner` creates `./tools/scanner`
and names the project `scanner`. Flags may appear before, after or around the
name.

### Layouts

| Layout    | Contents                                                          |
| --------- | ----------------------------------------------------------------- |
| `cli`     | `main.go` plus `internal/app` with flag parsing and tests (default) |
| `lib`     | a package named after the project, with table-driven tests         |
| `api`     | an HTTP service with graceful shutdown, `slog` and a health check   |
| `minimal` | a single `main.go`                                                 |

Every layout also gets `README.md`, `LICENSE`, `Makefile`, `.gitignore`,
`.editorconfig` and a GitHub Actions workflow, unless you turn them off.

### Flags

**Project**

| Flag                   | Description                                          |
| ---------------------- | ---------------------------------------------------- |
| `-d, --dir <path>`     | target directory (default `./<name>`)                |
| `-m, --module <path>`  | module path (default `github.com/<owner>/<name>`)    |
| `-t, --layout <kind>`  | `cli`, `lib`, `api` or `minimal`                     |
| `--desc <text>`        | project description, also used on GitHub             |
| `--author <name>`      | copyright holder (default: `git config user.name`)   |
| `--license <id>`       | `mit`, `apache-2.0`, `bsd-3-clause`, `isc`, `unlicense`, `none` |
| `--go <version>`       | `go` directive for `go.mod`                          |
| `-b, --branch <name>`  | initial branch (default `main`)                      |

**GitHub**

| Flag               | Description                                    |
| ------------------ | ---------------------------------------------- |
| `--private`        | create a private repository                    |
| `--org <name>`     | create it inside an organization               |
| `--topics <list>`  | comma separated repository topics              |
| `--token <token>`  | token to authenticate with                     |
| `--host <host>`    | GitHub Enterprise host (API at `/api/v3`)      |
| `--ssh`            | use the SSH remote instead of HTTPS            |
| `--remote <name>`  | remote name (default `origin`)                 |

**Skipping steps**

| Flag                | Description                             |
| ------------------- | --------------------------------------- |
| `--no-git`          | do not run `git init`                   |
| `--no-gh`           | do not create the GitHub repository     |
| `--no-mod`          | do not create `go.mod`                  |
| `--no-commit`       | do not create the initial commit        |
| `--no-push`         | do not push                             |
| `--no-ci`           | no GitHub Actions workflow              |
| `--no-readme`       | no `README.md`                          |
| `--no-makefile`     | no `Makefile`                           |
| `--no-gitignore`    | no `.gitignore`                         |
| `--no-editorconfig` | no `.editorconfig`                      |
| `--no-tidy`         | do not run `gofmt` and `go mod tidy`    |
| `--no-config`       | ignore the config file                  |

**Output and behavior**

| Flag             | Description                                        |
| ---------------- | -------------------------------------------------- |
| `--no-color`     | no ANSI colors (also honors `$NO_COLOR`)           |
| `--no-emoji`     | ASCII markers instead of emoji                     |
| `--no-spinner`   | no spinner animation                               |
| `-q, --quiet`    | errors only                                        |
| `-v, --verbose`  | print each generated file and extra detail         |
| `-n, --dry-run`  | print the plan and exit without touching anything  |
| `-y, --yes`      | skip the confirmation prompt                       |
| `-f, --force`    | scaffold into a directory that already has files   |
| `--config <path>`| use a specific config file                         |
| `-h, --help`     | show help                                          |
| `-V, --version`  | print the version                                  |

Colors, emoji and the spinner switch themselves off when the output is not a
terminal, so piping to a file or a CI log stays clean.

### Examples

```sh
mkgo hello-world                          # the full flow
mkgo api-gateway -t api --private         # a private HTTP service
mkgo toolkit -t lib --no-gh               # a library, local only
mkgo scratch --no-git --no-gh --no-mod    # only the files
mkgo demo --dry-run                       # show the plan, change nothing
mkgo infra --org acme --topics go,infra   # inside an organization
mkgo demo --no-color --no-emoji --quiet   # plain output for scripts
```

## Configuration

Defaults can live in `$XDG_CONFIG_HOME/mkgo/config.json`
(`~/.config/mkgo/config.json`). Flags always win over the file.

```json
{
  "author": "Jane Doe",
  "license": "apache-2.0",
  "layout": "cli",
  "branch": "main",
  "module_prefix": "github.com/janedoe",
  "org": "",
  "private": false,
  "ssh": true,
  "topics": ["go"]
}
```

`$MKGO_CONFIG_DIR` overrides the directory, `--config` overrides the full path
and `--no-config` ignores the file entirely.

## Environment variables

| Variable                                | Effect                              |
| --------------------------------------- | ----------------------------------- |
| `MKGO_GITHUB_TOKEN`, `GITHUB_TOKEN`, `GH_TOKEN` | GitHub credential           |
| `GITHUB_API_URL`                        | override the API endpoint           |
| `MKGO_CONFIG_DIR`, `XDG_CONFIG_HOME`    | where the config file lives         |
| `GH_CONFIG_DIR`                         | where `hosts.yml` is looked up      |
| `NO_COLOR`, `FORCE_COLOR`, `TERM`       | color detection                     |

## Exit codes

| Code | Meaning                                     |
| ---- | ------------------------------------------- |
| `0`  | success, or the user declined the prompt    |
| `1`  | the run failed                              |
| `2`  | invalid command line                        |

## How it works

1. **Resolve** — flags, config file and git config are merged into one plan.
2. **Preflight** — the token is verified with `GET /user` and the repository
   name is checked with `GET /repos/{owner}/{repo}`, before anything is written.
3. **Plan** — the plan and the file tree are printed, and confirmed when a
   GitHub repository is about to be created.
4. **Scaffold** — files are rendered from embedded templates, then `gofmt` and
   `go mod tidy` run over them.
5. **Git** — `git init`, `git add`, `git commit`.
6. **GitHub** — `POST /user/repos` or `POST /orgs/{org}/repos`, then the remote
   is wired up and the branch is pushed.

Nothing is written to disk until the preflight checks pass, so a taken
repository name or a bad token costs you nothing.

## Development

```sh
make check   # gofmt, go vet and go test -race
make build   # build bin/mkgo
make cover   # coverage report
```

The packages are:

```
main.go              entry point
internal/cli         flag parsing, plan resolution, orchestration, output
internal/scaffold    embedded templates, file rendering, licenses
internal/github      REST client, token resolution
internal/gitutil     git wrapper
internal/ui          colors, icons, spinner, boxes
internal/config      config file
```

## License

Apache-2.0. See [LICENSE](LICENSE).
