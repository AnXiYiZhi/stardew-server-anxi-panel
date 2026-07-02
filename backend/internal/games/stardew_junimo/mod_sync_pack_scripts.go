package stardew_junimo

const installBatchScript = `@echo off
chcp 65001 >nul
set "SCRIPT_DIR=%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%tools\install.ps1" %*
pause
`

const uninstallBatchScript = `@echo off
chcp 65001 >nul
set "SCRIPT_DIR=%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%tools\uninstall.ps1" %*
pause
`

const playerSyncReadme = `Stardew Anxi Panel 玩家同步包

推荐用法：
1. 解压整个 stardew-player-sync-pack.zip。
2. 双击「安装玩家同步包.bat」。
3. 按提示确认 Stardew Valley 目录。
4. 安装完成后从 Steam 启动 Stardew Valley。

包结构：
- payload/mods/：需要同步到玩家客户端的 SMAPI Mod。
- payload/smapi/：SMAPI 安装器或 SMAPI 元数据。
- tools/：安装、卸载和 Steam 启动项辅助脚本。
- pack-manifest.json：本包清单。
- checksums.sha256：payload 文件完整性校验。

安装脚本会：
- 检查 Windows、PowerShell、游戏目录写入权限。
- 尝试从 Steam 注册表与 libraryfolders.vdf 定位 Stardew Valley。
- 检查 Stardew Valley.exe / StardewModdingAPI.exe 是否正在运行。
- 校验 payload/mods 与 SMAPI ZIP 的 SHA256。
- 如果包内带 SMAPI ZIP，则解出官方 Windows install.dat payload 安装或更新 SMAPI。
- 备份同名 Mod 后复制本包 Mod 到游戏 Mods 目录。
- 尝试设置 Steam 启动项为 "<Stardew Valley>\StardewModdingAPI.exe" %command%。

安装状态会写入游戏目录下的 .anxi-sync/：
- installed.json
- backups/
- logs/

如果安装失败或想恢复，运行：
powershell -ExecutionPolicy Bypass -File .\tools\uninstall.ps1 -RestoreBackup

说明：
- 卸载脚本只处理本同步包安装/替换过的 Mod；不会默认卸载玩家原本已有的 SMAPI。
- 如果 Steam 正在运行，脚本不会强行修改 Steam 启动项，会打印可手动复制的启动项文本。
- 如果包内未携带 SMAPI ZIP，脚本会继续安装 Mod，并提示玩家先安装 SMAPI。
`

const playerModsUpdateReadme = `Stardew Anxi Panel 模组更新包

推荐用法：
1. 只发给已经运行过完整版玩家同步包、或已经手动安装好 SMAPI 的玩家。
2. 解压整个 stardew-player-mods-update-pack.zip。
3. 双击「安装模组更新.bat」。
4. 安装脚本会跳过完全相同的 Mod，只备份并覆盖内容不同的同名 Mod。

包结构：
- payload/mods/：本次需要同步到玩家客户端的 SMAPI Mod。
- tools/：安装、卸载和辅助脚本。
- pack-manifest.json：本包清单，packType 为 mods_update。
- checksums.sha256：payload 文件完整性校验。

注意：
- 本更新包不携带 SMAPI 安装器。
- 如果玩家电脑中未检测到 StardewModdingAPI.exe，脚本会停止，并提示先运行完整版玩家同步包。
- 本更新包不会读取或修改 Steam 启动项；会沿用完整版同步包已经设置好的启动项。
- 卸载脚本只处理本次更新包安装/替换过的 Mod；不会卸载 SMAPI。
`

