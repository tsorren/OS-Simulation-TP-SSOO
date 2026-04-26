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

# Log de resumen
declare -A LOG_RESUMEN
declare -a CARPETAS

# Recorrer subcarpetas con config
for carpeta in "$CFG_DIR"/*; do
  [ -d "$carpeta" ] || continue
  carpeta_nombre=$(basename "$carpeta")
  CARPETAS+=("$carpeta_nombre")

  for archivo in "$carpeta"/*.json; do
    [[ "$archivo" =~ cpu_config_.*\.json$ ]] && continue
    [[ "$archivo" =~ io_config_.*\.json$ ]] && continue

    sed -i \
      -e "s/\"ip_memory\": *\"[^\"]*\"/\"ip_memory\": \"$IP_MEMORIA\"/" \
      -e "s/\"port_memory\": *[0-9]*/\"port_memory\": 8000/" \
      -e "s/\"ip_kernel\": *\"[^\"]*\"/\"ip_kernel\": \"$IP_KERNEL\"/" \
      -e "s/\"port_kernel\": *[0-9]*/\"port_kernel\": 8001/" \
      "$archivo"
  done

  LOG_RESUMEN["$carpeta_nombre,memoria"]="IP: $IP_MEMORIA, Puerto: 8000"
  LOG_RESUMEN["$carpeta_nombre,kernel"]="IP: $IP_KERNEL, Puerto: 8001"

  # Actualizar archivos IO config
  for i in {1..10}; do
    IO_FILE="$carpeta/io_config_$i.json"
    if [ -f "$IO_FILE" ]; then
      PORT_IO=$((8100 + i - 1))
      sed -i \
        -e "s/\"port_io\": *[0-9]*/\"port_io\": $PORT_IO/" \
        -e "s/\"port_kernel\": *[0-9]*/\"port_kernel\": 8001/" \
        -e "s/\"ip_kernel\": *\"[^\"]*\"/\"ip_kernel\": \"$IP_KERNEL\"/" \
        "$IO_FILE"
      LOG_RESUMEN["$carpeta_nombre,io_$i"]="Puerto IO: $PORT_IO"
    fi
  done

  # Actualizar archivos CPU config
  for i in {1..10}; do
    CPU_FILE="$carpeta/cpu_config_$i.json"
    if [ -f "$CPU_FILE" ]; then
      PORT_CPU=$((8200 + i - 1))
      sed -i \
        -e "s/\"port_cpu\": *[0-9]*/\"port_cpu\": $PORT_CPU/" \
        -e "s/\"port_memory\": *[0-9]*/\"port_memory\": 8000/" \
        -e "s/\"port_kernel\": *[0-9]*/\"port_kernel\": 8001/" \
        -e "s/\"ip_memory\": *\"[^\"]*\"/\"ip_memory\": \"$IP_MEMORIA\"/" \
        -e "s/\"ip_kernel\": *\"[^\"]*\"/\"ip_kernel\": \"$IP_KERNEL\"/" \
        "$CPU_FILE"
      LOG_RESUMEN["$carpeta_nombre,cpu_$i"]="Puerto CPU: $PORT_CPU"
    fi
  done

done

printf "\nResumen final de configuración aplicada:\n"
printf "----------------------------------------\n"
for carpeta in "${CARPETAS[@]}"; do
  echo "Config: $carpeta"
  echo "  memoria: ${LOG_RESUMEN[$carpeta,memoria]}"
  echo "  kernel:  ${LOG_RESUMEN[$carpeta,kernel]}"
  # Mostrar todos los IOs configurados
  for i in {1..10}; do
    if [[ -n "${LOG_RESUMEN[$carpeta,io_$i]}" ]]; then
      echo "  io_$i:    ${LOG_RESUMEN[$carpeta,io_$i]}"
    fi
  done
  # Mostrar todas las CPUs configuradas
  for i in {1..10}; do
    if [[ -n "${LOG_RESUMEN[$carpeta,cpu_$i]}" ]]; then
      echo "  cpu_$i:   ${LOG_RESUMEN[$carpeta,cpu_$i]}"
    fi
  done
  echo ""
done

printf "Archivos .json actualizados correctamente.\n"
