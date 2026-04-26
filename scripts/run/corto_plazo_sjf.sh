#!/bin/bash
./cfgs/set.sh p_corto_plazo 2 1 1 && ./scripts/exec.sh PLANI_CORTO_PLAZO 0

RESULTS_PATH="resultados/corto_plazo_sjf/"

# Crear carpeta resultados si no existe
mkdir -p resultados
cp kernel/kernel.log "$RESULTS_PATH"
cp memoria/memoria.log "$RESULTS_PATH"
cp io/io_1.log "$RESULTS_PATH"
cp cpu/cpu_1.log "$RESULTS_PATH"

./scripts/grep.sh kernel/kernel.log  "Métrica|Tiempo" > "${RESULTS_PATH}resultados.log"
echo "Resultados de la prueba:"
echo "--------------------------------------------------------------------------------"
cat "${RESULTS_PATH}resultados.log"
echo "--------------------------------------------------------------------------------"