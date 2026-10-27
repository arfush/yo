; Per-user yoanime installer. A per-user location (instead of Program Files)
; keeps the in-app updater able to replace yoanime.exe without an elevation
; prompt, while still giving the app Start menu, uninstall and Apps entries.

#define AppName "yoanime"
#define AppExe "yoanime.exe"

#ifndef VersionTag
#define VersionTag "v0.0"
#endif
#ifndef VersionNumber
#define VersionNumber "0.0"
#endif
#ifndef Arch
#define Arch "amd64"
#endif

[Setup]
AppId={{B7B0F5A1-9C4E-4D63-9C2E-6F3D8C2A1B70}
AppName={#AppName}
AppVersion={#VersionNumber}
AppVerName={#AppName} {#VersionTag}
AppPublisher=arfush
AppPublisherURL=https://github.com/arfush/yo
AppSupportURL=https://github.com/arfush/yo/issues
AppUpdatesURL=https://github.com/arfush/yo/releases
DefaultDirName={localappdata}\Programs\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
AllowNoIcons=yes
PrivilegesRequired=lowest
OutputDir=dist
OutputBaseFilename=yoanime_{#VersionTag}_{#Arch}_setup
SetupIconFile=ui\assets\images\app-icon.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
AppMutex=Local\yoanime.SingleInstance
CloseApplications=no
SourceDir=..

#if Arch == "amd64"
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#elif Arch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#elif Arch == "386"
ArchitecturesAllowed=x86compatible
#endif

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "dist\yoanime_{#VersionTag}_{#Arch}.exe"; DestDir: "{app}"; DestName: "{#AppExe}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent
