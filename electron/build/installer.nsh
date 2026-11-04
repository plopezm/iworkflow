; Custom pages and steps for the NSIS installer (included by electron-builder).
;
; Asks where iworkflow keeps its data (database, worktrees, logs, models) and stores the answer
; as the per-user environment variable IWF_HOME. The default, $PROFILE\.iworkflow, is what the
; server uses without the variable, so choosing it removes the variable instead of pinning it.
; Silent installs (/S) skip the page and leave an existing IWF_HOME untouched.

!ifndef BUILD_UNINSTALLER
  !include nsDialogs.nsh
  !include LogicLib.nsh

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
!endif

!macro customPageAfterChangeDir
  Page custom IwfHomePageCreate IwfHomePageLeave
!macroend

!macro customInstall
  ${If} $IwfHomeValue != ""
    ${If} $IwfHomeValue == "$PROFILE\.iworkflow"
      DeleteRegValue HKCU "Environment" "IWF_HOME"
    ${Else}
      WriteRegStr HKCU "Environment" "IWF_HOME" "$IwfHomeValue"
    ${EndIf}
    ; Tell running programs (Explorer, new terminals) that the environment changed.
    System::Call 'user32::SendMessageTimeout(i 0xffff, i 0x1A, i 0, t "Environment", i 2, i 5000, *i .r0)'
  ${EndIf}
!macroend
