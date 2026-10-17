# g-shell

A small interactive shell written in Go. Built to explore how a shell turns a line of text into commands, connects processes with pipes, and manages an interactive terminal.

## Features

- Run external programs by name through `PATH` or by executable path.
- Parse single and double quotes, escaped characters, and shell variables declared with `declare`.
- Connect commands with multi-stage pipelines, including builtins.
- Redirect standard output and standard error, with overwrite and append modes.
- Complete command names, filenames, and directories with Tab, or register an external completion program.
- Inspect command history, read and write history files, and persist history through `HISTFILE`.
- Start external commands in the background and inspect them with `jobs`.

## Run locally

Requires **Go 1.26 or later** and a Unix-like system such as macOS or Linux.

```sh
git clone https://github.com/LightYagami674/g-shell.git
cd g-shell
go run ./app
```

To build a standalone executable:

```sh
go build -o g-shell ./app
./g-shell
```

The shell displays a `$ ` prompt. Type `exit` or press Ctrl+D to leave.

## Try it out

Enter these commands at the shell prompt:

```sh
echo "hello from g-shell"
pwd
type echo
declare greeting="hello world"
echo "$greeting"
echo "hello world" | wc -w
echo "first line" > notes.txt
echo "another line" >> notes.txt
ls /missing-directory 2> errors.txt
sleep 10 &
jobs
history 5
```

Use spaces around pipeline and redirection operators. Redirection currently supports one output or error destination at the end of each command.

## Builtin commands

| Command | Purpose |
| --- | --- |
| `echo [args...]` | Print arguments separated by spaces. |
| `pwd` | Print the current working directory. |
| `cd [path]` | Change directory; defaults to the home directory. |
| `type <name>...` | Identify a builtin or locate an executable. |
| `exit [status]` | Leave the shell with an optional exit status. |
| `history [count]` | Show all history entries or the most recent entries. |
| `history -r/-w/-a <path>` | Read, overwrite, or append a history file. |
| `jobs` | Show tracked background jobs. |
| `declare name=value` | Set a shell variable; use `declare -p name` to inspect it. |
| `complete -C <program> <command>` | Register an external program to supply argument completions. |
| `complete -p/-r <command>` | Inspect or remove a completion registration. |

To load history on startup and save it on exit, set `HISTFILE` before launching:

```sh
HISTFILE="$HOME/.g-shell_history" go run ./app
```

## Development

```sh
go test ./...
```

The implementation lives in `app/`: `main.go` runs the prompt loop, `parser.go` tokenizes input, and `execution.go` dispatches commands and pipelines. Builtins, completion, history, and background jobs each have their own source file.

This is a learning project with a growing subset of shell behavior. It does not implement full POSIX shell syntax: features such as globbing, command substitution, input redirection, and shell scripting control flow are not supported.

## Origins

Started as a solution to the [CodeCrafters Build Your Own Shell challenge](https://app.codecrafters.io/courses/shell/overview). The challenge configuration and `your_program.sh` launcher remain in the repository for continued submissions.
