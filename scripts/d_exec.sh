#!/bin/bash

# Verificar argumentos
if [ $# -lt 3 ]; then
    echo "Uso: $0 <archivo_de_procesos> <tamanio_proceso> <modulo_a_debuguear>"
    echo "Opciones de módulo: memoria | kernel | io | cpu"
    exit 1
fi

ARCHIVO_PROCESOS=$1
TAMANIO_PROCESO=$2
MODULO_DEBUG=${3,,}  # Convertir a minúscula
PROYECTO_DIR="$(pwd)"
TERMINAL_CMD="x-terminal-emulator -T"
ESPERA=3

# Limpiar logs
./scripts/limpiar_logs.sh
sleep $ESPERA

# Función auxiliar para lanzar módulos
lanzar_modulo() {
    NOMBRE=$1         # Ej: "CPU_1"
    CMD=$2            # Comando

    # Convertir el nombre del módulo a minúsculas y comparar sin sufijo
    NOMBRE_MIN=$(echo "$NOMBRE" | tr '[:upper:]' '[:lower:]')
    BASE_MODULO=$(echo "$NOMBRE_MIN" | cut -d'_' -f1)

    if [ "$MODULO_DEBUG" == "$BASE_MODULO" ]; then
        echo "Módulo \"$MODULO_DEBUG\" en modo DEBUG. Presione [Enter] para continuar..."
        read -r
    else
        $TERMINAL_CMD "$NOMBRE" -e bash -c "cd $PROYECTO_DIR && $CMD; exec bash" &
        sleep $ESPERA
    fi
}


# Lanzar módulos
lanzar_modulo "Memoria" "go run -race ./memoria"
lanzar_modulo "Kernel" "go run -race ./kernel $ARCHIVO_PROCESOS $TAMANIO_PROCESO"
lanzar_modulo "IO" "go run -race ./io DISCO"
lanzar_modulo "CPU_1" "go run -race ./cpu CPU_1"

# Esperar finalización
echo ""
echo "Todos los módulos fueron lanzados (excepto el debug)."
echo "Presioná [Enter] para finalizar los procesos..."
read -r

# Terminar procesos
echo "Terminando procesos..."

# Matar procesos cerrando las ventanas por título
wmctrl -c "Memoria" 2>/dev/null || true
wmctrl -c "Kernel" 2>/dev/null || true
wmctrl -c "IO" 2>/dev/null || true
wmctrl -c "CPU_1" 2>/dev/null || true

# Como respaldo, también matar por comando si wmctrl no funciona
pkill -f "go run -race ./memoria" 2>/dev/null || true
pkill -f "go run -race ./kernel" 2>/dev/null || true
pkill -f "go run -race ./io DISCO" 2>/dev/null || true
pkill -f "go run -race ./cpu CPU_1" 2>/dev/null || true

echo "Procesos finalizados"
