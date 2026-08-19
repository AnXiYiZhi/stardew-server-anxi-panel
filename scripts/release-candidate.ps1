[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$PreviousVersion,

    [string]$Commit = '',
    [string]$BuildDate = '',
    [string]$Image = '',
    [string]$MetadataOutput = '',
    [switch]$AllowDirty
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Assert-NativeSuccess {
    param([Parameter(Mandatory = $true)][string]$Action)
    if ($LASTEXITCODE -ne 0) {
        throw "$Action failed with exit code $LASTEXITCODE"
    }
}

function ConvertTo-UtcIsoString {
    param([Parameter(Mandatory = $true)]$Value)

    if ($Value -is [DateTime]) {
        $utc = $Value.ToUniversalTime()
        return $utc.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
    }
    if ($Value -is [DateTimeOffset]) {
        return $Value.UtcDateTime.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
    }

    $parsed = [DateTimeOffset]::Parse(
        [string]$Value,
        [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::AssumeUniversal
    )
    return $parsed.UtcDateTime.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
}

function Invoke-DockerPullWithTimeout {
    param(
        [Parameter(Mandatory = $true)][string]$ImageRef,
        [int]$TimeoutSeconds = 300,
        [int]$MaxAttempts = 3
    )

    $dockerPath = (Get-Command docker -ErrorAction Stop).Source
    $lastFailure = ''
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Output "release candidate: pre-fetching $ImageRef (attempt $attempt/$MaxAttempts)"
        $process = Start-Process -FilePath $dockerPath -ArgumentList @('pull', $ImageRef) -PassThru -NoNewWindow
        try {
            if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
                Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
                [void]$process.WaitForExit()
                $lastFailure = "timed out after $TimeoutSeconds seconds"
            } elseif ($process.ExitCode -ne 0) {
                $lastFailure = "exited with code $($process.ExitCode)"
            } else {
                & docker image inspect $ImageRef *> $null
                Assert-NativeSuccess "docker image inspect after pull: $ImageRef"
                return
            }
        } finally {
            $process.Dispose()
        }
        if ($attempt -lt $MaxAttempts) {
            Write-Warning "release candidate: pull $ImageRef $lastFailure; retrying the same exact reference"
            Start-Sleep -Seconds $attempt
        }
    }
    throw "docker pull failed after $MaxAttempts attempts ($lastFailure): $ImageRef"
}

function Wait-CandidatePanel {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$ExpectedVersion,
        [Parameter(Mandatory = $true)][string]$ExpectedCommit,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $deadline = [DateTimeOffset]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTimeOffset]::UtcNow -lt $deadline) {
        $healthRaw = & docker exec $Container wget -qO- http://127.0.0.1:8090/health 2>$null
        $healthExit = $LASTEXITCODE
        if ($healthExit -eq 0) {
            $health = $healthRaw | ConvertFrom-Json
            if ($health.status -eq 'ok') {
                $versionRaw = & docker exec $Container wget -qO- http://127.0.0.1:8090/api/version 2>$null
                $versionExit = $LASTEXITCODE
                if ($versionExit -eq 0) {
                    $versionInfo = $versionRaw | ConvertFrom-Json
                    if ($versionInfo.version -eq $ExpectedVersion -and $versionInfo.commit -eq $ExpectedCommit) {
                        return
                    }
                }
            }
        }
        Start-Sleep -Seconds 1
    }

    & docker logs $Container
    throw "candidate Panel did not become ready as $ExpectedVersion@$ExpectedCommit"
}

