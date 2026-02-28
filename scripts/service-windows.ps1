# NetPanel Windows Service 管理脚本
# 需要以管理员身份运行
# 用法:
#   .\service-windows.ps1 install   - 安装服务
#   .\service-windows.ps1 uninstall - 卸载服务
#   .\service-windows.ps1 start     - 启动服务
#   .\service-windows.ps1 stop      - 停止服务
#   .\service-windows.ps1 restart   - 重启服务
#   .\service-windows.ps1 status    - 查看服务状态

param(
    [Parameter(Mandatory=$true, Position=0)]
    [ValidateSet("install","uninstall","start","stop","restart","status")]
    [string]$Action
)

$ServiceName    = "NetPanel"
$DisplayName    = "NetPanel - Network Management Panel"
$Description    = "NetPanel 网络管理面板，提供端口映射、组网、DDNS 等功能。"
$InstallDir     = "C:\Program Files\NetPanel"
$BinaryName     = "netpanel.exe"
$BinaryPath     = Join-Path $InstallDir $BinaryName
$DataDir        = Join-Path $InstallDir "data"
$Port           = 8080

# ── 检查管理员权限 ──────────────────────────────────────────
function Assert-Admin {
    $current = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not $current.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-Error "请以管理员身份运行此脚本！"
        exit 1
    }
}

# ── 安装服务 ────────────────────────────────────────────────
function Install-Service {
    Assert-Admin

    if (-not (Test-Path $BinaryPath)) {
        Write-Error "未找到可执行文件: $BinaryPath"
        Write-Host "请先将 netpanel.exe 复制到 $InstallDir"
        exit 1
    }

    if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
        Write-Warning "服务 '$ServiceName' 已存在，请先卸载后重新安装。"
        exit 1
    }

    # 创建数据目录
    if (-not (Test-Path $DataDir)) {
        New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
    }

    $binPathWithArgs = "`"$BinaryPath`" --port $Port --data `"$DataDir`""

    New-Service `
        -Name        $ServiceName `
        -BinaryPathName $binPathWithArgs `
        -DisplayName $DisplayName `
        -Description $Description `
        -StartupType Automatic `
        | Out-Null

    # 设置服务失败恢复策略：前两次失败重启，第三次重启系统（可按需调整）
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    Write-Host "✅ 服务 '$ServiceName' 安装成功。"
    Write-Host "   安装目录: $InstallDir"
    Write-Host "   数据目录: $DataDir"
    Write-Host "   监听端口: $Port"
    Write-Host ""
    Write-Host "使用以下命令启动服务:"
    Write-Host "   .\service-windows.ps1 start"
}

# ── 卸载服务 ────────────────────────────────────────────────
function Uninstall-Service {
    Assert-Admin

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Warning "服务 '$ServiceName' 不存在。"
        exit 0
    }

    if ($svc.Status -eq "Running") {
        Write-Host "正在停止服务..."
        Stop-Service -Name $ServiceName -Force
        Start-Sleep -Seconds 2
    }

    Remove-Service -Name $ServiceName -ErrorAction SilentlyContinue
    # 兼容旧版 Windows（Remove-Service 在 PS5 不可用）
    if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
        sc.exe delete $ServiceName | Out-Null
    }

    Write-Host "✅ 服务 '$ServiceName' 已卸载。"
    Write-Host "   注意：安装目录 '$InstallDir' 未被删除，如需清理请手动删除。"
}

# ── 启动服务 ────────────────────────────────────────────────
function Start-NetPanelService {
    Assert-Admin
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) { Write-Error "服务未安装，请先执行 install。"; exit 1 }
    if ($svc.Status -eq "Running") { Write-Host "服务已在运行中。"; exit 0 }
    Start-Service -Name $ServiceName
    Start-Sleep -Seconds 2
    $svc.Refresh()
    if ($svc.Status -eq "Running") {
        Write-Host "✅ 服务已启动，访问 http://localhost:$Port"
    } else {
        Write-Error "服务启动失败，请检查事件日志。"
        exit 1
    }
}

# ── 停止服务 ────────────────────────────────────────────────
function Stop-NetPanelService {
    Assert-Admin
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) { Write-Error "服务未安装。"; exit 1 }
    if ($svc.Status -eq "Stopped") { Write-Host "服务已停止。"; exit 0 }
    Stop-Service -Name $ServiceName -Force
    Write-Host "✅ 服务已停止。"
}

# ── 重启服务 ────────────────────────────────────────────────
function Restart-NetPanelService {
    Assert-Admin
    Stop-NetPanelService
    Start-Sleep -Seconds 2
    Start-NetPanelService
}

# ── 查看状态 ────────────────────────────────────────────────
function Get-ServiceStatus {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host "服务状态: 未安装"
        exit 0
    }
    Write-Host "服务名称: $($svc.Name)"
    Write-Host "显示名称: $($svc.DisplayName)"
    Write-Host "运行状态: $($svc.Status)"
    Write-Host "启动类型: $($svc.StartType)"
    Write-Host "访问地址: http://localhost:$Port"
}

# ── 入口 ────────────────────────────────────────────────────
switch ($Action) {
    "install"   { Install-Service }
    "uninstall" { Uninstall-Service }
    "start"     { Start-NetPanelService }
    "stop"      { Stop-NetPanelService }
    "restart"   { Restart-NetPanelService }
    "status"    { Get-ServiceStatus }
}
