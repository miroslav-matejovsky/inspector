# Create and push a Go module release tag.
#
# Usage: task release -- 0.1.0   (or v0.1.0)
#
# A Go release is just a semver git tag (vX.Y.Z) on the remote. GitHub releases are not used.
# Preconditions, checked in order, each failing fast:
#   - version is valid semver (optional leading "v", optional pre-release / build suffix)
#   - current branch is main
#   - working tree has no uncommitted or untracked changes
#   - local main is identical to origin/main (nothing unpushed, nothing unpulled)
#   - tag does not exist locally or on origin
# Then an annotated tag is created on HEAD and pushed to origin.

param(
  [Parameter(Mandatory = $true)]
  [string]$Version
)

$ErrorActionPreference = "Stop"

function Invoke-Git {
  $out = & git @args
  if ($LASTEXITCODE -ne 0) { throw "git $($args -join ' ') failed" }
  return $out
}

$Version = $Version.Trim().Trim('"', "'")
if (-not $Version.StartsWith("v")) { $Version = "v$Version" }

$semver = '^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
if ($Version -notmatch $semver) { throw "invalid version '$Version', expected semver like 0.1.0 or v0.1.0" }

$root = Invoke-Git rev-parse --show-toplevel
Set-Location $root

$branch = Invoke-Git rev-parse --abbrev-ref HEAD
if ($branch -ne "main") { throw "must be on main, current branch is '$branch'" }

$status = Invoke-Git status --porcelain
if ($status) { throw "working tree has uncommitted changes:`n$($status -join "`n")" }

Invoke-Git fetch origin main --tags --quiet | Out-Null

$local = Invoke-Git rev-parse HEAD
$remote = Invoke-Git rev-parse origin/main
if ($local -ne $remote) {
  $ahead = Invoke-Git rev-list --count origin/main..HEAD
  $behind = Invoke-Git rev-list --count HEAD..origin/main
  throw "main differs from origin/main (ahead $ahead, behind $behind), push or pull first"
}

if (Invoke-Git tag --list $Version) { throw "tag $Version already exists locally" }
if (Invoke-Git ls-remote --tags origin "refs/tags/$Version") { throw "tag $Version already exists on origin" }

Invoke-Git tag -a $Version -m "Release $Version" | Out-Null
Invoke-Git push origin $Version --quiet | Out-Null

Write-Host "released $Version at $local"
