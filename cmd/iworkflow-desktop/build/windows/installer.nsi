; UNVERIFIED: written for `make wails-package` on Windows. makensis was not available when it was written.
;
; Per-user NSIS installer. scripts/wails_package.sh runs:
;   makensis -DVERSION=<x.y.z> -DARCH=<amd64|arm64> -DSTAGE=<folder with iworkflow-desktop.exe and server\>
;            -DBUILD=<this build folder> -DOUTDIR=<output folder> build/windows/installer.nsi
;
; The data-folder page mirrors electron/build/installer.nsh: it asks where iworkflow keeps its data and stores the
; answer as the per-user environment variable IWF_HOME. The default ($PROFILE\.iworkflow) is what the server uses
; without the variable, so choosing it removes the variable instead of pinning it. Uninstalling keeps the data folder.

!ifndef VERSION
  !error "VERSION is required (makensis -DVERSION=x.y.z)"
!endif
!ifndef ARCH
  !error "ARCH is required (makensis -DARCH=amd64)"
!endif
!ifndef STAGE
  !error "STAGE is required: the folder with iworkflow-desktop.exe and server\"
!endif
!ifndef BUILD
  !error "BUILD is required: the build folder with windows\icon.ico"
!endif
!ifndef OUTDIR
  !error "OUTDIR is required: the folder that receives the setup"
!endif

Unicode true
Name "iworkflow"
OutFile "${OUTDIR}\iworkflow-${VERSION}-windows-${ARCH}-setup.exe"
; Per-user install: no administrator rights, the same default as the Electron installer.
RequestExecutionLevel user
InstallDir "$LOCALAPPDATA\Programs\iworkflow"
InstallDirRegKey HKCU "Software\iworkflow" "InstallDir"
SetCompressor /SOLID lzma

!include MUI2.nsh
!include nsDialogs.nsh
!include LogicLib.nsh

!define MUI_ICON "${BUILD}\windows\icon.ico"
!define MUI_UNICON "${BUILD}\windows\icon.ico"

Var IwfHomeInput
Var IwfHomeValue

Function IwfHomePageCreate
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateLabel} 0 0 100% 24u "Folder where iworkflow stores its data (database, worktrees, logs, models). Leave the default unless you need it elsewhere."
  Pop $0
  ReadRegStr $IwfHomeValue HKCU "Environment" "IWF_HOME"
  ${If} $IwfHomeValue == ""
    StrCpy $IwfHomeValue "$PROFILE\.iworkflow"
  ${EndIf}
  ${NSD_CreateDirRequest} 0 30u 78% 12u "$IwfHomeValue"
  Pop $IwfHomeInput
  ${NSD_CreateBrowseButton} 80% 30u 20% 12u "Browse..."
  Pop $0
  ${NSD_OnClick} $0 IwfHomeBrowse
  nsDialogs::Show
FunctionEnd

Function IwfHomeBrowse
  ${NSD_GetText} $IwfHomeInput $0
  nsDialogs::SelectFolderDialog "Data folder" "$0"
  Pop $0
  ${If} $0 != error
    ${NSD_SetText} $IwfHomeInput "$0"
  ${EndIf}
FunctionEnd

Function IwfHomePageLeave
  ${NSD_GetText} $IwfHomeInput $IwfHomeValue
  ${If} $IwfHomeValue == ""
    MessageBox MB_OK|MB_ICONEXCLAMATION "Choose a folder for the data."
    Abort
  ${EndIf}
FunctionEnd

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
Page custom IwfHomePageCreate IwfHomePageLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "iworkflow"
  SetOutPath "$INSTDIR"
  File "${STAGE}\iworkflow-desktop.exe"
  SetOutPath "$INSTDIR\server"
  File /r "${STAGE}\server\*.*"
  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "Software\iworkflow" "InstallDir" "$INSTDIR"
  CreateShortCut "$SMPROGRAMS\iworkflow.lnk" "$INSTDIR\iworkflow-desktop.exe"
  CreateShortCut "$DESKTOP\iworkflow.lnk" "$INSTDIR\iworkflow-desktop.exe"

  ${If} $IwfHomeValue != ""
    ${If} $IwfHomeValue == "$PROFILE\.iworkflow"
      DeleteRegValue HKCU "Environment" "IWF_HOME"
    ${Else}
      WriteRegStr HKCU "Environment" "IWF_HOME" "$IwfHomeValue"
    ${EndIf}
    ; Tell running programs (Explorer, new terminals) that the environment changed.
    System::Call 'user32::SendMessageTimeout(i 0xffff, i 0x1A, i 0, t "Environment", i 2, i 5000, *i .r0)'
  ${EndIf}
SectionEnd

Section "un.iworkflow"
  Delete "$SMPROGRAMS\iworkflow.lnk"
  Delete "$DESKTOP\iworkflow.lnk"
  RMDir /r "$INSTDIR"
  DeleteRegKey HKCU "Software\iworkflow"
SectionEnd
