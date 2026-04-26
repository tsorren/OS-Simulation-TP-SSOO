#!/bin/bash

CFG_DIR="$(cd "$(dirname "$0")" && pwd)"

# Validar existencia del directorio
if [ ! -d "$CFG_DIR" ]; then
  echo "No se encontró la carpeta $CFG_DIR"
  exit 1
fi

# Leer IPs
read -p "Ingrese la IP para memoria: " IP_MEMORIA
read -p "Ingrese la IP para kernel: " IP_KERNEL

# Carpeta modificadas
MODIFICADAS=()

# Recorrer subcarpetas con config
for carpeta in "$CFG_DIR"/*; do
  [ -d "$carpeta" ] || continue
  carpeta_nombre=$(basename "$carpeta")
  MODIFICADO=0

  # Actualizar todos los .json
  for archivo in "$carpeta"/*.json; do
    [ -f "$archivo" ] || continue
    sed -i \
      -e "s/\"ip_memory\": *\"[^\"]*\"/\"ip_memory\": \"$IP_MEMORIA\"/" \
      -e "s/\"ip_kernel\": *\"[^\"]*\"/\"ip_kernel\": \"$IP_KERNEL\"/" \
      "$archivo"
    MODIFICADO=1
  done

  if [ $MODIFICADO -eq 1 ]; then
    MODIFICADAS+=("$carpeta_nombre")
  fi

done

# Mostrar carpetas modificadas
printf "\nCarpetas modificadas:\n"
for carpeta in "${MODIFICADAS[@]}"; do
  echo "  $carpeta"
done

printf "\nIP de memoria: %s\n" "$IP_MEMORIA"
printf "IP de kernel:  %s\n" "$IP_KERNEL"
printf "\nArchivos .json actualizados correctamente.\n"
