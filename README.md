# tidy — Deterministic Go CLI Folder Organizer

`tidy` is a fast, robust, and auditable command-line tool written in Go that automatically organizes files in designated directories using simple, declarative YAML rules.

Unlike traditional folder organization scripts or bloated GUI automation utilities, `tidy` is engineered as **resilient infrastructure** — operating with zero runtime dependencies, a tamper-evident audit log, and first-class reversible operations.

---

## 🌟 Core Identity & Unique Selling Points (USPs)

| Feature | `tidy` | Traditional Tools (Hazel, organize) |
| :--- | :--- | :--- |
| **Runtime Dependencies** | **Zero** — Single static binary (pure Go, CGO-free via `modernc.org/sqlite`) | Heavy (requires Python/Node interpreters, libraries) |
| **Undo System** | **First-class & selective** — Undo last move, by ID, by session, or by time | Minimal / Non-existent / Destructive |
| **Audit Log Integrity** | **Tamper-evident** — SHA-256 hash-chained history | Plain text logs or none |
| **Security & Safety** | **No code execution** — Strict match-and-move only | Can execute arbitrary shell/scripts/regex |
| **Folder Scoping** | **Top-level only guarantee** — Subdirectories are never scanned or mutated | Can inadvertently recurse into organized subtrees |
| **Rule Debugger** | Built-in `tidy explain <file>` trace tool | Guess-and-check or complex log parsing |
| **Shadowing Detection**| `tidy validate` warns on unreachable/overlapping rules | Silent rule overriding |
| **Daemon Philosophy**  | Headless CLI, native **systemd/cron** companion | Heavy background GUI / tray process |

---

## 🚀 Key Capabilities

### 1. Reversible History & First-Class Undo
Every single file movement executed by `tidy` is tracked in an embedded SQLite database (`~/.local/state/tidy/history.db`).
* `tidy undo`: Reverts the most recent move, restoring the file to its original path.
* `tidy undo --id <ID>`: Undoes a specific operation by its history ID.
* `tidy undo --session <run-id>`: Rollback an entire batch execution or watch session in a single transactional operation.
* `tidy undo --since <time>`: Rollback moves performed after a specific cutoff (e.g. `2h`, `today`, `2026-01-01`).
* **Conflict Guard**: If a new file already occupies the original source path, `tidy undo` aborts gracefully with a clear error rather than blindly overwriting data.

### 2. Hash-Chained Tamper-Evident History
Each record in the history database stores:
* Core metadata: `session_id`, `timestamp`, `source`, `dest`, `undone`.
* `prev_hash`: SHA-256 hash of the previous record.
* `record_hash`: SHA-256 hash calculated across the current record's fields concatenated with `prev_hash`.
* Provides mathematical proof that the move log has not been manually altered, truncated, or falsified.

### 3. Strict Safety & Top-Level Invariant
* **No Subdirectory Recursing**: `tidy` strictly considers top-level files in `watch_dirs`. If `~/Downloads/Documents` was previously populated, files inside it are never touched.
* **Collision-Safe Renaming**: If a file with the same name exists at the destination, `tidy` checks SHA-256 content hashes (streamed safely without ballooning memory). If identical, the redundant file is skipped. If different, the destination filename is automatically suffixed (`file_1.txt`, `file_2.txt`).
* **Atomic Destination Creation**: Missing destination directories are created on demand.
* **Intra-Folder Movement Protection**: If a file is already in its target folder, it is skipped.

### 4. Rule Engine & Intelligence
* **Top-to-Bottom Evaluation**: The first matching rule wins.
* **Extension Matching**: Case-insensitive (`.PDF` == `.pdf`), with native support for multi-part extensions (e.g. `.tar.gz`, `.tgz`).
* **Glob Pattern Matching**: Uses `filepath.Match` against base filenames (e.g. `"Screenshot*"`).
* **Rule Shadowing Warnings**: `tidy validate` parses rules and flags when an earlier rule supersedes and starves a later rule.
* **Rule Debugger (`tidy explain <file>`)**: Trace any hypothetical filename through the rule set to see which rule catches it and where it would be routed, with zero filesystem modifications.
* **Content-Sniffing Fallback (Optional)**: If `sniff_content: true` is configured, files with missing or ambiguous extensions have their magic bytes (leading bytes) inspected to identify MIME type and resume rule matching safely without code execution.

### 5. Live Watching with Smart Debounce
* Uses `fsnotify` to track filesystem events in real time.
* Debounces rapid writes (500ms–1s quiet period) to allow large downloads or file transfers to finish.
* Recognizes rename events (e.g. `invoice.pdf.crdownload` &rarr; `invoice.pdf`) as completed downloads.
* Filters out ephemeral or temporary files defined in `ignore` patterns.

---

## 🛠️ Command Reference

