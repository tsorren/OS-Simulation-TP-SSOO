#!/bin/bash
TERMINAL_CMD="x-terminal-emulator -T"
PROYECTO_DIR="$(pwd)"

$TERMINAL_CMD "IO 2" -e bash -c "cd $PROYECTO_DIR && \
echo Presioná [Enter] para iniciar la IO 2 && \
read -r && \
./cfgs/set.sh p_corto_plazo 1 1 2 && \
go run ./io DISCO"

./cfgs/set.sh p_corto_plazo 1 1 1 && ./scripts/m_exec.sh PLANI_CORTO_PLAZO 0 2 1 && \
echo "Ver resultados" && \
read -r && \
./scripts/grep.sh kernel/kernel.log  "Métrica|Tiempo"