$semverPattern = '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
if ($Version -notmatch $semverPattern -or $PreviousVersion -notmatch $semverPattern) {
    throw 'Version and PreviousVersion must be exact stable semantic versions'
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $repoRoot
try {
    & docker info *> $null
    Assert-NativeSuccess 'docker info'

    $headCommit = (& git rev-parse HEAD).Trim()
    Assert-NativeSuccess 'git rev-parse HEAD'
    if ([string]::IsNullOrWhiteSpace($Commit)) {
        $Commit = $headCommit
    }
    if ($Commit -notmatch '^[0-9a-f]{40}$' -or $Commit -ne $headCommit) {
        throw 'Commit must be the full current HEAD SHA'
    }

    if (-not $AllowDirty) {
        $branch = (& git rev-parse --abbrev-ref HEAD).Trim()
        Assert-NativeSuccess 'git branch check'
        if ($branch -ne 'main') {
            throw 'formal candidates must run on main'
        }
        $status = @(& git status --porcelain)
        Assert-NativeSuccess 'git status'
        if ($status.Count -ne 0) {
            throw 'formal candidates require a clean worktree'
        }
        & git fetch --no-tags origin 'main:refs/remotes/origin/main'
        Assert-NativeSuccess 'git fetch origin/main'
        $originMain = (& git rev-parse origin/main).Trim()
        Assert-NativeSuccess 'git rev-parse origin/main'
        if ($originMain -ne $headCommit) {
            throw 'main must exactly match origin/main'
        }
    }

    if ([string]::IsNullOrWhiteSpace($BuildDate)) {
        $BuildDate = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
    }
    $parsedBuildDate = [DateTimeOffset]::MinValue
    $validBuildDate = [DateTimeOffset]::TryParseExact(
        $BuildDate,
        'yyyy-MM-ddTHH:mm:ssZ',
        [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::AssumeUniversal,
        [ref]$parsedBuildDate
    )
    if (-not $validBuildDate) {
        throw 'BuildDate must be an ISO 8601 UTC timestamp'
    }

    $shortCommit = $Commit.Substring(0, 12)
    if ([string]::IsNullOrWhiteSpace($Image)) {
        $Image = "anxi-release-candidate:$Version-$shortCommit"
    }
    if ([string]::IsNullOrWhiteSpace($MetadataOutput)) {
        $MetadataOutput = Join-Path $repoRoot 'release-candidate.json'
    } elseif (-not [IO.Path]::IsPathRooted($MetadataOutput)) {
        $MetadataOutput = Join-Path $repoRoot $MetadataOutput
    }

    $owner = 'anxi-release-candidate-{0}-{1}' -f [DateTimeOffset]::UtcNow.ToUnixTimeSeconds(), $PID
    $freshContainer = "$owner-fresh"
    $freshVolume = "$owner-data"
    $dindContainer = "$owner-dind"
    $agentsRoot = Join-Path $repoRoot '.agents'
    $taskRoot = Join-Path $agentsRoot $owner
    $candidateTar = Join-Path $taskRoot 'candidate.tar'
    $fixturesTar = Join-Path $taskRoot 'fixtures.tar'
    $previousRef = "ghcr.io/anxiyizhi/stardew-server-anxi-panel:$PreviousVersion"
    $taskRootCreated = $false

    try {
        New-Item -ItemType Directory -Path $taskRoot | Out-Null
        $taskRootCreated = $true

        Write-Output "release candidate: building $Image"
        & docker build --provenance=false --sbom=false --file Dockerfile --tag $Image --build-arg "VERSION=$Version" --build-arg "COMMIT=$Commit" --build-arg "BUILD_DATE=$BuildDate" .
        Assert-NativeSuccess 'candidate image build'

        $imageInspectRaw = & docker image inspect $Image
        Assert-NativeSuccess 'candidate image inspect'
        $imageInspect = $imageInspectRaw | ConvertFrom-Json
        $imageId = $imageInspect[0].Id
        $labels = $imageInspect[0].Config.Labels
        $normalizedCreated = ConvertTo-UtcIsoString -Value $labels.'org.opencontainers.image.created'
        if (
            $labels.'org.opencontainers.image.version' -ne $Version -or
            $labels.'org.opencontainers.image.revision' -ne $Commit -or
            $normalizedCreated -ne $BuildDate
        ) {
            throw 'candidate OCI metadata does not match the frozen identity'
        }

        Write-Output 'release candidate: fresh install and restart smoke'
        & docker volume create --label "com.anxi-panel.test-owner=$owner" $freshVolume | Out-Null
        Assert-NativeSuccess 'candidate data volume create'
        & docker run -d --name $freshContainer --label "com.anxi-panel.test-owner=$owner" --mount "type=volume,src=$freshVolume,dst=/data" $Image | Out-Null
        Assert-NativeSuccess 'candidate fresh container start'
        Wait-CandidatePanel -Container $freshContainer -ExpectedVersion $Version -ExpectedCommit $Commit -TimeoutSeconds 120

        $setupRaw = & docker exec $freshContainer wget -qO- http://127.0.0.1:8090/api/setup/status
        Assert-NativeSuccess 'candidate setup status'
        $setup = $setupRaw | ConvertFrom-Json
        if ($setup.initialized -ne $false) {
            throw 'fresh candidate is unexpectedly initialized'
        }

        & docker restart $freshContainer | Out-Null
        Assert-NativeSuccess 'candidate restart'
        Wait-CandidatePanel -Container $freshContainer -ExpectedVersion $Version -ExpectedCommit $Commit -TimeoutSeconds 120
        & docker rm -f $freshContainer | Out-Null
        Assert-NativeSuccess 'fresh candidate cleanup'
        & docker volume rm $freshVolume | Out-Null
        Assert-NativeSuccess 'fresh data cleanup'

        Write-Output 'release candidate: exporting exact image for isolated Web-upgrade E2E'
        & docker save -o $candidateTar $Image
        Assert-NativeSuccess 'candidate image export'
        @($previousRef, 'registry:2', 'nginx:alpine', 'alpine:3.20') | ForEach-Object {
            Invoke-DockerPullWithTimeout -ImageRef $_
        }
        & docker save -o $fixturesTar $previousRef registry:2 nginx:alpine alpine:3.20
        Assert-NativeSuccess 'candidate fixture export'

        $dindArgs = @(
            'run', '-d', '--privileged',
            '--name', $dindContainer,
            '--label', "com.anxi-panel.test-owner=$owner",
            '--env', 'DOCKER_TLS_CERTDIR=',
            '--mount', "type=bind,src=$repoRoot,dst=/workspace,readonly",
            '--mount', "type=bind,src=$taskRoot,dst=/candidate,readonly",
            'docker:29-dind'
        )
        & docker @dindArgs | Out-Null
        Assert-NativeSuccess 'isolated Docker daemon start'

        $dindReady = $false
        for ($attempt = 0; $attempt -lt 90; $attempt++) {
            & docker exec $dindContainer docker info *> $null
            if ($LASTEXITCODE -eq 0) {
                $dindReady = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $dindReady) {
            & docker logs $dindContainer
            throw 'isolated Docker daemon did not become ready'
        }

        $dindToolsReady = $false
        for ($attempt = 1; $attempt -le 3; $attempt++) {
            & docker exec $dindContainer apk add --no-cache bash curl jq openssl sqlite docker-cli-compose zip | Out-Null
            if ($LASTEXITCODE -eq 0) {
                $dindToolsReady = $true
                break
            }
            if ($attempt -lt 3) {
                Write-Warning "release candidate: DinD tool install attempt $attempt failed; retrying the same required package set"
                Start-Sleep -Seconds ($attempt * 5)
            }
        }
        if (-not $dindToolsReady) {
            throw 'isolated candidate tools could not be installed after 3 attempts'
        }
        & docker exec $dindContainer bash /workspace/scripts/tests/test_release_candidate_upgrade.sh --candidate-tar /candidate/candidate.tar --fixtures-tar /candidate/fixtures.tar --candidate-image $Image --version $Version --previous-version $PreviousVersion
        Assert-NativeSuccess 'candidate Web-upgrade E2E'
        & docker rm -f $dindContainer | Out-Null
        Assert-NativeSuccess 'isolated Docker daemon cleanup'

        $metadata = [ordered]@{
            schemaVersion   = 1
            version         = $Version
            previousVersion = $PreviousVersion
            commit          = $Commit
            buildDate       = $BuildDate
            localImage      = $Image
            imageId         = $imageId
        }
        $metadataDirectory = Split-Path -Parent $MetadataOutput
        if (-not [string]::IsNullOrWhiteSpace($metadataDirectory)) {
            New-Item -ItemType Directory -Path $metadataDirectory -Force | Out-Null
        }
        $utf8NoBom = [Text.UTF8Encoding]::new($false)
        [IO.File]::WriteAllText($MetadataOutput, ($metadata | ConvertTo-Json) + [Environment]::NewLine, $utf8NoBom)
        Write-Output "release candidate: all gates passed; metadata written to $MetadataOutput"
    } finally {
        & docker rm -f $freshContainer $dindContainer *> $null
        & docker volume inspect $freshVolume *> $null
        if ($LASTEXITCODE -eq 0) {
            & docker volume rm $freshVolume *> $null
        }
        if ($taskRootCreated) {
            $resolvedAgentsRoot = [IO.Path]::GetFullPath($agentsRoot)
            $resolvedTaskRoot = [IO.Path]::GetFullPath($taskRoot)
            $expectedPrefix = $resolvedAgentsRoot.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar + 'anxi-release-candidate-'
            if ($resolvedTaskRoot.StartsWith($expectedPrefix, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTaskRoot -PathType Container)) {
                Remove-Item -LiteralPath $resolvedTaskRoot -Recurse -Force
            }
        }
    }
} finally {
    Pop-Location
}
