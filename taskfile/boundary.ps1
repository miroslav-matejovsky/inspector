# Check that the Inspector library does not name the harness or its simulated
# domain. go-arch-lint already rejects imports of harness packages; this check
# rejects vocabulary in code, comments, test fixtures and docs of the library
# folders given as arguments.
#
# Rejected words (case-insensitive). Letters and digits continue a word;
# every other character, including "_" and "-", separates words:
#   harness, /inspected                  the harness and its URL space
#   fulfillment, sku, product, payment,
#   warehouse, restock*, reorder*        the simulated fulfillment domain
#   tick, ticks                          the simulation time model
#   health_check, has_check, waits_on,
#   failure_reason                       vocabulary of harness/adapter
#   "order", "orders"                    hardcoded entity kinds (quoted literals only;
#                                        the English word "order" stays allowed)
param(
    [Parameter(Mandatory, ValueFromRemainingArguments)]
    [string[]]$Paths
)

$pattern = '(?i)(?<![a-z0-9])(harness|fulfillment|skus?|products?|payments?|warehouses?|restock\w*|reorder\w*|ticks?|health_check|has_check|waits_on|failure_reason)(?![a-z0-9])|"orders?"|/inspected(?![a-z0-9])'

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
