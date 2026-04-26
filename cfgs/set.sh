#!/bin/bash

# Ruta base del proyecto (el script está dentro de /cfgs/, así que subimos un nivel)
BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CFG_DIR="$BASE_DIR/cfgs"

# Validar argumentos
if [ $# -lt 4 ]; then
  echo "Uso: $0 <nombre_de_config> <num_kernel> <num_cpu> <num_io>"
  echo "Ejemplo: ./set.sh corto_plazo 1 2 1"
  exit 1
fi

CONFIG_SET=$1
NUM_KERNEL=$2
NUM_CPU=$3
NUM_IO=$4

CONFIG_PATH="$CFG_DIR/$CONFIG_SET"

# Verificar que exista el set de configuración
if [ ! -d "$CONFIG_PATH" ]; then
  echo "No se encontró el conjunto de configuración: $CONFIG_PATH"
  exit 1
fi

echo "Creando enlaces simbólicos desde $CONFIG_PATH..."

# Memoria usa configuración estándar (sin número)
SRC="$CONFIG_PATH/memoria_config.json"
DEST="$BASE_DIR/memoria/memoria_config.json"

if [ -e "$DEST" ] || [ -L "$DEST" ]; then
  rm "$DEST"
fi

ln -s "$SRC" "$DEST"
echo "Enlace creado: $DEST > $SRC"

# Kernel usa configuración numerada
SRC="$CONFIG_PATH/kernel_config_${NUM_KERNEL}.json"
DEST="$BASE_DIR/kernel/kernel_config.json"

if [ -e "$DEST" ] || [ -L "$DEST" ]; then
  rm "$DEST"
fi

ln -s "$SRC" "$DEST"
echo "Enlace creado: $DEST > $SRC"

# CPU usa configuración numerada
SRC="$CONFIG_PATH/cpu_config_${NUM_CPU}.json"
DEST="$BASE_DIR/cpu/cpu_config.json"

if [ -e "$DEST" ] || [ -L "$DEST" ]; then
  rm "$DEST"
fi

ln -s "$SRC" "$DEST"
echo "Enlace creado: $DEST > $SRC"

# IO usa configuración numerada
SRC="$CONFIG_PATH/io_config_${NUM_IO}.json"
DEST="$BASE_DIR/io/io_config.json"

if [ -e "$DEST" ] || [ -L "$DEST" ]; then
  rm "$DEST"
fi

ln -s "$SRC" "$DEST"
echo "Enlace creado: $DEST > $SRC"

echo "Configuración aplicada exitosamente desde '$CONFIG_SET' con kernel($NUM_KERNEL), cpu($NUM_CPU), io($NUM_IO)."