const installPowerShellScript = `param(
  [string]$GamePath,
  [switch]$SkipSteamLaunchOptions
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
Set-StrictMode -Version 2.0

$PackRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$LogRoot = Join-Path $PackRoot ".anxi-sync\logs"
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null
$LogPath = Join-Path $LogRoot ("install-{0}.log" -f (Get-Date -Format "yyyyMMdd-HHmmss"))

$script:LastProgressPercent = -1
$script:LastProgressStatus = ""
$script:ProgressVisible = $false
$script:ProgressLineLength = 0
$script:CarriageReturn = [string][Convert]::ToChar(13)
$script:LogEncoding = New-Object System.Text.UTF8Encoding $false

function Write-LogLine([string]$Line) {
  for ($attempt = 0; $attempt -lt 5; $attempt++) {
    try {
      [System.IO.File]::AppendAllText($LogPath, $Line + [Environment]::NewLine, $script:LogEncoding)
      return
    } catch {
      Start-Sleep -Milliseconds (50 * ($attempt + 1))
    }
  }
}

function Test-ProgressConsole {
  try {
    return -not [Console]::IsOutputRedirected
  } catch {
    return $false
  }
}

function Get-InstallProgressStage([int]$Percent) {
  if ($Percent -lt 15) { return "START" }
  if ($Percent -lt 26) { return "LOCATE" }
  if ($Percent -lt 53) { return "CHECK" }
  if ($Percent -lt 71) { return "SMAPI" }
  if ($Percent -lt 90) { return "MODS" }
  if ($Percent -lt 96) { return "STEAM" }
  if ($Percent -lt 100) { return "RECORD" }
  return "DONE"
}

function Render-InstallProgressLine([int]$Percent) {
  if (-not (Test-ProgressConsole)) { return }
  $barWidth = 28
  $filled = [int][Math]::Floor(($Percent * $barWidth) / 100)
  if ($filled -lt 0) { $filled = 0 }
  if ($filled -gt $barWidth) { $filled = $barWidth }
  $bar = ("=" * $filled) + (" " * ($barWidth - $filled))
  $stage = Get-InstallProgressStage $Percent
  $renderLine = "[{0}] {1,3}% {2}" -f $bar, $Percent, $stage
  $padding = ""
  if ($script:ProgressLineLength -gt $renderLine.Length) {
    $padding = " " * ($script:ProgressLineLength - $renderLine.Length)
  }
  [Console]::Write($script:CarriageReturn + $renderLine + $padding)
  $script:ProgressLineLength = $renderLine.Length
  $script:ProgressVisible = $true
}

function Clear-InstallProgressLine {
  if (-not $script:ProgressVisible) { return }
  if (-not (Test-ProgressConsole)) {
    $script:ProgressVisible = $false
    return
  }
  $blank = " " * $script:ProgressLineLength
  [Console]::Write($script:CarriageReturn + $blank + $script:CarriageReturn)
  $script:ProgressVisible = $false
}

function Redraw-InstallProgressLine {
  if ($script:LastProgressPercent -ge 0) {
    Render-InstallProgressLine $script:LastProgressPercent
  }
}

function Finish-InstallProgressLine {
  if (-not $script:ProgressVisible) { return }
  if (Test-ProgressConsole) {
    [Console]::Write([Environment]::NewLine)
  }
  $script:ProgressVisible = $false
  $script:ProgressLineLength = 0
}

function Write-Step([string]$Message) {
  $line = "[{0}] {1}" -f (Get-Date -Format "HH:mm:ss"), $Message
  Clear-InstallProgressLine
  Write-Host $line
  Write-LogLine $line
  Redraw-InstallProgressLine
}

function Show-InstallProgress([int]$Percent, [string]$Status) {
  if ($Percent -lt 0) { $Percent = 0 }
  if ($Percent -gt 100) { $Percent = 100 }
  if ($Percent -eq $script:LastProgressPercent -and $Status -eq $script:LastProgressStatus) {
    return
  }
  $script:LastProgressPercent = $Percent
  $script:LastProgressStatus = $Status
  $line = "[{0}] progress {1,3}%  {2}" -f (Get-Date -Format "HH:mm:ss"), $Percent, $Status
  Write-LogLine $line
  Render-InstallProgressLine $Percent
}

function Complete-InstallProgress([string]$Status) {
  Show-InstallProgress 100 $Status
  Finish-InstallProgressLine
}

function Fail-WithRestoreHint([string]$Message, [string]$ResolvedGameDir) {
  Write-Step "失败：$Message"
  Clear-InstallProgressLine
  if ($ResolvedGameDir) {
    Write-Host ""
    Write-Host "可尝试恢复备份："
    $restoreCommand = 'powershell -ExecutionPolicy Bypass -File "' + (Join-Path $PackRoot "tools\uninstall.ps1") + '" -GamePath "' + $ResolvedGameDir + '" -RestoreBackup'
    Write-Host $restoreCommand
  }
  throw $Message
}

. (Join-Path $PackRoot "tools\vdf.ps1")

function Test-WindowsEnvironment {
  $isWindowsValue = $true
  if (Get-Variable -Name IsWindows -Scope Global -ErrorAction SilentlyContinue) {
    $isWindowsValue = $IsWindows
  }
  if (-not $isWindowsValue -and $env:OS -ne "Windows_NT") {
    throw "本安装包仅支持 Windows。"
  }
  if ($PSVersionTable.PSVersion.Major -lt 5) {
    throw "需要 PowerShell 5.0 或更高版本。"
  }
  $probe = Join-Path $PackRoot ".anxi-sync\write-test.tmp"
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $probe) | Out-Null
  Set-Content -Path $probe -Value "ok" -Encoding ASCII
  Remove-Item -Force $probe
}

function Get-SteamPath {
  $candidates = @(
    "HKCU:\Software\Valve\Steam",
    "HKLM:\SOFTWARE\WOW6432Node\Valve\Steam",
    "HKLM:\SOFTWARE\Valve\Steam"
  )
  foreach ($key in $candidates) {
    try {
      $item = Get-ItemProperty -Path $key -ErrorAction Stop
      foreach ($prop in @("SteamPath", "InstallPath")) {
        if ($item.$prop -and (Test-Path $item.$prop)) {
          return (Resolve-Path $item.$prop).Path
        }
      }
    } catch {}
  }
  return $null
}

function Find-StardewFromSteam {
  $steam = Get-SteamPath
  if (-not $steam) { return $null }
  $libraries = Get-SteamLibraryPaths -SteamPath $steam
  foreach ($library in $libraries) {
    $manifest = Join-Path $library "steamapps\appmanifest_413150.acf"
    if (-not (Test-Path $manifest)) { continue }
    $installDir = Get-SteamAppInstallDir -ManifestPath $manifest
    if (-not $installDir) { $installDir = "Stardew Valley" }
    $gameDir = Join-Path $library ("steamapps\common\{0}" -f $installDir)
    if (Test-Path (Join-Path $gameDir "Stardew Valley.exe")) {
      return (Resolve-Path $gameDir).Path
    }
  }
  return $null
}

function Resolve-StardewGameDir([string]$InputPath) {
  if ($InputPath) {
    if (Test-Path (Join-Path $InputPath "Stardew Valley.exe")) {
      return (Resolve-Path $InputPath).Path
    }
    throw "传入路径不是 Stardew Valley 目录：$InputPath"
  }
  $found = Find-StardewFromSteam
  if ($found) { return $found }
  Clear-InstallProgressLine
  Write-Host "未能自动定位 Stardew Valley。请拖入或粘贴游戏目录，例如：C:\Program Files (x86)\Steam\steamapps\common\Stardew Valley"
  while ($true) {
    $manual = (Read-Host "Stardew Valley 目录").Trim('" ')
    if (Test-Path (Join-Path $manual "Stardew Valley.exe")) {
      return (Resolve-Path $manual).Path
    }
    Clear-InstallProgressLine
    Write-Host "目录无效，请确认其中包含 Stardew Valley.exe。"
  }
}

function Assert-GameNotRunning {
  $running = Get-Process -ErrorAction SilentlyContinue |
    Where-Object { $_.ProcessName -in @("Stardew Valley", "StardewModdingAPI") }
  if ($running) {
    $names = ($running | Select-Object -ExpandProperty ProcessName -Unique) -join ", "
    throw "检测到游戏进程正在运行：$names。请先关闭游戏再重试。"
  }
}

function Assert-GameDirWritable([string]$GameDir) {
  $stateDir = Join-Path $GameDir ".anxi-sync"
  New-Item -ItemType Directory -Force -Path $stateDir | Out-Null
  $probe = Join-Path $stateDir "write-test.tmp"
  Set-Content -Path $probe -Value "ok" -Encoding ASCII
  Remove-Item -Force $probe
}

function Test-PayloadChecksums([int]$BasePercent = 30, [int]$Span = 20) {
  $checksumPath = Join-Path $PackRoot "checksums.sha256"
  if (-not (Test-Path $checksumPath)) {
    throw "缺少 checksums.sha256。"
  }
  $lines = @(Get-Content -Path $checksumPath -Encoding UTF8 | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
  $total = [Math]::Max(1, $lines.Count)
  $index = 0
  foreach ($line in $lines) {
    $index++
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    if ($line -notmatch '^([a-fA-F0-9]{64})\s+\*?(.+)$') {
      throw "checksum 行格式无效：$line"
    }
    $expected = $Matches[1].ToLowerInvariant()
    $relative = $Matches[2].Trim()
    $file = Join-Path $PackRoot ($relative -replace "/", [IO.Path]::DirectorySeparatorChar)
    if (-not (Test-Path -LiteralPath $file)) {
      throw "checksum 指向的文件不存在：$relative"
    }
    $percent = $BasePercent + [int][Math]::Floor(($index - 1) * $Span / $total)
    Show-InstallProgress $percent ("校验文件 {0}/{1}: {2}" -f $index, $total, $relative)
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
      throw "文件校验失败：$relative"
    }
  }
  Show-InstallProgress ($BasePercent + $Span) "payload 完整性校验完成"
}

function Get-RelativePath([string]$Root, [string]$Path) {
  $rootFull = [IO.Path]::GetFullPath($Root).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
  $pathFull = [IO.Path]::GetFullPath($Path)
  $rootUri = New-Object System.Uri(($rootFull + [IO.Path]::DirectorySeparatorChar))
  $pathUri = New-Object System.Uri($pathFull)
  return [Uri]::UnescapeDataString($rootUri.MakeRelativeUri($pathUri).ToString()).Replace("/", "\")
}

function Get-DirectoryFingerprint([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) { return $null }
  $separator = [string][char]9
  $entries = New-Object System.Collections.Generic.List[string]
  $dirs = Get-ChildItem -LiteralPath $Path -Directory -Recurse -Force -ErrorAction SilentlyContinue |
    Sort-Object FullName
  foreach ($dir in $dirs) {
    $relative = (Get-RelativePath $Path $dir.FullName).Replace("\", "/")
    $entries.Add(("D" + $separator + $relative))
  }
  $files = Get-ChildItem -LiteralPath $Path -File -Recurse -Force -ErrorAction SilentlyContinue |
    Sort-Object FullName
  foreach ($file in $files) {
    $relative = (Get-RelativePath $Path $file.FullName).Replace("\", "/")
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
    $entries.Add(("F" + $separator + $relative + $separator + $file.Length + $separator + $hash))
  }
  $text = [string]::Join([string][char]10, $entries)
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $bytes = [Text.Encoding]::UTF8.GetBytes($text)
    return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace("-", "").ToLowerInvariant()
  } finally {
    $sha.Dispose()
  }
}

function Install-SMAPIIfBundled([string]$GameDir) {
  $zip = Get-ChildItem -Path (Join-Path $PackRoot "payload\smapi") -Filter "SMAPI*.zip" -File -ErrorAction SilentlyContinue |
    Sort-Object Name -Descending |
    Select-Object -First 1
  if (-not $zip) {
    Write-Step "未发现包内 SMAPI 安装器；将只安装 Mod，并提示玩家自行安装 SMAPI。"
    return @{ installed = $false; reason = "smapi_zip_missing" }
  }
  $temp = Join-Path ([IO.Path]::GetTempPath()) ("stardew-smapi-{0}" -f ([guid]::NewGuid()))
  New-Item -ItemType Directory -Force -Path $temp | Out-Null
  try {
    Show-InstallProgress 56 "解压 SMAPI 安装包"
    Expand-Archive -LiteralPath $zip.FullName -DestinationPath $temp -Force
    $installPayload = Get-ChildItem -LiteralPath $temp -Recurse -File -Filter "install.dat" |
      Where-Object { $_.FullName -match "\\internal\\windows\\install\.dat$" } |
      Select-Object -First 1
    if (-not $installPayload) {
      $installPayload = Get-ChildItem -LiteralPath $temp -Recurse -File -Filter "install.dat" |
        Select-Object -First 1
    }
    if (-not $installPayload) {
      Write-Step "未能在 SMAPI ZIP 内找到官方 Windows install.dat payload。"
      return @{ installed = $false; reason = "install_payload_not_found"; zip = $zip.Name }
    }
    Write-Step "安装/更新 SMAPI：$($zip.Name)"
    Show-InstallProgress 62 "释放 SMAPI 官方安装文件"
    $payloadZip = Join-Path $temp "smapi-install-payload.zip"
    Copy-Item -LiteralPath $installPayload.FullName -Destination $payloadZip -Force
    Expand-Archive -LiteralPath $payloadZip -DestinationPath $GameDir -Force
    if (-not (Test-Path -LiteralPath (Join-Path $GameDir "StardewModdingAPI.exe"))) {
      throw "SMAPI 官方 payload 解压后未发现 StardewModdingAPI.exe。"
    }
    Show-InstallProgress 70 "SMAPI 安装完成"
    return @{ installed = $true; zip = $zip.Name; method = "official_install_payload" }
  } finally {
    Remove-Item -Recurse -Force $temp -ErrorAction SilentlyContinue
  }
}

function Assert-SMAPIAlreadyInstalled([string]$GameDir) {
  $smapiExe = Join-Path $GameDir "StardewModdingAPI.exe"
  if (-not (Test-Path -LiteralPath $smapiExe)) {
    throw "未检测到 SMAPI。请先运行完整版玩家同步包，或先手动安装 SMAPI 后再运行本模组更新包。"
  }
  Write-Step "已检测到 SMAPI，本次只安装/更新 Mod。"
  Show-InstallProgress 70 "SMAPI 已存在"
  return @{ installed = $false; bundled = $false; existing = $true; reason = "already_installed" }
}

function Install-ModPayload([string]$GameDir, [object]$Manifest, [int]$BasePercent = 72, [int]$Span = 16) {
  $modsDir = Join-Path $GameDir "Mods"
  New-Item -ItemType Directory -Force -Path $modsDir | Out-Null
  $backupId = Get-Date -Format "yyyyMMdd-HHmmss"
  $backupRoot = Join-Path $GameDir ".anxi-sync\backups\$backupId\Mods"
  $backupCreated = $false
  $installed = @()
  $packagedMods = @($Manifest.mods | Where-Object { $_.packaged })
  $total = [Math]::Max(1, $packagedMods.Count)
  $index = 0
  foreach ($mod in $packagedMods) {
    $index++
    if (-not $mod.packaged) { continue }
    $name = [string]$mod.folderName
    if ([string]::IsNullOrWhiteSpace($name)) { continue }
    $percent = $BasePercent + [int][Math]::Floor(($index - 1) * $Span / $total)
    Show-InstallProgress $percent ("安装 Mod {0}/{1}: {2}" -f $index, $total, $name)
    $source = Join-Path $PackRoot ("payload\mods\{0}" -f $name)
    if (-not (Test-Path -LiteralPath $source)) {
      throw "包内缺少 Mod 目录：$name"
    }
    $target = Join-Path $modsDir $name
    $hadExisting = Test-Path -LiteralPath $target
    $skippedIdentical = $false
    if ($hadExisting) {
      Show-InstallProgress $percent ("比对 Mod {0}/{1}: {2}" -f $index, $total, $name)
      $sourceFingerprint = Get-DirectoryFingerprint $source
      $targetFingerprint = Get-DirectoryFingerprint $target
      if ($sourceFingerprint -and $sourceFingerprint -eq $targetFingerprint) {
        $skippedIdentical = $true
        Write-Step "已跳过相同 Mod：$name"
      }
    }
    if ($hadExisting) {
      if (-not $skippedIdentical) {
        New-Item -ItemType Directory -Force -Path $backupRoot | Out-Null
        $backupCreated = $true
        Move-Item -LiteralPath $target -Destination (Join-Path $backupRoot $name)
        Write-Step "已备份原有 Mod：$name"
      }
    }
    if (-not $skippedIdentical) {
      Copy-Item -LiteralPath $source -Destination $target -Recurse -Force
    }
    $installed += [pscustomobject]@{
      folderName = $name
      uniqueId = [string]$mod.uniqueId
      name = [string]$mod.name
      version = [string]$mod.version
      hadExisting = $hadExisting
      skippedIdentical = $skippedIdentical
    }
    if (-not $skippedIdentical) {
      Write-Step "已安装 Mod：$name"
    }
  }
  Show-InstallProgress ($BasePercent + $Span) "Mod 安装完成"
  $resultBackupId = $null
  if ($backupCreated) { $resultBackupId = $backupId }
  return @{ backupId = $resultBackupId; mods = $installed }
}

try {
  Show-InstallProgress 2 "开始"
  Write-Step "开始安装 Stardew 玩家同步包。"
  Show-InstallProgress 8 "检查 Windows 和解压目录"
  Test-WindowsEnvironment
  Show-InstallProgress 14 "读取同步包清单"
  $manifestPath = Join-Path $PackRoot "pack-manifest.json"
  if (-not (Test-Path $manifestPath)) { throw "缺少 pack-manifest.json。" }
  $manifest = Get-Content -Path $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
  $packType = [string]$manifest.packType
  if ([string]::IsNullOrWhiteSpace($packType)) { $packType = "full" }
  Show-InstallProgress 20 "定位 Stardew Valley"
  $gameDir = Resolve-StardewGameDir $GamePath
  Write-Step "游戏目录：$gameDir"
  Show-InstallProgress 25 "检查游戏进程和目录权限"
  Assert-GameNotRunning
  Assert-GameDirWritable $gameDir
  Test-PayloadChecksums -BasePercent 30 -Span 20

  Show-InstallProgress 52 "准备安装记录目录"
  $stateDir = Join-Path $gameDir ".anxi-sync"
  New-Item -ItemType Directory -Force -Path (Join-Path $stateDir "logs") | Out-Null
  Copy-Item -LiteralPath $LogPath -Destination (Join-Path $stateDir "logs") -Force -ErrorAction SilentlyContinue

  Show-InstallProgress 55 "准备安装 SMAPI"
  if ($packType -eq "mods_update") {
    $smapiResult = Assert-SMAPIAlreadyInstalled $gameDir
  } else {
    $smapiResult = Install-SMAPIIfBundled $gameDir
  }
  $modResult = Install-ModPayload $gameDir $manifest -BasePercent 72 -Span 16

  $steamResult = @{ configured = $false; skipped = $true; reason = "skipped" }
  if ($packType -eq "mods_update") {
    $steamResult = @{ configured = $false; skipped = $true; reason = "mods_update_pack" }
    Show-InstallProgress 90 "跳过 Steam 启动项"
    Write-Step "模组更新包已跳过 Steam 启动项配置。"
  } elseif (-not $SkipSteamLaunchOptions) {
    Show-InstallProgress 90 "配置 Steam 启动项"
    $steamScript = Join-Path $PackRoot "tools\steam-launch-options.ps1"
    Clear-InstallProgressLine
    $steamResult = & $steamScript -GameDir $gameDir
  }

  Show-InstallProgress 96 "写入安装记录"
  $recordBackupId = $null
  if (-not [string]::IsNullOrWhiteSpace([string]$modResult.backupId)) {
    $recordBackupId = [string]$modResult.backupId
  }
  $installed = [pscustomobject]@{
    installedAt = (Get-Date).ToUniversalTime().ToString("o")
    packId = [string]$manifest.packId
    packVersion = [string]$manifest.packVersion
    gameDir = $gameDir
    backupId = $recordBackupId
    smapi = $smapiResult
    steam = $steamResult
    mods = $modResult.mods
    logPath = $LogPath
  }
  $installed | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $stateDir "installed.json") -Encoding UTF8
  Complete-InstallProgress "安装完成"

  Write-Host ""
  Write-Host "安装完成。"
  $smapiSummary = "未随包安装"
  if ($smapiResult.installed) {
    $smapiSummary = "已安装/更新"
  } elseif ($smapiResult.existing) {
    $smapiSummary = "已存在，本次未改动"
  }
  Write-Host ("SMAPI：{0}" -f $smapiSummary)
  Write-Host ("Mod：{0}" -f (($modResult.mods | ForEach-Object { $_.folderName }) -join ", "))
  $steamReason = $null
  if ($steamResult.ContainsKey("reason")) {
    $steamReason = [string]$steamResult.reason
  }
  $steamSummary = "未自动设置"
  if ($steamResult.configured) {
    $steamSummary = "已设置"
  } elseif ($steamReason -eq "mods_update_pack") {
    $steamSummary = "已跳过，沿用已有设置"
  }
  Write-Host ("Steam 启动项：{0}" -f $steamSummary)
  if ($steamReason -ne "mods_update_pack") {
    $smapiExePath = Join-Path $gameDir "StardewModdingAPI.exe"
    $launchOptionsText = $steamResult.launchOptions
    if (-not $launchOptionsText) {
      $launchOptionsText = '"{0}" %command%' -f $smapiExePath
    }
    Write-Host "Steam 启动项文本：" -ForegroundColor Yellow
    Write-Host "请复制到 Steam 的游戏启动项中。" -ForegroundColor Yellow
    Write-Host $launchOptionsText -ForegroundColor Cyan
  }
  Write-Host "日志：$LogPath"
} catch {
  if ($gameDir) {
    Fail-WithRestoreHint $_.Exception.Message $gameDir
  }
  throw
}
`

