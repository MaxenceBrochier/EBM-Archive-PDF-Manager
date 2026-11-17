# EBM Archive

[Version française](README.md)

**Proposed next version: v1.1.0**, following v1.0.1. See the [changelog](CHANGELOG.md) (French) for changes, impacts, and validation results.

EBM Archive is a Windows tool for quickly finding PDFs in a large archive, including on a network share. It extracts configurable metadata from PDFs, indexes it, and lets users search for and open documents from a terminal interface.

The tool does not modify PDFs or replace the document archive: it creates a search index alongside the files. The opened PDF remains the reference document.

## The two executables

| Executable | Purpose | Requirements |
|---|---|---|
| `indexer.exe` | Scans the archive and extracts PDF fields to create or update the index; also exports the index as an XLSX manifest. | Go to build; Poppler (`pdftotext.exe`) only for PDF extraction. |
| `finder.exe` | Searches the index and opens the selected PDF or its folder in File Explorer. | No Go or Poppler installation required on the user's machine. |

## How it works

1. The indexer finds PDFs directly in the archive root and in first-level subdirectories, which it scans recursively.
2. `pdftotext` extracts text from the first pages (2 with the supplied configuration).
3. The supplied configuration selects primary header metadata, then applies the regular expressions in `fields.json`, keeping one value per field. PDFs are read but their contents are not changed.
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

The supplied build targets 64-bit Windows. The module requires Go 1.24.2 or later and declares the Go 1.24.7 toolchain, also used in CI.

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

The proposed version is `v1.1.0`: XLSX export warrants a minor release, alongside extraction fixes. Version numbers do not count individual fixes: `MAJOR.MINOR.PATCH` increases according to the level of change, resetting subsequent components to zero. The index format and existing commands remain supported; the supplied metadata selection policy intentionally changes as described below.

After approving the version and the commit to publish, push its tag to build the application and publish its assets to a GitHub Release:

```powershell
git tag v1.1.0
git push origin v1.1.0
```

The workflow creates the Release if it does not exist and adds `finder.exe`, `indexer.exe`, and `fields.json`. If a Release already exists for that tag, assets with these names are replaced.

### Upgrading from v1.0.1

1. Deploy the rebuilt executables and the new `fields.json`. If a scheduled task uses `-config`, also update that configuration copy.
2. Run indexing again. The internal extractor version changes from 3 to 4, invalidating the cache: the first run re-extracts documents in processed sections without requiring `-force`. Do not use `-only` when updating the entire archive.
3. Reload the finder index (`F5`), then export the manifest if needed.

```powershell
.\dist\indexer.exe -root "\\SERVER\EBM\VAL\PDF" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 16
.\dist\indexer.exe -root "\\SERVER\EBM\VAL\PDF" `
  -manifest ".\manifest-v1.1.0.xlsx"
```

The supplied configuration changes from 3 to 2 pages and from up to 10 values to one primary value per field. Secondary references are therefore no longer searchable through those fields, and data after page 2 is not read. Missing reliable metadata leaves a blank cell; it does not prove that the information is absent elsewhere in the PDF. Custom generic configurations can still use `primary_only: false`, `max_matches`, and `max_pages`; disabling `primary_only` alone does not restore the previous regular expressions.

## Indexer

Install Poppler for Windows and locate `pdftotext.exe`. From PowerShell:

```powershell
# Check text extraction and field expressions on a PDF
.\dist\indexer.exe -test "\\SERVER\EBM\VAL\example.pdf" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe"

# Index or update the archive
.\dist\indexer.exe -root "\\SERVER\EBM" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 16
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
| `-manifest` | — | Export the existing index to the specified `.xlsx` file, then exit without reindexing. |

Indexing is incremental: a PDF is reprocessed if its size, modification time, extraction configuration, or internal extractor version has changed. Transient extraction errors are retried on the next run. The indexer prevents two runs from processing the same index directory at the same time and writes index files atomically.

`errors.txt` lists PDFs that failed or had no fields detected. A scanned PDF without a text layer cannot provide metadata without OCR; this tool does not perform OCR.

### XLSX manifest export

```powershell
.\dist\indexer.exe -root "\\SERVER\EBM\VAL\PDF" -manifest ".\manifest.xlsx"
```

