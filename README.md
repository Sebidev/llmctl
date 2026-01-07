# llmctl

`llmctl` is a minimal, Unix-style command-line tool for interacting with Large Language Models (LLMs).

It is designed for **ask-only workflows**, where LLM output is treated like any other CLI program output:
plain text via `stdout`, suitable for piping, redirection, and version control.

No agents.  
No hidden state.  
No browser UI.  
No IDE lock-in.

---

## Philosophy

`llmctl` follows classic Unix principles:

- **Do one thing well**
- **Text in, text out**
- **Explicit context instead of implicit memory**
- **User stays in control**

LLMs are treated as **stateless text processors**, not autonomous agents.

---

## Features

- Ask LLMs directly from the command line
- Streamed output to `stdout`
- Explicit context via files
- Works with OpenAI-compatible APIs
- Easy integration with editors, pipes, and scripts
- No automatic code execution
- No filesystem side effects unless you explicitly redirect output

---

## Installation

Build from source (Go):

```bash
git clone https://github.com/sebidev/llmctl.git
cd llmctl
go get github.com/openai/openai-go/v3 
go build -o llmctl
````

Place the binary somewhere in your `PATH`.

---

## Configuration

`llmctl` reads configuration from environment variables:

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4.1-mini"
```

This allows easy switching between cloud providers and local OpenAI-compatible servers
(e.g. LM Studio, llama.cpp servers, etc.).

---

## Basic Usage

Simple prompt:

```bash
llmctl "explain systemd timers"
```

Redirect output to a file:

```bash
llmctl "hello world program in C++" > hello.cpp
```

Append to an existing document:

```bash
llmctl "add a summary section" >> notes.md
```

---

## Using Context Files

Provide explicit context using files:

```bash
llmctl --context notes.md "expand section 5"
```

Append the result back into the same file:

```bash
llmctl --context notes.md "expand section 5" >> notes.md
```

This makes the file itself the **source of truth** and conversation memory.

---

## Context Size Control

To avoid sending entire large files, limit context size:

```bash
llmctl --context notes.md --tail 2500 "continue this section"
```

Only the last N characters (or tokens, depending on implementation) are sent.

---

## Unix Pipelines

Because `llmctl` uses `stdout`, it works naturally with pipes:

```bash
cat error.log | llmctl "explain this error"
```

```bash
llmctl "list common nginx systemd units" | grep nginx
```

```bash
llmctl "generate markdown outline" >> draft.md
```

---

## Typical Use Cases

* Writing and extending Markdown documents
* Code generation and explanation
* Infrastructure notes and documentation
* Learning and research
* Shell-based workflows without browser or IDE integration

---

## What `llmctl` Is Not

* Not an autonomous agent
* Not an IDE plugin
* Not a chat application
* Not a code executor

If you want automation, use scripts.
If you want memory, use files.
If you want control, use `llmctl`.

---

## License

MIT License

---

## Final Note

`llmctl` is intentionally boring.

That is its strength.