const uninstallPowerShellScript = `param(
  [string]$GamePath,
  [switch]$RestoreBackup
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

$PackRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
. (Join-Path $PackRoot "tools\vdf.ps1")

function Resolve-GameDirForUninstall([string]$InputPath) {
  if ($InputPath -and (Test-Path (Join-Path $InputPath ".anxi-sync\installed.json"))) {
    return (Resolve-Path $InputPath).Path
  }
  if ($InputPath -and (Test-Path (Join-Path $InputPath "Stardew Valley.exe"))) {
    return (Resolve-Path $InputPath).Path
  }
  Write-Host "请粘贴 Stardew Valley 目录。"
  while ($true) {
    $manual = (Read-Host "Stardew Valley 目录").Trim('" ')
    if (Test-Path (Join-Path $manual "Stardew Valley.exe")) {
      return (Resolve-Path $manual).Path
    }
    Write-Host "目录无效。"
  }
}

$gameDir = Resolve-GameDirForUninstall $GamePath
$stateDir = Join-Path $gameDir ".anxi-sync"
$installedPath = Join-Path $stateDir "installed.json"
if (-not (Test-Path -LiteralPath $installedPath)) {
  throw "未找到安装记录：$installedPath"
}
$installed = Get-Content -Path $installedPath -Raw -Encoding UTF8 | ConvertFrom-Json
$modsDir = Join-Path $gameDir "Mods"
$backupMods = Join-Path $stateDir ("backups\{0}\Mods" -f $installed.backupId)

foreach ($mod in $installed.mods) {
  $name = [string]$mod.folderName
  if ([string]::IsNullOrWhiteSpace($name)) { continue }
  $target = Join-Path $modsDir $name
  if (Test-Path -LiteralPath $target) {
    Remove-Item -LiteralPath $target -Recurse -Force
    Write-Host "已移除本包 Mod：$name"
  }
  $backup = Join-Path $backupMods $name
  if ($RestoreBackup -and (Test-Path -LiteralPath $backup)) {
    Copy-Item -LiteralPath $backup -Destination $target -Recurse -Force
    Write-Host "已恢复备份 Mod：$name"
  }
}

Write-Host "卸载完成。SMAPI 不会被自动卸载；如需移除请使用 SMAPI 官方卸载器。"
`

