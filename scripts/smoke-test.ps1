#Requires -Version 5.1
<#
.SYNOPSIS
    Anxi Panel 发版冒烟测试脚本
.DESCRIPTION
    在本地 Windows 开发环境验证后端测试、前端构建、Docker 镜像构建。
    不会修改用户真实数据，仅清理脚本自己创建的临时资源。
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
#>

param(
    [switch]$SkipDocker,
    [switch]$SkipFrontend,
    [switch]$SkipBackend
)

$ErrorActionPreference = 'Continue'
$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$exitCode = 0
$passed = 0
$failed = 0
$skipped = 0

function Write-Step {
    param([string]$Name, [string]$Status, [string]$Detail = '')
    $icon = switch ($Status) {
        'PASS' { '[PASS]'; break }
        'FAIL' { '[FAIL]'; break }
        'SKIP' { '[SKIP]'; break }
        'INFO' { '[INFO]'; break }
        default { '[????]'; break }
    }
    $color = switch ($Status) {
        'PASS' { 'Green'; break }
        'FAIL' { 'Red'; break }
        'SKIP' { 'Yellow'; break }
        'INFO' { 'Cyan'; break }
        default { 'White'; break }
    }
    $msg = "$icon $Name"
    if ($Detail) { $msg += " -- $Detail" }
    Write-Host $msg -ForegroundColor $color
}

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action,
        [switch]$Skip
    )
    if ($Skip) {
        Write-Step $Name 'SKIP' '已跳过'
        $script:skipped++
        return
    }
    Write-Step $Name 'INFO' '执行中...'
    try {
        $result = & $Action
        if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) {
            Write-Step $Name 'FAIL' "退出码: $LASTEXITCODE"
            $script:failed++
            $script:exitCode = 1
        } else {
            Write-Step $Name 'PASS'
            $script:passed++
        }
    } catch {
        Write-Step $Name 'FAIL' $_.Exception.Message
        $script:failed++
        $script:exitCode = 1
    }
}

Write-Host ''
Write-Host '========================================' -ForegroundColor Cyan
Write-Host '  Anxi Panel 发版冒烟测试' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host "项目目录: $projectRoot"
Write-Host ''

# ── 1. 后端测试 ──────────────────────────────────────────────────────────────

Invoke-Step -Name '后端测试 (go test ./...)' -Skip:$SkipBackend -Action {
    Push-Location (Join-Path $projectRoot 'backend')
    try {
        $env:GOCACHE = Join-Path $projectRoot '.gocache'
        $output = & go test ./... 2>&1
        $output | ForEach-Object { Write-Host "  $_" }
        if ($LASTEXITCODE -ne 0) {
            throw "go test 失败，退出码: $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

# ── 2. 前端构建 ──────────────────────────────────────────────────────────────

Invoke-Step -Name '前端构建 (npm run build)' -Skip:$SkipFrontend -Action {
    Push-Location (Join-Path $projectRoot 'frontend')
    try {
        $output = & npm.cmd run build 2>&1
        $output | ForEach-Object { Write-Host "  $_" }
        if ($LASTEXITCODE -ne 0) {
            throw "npm build 失败，退出码: $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

# ── 3. Docker 镜像构建 ──────────────────────────────────────────────────────

$dockerImage = 'anxi-panel-smoke-test'
$dockerContainer = 'anxi-panel-smoke-test'
$dockerVolume = 'anxi-panel-smoke-test-data'

Invoke-Step -Name 'Docker 镜像构建' -Skip:$SkipDocker -Action {
    Push-Location $projectRoot
    try {
        $output = & docker build -t $script:dockerImage --build-arg VERSION=smoke-test --build-arg COMMIT=smoke --build-arg BUILD_DATE=(Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ') . 2>&1
        $output | ForEach-Object { Write-Host "  $_" }
        if ($LASTEXITCODE -ne 0) {
            throw "docker build 失败，退出码: $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

# ── 4. Docker 容器启动与健康检查 ─────────────────────────────────────────────

if (-not $SkipDocker) {
    Invoke-Step -Name 'Docker 容器启动与健康检查' -Action {
        # 清理可能残留的旧容器
        & docker rm -f $dockerContainer 2>$null | Out-Null
        & docker volume rm $dockerVolume 2>$null | Out-Null

        # 启动容器
        $output = & docker run -d `
            --name $dockerContainer `
            -p 18090:8090 `
            -v /var/run/docker.sock:/var/run/docker.sock `
            -v "${dockerVolume}:/data" `
            $dockerImage 2>&1
        $output | ForEach-Object { Write-Host "  $_" }
        if ($LASTEXITCODE -ne 0) {
            throw "docker run 失败，退出码: $LASTEXITCODE"
        }

        # 等待容器启动
        Write-Host '  等待容器启动...'
        Start-Sleep -Seconds 5

        # 检查容器是否在运行
        $containerState = & docker inspect -f '{{.State.Running}}' $dockerContainer 2>&1
        if ($containerState -ne 'true') {
            $logs = & docker logs $dockerContainer 2>&1
            $logs | ForEach-Object { Write-Host "  $_" }
            throw '容器未在运行'
        }

        # 检查健康接口
        Write-Host '  检查 /health 接口...'
        $retries = 0
        $maxRetries = 10
        while ($retries -lt $maxRetries) {
            try {
                $response = Invoke-WebRequest -Uri 'http://localhost:18090/health' -UseBasicParsing -TimeoutSec 5
                if ($response.StatusCode -eq 200) {
                    Write-Host "  /health 返回 200: $($response.Content)" -ForegroundColor Green
                    break
                }
            } catch {
                # 继续重试
            }
            $retries++
            Start-Sleep -Seconds 2
        }
        if ($retries -ge $maxRetries) {
            throw '/health 接口在 20 秒内未返回 200'
        }

        # 检查版本信息
        Write-Host '  检查 /api/version 接口...'
        $versionResponse = Invoke-WebRequest -Uri 'http://localhost:18090/api/version' -UseBasicParsing -TimeoutSec 5
        if ($versionResponse.StatusCode -eq 200) {
            Write-Host "  /api/version 返回 200: $($versionResponse.Content)" -ForegroundColor Green
        } else {
            Write-Host '  /api/version 未返回 200' -ForegroundColor Yellow
        }
    }
}

# ── 清理 ─────────────────────────────────────────────────────────────────────

Write-Host ''
Write-Host '清理临时资源...' -ForegroundColor Cyan

if (-not $SkipDocker) {
    & docker rm -f $dockerContainer 2>$null | Out-Null
    & docker volume rm $dockerVolume 2>$null | Out-Null
    & docker rmi $dockerImage 2>$null | Out-Null
    Write-Host '  已清理临时容器、镜像和 volume'
}

# ── 结果汇总 ─────────────────────────────────────────────────────────────────

Write-Host ''
Write-Host '========================================' -ForegroundColor Cyan
Write-Host "  测试结果: $passed 通过, $failed 失败, $skipped 跳过" -ForegroundColor $(if ($failed -gt 0) { 'Red' } else { 'Green' })
Write-Host '========================================' -ForegroundColor Cyan

if ($failed -gt 0) {
    Write-Host ''
    Write-Host '冒烟测试失败，请检查上方错误输出。' -ForegroundColor Red
} else {
    Write-Host ''
    Write-Host '冒烟测试全部通过！' -ForegroundColor Green
}

exit $exitCode
