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