| Command | Description |
| :--- | :--- |
| `tidy init` | Writes a standard starter configuration to `~/.config/tidy/config.yaml`. |
| `tidy validate` | Validates YAML syntax, verifies path existence, checks rule formats, and detects shadowed rules. |
| `tidy run` | Executes a one-shot organization across top-level files in all `watch_dirs`. |
| `tidy run --dry-run` | Previews planned moves in terminal without modifying the filesystem. |
| `tidy watch` | Starts continuous event-driven folder monitoring with quiet-period debouncing. |
| `tidy watch --dry-run` | Monitors filesystem events and prints intended moves without performing them. |
| `tidy explain <filename>` | Simulates rule evaluation for a specific filename and prints match reasoning. |
| `tidy history` | Displays a formatted table of recorded moves and their undone status. |
| `tidy history --export <csv\|json>` | Exports the complete move history to CSV or JSON format. |
| `tidy undo` | Reverts the most recent move operation. |
| `tidy undo --id <ID>` | Reverts a specific historical move by ID. |
| `tidy undo --session <session-id>` | Atomically rolls back all moves belonging to a specific session run. |
| `tidy undo --since <timestamp>` | Rolls back all moves executed after the given time (e.g. `2h`, `today`, `2026-01-01`). |

### Global Flags
* `--config <path>`: Override the default configuration path (`~/.config/tidy/config.yaml`).
* `--dry-run`: Enable preview mode (applicable to `run` and `watch`).

---

## ⚙️ Configuration Schema (`config.yaml`)

Path: `~/.config/tidy/config.yaml` (Supports `~` expansion to home directory)

```yaml
# Directories to scan and organize (Top-level only, never recursive)
watch_dirs:
  - ~/Downloads

# Optional content sniffing fallback (magic-byte detection)
sniff_content: false

# Patterns to ignore during scans and active debounce windows
ignore:
  - "*.crdownload"
  - "*.part"
  - "*.tmp"
  - ".DS_Store"
  - "desktop.ini"
  - "*.download"

# Ordered rule definitions: First match wins!
rules:
  - name: Documents
    extensions: [".pdf", ".doc", ".docx", ".txt", ".md", ".csv"]
    dest: ~/Downloads/Documents

  - name: Screenshots
    pattern: "Screenshot*"
    dest: ~/Downloads/Images/Screenshots

  - name: Images
    extensions: [".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"]
    dest: ~/Downloads/Images

  - name: Archives
    extensions: [".zip", ".rar", ".7z", ".tar", ".tar.gz", ".tgz"]
    dest: ~/Downloads/Compressed

  - name: Torrents
    extensions: [".torrent"]
    dest: ~/Downloads/Torrents
```

---

## 🏛️ Internal Architecture

```
tidy/
├── cmd/             # Cobra CLI command layer (init, validate, run, watch, history, undo, explain)
├── config/          # YAML schema definitions, path expansion, validation, and rule shadowing detection
├── engine/          # Rule matching, magic-byte sniffing, streaming SHA-256 checks, collision resolution, and file moving
├── history/         # SQLite repository (modernc.org/sqlite), tamper-evident hash chaining, undo transaction engine
├── watcher/         # fsnotify wrapper with per-file quiet-period debouncing and ignore pattern filtering
├── config.yaml      # Sample default starter configuration
├── go.mod           # Go module declaration
└── main.go          # CLI entrypoint
```

---

## 📦 Planned 14-Step Implementation Roadmap

1. **Config Struct + YAML Loading**: Strong typing, tilde expansion, unmarshaling.
2. **`tidy validate` + Rule Shadowing Detection**: Static analysis detecting unreachable rules.
3. **Rule Matching Engine**: Extension normalization, multi-part handling (`.tar.gz`), glob patterns, unit tests.
4. **Content-Sniffing Fallback**: Safe magic-byte sniffing without file execution.
5. **`tidy run --dry-run`**: Non-destructive preview pipeline.
6. **`tidy explain <file>`**: Rule tracer and debugging command.
7. **Real File Mover & Conflict Engine**: Streaming SHA-256 duplicate skip, auto-suffix collision handling.
8. **SQLite History Logging**: Base database schema and move ledger.
9. **Hash-Chained Tamper-Evidence**: Cryptographic chaining (`prev_hash` & `record_hash`) and integrity checks.
10. **`tidy history` + CSV/JSON Export**: Formatted table and structured audit exports.
11. **Single Record Undo (`tidy undo`, `--id`)**: In-place reversal with destination-occupied checks.
12. **Batch Undo (`--session`, `--since`)**: Transactional multi-file rollbacks.
13. **`tidy watch`**: `fsnotify` event loop with per-file settling/debouncing.
14. **`tidy watch --dry-run`**: Real-time event monitoring with dry-run logging.
