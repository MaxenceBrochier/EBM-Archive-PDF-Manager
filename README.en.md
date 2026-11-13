# EBM Archive

[Version française](README.md)

EBM Archive is a Windows tool for quickly finding PDFs in a large archive, including on a network share. It extracts configurable metadata from PDFs, indexes it, and lets users search for and open documents from a terminal interface.

The tool does not modify PDFs or replace the document archive: it creates a search index alongside the files. The opened PDF remains the reference document.

## The two executables

| Executable | Purpose | Requirements |
|---|---|---|
| `indexer.exe` | Scans the archive and extracts PDF fields to create or update the index. It is intended for an indexing machine, for example as a scheduled task. | Go to build; Poppler (`pdftotext.exe`) at runtime. |
| `finder.exe` | Searches the index and opens the selected PDF or its folder in File Explorer. | No Go or Poppler installation required on the user's machine. |

## How it works

1. The indexer finds PDFs directly in the archive root and in first-level subdirectories, which it scans recursively.
2. `pdftotext` extracts text from the first pages (3 by default).
3. The regular expressions in `fields.json` identify the fields to keep, such as a batch number. PDFs are read but their contents are not changed.
4. The indexer writes an index to `.index`, with one compressed file per section.
5. The finder loads this index and lets users filter results before opening a document.

Hidden directories and directories listed in `exclude_dirs` are ignored. PDFs in the root form the "Root" section; each first-level subdirectory forms a section (a tab). The "All" tab combines all sections.

```text
<archive>\
├── finder.exe
├── fields.json
├── .index\
│   ├── index.json
│   ├── _racine.json.gz
│   ├── <section>.json.gz
│   └── errors.txt
├── VAL\
│   └── ...
└── <other sections>\
```

The root section's shard is named `_racine.json.gz`; other shards are named after their section.

## Build

The supplied build targets 64-bit Windows. Go 1.24 or later is required.

```powershell
.\build.ps1
```

The script runs `go mod tidy`, `go vet ./...`, and `go test ./...`, then builds the executables in `dist\` and copies `fields.json` there.

## Tests and continuous integration

The [GitHub Actions workflow](.github/workflows/go.yml) runs `go vet`, tests with coverage, and a Windows build on every push or pull request to `main`. It can also be run manually from GitHub. To run these checks locally:

```powershell
go vet ./...
go test ./... -cover
go build ./...
```

## Releases

Push a version tag (for example, `v1.0.0`) to build the application and publish its assets to a GitHub Release:

```powershell
git tag v1.0.0
git push origin v1.0.0
```

The workflow creates the Release if it does not exist and adds `finder.exe`, `indexer.exe`, and `fields.json`. If a Release already exists for that tag, assets with these names are replaced.

## Indexer

Install Poppler for Windows and locate `pdftotext.exe`. From PowerShell:

```powershell
# Check text extraction and field expressions on a PDF
.\dist\indexer.exe -test "\\SERVER\EBM\VAL\example.pdf" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe"

# Index or update the archive
.\dist\indexer.exe -root "\\SERVER\EBM" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 8
```

Main options:

| Option | Default | Description |
|---|---|---|
| `-root` | None | Archive root directory; required except in `-test` mode. |
| `-out` | `<root>\.index` | Index output directory. |
| `-config` | `fields.json` next to the executable (otherwise in the current search path) | Field definition file. |
| `-pdftotext` | `pdftotext` | Path to Poppler's `pdftotext.exe` if the command is not in `PATH`. |
| `-workers` | Number of processors | Number of parallel extractions. |
| `-timeout` | `60s` | Maximum extraction time per PDF. |
| `-only` | All sections | Limit processing to one or more comma-separated sections. |
| `-force` | Disabled | Re-extract every PDF in the processed sections. |
| `-test` | — | Display the extracted text and detected values for a PDF, then exit. |

Indexing is incremental: a PDF is reprocessed if its size, modification time, or extraction configuration has changed. Transient extraction errors are retried on the next run. The indexer prevents two runs from processing the same index directory at the same time and writes manifests atomically.

`errors.txt` lists PDFs that failed or had no fields detected. A scanned PDF without a text layer cannot provide metadata without OCR; this tool does not perform OCR.

## Field configuration

`fields.json` defines the number of pages to read, directories to exclude, and fields to extract:

```json
{
  "max_pages": 3,
  "max_matches": 10,
  "exclude_dirs": ["$RECYCLE.BIN", "System Volume Information"],
  "fields": [
    {
      "name": "lot",
      "label": "N° de lot",
      "regex": "(?i:N°|No\\.?)\\s*(?i:de\\s+)?(?i:lot)\\s*[:.]?\\s*([A-Z0-9/-]*[0-9][A-Z0-9/-]*)",
      "uppercase": true
    }
  ]
}
```

- Regular expressions use Go's RE2 syntax; lookaheads are not supported.
- The first capture group is stored as the value; if there is no capture group, the full match is used.
- `uppercase: true` converts values to uppercase.
- `sections` limits a field to sections whose path contains one of the specified path segments. Without `sections`, the field applies to all sections.
- Changing the configuration causes affected documents to be re-extracted during the next indexing run.

The supplied configuration extracts the batch number in all sections and includes fields specific to `VAL`: manufacturing order, product code, manufacturing start and end dates, and batch size.

## Finder

Place `finder.exe` at the archive root, next to `.index`, and run it. Without options, it looks for the index next to the executable; use `-root` to specify another path:

```powershell
.\finder.exe
.\finder.exe -root "\\SERVER\EBM"
```

Search is case-insensitive. Multiple words are combined with AND. Search across all fields, or restrict a term to a field by using its technical name, for example:

```text
lot:L24A
VAL 17CU004
```

| Key | Action |
|---|---|
| Typing | Filter results in real time. |
| `Tab` / `Shift+Tab` | Move to the next / previous tab. |
| `↑` / `↓`, `PgUp` / `PgDn` | Move through results. |
| `Enter` | Open the PDF with its associated Windows application. |
| `Ctrl+O` | Open File Explorer at the selected PDF. |
| `Ctrl+R` / `F5` | Reload the index. |
| `Esc` | Clear the search, then exit if it is already empty. |

Up to 1,000 results are displayed at a time; the finder shows the total number of matches.

## Scheduled runs and permissions

`scheduled-task.ps1` provides an example of a daily task at 2 a.m. Before using it, update the paths to the executable, archive, and Poppler, as well as the service account.

The account running the indexer must be able to read the PDFs and write to `.index`. Finder users need read access to the PDFs and index. On a company network share, also check with IT about policies for running executables from the network.

## Usage limitations

The finder is a search and navigation tool, not a validation or archiving system. PDFs without searchable text or detected fields cannot be found by those fields; check `errors.txt` to identify them.
