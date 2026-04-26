#!/bin/bash
#set -x

# Verificar argumento obligatorio
if [ $# -lt 2 ]; then
    echo "Uso: $0 <archivo_de_procesos> <tamanio_de_proceso>"
    exit 1
fi

ARCHIVO_PROCESOS=$1
TAMANIO_PROCESO=$2
PROYECTO_DIR="$(pwd)"
TERMINAL_CMD="x-terminal-emulator -T"
ESPERA=1
BIN_DIR="$PROYECTO_DIR/bin"

# Borrar estructuras 
./scripts/borrar_estructuras.sh

sleep $ESPERA

# Crear carpeta bin
mkdir -p "$BIN_DIR"

# Compilar todos los módulos

echo "Compilando memoria..."
go build -race -o "$BIN_DIR/memoria" ./memoria

echo "Compilando kernel..."
go build -race -o "$BIN_DIR/kernel" ./kernel

echo "Compilando io..."
go build -race -o "$BIN_DIR/io" ./io

echo "Compilando cpu..."
go build -race -o "$BIN_DIR/cpu" ./cpu

# Lanzar los módulos
$TERMINAL_CMD "Memoria" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/memoria" &
pid_memoria=$!

sleep $ESPERA

$TERMINAL_CMD "Kernel" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/kernel $ARCHIVO_PROCESOS $TAMANIO_PROCESO" &
pid_kernel=$!

sleep $ESPERA

$TERMINAL_CMD "IO" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/io DISCO" &
pid_io=$!

sleep $ESPERA

$TERMINAL_CMD "CPU_1" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/cpu CPU_1" &
pid_cpu=$!

echo ""
echo "Todos los módulos fueron lanzados"
echo "Presioná [Enter] para finalizar los procesos..."

read -r

echo "Terminando procesos..."

# Matar procesos cerrando las ventanas por título
wmctrl -c "Memoria" 2>/dev/null || true
wmctrl -c "Kernel" 2>/dev/null || true
wmctrl -c "IO" 2>/dev/null || true
wmctrl -c "CPU_1" 2>/dev/null || true

# Como respaldo, también matar por comando si wmctrl no funciona
pkill -f "$BIN_DIR/memoria" 2>/dev/null || true
pkill -f "$BIN_DIR/kernel" 2>/dev/null || true
pkill -f "$BIN_DIR/io DISCO" 2>/dev/null || true
pkill -f "$BIN_DIR/cpu" 2>/dev/null || true

# Borrar carpeta bin
rm -rf "$BIN_DIR"

echo "Procesos finalizados"
