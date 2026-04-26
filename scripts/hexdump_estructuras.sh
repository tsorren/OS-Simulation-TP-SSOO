#!/bin/bash

# =============================================================================
# Script: ver_estructuras.sh
# Descripción: Muestra el contenido de archivos dump y swapfile en hexadecimal
# Uso: ./scripts/ver_estructuras.sh
# =============================================================================

# Colores para output
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Variables de configuración
DUMP_DIR="$HOME/dump_files"
SWAP_FILE="$HOME/swapfile.bin"

# Función para mostrar separador
show_separator() {
    echo -e "${BLUE}===============================================================================${NC}"
}

# Función para mostrar archivos dump
show_dump_files() {
    echo -e "${GREEN}ARCHIVOS DE DUMP${NC}"
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
            
            echo -e "${YELLOW}[$count] Archivo: $filename${NC}"
            echo -e "${YELLOW}    PID: $pid${NC}"
            echo -e "${YELLOW}    Ruta: $dump_file${NC}"
            echo ""
            
            # Mostrar hexdump
            hexdump -C "$dump_file"
            echo ""
            show_separator
            
            ((count++))
        fi
    done <<< "$dump_files"
}

# Función para mostrar swapfile
show_swap_file() {
    echo -e "${GREEN}ARCHIVO DE SWAP${NC}"
    show_separator
    
    if [ ! -f "$SWAP_FILE" ]; then
        echo -e "${RED}ERROR: Archivo $SWAP_FILE no existe${NC}"
        return 1
    fi
    
    local swap_size=$(du -h "$SWAP_FILE" 2>/dev/null | cut -f1)
    echo -e "${YELLOW}Archivo: $(basename "$SWAP_FILE")${NC}"
    echo -e "${YELLOW}Ruta: $SWAP_FILE${NC}"
    echo -e "${YELLOW}Tamaño: $swap_size${NC}"
    echo ""
    
    # Mostrar hexdump
    hexdump -C "$SWAP_FILE"
    echo ""
    show_separator
}

# Función principal
main() {
    echo -e "${BLUE}=== VISUALIZACIÓN DE ESTRUCTURAS ===${NC}"
    echo "Timestamp: $(date)"
    echo ""
    
    # Verificar que estamos en el directorio correcto
    if [ ! -f "go.mod" ] || [ ! -d "scripts" ]; then
        echo -e "${RED}ERROR: Este script debe ejecutarse desde la raíz del proyecto tp-2025-1c-FSociety${NC}"
        exit 1
    fi
    
    # Mostrar archivos dump
    show_dump_files
    echo ""
    
    # Mostrar swapfile
    show_swap_file
    
    echo -e "${GREEN}Visualización completada${NC}"
}

# Ejecutar función principal
main