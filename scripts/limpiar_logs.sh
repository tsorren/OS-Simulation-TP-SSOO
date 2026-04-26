#!/bin/bash

# Directorio base del proyecto (desde donde se ejecuta el script)
BASE_DIR="$(pwd)"
OLD_LOGS_DIR="$BASE_DIR/old_logs"

# Crear carpeta old_logs si no existe
mkdir -p "$OLD_LOGS_DIR"

# Función para remover códigos de color ANSI
remove_ansi_colors() {
    sed 's/\x1b\[[0-9;]*m//g'
}

# Lista de módulos únicos y sus rutas relativas
declare -A logs=(
  ["kernel"]="kernel/kernel.log"
  ["memoria"]="memoria/memoria.log"
)

# Procesar cada log único
for modulo in "${!logs[@]}"; do
  log_actual="${BASE_DIR}/${logs[$modulo]}"
  log_historial="${OLD_LOGS_DIR}/${modulo}.log"

  if [ -f "$log_actual" ]; then
    echo "Procesando log de $modulo..."

    # Crear archivo de historial si no existe
    touch "$log_historial"

    # Agregar separador con timestamp
    echo "===================================" >> "$log_historial"
    echo "== N U E V A   E J E C U C I Ó N ==" >> "$log_historial"
    echo -e "===================================\n" >> "$log_historial"

    # Appendea el contenido del log actual al histórico sin colores
    cat "$log_actual" | remove_ansi_colors >> "$log_historial"
    echo "" >> "$log_historial"

    # Elimina el log actual
    rm "$log_actual"
  else
    echo "No se encontró log actual para $modulo, se omite"
  fi
done

# Procesar logs de CPU (cpu_1.log, cpu_2.log, etc.)
echo "Procesando logs de CPU..."
for cpu_log in "${BASE_DIR}/cpu"/cpu_*.log; do
  if [ -f "$cpu_log" ]; then
    # Extraer el nombre del archivo (ej: cpu_1.log)
    cpu_name=$(basename "$cpu_log")
    # Crear nombre para el historial (ej: cpu_1.log)
    cpu_historial="${OLD_LOGS_DIR}/${cpu_name}"
    
    echo "Procesando $cpu_name..."
    
    # Crear archivo de historial si no existe
    touch "$cpu_historial"
    
    # Agregar separador con timestamp
    echo "===================================" >> "$cpu_historial"
    echo "== N U E V A   E J E C U C I Ó N ==" >> "$cpu_historial"
    echo -e "===================================\n" >> "$cpu_historial"
    
    # Appendea el contenido del log actual al histórico sin colores
    cat "$cpu_log" | remove_ansi_colors >> "$cpu_historial"
    echo "" >> "$cpu_historial"
    
    # Elimina el log actual
    rm "$cpu_log"
  fi
done

# Procesar logs de IO (io_1.log, io_2.log, etc.)
echo "Procesando logs de IO..."
for io_log in "${BASE_DIR}/io"/io_*.log; do
  if [ -f "$io_log" ]; then
    # Extraer el nombre del archivo (ej: io_1.log)
    io_name=$(basename "$io_log")
    # Crear nombre para el historial (ej: io_1.log)
    io_historial="${OLD_LOGS_DIR}/${io_name}"
    
    echo "Procesando $io_name..."
    
    # Crear archivo de historial si no existe
    touch "$io_historial"
    
    # Agregar separador con timestamp
    echo "===================================" >> "$io_historial"
    echo "== N U E V A   E J E C U C I Ó N ==" >> "$io_historial"
    echo -e "===================================\n" >> "$io_historial"
    
    # Appendea el contenido del log actual al histórico sin colores
    cat "$io_log" | remove_ansi_colors >> "$io_historial"
    echo "" >> "$io_historial"
    
    # Elimina el log actual
    rm "$io_log"
  fi
done

echo "Limpieza de logs finalizada"
