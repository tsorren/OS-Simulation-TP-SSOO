#!/bin/bash

# Verificar que se proporcionaron al menos 2 argumentos
if [ $# -lt 2 ]; then
    echo "Uso: $0 <archivo> <expresion_regular> [pid]"
    exit 1
fi

ARCHIVO="$1"
EXPRESION="$2"

# Si hay un tercer parámetro (PID), filtrar por él también
if [ $# -eq 3 ]; then
    PID="$3"
    
    # Buscar el PID con cualquier código de color ANSI o sin color
    # Patrón genérico que matchea: ## (ESC[cualquier_colorZm PID ESC[0m) o ## (PID)
    cat "$ARCHIVO" | grep -a --color=never -E "$EXPRESION" | grep -a --color=never -E $'\033\\[[0-9;]*m'${PID}$'\033\\[0m|## \\('${PID}'\\)'
else
    # Si no hay PID, simplemente buscar la expresión
    cat "$ARCHIVO" | grep -a --color=never -E "$EXPRESION"
fi