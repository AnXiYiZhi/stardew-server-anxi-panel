param(
    [Parameter(Mandatory = $true)][string]$SteamCmd,
    [Parameter(Mandatory = $true)][string]$OutputPath
)

$ErrorActionPreference = 'Stop'

function Find-PublicBuildId {
    param([string[]]$Lines)
    $inBranches = $false
    $branchesDepth = -1
    $inPublic = $false
    $publicDepth = -1
    $depth = 0
    foreach ($line in $Lines) {
        $trimmed = $line.Trim()
        if (-not $inBranches -and $trimmed -eq '"branches"') { $inBranches = $true; $branchesDepth = $depth; continue }
        if ($inBranches -and -not $inPublic -and $trimmed -eq '"public"') { $inPublic = $true; $publicDepth = $depth; continue }
        if ($inPublic -and $trimmed -match '^"buildid"\s+"([0-9]+)"$') { return $Matches[1] }
        if ($trimmed -eq '{') { $depth++ }
        if ($trimmed -eq '}') {
            $depth--
            if ($inPublic -and $depth -le $publicDepth) { $inPublic = $false }
            if ($inBranches -and $depth -le $branchesDepth) { $inBranches = $false }
        }
    }
    throw 'public branch buildid not found in Steam app info'
}

function Invoke-SteamAppInfo {
    param([string]$AppId, [bool]$NeedsAccount)
    $username = if ($NeedsAccount) { $env:STEAM_USERNAME } else { 'anonymous' }
    $password = if ($NeedsAccount) { $env:STEAM_PASSWORD } else { $null }
    if ($NeedsAccount -and ([string]::IsNullOrWhiteSpace($username) -or [string]::IsNullOrWhiteSpace($password))) {
        throw "Steam App $AppId requires STEAM_USERNAME and STEAM_PASSWORD from a protected environment"
    }
    $runScript = [System.IO.Path]::GetTempFileName()
    try {
        if ($NeedsAccount) {
            [System.IO.File]::WriteAllLines($runScript, @("@ShutdownOnFailedCommand 1", "@NoPromptForPassword 1", "login $username $password", "app_info_update 1", "app_info_print $AppId", "quit"))
        } else {
            [System.IO.File]::WriteAllLines($runScript, @("@ShutdownOnFailedCommand 1", "login anonymous", "app_info_update 1", "app_info_print $AppId", "quit"))
        }
        if (-not $IsWindows) { & chmod 600 $runScript }
        $lines = @(& $SteamCmd +runscript $runScript 2>&1 | ForEach-Object { [string]$_ })
        if ($LASTEXITCODE -ne 0) { throw "SteamCMD app info query failed for App $AppId (exit $LASTEXITCODE)" }
        return Find-PublicBuildId -Lines $lines
    } finally {
        Remove-Item -LiteralPath $runScript -Force -ErrorAction SilentlyContinue
    }
}

$result = [ordered]@{
    schemaVersion = 1
    classification = 'discovered'
    generatedAt = [DateTime]::UtcNow.ToString('o')
    source = 'SteamCMD app_info_print public branch'
    apps = @(
        [ordered]@{ appId = '413150'; name = 'Stardew Valley'; buildId = (Invoke-SteamAppInfo -AppId '413150' -NeedsAccount $true) },
        [ordered]@{ appId = '1007'; name = 'Steamworks SDK Redistributables'; buildId = (Invoke-SteamAppInfo -AppId '1007' -NeedsAccount $false) }
    )
}

$parent = Split-Path -Parent $OutputPath
if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
$result | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $OutputPath -Encoding utf8
Write-Output "Discovered candidate buildids written to $OutputPath (classification=discovered)."
