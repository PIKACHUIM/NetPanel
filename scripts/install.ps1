#Requires -Version 5.1
<#
.SYNOPSIS
    NetPanel Windows 一键安装脚本
.DESCRIPTION
    自动下载、安装 NetPanel 并注册为 Windows 服务
.PARAMETER Version
    指定版本，如 v0.1.0（默认: latest）
.PARAMETER Port
    监听端口（默认: 8080）
.PARAMETER InstallDir
    安装目录（默认: C:\Program Files\NetPanel）
.PARAMETER DataDir
    数据目录（默认: C:\ProgramData\NetPanel）
.PARAMETER NoService
    不注册 Windows 服务
.EXAMPLE
    .\install.ps1
    .\install.ps1 -Version v0.1.0 -Port 9090
    .\install.ps1 -NoService
#>
[CmdletBinding()]
param(
    [string]$Version    = "latest",
    [int]   $Port       = 8080,
    [string]$InstallDir = "$env:ProgramFiles\NetPanel",
    [string]$DataDir    = "$env:ProgramData\NetPanel",
    [switch]$NoService
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ─── 配置 ─────────────────────────────────────────────────────────────────────
$Repo        = "YOUR_ORG/netpanel"
$ServiceName = "NetPanel"
$BinaryName  = "netpanel.exe"
$LogDir      = "$DataDir\logs"

# ─── 颜色输出 ─────────────────────────────────────────────────────────────────
function Write-Info    { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Cyan }
function Write-Success { param($msg) Write-Host "[OK]    $msg" -ForegroundColor Green }
function Write-Warn    { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Fail    { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

# ─── 检查管理员权限 ───────────────────────────────────────────────────────────
function Assert-Admin {
    $current = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not $current.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-Fail "请以管理员身份运行此脚本（右键 -> 以管理员身份运行）"
    }
}

# ─── 检测架构 ─────────────────────────────────────────────────────────────────
function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64"  { return "amd64" }
        "ARM64"  { return "arm64" }
        default  { return "amd64" }
    }
}

# ─── 获取最新版本 ─────────────────────────────────────────────────────────────
function Get-LatestVersion {
    try {
        $resp = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" -TimeoutSec 15
        return $resp.tag_name
    } catch {
        Write-Fail "无法获取最新版本，请使用 -Version 手动指定: $_"
    }
}

# ─── 下载二进制 ───────────────────────────────────────────────────────────────
function Download-Binary {
    param([string]$Ver, [string]$Arch)

    $url     = "https://github.com/$Repo/releases/download/$Ver/netpanel-windows-$Arch.exe"
    $tmpFile = Join-Path $env:TEMP "netpanel-install.exe"

    Write-Info "下载 NetPanel $Ver (windows/$Arch)..."
    Write-Info "URL: $url"

    try {
        $wc = New-Object System.Net.WebClient
        $wc.DownloadFile($url, $tmpFile)
    } catch {
        # 尝试 zip 格式
        $zipUrl  = "https://github.com/$Repo/releases/download/$Ver/netpanel-windows-$Arch.zip"
        $zipFile = Join-Path $env:TEMP "netpanel-install.zip"
        Write-Warn "直接下载失败，尝试 zip 格式: $zipUrl"
        try {
            $wc.DownloadFile($zipUrl, $zipFile)
            Expand-Archive -Path $zipFile -DestinationPath $env:TEMP -Force
            $tmpFile = Join-Path $env:TEMP "netpanel.exe"
            Remove-Item $zipFile -Force -ErrorAction SilentlyContinue
        } catch {
            Write-Fail "下载失败: $_"
        }
    }

    return $tmpFile
}

# ─── 安装二进制 ───────────────────────────────────────────────────────────────
function Install-Binary {
    param([string]$TmpFile)

    Write-Info "安装到 $InstallDir ..."
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    New-Item -ItemType Directory -Path $DataDir    -Force | Out-Null
    New-Item -ItemType Directory -Path $LogDir     -Force | Out-Null

    $dest = Join-Path $InstallDir $BinaryName

    # 备份旧版本
    if (Test-Path $dest) {
        Copy-Item $dest "$dest.bak" -Force
        Write-Warn "已备份旧版本到 $dest.bak"
    }

    Copy-Item $TmpFile $dest -Force
    Remove-Item $TmpFile -Force -ErrorAction SilentlyContinue

    # 添加到 PATH（当前会话 + 系统永久）
    $sysPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($sysPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$sysPath;$InstallDir", "Machine")
        $env:Path += ";$InstallDir"
        Write-Info "已将 $InstallDir 添加到系统 PATH"
    }

    Write-Success "二进制文件安装完成: $dest"
}

