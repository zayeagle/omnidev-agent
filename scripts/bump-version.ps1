# Bump semver in VERSION. Usage: .\scripts\bump-version.ps1 [patch|minor|major]
param(
    [ValidateSet("patch", "minor", "major")]
    [string]$Part = "patch"
)

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$File = Join-Path $Root "VERSION"

if (-not (Test-Path $File)) {
    Set-Content -Path $File -Value "0.0.0" -NoNewline
}

$ver = (Get-Content $File -Raw).Trim().TrimStart("v")
$pieces = $ver.Split(".")
$major = [int]($(if ($pieces.Length -gt 0 -and $pieces[0]) { $pieces[0] } else { "0" }))
$minor = [int]($(if ($pieces.Length -gt 1 -and $pieces[1]) { $pieces[1] } else { "0" }))
$patch = [int]($(if ($pieces.Length -gt 2 -and $pieces[2]) { $pieces[2] } else { "0" }))

switch ($Part) {
    "major" { $major++; $minor = 0; $patch = 0 }
    "minor" { $minor++; $patch = 0 }
    default  { $patch++ }
}

$new = "$major.$minor.$patch"
Set-Content -Path $File -Value $new -NoNewline
Write-Output "v$new"