const steamLaunchOptionsPowerShellScript = `param(
  [Parameter(Mandatory=$true)][string]$GameDir
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) "vdf.ps1")

function Get-SteamPathForLaunchOptions {
  foreach ($key in @("HKCU:\Software\Valve\Steam", "HKLM:\SOFTWARE\WOW6432Node\Valve\Steam", "HKLM:\SOFTWARE\Valve\Steam")) {
    try {
      $item = Get-ItemProperty -Path $key -ErrorAction Stop
      foreach ($prop in @("SteamPath", "InstallPath")) {
        if ($item.$prop -and (Test-Path $item.$prop)) { return (Resolve-Path $item.$prop).Path }
      }
    } catch {}
  }
  return $null
}

$launchOptions = '"{0}" %command%' -f (Join-Path $GameDir "StardewModdingAPI.exe")
Write-Host "建议 Steam 启动项：" -ForegroundColor Yellow
Write-Host "请复制到 Steam 的游戏启动项中。" -ForegroundColor Yellow
Write-Host $launchOptions -ForegroundColor Cyan

if (Get-Process -Name "steam" -ErrorAction SilentlyContinue) {
  Write-Host "Steam 正在运行，已跳过自动修改。请关闭 Steam 后重试，或手动复制上面的启动项。"
  return @{ configured = $false; skipped = $true; reason = "steam_running"; launchOptions = $launchOptions }
}

$steam = Get-SteamPathForLaunchOptions
if (-not $steam) {
  Write-Host "未找到 Steam 目录，请手动复制启动项。"
  return @{ configured = $false; skipped = $true; reason = "steam_not_found"; launchOptions = $launchOptions }
}

$userdata = Join-Path $steam "userdata"
if (-not (Test-Path $userdata)) {
  Write-Host "未找到 Steam userdata，请手动复制启动项。"
  return @{ configured = $false; skipped = $true; reason = "userdata_not_found"; launchOptions = $launchOptions }
}

$configs = Get-ChildItem -Path $userdata -Recurse -Filter "localconfig.vdf" -File -ErrorAction SilentlyContinue |
  Where-Object { $_.FullName -match "\\config\\localconfig\.vdf$" }
if (-not $configs -or $configs.Count -ne 1) {
  Write-Host "无法唯一确定当前 Steam 用户，请手动复制启动项。"
  return @{ configured = $false; skipped = $true; reason = "steam_user_not_unique"; launchOptions = $launchOptions }
}

$config = $configs[0].FullName
$backup = Backup-FileWithTimestamp -Path $config
$changed = Set-StardewLaunchOptionsInLocalConfig -Path $config -LaunchOptions $launchOptions
if (-not $changed) {
  Write-Host "未能可靠修改 localconfig.vdf，请手动复制启动项。备份：$backup"
  return @{ configured = $false; skipped = $true; reason = "vdf_update_failed"; launchOptions = $launchOptions; localConfig = $config; backup = $backup }
}
Write-Host "已设置 Steam 启动项。备份：$backup"
return @{ configured = $true; skipped = $false; launchOptions = $launchOptions; localConfig = $config; backup = $backup }
`

