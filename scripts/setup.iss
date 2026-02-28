; NetPanel Inno Setup 安装脚本
; 用法: iscc scripts/setup.iss
; 需要先构建: make build-frontend build-windows-amd64
; 输出: dist/NetPanel-Setup-x.x.x-windows-amd64.exe

#define AppName      "NetPanel"
#define AppVersion   GetEnv("VERSION")
#if AppVersion == ""
  #define AppVersion "0.1.0"
#endif
#define AppPublisher "NetPanel Team"
#define AppURL       "https://github.com/your-org/netpanel"
#define AppExeName   "netpanel.exe"
#define ServiceName  "netpanel"
#define AppDataDir   "{commonappdata}\NetPanel"

[Setup]
; 基本信息
AppId={{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} v{#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
AppUpdatesURL={#AppURL}/releases

; 安装目录
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes

; 输出
OutputDir=..\dist
OutputBaseFilename=NetPanel-Setup-{#AppVersion}-windows-amd64
SetupIconFile=..\webpage\public\favicon.ico

; 压缩
Compression=lzma2/ultra64
SolidCompression=yes
LZMAUseSeparateProcess=yes

; 权限：必须以管理员身份运行（注册服务需要）
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=

; 架构
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

; 界面
WizardStyle=modern
WizardSizePercent=120
DisableWelcomePage=no
LicenseFile=..\LICENSE

; 卸载
UninstallDisplayIcon={app}\{#AppExeName}
UninstallDisplayName={#AppName} v{#AppVersion}
CreateUninstallRegKey=yes

; 版本信息
VersionInfoVersion={#AppVersion}
VersionInfoCompany={#AppPublisher}
VersionInfoDescription={#AppName} Network Manager
VersionInfoProductName={#AppName}
VersionInfoProductVersion={#AppVersion}

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[CustomMessages]
chinesesimplified.InstallService=安装为 Windows 系统服务（推荐，开机自启）
chinesesimplified.StartAfterInstall=安装完成后立即启动服务
chinesesimplified.OpenWebUI=安装完成后打开管理界面
english.InstallService=Install as Windows Service (recommended, auto-start)
english.StartAfterInstall=Start service immediately after installation
english.OpenWebUI=Open management UI after installation

[Tasks]
Name: "installservice"; Description: "{cm:InstallService}"; GroupDescription: "服务选项:"; Flags: checked
Name: "startservice"; Description: "{cm:StartAfterInstall}"; GroupDescription: "服务选项:"; Flags: checked; OnlyBelowVersion: 0
Name: "openwebui"; Description: "{cm:OpenWebUI}"; GroupDescription: "其他选项:"; Flags: checked

[Dirs]
Name: "{#AppDataDir}"; Permissions: everyone-full
Name: "{#AppDataDir}\data"
Name: "{#AppDataDir}\logs"
Name: "{#AppDataDir}\bin"

[Files]
; 主程序
Source: "..\dist\netpanel-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#AppExeName}"; Flags: ignoreversion

; 配置文件（首次安装时复制，升级时不覆盖）
Source: "..\scripts\config.example.yaml"; DestDir: "{#AppDataDir}"; DestName: "config.yaml"; Flags: onlyifdoesntexist uninsneveruninstall; Check: FileExists(ExpandConstant('..\scripts\config.example.yaml'))

; 脚本工具
Source: "..\scripts\service-windows.ps1"; DestDir: "{app}"; Flags: ignoreversion

; 可选：EasyTier 二进制（如果存在）
Source: "..\dist\bin\easytier-core.exe"; DestDir: "{#AppDataDir}\bin"; Flags: ignoreversion skipifsourcedoesntexist
Source: "..\dist\bin\easytier-cli.exe"; DestDir: "{#AppDataDir}\bin"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
; 开始菜单
Name: "{group}\{#AppName} 管理界面"; Filename: "{app}\{#AppExeName}"; Parameters: "--open-browser"; WorkingDir: "{app}"; Comment: "打开 NetPanel 管理界面"
Name: "{group}\{#AppName} 服务管理"; Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\service-windows.ps1"" status"; WorkingDir: "{app}"; Comment: "查看服务状态"
Name: "{group}\卸载 {#AppName}"; Filename: "{uninstallexe}"

; 桌面快捷方式（可选）
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Parameters: "--open-browser"; WorkingDir: "{app}"; Tasks: not installservice

[Registry]
; 写入安装路径，供其他工具查询
Root: HKLM; Subkey: "SOFTWARE\{#AppName}"; ValueType: string; ValueName: "InstallPath"; ValueData: "{app}"; Flags: uninsdeletekey
Root: HKLM; Subkey: "SOFTWARE\{#AppName}"; ValueType: string; ValueName: "DataPath"; ValueData: "{#AppDataDir}"; Flags: uninsdeletekey
Root: HKLM; Subkey: "SOFTWARE\{#AppName}"; ValueType: string; ValueName: "Version"; ValueData: "{#AppVersion}"; Flags: uninsdeletekey

; 防火墙规则（允许入站）
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\FirewallRules"; ValueType: string; ValueName: "NetPanel-In-TCP"; ValueData: "v2.30|Action=Allow|Active=TRUE|Dir=In|Protocol=6|LPort=8080|Name=NetPanel|Desc=NetPanel Network Manager|App={app}\{#AppExeName}|"; Flags: uninsdeletevalue

[Run]
; 注册并启动 Windows 服务
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""& '{{app}}\service-windows.ps1' install"""; WorkingDir: "{app}"; StatusMsg: "正在注册系统服务..."; Flags: runhidden waituntilterminated; Tasks: installservice

Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Start-Service -Name '{#ServiceName}'"""; WorkingDir: "{app}"; StatusMsg: "正在启动服务..."; Flags: runhidden waituntilterminated; Tasks: installservice and startservice

; 非服务模式：直接启动
Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"; StatusMsg: "正在启动 NetPanel..."; Flags: nowait postinstall skipifsilent; Tasks: not installservice

; 打开管理界面
Filename: "cmd.exe"; Parameters: "/c start http://localhost:8080"; StatusMsg: "正在打开管理界面..."; Flags: nowait postinstall skipifsilent; Tasks: openwebui

[UninstallRun]
; 卸载前停止并删除服务
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Stop-Service -Name '{#ServiceName}' -Force -ErrorAction SilentlyContinue; & '{{app}}\service-windows.ps1' uninstall"""; WorkingDir: "{app}"; Flags: runhidden waituntilterminated; RunOnceId: "StopService"

[UninstallDelete]
; 卸载时清理日志（保留用户数据）
Type: filesandordirs; Name: "{#AppDataDir}\logs"

[Code]
// ─── 安装前检查 ───────────────────────────────────────────────────────────────

function InitializeSetup(): Boolean;
var
  OldVersion: String;
  Uninstaller: String;
  ResultCode: Integer;
begin
  Result := True;

  // 检查是否已安装旧版本
  if RegQueryStringValue(HKLM, 'SOFTWARE\{#AppName}', 'Version', OldVersion) then
  begin
    if MsgBox('检测到已安装 ' + '{#AppName}' + ' v' + OldVersion + '。' + #13#10 +
              '是否先卸载旧版本再继续安装？', mbConfirmation, MB_YESNO) = IDYES then
    begin
      // 先停止服务
      Exec('powershell.exe',
        '-ExecutionPolicy Bypass -Command "Stop-Service -Name ''{#ServiceName}'' -Force -ErrorAction SilentlyContinue"',
        '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

      // 运行卸载程序
      if RegQueryStringValue(HKLM,
        'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}_is1',
        'UninstallString', Uninstaller) then
      begin
        Exec(RemoveQuotes(Uninstaller), '/SILENT', '', SW_SHOW, ewWaitUntilTerminated, ResultCode);
      end;
    end;
  end;
end;

// ─── 安装完成页面 ─────────────────────────────────────────────────────────────

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then
  begin
    // 将 {#AppDataDir}\bin 加入系统 PATH（可选）
    // 此处仅记录，不强制修改 PATH
  end;
end;

// ─── 卸载前确认 ───────────────────────────────────────────────────────────────

function InitializeUninstall(): Boolean;
begin
  Result := MsgBox('确定要卸载 {#AppName} 吗？' + #13#10 +
                   '注意：用户数据目录 %ProgramData%\NetPanel 将被保留。',
                   mbConfirmation, MB_YESNO) = IDYES;
end;
