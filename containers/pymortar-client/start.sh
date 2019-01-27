#!/bin/bash
export NB_USER=jovyan
export CHOWN_HOME=yes
export GRANT_SUDO=yes
export JUPYTER_ENABLE_LAB=yes
start-notebook.sh --NotebookApp.ip=0.0.0.0
