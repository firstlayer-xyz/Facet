# Installs (or removes) the Facet thumbnail handler (.fct/.stl/.obj/.3mf) for Windows Explorer.
# Run from an ELEVATED PowerShell. Expects facet_thumbnail.dll (and ideally
# facetc.exe) next to this script — build the DLL with build.sh first.
#
#   .\install.ps1               # install
#   .\install.ps1 -Uninstall    # remove
param([switch]$Uninstall)

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$dest = Join-Path $env:ProgramFiles 'Facet'
$dll  = Join-Path $dest 'facet_thumbnail.dll'

if ($Uninstall) {
    if (Test-Path $dll) { & regsvr32 /u /s $dll }
    Remove-Item -Recurse -Force $dest -ErrorAction SilentlyContinue
    Write-Host 'Removed the Facet thumbnail handler.'
    return
}

New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item (Join-Path $here 'facet_thumbnail.dll') $dll -Force

$facetc = Join-Path $here 'facetc.exe'
if (Test-Path $facetc) {
    Copy-Item $facetc (Join-Path $dest 'facetc.exe') -Force
} else {
    Write-Warning "facetc.exe not found next to this script — place it in $dest so the handler can render."
}

& regsvr32 /s $dll
Write-Host 'Installed. .fct/.stl/.obj/.3mf files will show 3D thumbnails (restart Explorer if needed: Stop-Process -Name explorer).'