The `-manifest` mode reads the existing index in `<root>\.index` (or the directory passed to `-out`). Neither Poppler nor `fields.json` is required. Update the index first if PDFs or metadata have changed. The output file must not already exist. `-manifest` cannot be combined with `-test`, `-only`, or `-force`.

The workbook preserves the three sheets and formatting of the supplied XLSX template, embedded in the executable: `Manifest Template`, `Manifest Description`, and `Manifest Sample`. Only `Manifest Sample` is populated, with one row per PDF and the following six columns in order. `Manifest Template` and `Manifest Description` are kept unchanged.

| Column | Content |
|---|---|
| `DOS File` | PDF path starting at and including `VAL`, e.g. `VAL\PDF\Batch Report\2015\report.pdf`. No drive letter or parent directories are exported. |
| `File Name` | PDF filename including its extension, without the path. |
| `Batch Number` | Primary batch number (`lot`). |
| `Work Order` | Primary WO/manufacturing order (`of`). |
| `Product Code` | Primary product code (`code_produit`), stored as text to preserve leading zeros. |
| `Batch Size` | Primary batch size (`taille_lot`), or eligible transferred quantity, including its unit. |

Columns retain the template's styles: path and filename first, followed by the four custom metadata fields. Each PDF must be under a directory named `VAL` (case-insensitive); otherwise export fails without creating a workbook. The supplied configuration keeps one primary value per field. Missing metadata stays blank; documents with extraction errors are included and reported in the terminal. Each PDF edition remains a separate row even when files share the same WO and batch.

The RIM/IT technical columns remain visible in the original description sheet but are not populated with invented values. PDFs are not renamed or moved; only exported paths are limited to `VAL\...`. The character restrictions referenced by the template must still be checked against the recipient team's requirements document.

## Field configuration

`fields.json` defines the number of pages to read, directories to exclude, and fields to extract:

```json
{
  "max_pages": 2,
  "max_matches": 1,
  "primary_only": true,
  "exclude_dirs": ["$RECYCLE.BIN", "System Volume Information"],
  "fields": [
    {
      "name": "lot",
      "label": "N° de lot",
      "regex": "(?mi)^[ \\t]*(?:Lot Number|Batch Number|Numéro de lot|N° de lot|Lot)[ \\t]*:[ \\t]*([A-Z0-9/-]*[0-9][A-Z0-9/-]*)(?:[ \\t\\r]|$)",
      "uppercase": true
    }
  ]
}
```

- Regular expressions use Go's RE2 syntax; lookaheads are not supported.
- The first capture group is stored as the value; if there is no capture group, the full match is used.
- `uppercase: true` converts values to uppercase.
- `sections` limits a field to sections whose path contains one of the specified path segments. Without `sections`, the field applies to all sections.
- `primary_only: true` restricts extraction to the first-page header and keeps one match per field. Supplemental metadata tables are used only when their WO matches the primary WO. For a legacy production report whose header identity is entirely empty, the first table may supply the identity if it contains an explicit non-technical WO, batch number and product code. Child operations, consumptions and SIP references are excluded. Without this option, generic extraction remains available with the `max_matches` limit.
- Recognized batch labels include `Lot`, `Lot Number`, `Batch Number`, `Numéro de lot`, and `N° de lot`. The value must be on the same line to the right: line breaks, standalone identifiers and filenames are not used as fallbacks.
- Changing the configuration causes affected documents to be re-extracted during the next indexing run.

The supplied configuration extracts the batch number in all sections and includes fields specific to `VAL`: manufacturing order, product code, manufacturing start and end dates, and batch size.
The `Batch Report` and `Batch Report intermédiaire` sections also extract the manufacturing order (`OF`, `WO`, `Numéro de WO`), product code (`Product code`, `Code article`), and batch size (`Quantité du WO`). Search for work orders with `of:WO3399985`. A WO alone is not used as a batch number when the PDF has no batch field.
Batch size may use the first `Quantité transférée`, preserving decimals and the unit, when no primary size is available and a matching primary-WO table is identified or the primary WO starts with `STO`. This supplement may come from the already extracted second page. References to buffer preparation or storage tank solution batches do not replace a missing primary batch number; that field stays blank. No OCR or additional PDF pages are introduced.

Manufacturing dates require the explicit labels `Début de Fabrication` and `Fin de Fabrication`. Operation, transfer, and printing dates are not substitutes. These dates remain searchable index fields; they are not among the six requested manifest columns.

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