const vdfPowerShellScript = `function ConvertFrom-VDFString([string]$Value) {
  return $Value -replace '\\"','"' -replace '\\\\','\'
}

function ConvertTo-VDFString([string]$Value) {
  return ($Value -replace '\\','\\' -replace '"','\"')
}

function Get-SteamLibraryPaths([string]$SteamPath) {
  $paths = New-Object System.Collections.Generic.List[string]
  if ($SteamPath -and (Test-Path $SteamPath)) { $paths.Add((Resolve-Path $SteamPath).Path) }
  $vdf = Join-Path $SteamPath "steamapps\libraryfolders.vdf"
  if (Test-Path $vdf) {
    foreach ($line in Get-Content -Path $vdf -Encoding UTF8) {
      if ($line -match '"path"\s+"([^"]+)"') {
        $p = ConvertFrom-VDFString $Matches[1]
        if (Test-Path $p) { $paths.Add((Resolve-Path $p).Path) }
      }
    }
  }
  return $paths | Select-Object -Unique
}

function Get-SteamAppInstallDir([string]$ManifestPath) {
  if (-not (Test-Path $ManifestPath)) { return $null }
  foreach ($line in Get-Content -Path $ManifestPath -Encoding UTF8) {
    if ($line -match '"installdir"\s+"([^"]+)"') {
      return ConvertFrom-VDFString $Matches[1]
    }
  }
  return $null
}

function Backup-FileWithTimestamp([string]$Path) {
  $backup = "{0}.anxi-sync.{1}.bak" -f $Path, (Get-Date -Format "yyyyMMdd-HHmmss")
  Copy-Item -LiteralPath $Path -Destination $backup -Force
  return $backup
}

function Set-StardewLaunchOptionsInLocalConfig([string]$Path, [string]$LaunchOptions) {
  $content = Get-Content -Path $Path -Raw -Encoding UTF8
  $escaped = ConvertTo-VDFString $LaunchOptions

  if ($content -match '"413150"') {
    if ($content -match '"LaunchOptions"\s+"[^"]*"') {
      $content = [regex]::Replace($content, '"LaunchOptions"\s+"[^"]*"', ('"LaunchOptions"		"{0}"' -f $escaped), 1)
      Set-Content -Path $Path -Value $content -Encoding UTF8
      return $true
    }
    $idx = $content.IndexOf('"413150"')
    $open = $content.IndexOf('{', $idx)
    if ($open -ge 0) {
      $insert = [Environment]::NewLine + '					"LaunchOptions"		"' + $escaped + '"'
      $content = $content.Insert($open + 1, $insert)
      Set-Content -Path $Path -Value $content -Encoding UTF8
      return $true
    }
  }
  return $false
}
`
