#!/bin/bash
#set -x

# Verificar argumentos obligatorios
if [ $# -lt 4 ]; then
    echo "Uso: $0 <archivo_de_procesos> <tamanio_de_proceso> <cantidad_de_cpus> <cantidad_de_ios>"
    exit 1
fi

ARCHIVO_PROCESOS=$1
TAMANIO_PROCESO=$2
CANTIDAD_CPUS=$3
CANTIDAD_IOS=$4

# Validar que las cantidades sean números enteros positivos
if ! [[ "$CANTIDAD_CPUS" =~ ^[1-9][0-9]*$ ]]; then
    echo "Error: la cantidad de CPUs debe ser un entero mayor o igual a 1"
    exit 1
fi

if ! [[ "$CANTIDAD_IOS" =~ ^[1-9][0-9]*$ ]]; then
    echo "Error: la cantidad de IOs debe ser un entero mayor o igual a 1"
    exit 1
fi

PROYECTO_DIR="$(pwd)"
CFG_DIR="$PROYECTO_DIR/cfgs"
TERMINAL_CMD="x-terminal-emulator -T"
ESPERA=1
ESPERA_LOOP=1
BIN_DIR="$PROYECTO_DIR/bin"

echo "Obteniendo configuración inicial..."

# Obtener symlinks actuales
SYMLINK_PATH_IO="$PROYECTO_DIR/io/io_config.json"
SYMLINK_PATH_CPU="$PROYECTO_DIR/cpu/cpu_config.json"

# Validar que sean symlinks
if [ ! -L "$SYMLINK_PATH_IO" ]; then
    echo "Error: $SYMLINK_PATH_IO no es un enlace simbólico válido"
    exit 1
fi

if [ ! -L "$SYMLINK_PATH_CPU" ]; then
    echo "Error: $SYMLINK_PATH_CPU no es un enlace simbólico válido"
    exit 1
fi

# Obtener paths reales
REAL_PATH_IO=$(readlink -f "$SYMLINK_PATH_IO")
REAL_PATH_CPU=$(readlink -f "$SYMLINK_PATH_CPU")

if [ ! -f "$REAL_PATH_IO" ]; then
    echo "Error: el archivo apuntado por el symlink IO no existe: $REAL_PATH_IO"
    exit 1
fi

if [ ! -f "$REAL_PATH_CPU" ]; then
    echo "Error: el archivo apuntado por el symlink CPU no existe: $REAL_PATH_CPU"
    exit 1
fi

# Obtener números actuales desde los nombres de archivo
CURRENT_NUM_IO=$(basename "$REAL_PATH_IO" | grep -oE '_([0-9]+)\.json$' | grep -oE '[0-9]+')
CURRENT_NUM_CPU=$(basename "$REAL_PATH_CPU" | grep -oE '_([0-9]+)\.json$' | grep -oE '[0-9]+')

if [ -z "$CURRENT_NUM_IO" ] || [ -z "$CURRENT_NUM_CPU" ]; then
    echo "Error: No se pudo determinar el número actual del archivo de configuración"
    echo "Archivo IO real: $(basename "$REAL_PATH_IO")"
    echo "Archivo CPU real: $(basename "$REAL_PATH_CPU")"
    exit 1
fi

# Obtener carpeta de configuración
CONFIG_SET=$(basename "$(dirname "$REAL_PATH_CPU")")

echo "Iniciando con configuración: $CONFIG_SET"

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

ESPERA=0.5
ESPERA_LOOP=1

# Lanzar Memoria
$TERMINAL_CMD "Memoria" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/memoria" &
pid_memoria=$!

sleep $ESPERA

# Lanzar Kernel
$TERMINAL_CMD "Kernel" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/kernel $ARCHIVO_PROCESOS $TAMANIO_PROCESO" &
pid_kernel=$!

sleep $ESPERA

# Lanzar IOs
echo "Lanzando $CANTIDAD_IOS IO(s)..."
for (( i=1; i<=CANTIDAD_IOS; i++ )); do
    IO_ID="IO_$i"
    CONFIG_NUM=$((CURRENT_NUM_IO + i - 1))
    SRC="$CFG_DIR/$CONFIG_SET/io_config_${CONFIG_NUM}.json"
    DEST="$PROYECTO_DIR/io/io_config.json"

    if [ ! -f "$SRC" ]; then
        echo "Error: no existe el archivo $SRC"
        exit 1
    fi

    # Actualizar el symlink
    rm -f "$DEST"
    ln -s "$SRC" "$DEST"

    $TERMINAL_CMD "$IO_ID" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/io DISCO" &
    sleep "$ESPERA_LOOP"
done

# Lanzar CPUs
echo "Lanzando $CANTIDAD_CPUS CPU(s)..."
for (( i=1; i<=CANTIDAD_CPUS; i++ )); do
    CPU_ID="CPU_$i"
    CONFIG_NUM=$((CURRENT_NUM_CPU + i - 1))
    SRC="$CFG_DIR/$CONFIG_SET/cpu_config_${CONFIG_NUM}.json"
    DEST="$PROYECTO_DIR/cpu/cpu_config.json"

    if [ ! -f "$SRC" ]; then
        echo "Error: no existe el archivo $SRC"
        exit 1
    fi

    # Actualizar el symlink
    rm -f "$DEST"
    ln -s "$SRC" "$DEST"

    $TERMINAL_CMD "$CPU_ID" -e bash -c "cd $PROYECTO_DIR && $BIN_DIR/cpu $CPU_ID" &
    sleep "$ESPERA_LOOP"
done

echo ""
echo "Todos los módulos fueron lanzados con $CANTIDAD_CPUS CPU(s) y $CANTIDAD_IOS IO(s)"
echo "Presioná [Enter] para finalizar los procesos..."

read -r

echo "Terminando procesos..."

# Matar procesos cerrando las ventanas por titulo
wmctrl -c "Memoria" 2>/dev/null || true
wmctrl -c "Kernel" 2>/dev/null || true

for (( i=1; i<=CANTIDAD_IOS; i++ )); do
    IO_ID="IO_$i"
    wmctrl -c "$IO_ID" 2>/dev/null || true
done

for (( i=1; i<=CANTIDAD_CPUS; i++ )); do
    CPU_ID="CPU_$i"
    wmctrl -c "$CPU_ID" 2>/dev/null || true
done

# Como respaldo, también matar por comando si wmctrl no funciona
pkill -f "$BIN_DIR/memoria" 2>/dev/null || true
pkill -f "$BIN_DIR/kernel" 2>/dev/null || true
pkill -f "$BIN_DIR/io DISCO" 2>/dev/null || true
pkill -f "$BIN_DIR/cpu" 2>/dev/null || true

# Restaurar los symlinks a los valores originales
ln -sf "$REAL_PATH_IO" "$PROYECTO_DIR/io/io_config.json"
ln -sf "$REAL_PATH_CPU" "$PROYECTO_DIR/cpu/cpu_config.json"

# Borrar carpeta bin
rm -rf "$BIN_DIR"

echo "Procesos finalizados"
