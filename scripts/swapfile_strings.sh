#!/bin/bash

SWAP_FILE="$HOME/swapfile.bin"

echo "=== STRINGS EN SWAPFILE ==="

if [ ! -f "$SWAP_FILE" ]; then
    echo "ERROR: No existe $SWAP_FILE"
    exit 1
fi

# Mostrar información básica del archivo
echo "Archivo: $SWAP_FILE"
echo "Tamaño: $(du -h "$SWAP_FILE" | cut -f1)"
echo ""

# Mostrar strings encontradas
echo "--- Strings encontradas ---"
strings "$SWAP_FILE" -n 1

echo ""
echo "=== FIN ==="