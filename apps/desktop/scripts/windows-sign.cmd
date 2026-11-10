@echo off
setlocal DisableDelayedExpansion
set "signTool=%GLH_DESKTOP_WINDOWS_SIGNTOOL%"
set "certificateFile=%GLH_DESKTOP_WINDOWS_CER_FILE%"
set "tokenPin=%GLH_DESKTOP_WINDOWS_TOKEN_PIN%"
set "keyContainer=%GLH_DESKTOP_WINDOWS_KEY_CONTAINER%"
set "targetFile=%GLH_DESKTOP_WINDOWS_SIGN_TARGET%"
set "appendSignature="
if "%GLH_DESKTOP_WINDOWS_SIGN_APPEND%"=="1" set "appendSignature=/as"
set "GLH_DESKTOP_WINDOWS_SIGNTOOL="
set "GLH_DESKTOP_WINDOWS_CER_FILE="
set "GLH_DESKTOP_WINDOWS_TOKEN_PIN="
set "GLH_DESKTOP_WINDOWS_KEY_CONTAINER="
set "GLH_DESKTOP_WINDOWS_SIGN_TARGET="
set "GLH_DESKTOP_WINDOWS_SIGN_APPEND="
set "signTool=" & set "certificateFile=" & set "tokenPin=" & set "keyContainer=" & set "targetFile=" & set "appendSignature=" & "%signTool%" sign /v /fd sha256 /f "%certificateFile%" /kc "[{{%tokenPin%}}]=%keyContainer%" /csp "eToken Base Cryptographic Provider" %appendSignature% "%targetFile%"
exit /b %errorlevel%
