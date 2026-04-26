#!/bin/bash

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
CFG_DIR="$BASE_DIR"

# Buscar un archivo que tenga log_level y leer su valor
PRIMER_ARCHIVO=$(find "$CFG_DIR" -name '*.json' | while read -r file; do
    if grep -q '"log_level":' "$file"; then
        echo "$file"
        break
    fi
done)

if [ -z "$PRIMER_ARCHIVO" ]; then
    echo "No se encontró ninguna clave log_level en archivos .json"
    exit 1
fi

# Obtener el valor actual de log_level
ACTUAL=$(grep '"log_level":' "$PRIMER_ARCHIVO" | sed -n 's/.*"log_level": *"\([^"]*\)".*/\1/p')

if [[ "$ACTUAL" == "INFO" ]]; then
    NUEVO="DEBUG"
elif [[ "$ACTUAL" == "DEBUG" ]]; then
    NUEVO="INFO"
else
    echo "Valor de log_level desconocido: $ACTUAL"
    exit 2
fi

echo "log_level actual: $ACTUAL > cambiando a: $NUEVO"

# Reemplazar en todos los archivos
find "$CFG_DIR" -name '*.json' -exec sed -i "s/\"log_level\": *\"$ACTUAL\"/\"log_level\": \"$NUEVO\"/g" {} +

echo "Todos los archivos actualizados a log_level = $NUEVO"
