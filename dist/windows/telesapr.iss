; The Windows installer of a release (scripts/dist-windows.sh), built
; by Inno Setup 6 with these defined on its command line:
;   VERSION  pkg/version.Version
;   ARCH     amd64 or arm64
;   SRC      the folder with telesapr.exe and telesapr-cli.exe
;   ROOT     the repository (LICENSE, NOTICE, README.md)
;   OUT      where the installer goes
; It installs for the user or, when run as administrator, for the
; machine; registers .tlspr with telesapr.exe; and can add the folder
; to PATH for telesapr-cli.

#ifndef VERSION
  #error VERSION is not defined
#endif
#ifndef ARCH
  #error ARCH is not defined
#endif
#ifndef SRC
  #error SRC is not defined
#endif
#ifndef ROOT
  #error ROOT is not defined
#endif
#ifndef OUT
  #error OUT is not defined
#endif

[Setup]
AppId={{7C1F2B5A-9E7D-4B0B-8E6B-5D3C2A1F0E42}
AppName=TeleSAPR
AppVersion={#VERSION}
AppVerName=TeleSAPR {#VERSION}
AppPublisher=The TeleSAPR authors
AppPublisherURL=https://github.com/telesapr/telesapr
AppSupportURL=https://github.com/telesapr/telesapr/issues
AppUpdatesURL=https://github.com/telesapr/telesapr/releases
VersionInfoVersion={#VERSION}
DefaultDirName={autopf}\TeleSAPR
DefaultGroupName=TeleSAPR
DisableProgramGroupPage=yes
LicenseFile={#ROOT}\LICENSE
OutputDir={#OUT}
OutputBaseFilename=telesapr-{#VERSION}-windows-{#ARCH}-setup
SetupIconFile={#ROOT}\dist\windows\telesapr.ico
UninstallDisplayIcon={app}\telesapr.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ChangesAssociations=yes
ChangesEnvironment=yes
#if ARCH == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "addtopath"; Description: "Add the install folder to PATH, for telesapr-cli"; GroupDescription: "Command line:"; Flags: unchecked

[Files]
Source: "{#SRC}\telesapr.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SRC}\telesapr-cli.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#ROOT}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#ROOT}\NOTICE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#ROOT}\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\TeleSAPR"; Filename: "{app}\telesapr.exe"
Name: "{autodesktop}\TeleSAPR"; Filename: "{app}\telesapr.exe"; Tasks: desktopicon

[Registry]
; .tlspr opens in TeleSAPR. HKA is HKLM for a machine install and HKCU
; for a user one.
Root: HKA; Subkey: "Software\Classes\.tlspr"; ValueType: string; ValueName: ""; ValueData: "TeleSAPR.Model"; Flags: uninsdeletevalue
Root: HKA; Subkey: "Software\Classes\.tlspr"; ValueType: string; ValueName: "Content Type"; ValueData: "application/x-telesapr"; Flags: uninsdeletevalue
Root: HKA; Subkey: "Software\Classes\TeleSAPR.Model"; ValueType: string; ValueName: ""; ValueData: "TeleSAPR model"; Flags: uninsdeletekey
Root: HKA; Subkey: "Software\Classes\TeleSAPR.Model\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\telesapr.exe,0"
Root: HKA; Subkey: "Software\Classes\TeleSAPR.Model\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\telesapr.exe"" ""%1"""
; PATH, when asked for; removed again by the uninstaller (see [Code]).
Root: HKA; Subkey: "{code:EnvironmentKey}"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NotOnPath(ExpandConstant('{app}'))

[Run]
Filename: "{app}\telesapr.exe"; Description: "{cm:LaunchProgram,TeleSAPR}"; Flags: nowait postinstall skipifsilent

[Code]
function EnvironmentKey(Param: string): string;
begin
  if IsAdminInstallMode then
    Result := 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment'
  else
    Result := 'Environment';
end;

function NotOnPath(Dir: string): Boolean;
var
  Path: string;
begin
  if not RegQueryStringValue(HKA, EnvironmentKey(''), 'Path', Path) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Path) + ';') = 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path, Dir: string;
  P: Integer;
begin
  if CurUninstallStep <> usPostUninstall then
    exit;
  if not RegQueryStringValue(HKA, EnvironmentKey(''), 'Path', Path) then
    exit;
  Dir := ExpandConstant('{app}');
  P := Pos(';' + Uppercase(Dir), Uppercase(Path));
  if P > 0 then
  begin
    Delete(Path, P, Length(Dir) + 1);
    RegWriteExpandStringValue(HKA, EnvironmentKey(''), 'Path', Path);
  end;
end;
