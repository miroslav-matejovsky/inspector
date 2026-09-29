---
title: "01 - Library boundary guard"
dependencies: []
effort: "S"
complexity: "low"
---

# 01 - Library boundary guard

## Objective

`task all` fails when any file in a library folder (`observation/`, `explanation/`, `navigation/`, `representation/`, `connectivity/`) names the harness or the simulated fulfillment domain. `.go-arch-lint.yml` states the import rule for library components. Root `README.md` documents both rules. The guard exists before the first line of library code.

## Target Artifacts

| File | Change |
| --- | --- |
| `taskfile/boundary.ps1` | New: vocabulary check over the library folders. |
| `Taskfile.yml` | New task `boundary`; task `all` runs it right after `arch-lint`. |
| `.go-arch-lint.yml` | Comment block above `components:` stating the library import rule. |
| `README.md` (root) | New section `## Library`. |

## Implementation Tasks

1. Create `taskfile/boundary.ps1` with the content in Technical Details.
2. Run the negative probe from Verification. Confirm exit code 1 and that the probe line is printed.
3. In `Taskfile.yml` add task `boundary` (Technical Details) after task `arch-lint`, and add `- task boundary` to `all` right after `- task arch-lint`.
4. In `.go-arch-lint.yml` add the comment block (Technical Details) directly above `components:`.
5. In root `README.md` add the `## Library` section (Technical Details) above `## Harness`.
6. Run `task all`.

## Technical Details

### `taskfile/boundary.ps1`

```powershell
# Check that the Inspector library does not name the harness or its simulated
# domain. go-arch-lint already rejects imports of harness packages; this check
# rejects vocabulary in code, comments, test fixtures and docs of the library
# folders given as arguments.
#
# Rejected words (case-insensitive, whole words):
#   harness, /inspected                  the harness and its URL space
#   fulfillment, sku, product, payment,
#   warehouse, restock*, reorder*        the simulated fulfillment domain
#   tick, ticks                          the simulation time model
#   "order", "orders"                    hardcoded entity kinds (quoted literals only;
#                                        the English word "order" stays allowed)
param(
    [Parameter(Mandatory, ValueFromRemainingArguments)]
    [string[]]$Paths
)

$pattern = '(?i)\b(harness|fulfillment|skus?|products?|payments?|warehouses?|restock\w*|reorder\w*|ticks?)\b|"orders?"|/inspected\b'

foreach ($p in $Paths) {
    if (-not (Test-Path -LiteralPath $p -PathType Container)) {
        Write-Host "boundary: folder not found: $p"
        exit 1
    }
}

$found = Get-ChildItem -LiteralPath $Paths -Recurse -File | Select-String -Pattern $pattern -AllMatches
if ($found) {
    Write-Host "boundary: harness domain vocabulary found in the Inspector library:"
    $found | ForEach-Object { Write-Host ("{0}:{1}: {2}" -f (Resolve-Path -Relative $_.Path), $_.LineNumber, $_.Line.Trim()) }
    exit 1
}

Write-Host "boundary: no issues found"
```

The folder list is not a default inside the script. It is passed explicitly by `Taskfile.yml`, so the task definition shows which folders form the library.

### `Taskfile.yml`

```yaml
  boundary:
    desc: Check that the Inspector library folders do not name the harness or its simulated domain
    silent: true
    cmds:
      - pwsh -NoProfile -NonInteractive -File taskfile/boundary.ps1 observation explanation navigation representation connectivity
```

`all` becomes:

```yaml
      - task arch-lint
      - task boundary
      - task lint
```

### `.go-arch-lint.yml` comment

```yaml
# Inspector library components (observation, navigation, connectivity,
# representation, and later explanation) list only other library components
# in mayDependOn, never a harness component: the library knows no inspected
# system. Every Go package must be mapped to a component; go-arch-lint fails
# on files that are not attached to one. Vocabulary is checked by task boundary.
```

### Root `README.md`

```markdown
## Library

Inspector is a Go library. Each supporting domain of the [domain map](dev/principles/4-domain-map.md) has one top-level folder: `observation/`, `explanation/`, `navigation/`, `representation/`, `connectivity/`.

The library knows no inspected system:

- Library packages never import `harness/...`. `go-arch-lint` enforces it: library components list only library components in `mayDependOn`.
- Library folders never name the harness or its simulated domain, in code, comments, tests or docs. `task boundary` enforces it and runs in `task all`.
```

### Tests

No Go tests. The script is a task script like `taskfile/deadcode.ps1`. It is verified by the positive run inside `task all` and by the negative probe below, which uses a temporary folder outside the repository.

## Verification

```powershell
task boundary

$probe = Join-Path $env:TEMP "boundary-probe"
New-Item -ItemType Directory -Force $probe | Out-Null
Set-Content (Join-Path $probe "leak.go") 'const kind = "order"'
pwsh -NoProfile -NonInteractive -File taskfile/boundary.ps1 $probe; "exit=$LASTEXITCODE"
pwsh -NoProfile -NonInteractive -File taskfile/boundary.ps1 observation missing-folder; "exit=$LASTEXITCODE"
Remove-Item -Recurse -Force $probe

task all
```

## Acceptance Criteria

- `task boundary` prints `boundary: no issues found` and exits 0.
- The probe run prints `boundary: harness domain vocabulary found in the Inspector library:`, a line ending in `const kind = "order"`, and `exit=1`.
- The run with `missing-folder` prints `boundary: folder not found: missing-folder` and `exit=1`.
- `Taskfile.yml` task `all` lists `task boundary` directly after `task arch-lint`.
- `.go-arch-lint.yml` contains the comment block above `components:`.
- Root `README.md` contains the section `## Library`.
- `task all` exits 0 and its output contains `boundary: no issues found`.

## Non-Goals

- Go packages. No library code is written in this step.
- Scanning `harness/`, `cmd/` or `dev/`. The harness is allowed to know the simulated domain.
- Parsing Go source. The check is a text scan.
