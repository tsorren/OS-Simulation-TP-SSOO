#!/bin/bash

# =============================================================================
# Script: dump_strings.sh
# Descripción: Muestra las strings de archivos dump
# Uso: ./scripts/dump_strings.sh
# =============================================================================

# Colores para output
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Variables de configuración
DUMP_DIR="$HOME/dump_files"

# Función para mostrar separador
show_separator() {
    echo -e "${BLUE}===============================================================================${NC}"
}

# Función para mostrar archivos dump
show_dump_files() {
    echo -e "${GREEN}STRINGS EN ARCHIVOS DE DUMP${NC}"
    show_separator
    
    if [ ! -d "$DUMP_DIR" ]; then
        echo -e "${RED}ERROR: Directorio $DUMP_DIR no existe${NC}"
        return 1
    fi
    
    # Obtener archivos .dmp ordenados por fecha de creación
    local dump_files=$(find "$DUMP_DIR" -name "*.dmp" -type f -printf '%T+ %p\n' 2>/dev/null | sort | cut -d' ' -f2-)
    
    if [ -z "$dump_files" ]; then
        echo -e "${YELLOW}No se encontraron archivos .dmp en $DUMP_DIR${NC}"
        return 0
    fi
    
    local count=1
    while IFS= read -r dump_file; do
        if [ -f "$dump_file" ]; then
            # Extraer PID del nombre del archivo (PID son los primeros números hasta el primer guion)
            local filename=$(basename "$dump_file")
            local pid=$(echo "$filename" | sed 's/^\([0-9]\+\)-.*/\1/')
            local file_size=$(du -h "$dump_file" | cut -f1)
            
            echo -e "${YELLOW}[$count] Archivo: $filename${NC}"
            echo -e "${YELLOW}    PID: $pid${NC}"
            echo -e "${YELLOW}    Tamaño: $file_size${NC}"
            echo -e "${YELLOW}    Ruta: $dump_file${NC}"
            echo ""
            
            # Mostrar strings de 3+ caracteres
            echo "--- Strings (3+ caracteres) ---"
            strings "$dump_file" -n 3
            echo ""
            
            # Mostrar strings de 1+ caracteres (limitado)
            echo "--- Strings (1+ caracteres, primeras 20) ---"
            strings "$dump_file" -n 1 | head -20
            echo ""
            show_separator
            
            ((count++))
        fi
    done <<< "$dump_files"
}

# Función principal
main() {
    echo -e "${BLUE}=== STRINGS EN DUMPS ===${NC}"
    echo "Timestamp: $(date)"
    echo ""
    
    # Verificar que estamos en el directorio correcto
    if [ ! -f "go.mod" ] || [ ! -d "scripts" ]; then
        echo -e "${RED}ERROR: Este script debe ejecutarse desde la raíz del proyecto tp-2025-1c-FSociety${NC}"
        exit 1
    fi
    
    # Mostrar archivos dump
    show_dump_files
    
    echo -e "${GREEN}Visualización de strings completada${NC}"
}

# Ejecutar función principal
main