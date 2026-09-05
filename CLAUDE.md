# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

A toy Redis clone built for the CodeCrafters "Build Your Own Redis" challenge. Written in Go (module `github.com/codecrafters-io/redis-starter-go`, go 1.22, no external dependencies). Progress is submitted by pushing to the CodeCrafters remote; tests run server-side, so there is no local test suite.

## Commands

- Run the server locally: `./your_program.sh` — builds `app/*.go` to `/tmp/codecrafters-build-redis-go` and execs it.
- Run with args (e.g. port): `./your_program.sh --port 6380`
- Verify it builds: `go build ./...`
- `gofmt`: run `gofmt -w app/` to keep formatting consistent (the submit flow is sensitive to nothing, but tidiness helps diffs).
- Submit to CodeCrafters: commit and `git push origin master` (recent history uses the message `codecrafters submit [skip ci]`). Remote compile settings live in `codecrafters.yml` and `.codecrafters/compile.sh` — the launch behavior is duplicated in `your_program.sh`.

## Architecture

Two packages:

- **`app/server.go`** (`package main`) — everything lives here: `main()` listens on TCP `0.0.0.0:6379` (or `--port`), spawns one goroutine per connection via `handleClient`, and each connection is served in a read→parse→dispatch→write loop.
- **`app/resp/resp.go`** (`package resp`) — RESP wire-format *encoding* helpers only: `EncodeBulkString`, `EncodeArray`, `EncodeInt`, plus constants `PONG`, `OK`, `NULL`.

### In-memory store

A single shared `Store` struct (created once in `main`, passed by pointer to every handler) holds three maps guarded by one `*sync.Mutex`:
- `data map[string]redisValue` — string values with an optional `expiresAt` unix-ms timestamp (SET ... PX). Expiry is checked lazily at GET time, never swept.
- `list map[string][]string` — list values for RPUSH/LPUSH/LRANGE/LLEN/LPOP/BLPOP.
- `listChans map[string][]chan struct{}` — intended to wake blocked BLPOP callers.

### Command dispatch pattern

Command handlers are plain functions matching `commandHandler func(args []string, s *Store) (string, error)`, registered in the `handlers` map, keyed by uppercase command name. `handleCommands` uppercases `args[0]`, looks up the handler, and calls it. Every handler returns a **fully-encoded RESP string** (via the `resp` package) or an error. New commands = write a `handleX` function, add it to the registry, and nothing else.

### RESP parsing

`RESP` is a struct wrapping the raw request bytes plus a cursor (`idx`). `parseResp` recursively walks it:`*` starts an array, `$` a bulk string. Parsing happens on whatever a single `client.Read` returns — the 512-byte read buffer means a command must fit in one read (no request-coalescing/streaming), which is a known limitation for pipelined or very large inputs. `interfaceToString` converts the parsed `[]interface{}` into `[]string`, trimming blanks.

### CONFIG GET

Config values come from process arguments rather than stored config: `getArgs` scans `os.Args` for `--<param> <value>` pairs (e.g. `--dir`, `--dbfilename` from the CodeCrafters runner) and returns the value or an error.

## Notes on current state

- List support (RPUSH/LPUSH/LRANGE/LLEN/LPOP/BLPOP) is recently added. BLPOP's wake-up path is not wired end-to-end: pushing to a list never signals `listChans`, so a blocked BLPOP with a positive timeout only wakes on its timeout timer. The BLPOP timeout path uses `time.After(time.Second)` rather than the requested timeout.