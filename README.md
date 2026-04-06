# pm-llm-wiki

A Go implementation of the [LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) by Andrej Karpathy, specialised for **project management** knowledge bases.

Drop a raw source file (transcript, SOW, schedule, invoice, etc.) into a client folder. The tool converts it, sends it to a local Ollama model, and the LLM incrementally builds and maintains a structured wiki — updating meeting notes, decisions, risks, deliverables, entity pages, and cross-references automatically.

---

## The core idea

Most approaches to LLMs and documents use RAG: upload files, retrieve chunks at query time, generate an answer. Nothing accumulates. Ask the same question next week and the LLM rediscovers the same answer from scratch.

This tool takes the approach Karpathy describes: the LLM **builds and maintains a persistent wiki** that sits between you and your raw sources. When you add a new source, the LLM reads it and integrates it into the existing wiki — updating relevant pages, cross-referencing entities, noting contradictions, and keeping everything consistent. The knowledge is compiled once and kept current, not re-derived on every query.

For project management, this means: drop in a call transcript and the LLM updates the meeting page, appends decisions to `decisions.md`, flags new risks in `risks.md`, and adds action items to `deliverables.md` — all in one pass.

---

## Architecture

Three layers, as described in the LLM Wiki pattern:

```
raw/{ClientName}/          ← immutable drop zone — you add files here, the LLM never touches this
wiki/{ClientName}/         ← LLM-owned layer — created and maintained entirely by the model
  index.md                 ← catalog of all pages, updated on every ingest
  log.md                   ← append-only chronological record of ingests
  sow.md                   ← Statement of Work: scope, deliverables, pricing, terms
  timeline.md              ← milestones, deadlines, schedule tables
  decisions.md             ← key decisions with date, owner, rationale
  risks.md                 ← risks, blockers, mitigations
  deliverables.md          ← deliverable tracker (status, owner, due date)
  entities/                ← one page per person, org, tool, or system
  meetings/                ← {YYYY-MM-DD}-{slug}.md per call or transcript
  summaries/               ← {source-filename}.md per ingested raw file
PM-WIKI.md                 ← schema — tells the LLM how the wiki is structured and what to do
```

**Raw sources** are immutable. The LLM reads from them but never writes to them.

**The wiki** is fully owned by the LLM. It creates pages, updates them when new sources arrive, maintains cross-references, and keeps everything consistent. You read it; the LLM writes it.

**The schema** (`PM-WIKI.md`) is the configuration file that turns the LLM from a generic chatbot into a disciplined wiki maintainer. It defines the directory layout, page formats, document classification rules, and workflow steps.

---

## How it works

```
raw/{ClientName}/new-file  →  converter  →  Ollama (local LLM)  →  wiki/{ClientName}/
```

1. **Watch** — `pm-wiki` watches the `raw/` directory for new files using filesystem events.
2. **Convert** — the file is converted to plain text/markdown (see supported formats below).
3. **Context load** — relevant existing wiki pages are loaded to give the LLM full context.
4. **Prompt** — the schema, existing context, and new source content are assembled into a prompt.
5. **Generate** — Ollama streams the LLM response, which contains page blocks in a structured format.
6. **Write** — each page block is parsed and written to the wiki directory.

A single ingest may touch 10–15 wiki pages. The LLM handles the cross-referencing and bookkeeping; you just supply the documents.

---

## Supported file types

| Extension | Converter |
|-----------|-----------|
| `.txt`, `.md` | read as-is |
| `.vtt` | WebVTT transcript strip (timestamps removed) |
| `.pdf` | text extraction |
| `.docx` | Word document extraction |
| `.xlsx`, `.csv` | spreadsheet to text |

---

## Prerequisites

- [Go](https://go.dev/) 1.22+
- [Ollama](https://ollama.com/) running locally (`http://localhost:11434` by default)
- A capable model pulled in Ollama — the default is `gemma4:31b-cloud`; a large context model works best

---

## Installation

```bash
go install github.com/pm-wiki/pm-wiki/cmd/pm-wiki@latest
```

Or build from source:

```bash
git clone https://github.com/xorType/pm-llm-wiki
cd pm-llm-wiki
go build -v ./...
```

---

## Usage

```bash
pm-wiki [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-raw` | `raw` | Directory to watch for new source files |
| `-wiki` | `wiki` | Directory where the wiki is maintained |
| `-schema` | `PM-WIKI.md` | Path to the schema file |
| `-ollama-url` | `http://localhost:11434` | Ollama API base URL |
| `-model` | `gemma4:31b-cloud` | Ollama model name |

**Example:**

```bash
# Start the watcher with defaults
pm-wiki

# Use a different model
pm-wiki -model llama3.3:70b

# Point at a non-default Ollama instance
pm-wiki -ollama-url http://192.168.1.10:11434 -model qwen2.5:32b
```

Once running, drop any supported file into `raw/{ClientName}/` and the wiki updates automatically:

```
raw/
  AcmeCorp/
    2026-04-01-kickoff-call.vtt   ← drop this in
    sow-v1.docx
wiki/
  AcmeCorp/
    index.md                      ← LLM updates these
    meetings/2026-04-01-kickoff.md
    decisions.md
    risks.md
    ...
```

---

## The schema

`PM-WIKI.md` is the key configuration file. It tells the LLM:

- What the wiki directory structure is and what each file is for
- How to classify incoming documents (transcript, SOW, schedule, deliverable, financial, general)
- What to extract from each document type and where to put it
- What frontmatter format to use on each page type
- How to update the index and log on every ingest

You can evolve the schema as your workflow develops. The LLM reads it at the start of every ingest, so changes take effect immediately on the next dropped file.

---

## Why this works

The tedious part of project management documentation is not the thinking — it's the bookkeeping. Updating the decisions log after a call, filing the risk that was raised in passing, keeping the deliverable tracker current, making sure the entity page for a new stakeholder exists and is linked from the right meeting notes. These are the tasks that fall through the cracks and cause wikis to go stale.

LLMs don't get bored, don't forget to update a cross-reference, and can touch 15 files in one pass. The wiki stays maintained because the cost of maintenance is near zero. Your job is to drop in the source documents and ask the right questions. The LLM does everything else.

---

## Acknowledgements

Inspired by Andrej Karpathy's [LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) gist.
