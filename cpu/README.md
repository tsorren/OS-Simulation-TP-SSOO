# Módulo CPU - Implementación Fetch-Decode-Execute

## Cambios realizados

Se implementó el ciclo básico de instrucciones en el archivo `cpu.go` con las siguientes funcionalidades:

### Estructuras agregadas
- `CPUState`: Mantiene el estado del proceso (PID, PC, instrucción actual, estado running)

### Funciones implementadas
1. **Fetch**
   - `fetchInstruction()`: Obtiene la instrucción actual (mock por ahora)
   - Log: `## PID:<PID>-FETCH-PC:<PC>`

2. **Decode** 
   - `decodeInstruction()`: Parsea la instrucción en opcode y parámetros
   - Log: `## PID:<PID>-DECODE-Instrucción:<opcode>-Params:<params>`

3. **Execute**
   - `executeInstruction()`: Ejecuta la instrucción según su tipo
   - Log: `## PID:<PID> -EXECUTE- instrucción><params>`

### Instrucciones soportadas
- `NOOP`: No operation
- `WRITE <dir> <dato>`: Operación de escritura (mock)
- `READ <dir> <tam>`: Operación de lectura (mock)  
- `GOTO <valor>`: Salto a dirección
- `IO <tiempo>`: Operación de E/S
- `INIT_PROC <archivo> <tam>`: Inicialización de proceso
- `DUMP_MEMORY`: Volcado de memoria
- `EXIT`: Terminar proceso

### Otros cambios
- Se mantuvo el servidor HTTPexistente
- Se agregó logging estructurado según especificaciones
- El ciclo principal se ejecuta en una goroutine concurrente

## Próximos pasos
- Implementar comunicación real con Memoria/Kernel
- Obtener PID real del kernel
- Reemplazar mocks por implementaciones reales
