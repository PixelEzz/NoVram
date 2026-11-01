@echo off
title novram CPU Chat
cd /d "%~dp0"
echo ==========================================================
echo  Starting novram interactive CPU chat...
echo ==========================================================
novram.exe chat --mock
pause