# ─── 写入配置 ─────────────────────────────────────────────────────────────────
function Write-Config {
    $confFile = Join-Path $DataDir "config.yaml"
    if (Test-Path $confFile) {
        Write-Warn "配置文件已存在，跳过: $confFile"
        return
    }

    Write-Info "写入默认配置..."
    $dbPath  = (Join-Path $DataDir "netpanel.db").Replace("\", "/")
    $logPath = (Join-Path $LogDir  "netpanel.log").Replace("\", "/")

    @"
# NetPanel 配置文件
server:
  port: $Port
  host: "0.0.0.0"

database:
  path: "$dbPath"

log:
  level: "info"
  path: "$logPath"
"@ | Set-Content $confFile -Encoding UTF8

    Write-Success "配置文件已写入: $confFile"
}

# ─── 注册 Windows 服务 ────────────────────────────────────────────────────────
function Register-NetPanelService {
    $exePath = Join-Path $InstallDir $BinaryName

    # 停止并删除旧服务
    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Warn "检测到已有服务，正在重新安装..."
        if ($existing.Status -eq "Running") {
            Stop-Service -Name $ServiceName -Force
            Start-Sleep -Seconds 2
        }
        & sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 1
    }

    Write-Info "注册 Windows 服务: $ServiceName ..."
    $binPath = "`"$exePath`" --service --port $Port --data `"$DataDir`""

    & sc.exe create $ServiceName `
        binPath= $binPath `
        DisplayName= "NetPanel Network Manager" `
        start= auto `
        obj= LocalSystem | Out-Null

    & sc.exe description $ServiceName "NetPanel 网络管理面板服务" | Out-Null

    # 配置失败自动重启
    & sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    Start-Service -Name $ServiceName
    Write-Success "服务已注册并启动: $ServiceName"
    Write-Info "查看状态: Get-Service $ServiceName"
    Write-Info "查看日志: Get-EventLog -LogName Application -Source $ServiceName -Newest 20"
}

# ─── 防火墙规则 ───────────────────────────────────────────────────────────────
function Add-FirewallRule {
    $ruleName = "NetPanel-$Port"
    $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    if (-not $existing) {
        Write-Info "添加防火墙入站规则（端口 $Port）..."
        New-NetFirewallRule -DisplayName $ruleName `
            -Direction Inbound -Protocol TCP -LocalPort $Port `
            -Action Allow -Profile Any | Out-Null
        Write-Success "防火墙规则已添加: $ruleName"
    }
}

# ─── 主流程 ───────────────────────────────────────────────────────────────────
function Main {
    Write-Host ""
    Write-Host "╔══════════════════════════════════════╗" -ForegroundColor Blue
    Write-Host "║      NetPanel 一键安装脚本 (Win)     ║" -ForegroundColor Blue
    Write-Host "╚══════════════════════════════════════╝" -ForegroundColor Blue
    Write-Host ""

    Assert-Admin

    $arch = Get-Arch
    Write-Info "检测到架构: $arch"

    if ($Version -eq "latest") {
        $Version = Get-LatestVersion
        Write-Info "最新版本: $Version"
    }

    $tmpFile = Download-Binary -Ver $Version -Arch $arch
    Install-Binary -TmpFile $tmpFile
    Write-Config

    if (-not $NoService) {
        Register-NetPanelService
        Add-FirewallRule
    }

    $ip = (Get-NetIPAddress -AddressFamily IPv4 |
           Where-Object { $_.InterfaceAlias -notlike "*Loopback*" } |
           Select-Object -First 1).IPAddress

    Write-Host ""
    Write-Host "╔══════════════════════════════════════════════════╗" -ForegroundColor Green
    Write-Host "║  ✅ NetPanel 安装完成！                          ║" -ForegroundColor Green
    Write-Host "║                                                  ║" -ForegroundColor Green
    Write-Host "║  访问地址: http://${ip}:${Port}                  ║" -ForegroundColor Green
    Write-Host "║  安装目录: $InstallDir" -ForegroundColor Green
    Write-Host "║  数据目录: $DataDir" -ForegroundColor Green
    Write-Host "║                                                  ║" -ForegroundColor Green
    Write-Host "║  服务管理:                                       ║" -ForegroundColor Green
    Write-Host "║    启动: Start-Service NetPanel                  ║" -ForegroundColor Green
    Write-Host "║    停止: Stop-Service NetPanel                   ║" -ForegroundColor Green
    Write-Host "║    卸载: sc.exe delete NetPanel                  ║" -ForegroundColor Green
    Write-Host "╚══════════════════════════════════════════════════╝" -ForegroundColor Green
    Write-Host ""
}

Main
