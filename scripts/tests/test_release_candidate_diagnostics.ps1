$ErrorActionPreference = 'Stop'
$tokens = $null
$errors = $null
$source = Join-Path $PSScriptRoot '../release-candidate.ps1'
$ast = [Management.Automation.Language.Parser]::ParseFile((Resolve-Path -LiteralPath $source), [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw 'candidate wrapper parse failed' }
$functions = @($ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Complete-CandidateUpgrade' }, $true))
if ($functions.Count -ne 1) { throw 'exact diagnostic function missing' }
. ([scriptblock]::Create($functions[0].Extent.Text))
$root = Join-Path ([IO.Path]::GetTempPath()) ('anxi-diagnostics-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $root | Out-Null
$inputPath = Join-Path $root 'source.safe.jsonl'
$outputPath = Join-Path $root 'output.safe.jsonl'
[IO.File]::WriteAllText($inputPath, "{}`n", [Text.UTF8Encoding]::new($false))
function Copy-Item {
    param([string]$LiteralPath, [string]$Destination)
    if ($script:copyFails) { throw 'controlled copy failure' }
    Microsoft.PowerShell.Management\Copy-Item -LiteralPath $LiteralPath -Destination $Destination
}
try {
    foreach ($original in @(0, 42)) {
        foreach ($script:copyFails in @($false, $true)) {
            $result = Complete-CandidateUpgrade -UpgradeExit $original -DiagnosticsSource $inputPath -DiagnosticsOutput $outputPath
            $expected = if ($original -eq 0 -and $script:copyFails) { 1 } else { $original }
            if ($result -ne $expected) { throw 'diagnostic export masked the original upgrade status' }
        }
    }
    Write-Output 'candidate diagnostics: PowerShell export failures preserve the original upgrade exit status'
} finally {
    foreach ($file in @($inputPath, $outputPath)) {
        if (Test-Path -LiteralPath $file -PathType Leaf) { Remove-Item -LiteralPath $file }
    }
    Remove-Item -LiteralPath $root
}
