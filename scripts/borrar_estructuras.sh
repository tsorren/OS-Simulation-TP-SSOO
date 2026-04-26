#!/bin/bash

# =============================================================================
# Script: borrar_estructuras.sh
# Descripción: Limpia archivos de dump, swap y llama a limpiar_logs
# Uso: ./scripts/borrar_estructuras.sh
# =============================================================================

set -e  # Salir si cualquier comando falla (excepto los que manejamos explícitamente)

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Variables de configuración
DUMP_DIR="$HOME/dump_files"
SWAP_FILE="$HOME/swapfile.bin"

# Función para logging con timestamp
log_info() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')] INFO:${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[$(date '+%H:%M:%S')] WARNING:${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date '+%H:%M:%S')] ERROR:${NC} $1"
}

# Función para limpiar archivos de dump
clean_dump_files() {
    log_info "Limpiando archivos de dump..."
    
    if [ ! -d "$DUMP_DIR" ]; then
        log_warning "Directorio $DUMP_DIR no existe"
        return 0
    fi
    
    local all_files_count=$(find "$DUMP_DIR" -type f 2>/dev/null | wc -l)
    
    if [ "$all_files_count" -eq 0 ]; then
        log_info "No hay archivos en $DUMP_DIR"
        return 0
    fi
    
    rm -rf "${DUMP_DIR:?}"/* 2>/dev/null || true
    log_info "Archivos de dump eliminados exitosamente ($all_files_count archivos)"
    
    # Crear el directorio nuevamente para futuras ejecuciones
    mkdir -p "$DUMP_DIR"
}

# Función para limpiar archivo de swap
clean_swap_file() {
    log_info "Limpiando archivo de swap..."
    
    if [ ! -f "$SWAP_FILE" ]; then
        log_warning "Archivo $SWAP_FILE no existe"
        return 0
    fi
    
    local swap_size=$(du -h "$SWAP_FILE" 2>/dev/null | cut -f1)
    rm -f "$SWAP_FILE"
    log_info "Archivo de swap eliminado exitosamente (${swap_size})"
}

# Función para limpiar logs
clean_logs() {
    log_info "Ejecutando limpieza de logs..."
    
    if [ ! -f "scripts/limpiar_logs.sh" ]; then
        log_error "Script limpiar_logs.sh no encontrado"
        return 1
    fi
    
    bash scripts/limpiar_logs.sh
    log_info "Limpieza de logs completada"
}

# Función principal
main() {
    echo -e "${BLUE}=== LIMPIEZA DE ESTRUCTURAS DEL SISTEMA ===${NC}"
    echo "Timestamp: $(date)"
    echo ""
    
    # Verificar que estamos en el directorio correcto
    if [ ! -f "go.mod" ] || [ ! -d "scripts" ]; then
        log_error "Este script debe ejecutarse desde la raíz del proyecto tp-2025-1c-FSociety"
        exit 1
    fi
    
    # Ejecutar limpieza de cada componente
    clean_dump_files
    echo ""
    clean_swap_file
    echo ""
    clean_logs
    echo ""
    
    log_info "Limpieza completada"
    echo -e "${GREEN}Proceso de limpieza finalizado exitosamente${NC}"
}

# Ejecutar función principal
main