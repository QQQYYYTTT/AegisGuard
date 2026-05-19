# PowerShell wrapper: scripts/run_asb_benchmark.ps1
# 用法 (在仓库根目录运行):
# .\scripts\run_asb_benchmark.ps1 -Benchmark "data\agent_task_pot.jsonl" -Venv ".venv"
param(
  [string]$Benchmark = "data\agent_task_pot.jsonl",
  [string]$Venv = ".venv",
  [string]$Requirements = "requirements.txt",
  [string]$Output = ""
)

$root = Split-Path -Parent $MyInvocation.MyCommand.Path | Split-Path -Parent
Write-Host "Repo root: $root"

$python = $Venv
if (Test-Path (Join-Path $Venv "Scripts\python.exe")) {
  $python = Join-Path $Venv "Scripts\python.exe"
}

$script = Join-Path $root "scripts\run_asb_benchmark.py"
$cmd = @($python, $script, '--benchmark', $Benchmark, '--venv', $python, '--requirements', $Requirements)
if ($Output -ne "") { $cmd += @('--output', $Output) }

Write-Host "Running: $($cmd -join ' ')"
& $cmd

if ($LASTEXITCODE -ne 0) { Write-Error "Benchmark failed with exit code $LASTEXITCODE" } else { Write-Host "Benchmark finished" }
