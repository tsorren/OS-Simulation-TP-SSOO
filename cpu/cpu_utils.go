package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	globals "tp-2025-1c-FSociety/utils"
	queues "tp-2025-1c-FSociety/utils/Queues"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

type CPUState struct { //representación del estado de la CPU
	PID         int
	PC          int
	CantPaginas int
	CurrentInst string
	Running     bool
}

var (
	CPUConfig       *globals.CPU_config
	identificador   string
	cpuState        *CPUState
	mutexCPU        sync.Mutex
	mutexTLB        sync.Mutex
	mutexCACHE      sync.Mutex
	interrupciones  queues.Queue // Cola para manejar interrupciones
	MEMORIAInfo     globals.MemoriaInfo
	TLB             []globals.EntradaTLB
	cachePaginas    []globals.EntradaCache
	posicionPuntero int = 0
	metricasCache   globals.MetricasCPU
	metricasTLB     globals.MetricasCPU
)

// smeaforo que arranca el ciclo de cpu porque le avisa que llego un proceso
var (
	notficarProcesoAEjecutar = make(chan struct{})
	desalojoSRT              = make(chan struct{})
)

func initCPU() {
	MEMORIAInfo = InformacionBasicaDeMemoria()
	mutexTLB.Lock()
	if CPUConfig.TLB_entries > 0 {
		utils_general.LoggearMensaje("INFO", "TLB Habilitada - Entradas: %d - Algoritmo: %s", CPUConfig.TLB_entries, CPUConfig.TLB_replacement)
		initTLB()
	} else {
		utils_general.LoggearMensaje("INFO", "TLB Deshabilitada")
	}
	mutexTLB.Unlock()

	mutexCACHE.Lock()
	if cacheEstaActivada() {
		utils_general.LoggearMensaje("INFO", "Cache Habilitada - Entradas: %d - Algoritmo: %s", CPUConfig.Cache_entries, CPUConfig.Cache_replacement)
		initCache()
	} else {
		utils_general.LoggearMensaje("INFO", "Cache Deshabilitada")
	}
	mutexCACHE.Unlock()

	interrupciones = queues.Queue{}                       // Inicializar la cola de interrupciones
	interrupciones.Payload_items = make([]interface{}, 0) // Inicializar el slice de paquetes
}

func InformacionBasicaDeMemoria() globals.MemoriaInfo {
	respuesta, _ := comunicacion.LeerDeServidor[globals.MemoriaInfo](CPUConfig.Ip_memory, CPUConfig.Port_memory, "infoBasica")
	return respuesta
}

// F E T C H:
func fetchInstruction(cpuState *CPUState) string {

	utils_general.LoggearMensaje("INFO", "## PID: %d - FETCH - Program Counter: %d", cpuState.PID, cpuState.PC)
	params := []string{
		strconv.Itoa(cpuState.PID),
		strconv.Itoa(cpuState.PC)}

	respuesta, _ := comunicacion.LeerDeServidor[globals.RespuestaInstruccion](CPUConfig.Ip_memory, CPUConfig.Port_memory, "proximaInstruccion", params...)
	return respuesta.Instruccion
}

// D E C O D E:
/*
func decodeInstruction(inst string) (string, []string) {
	// Implementación básica de decode
	parts := strings.Split(inst, " ")
	if len(parts) == 0 {
		return "", nil
	}
	opcode := parts[0]
	params := parts[1:]
	utils_general.LoggearMensaje("INFO", "## PID: %d - DECODE - Instrucción: %s - Params: %v", cpuState.PID, opcode, params)

	if (opcode == "WRITE" || opcode == "READ") && !cacheEstaActivada() {
		params[0] = traducirDireccion(params[0])
		utils_general.LoggearMensaje("DEBUG", "Dirección lógica traducida a física")
	}

	return opcode, params
}
*/
func decodeInstruction(inst string) (string, []string) {
	parts := strings.Split(inst, " ")
	if len(parts) == 0 {
		return "", nil
	}
	opcode := parts[0]
	params := parts[1:]
	utils_general.LoggearMensaje("INFO", "## PID: %d - DECODE - Instrucción: %s - Params: %v", cpuState.PID, opcode, params)

	// Las instrucciones READ y WRITE siempre trabajan con direcciones lógicas
	// La traducción se hace internamente en los métodos correspondientes
	// No modificamos los parámetros aquí, ya que necesitamos la dirección lógica
	// para poder trabajar correctamente con la caché

	return opcode, params
}

func calcularEntrada(nPag int, nivel int) int {
	exp := float64(MEMORIAInfo.Niveles - nivel)
	divisor := int(math.Pow(float64(MEMORIAInfo.CantEntradasTabla), exp))
	return (nPag / divisor) % MEMORIAInfo.CantEntradasTabla
}

func calcularEntradas(nPag int) []int {
	entradas := make([]int, MEMORIAInfo.Niveles)
	for nivel := 1; nivel <= MEMORIAInfo.Niveles; nivel++ {
		entradas[nivel-1] = calcularEntrada(nPag, nivel)
	}
	return entradas
}

func frameDesdeMemoria(pid int, nroPagina int) int {
	entradas := calcularEntradas(nroPagina)
	params := []string{strconv.Itoa(pid)}
	for _, entrada := range entradas {
		params = append(params, strconv.Itoa(entrada))
	}
	utils_general.LoggearMensaje("DEBUG", "Obteniendo marco con params %s", params)

	respuesta, _ := comunicacion.LeerDeServidor[globals.RespuestaFrame](CPUConfig.Ip_memory, CPUConfig.Port_memory, "marco", params...)
	utils_general.LoggearMensaje("INFO", "## PID: %d - OBTENER MARCO - Página: %d - Marco: %d", pid, nroPagina, respuesta.Frame)
	return respuesta.Frame
}

func direccionFisica(frame int, desplazamiento int) string {
	if frame < 0 || frame > MEMORIAInfo.CantFrames {
		panic("direccion fisica invalida: frame fuera de rango")
	}
	if desplazamiento < 0 || desplazamiento >= MEMORIAInfo.TamanioPagina {
		panic("direccion fisica invalida: desplazamiento fuera de rango")
	}
	return strconv.Itoa(frame*MEMORIAInfo.TamanioPagina + desplazamiento)
}

func separarDireccionLogica(direccionLogica string) (int, int) {
	direccionLogicaInt, _ := strconv.Atoi(direccionLogica)
	nroPagina := direccionLogicaInt / MEMORIAInfo.TamanioPagina
	desplazamiento := direccionLogicaInt % MEMORIAInfo.TamanioPagina
	utils_general.LoggearMensaje("DEBUG", "Dirección lógica: %s separada en página: %d y desplazamiento: %d", direccionLogica, nroPagina, desplazamiento)

	// Validar que la página y el desplazamiento estén dentro de los límites
	if nroPagina < 0 || nroPagina >= cpuState.CantPaginas {
		panic("dirección lógica inválida: número de página fuera de rango")
	}
	if desplazamiento < 0 || desplazamiento >= MEMORIAInfo.TamanioPagina {
		panic("dirección lógica inválida: desplazamiento fuera de rango")
	}
	return nroPagina, desplazamiento
}

func obtenerFrame(nroPagina int) int {
	var frame int
	if !tlbDeshabilitada() {
		var estaEnTLB bool
		frame, estaEnTLB = accederTLB(cpuState.PID, nroPagina)
		if !estaEnTLB {
			frame = frameDesdeMemoria(cpuState.PID, nroPagina)
			agregarEntradaTLB(cpuState.PID, nroPagina, frame)
		}
		// TLB deshabilitada
	} else {
		frame = frameDesdeMemoria(cpuState.PID, nroPagina)
	}
	return frame
}

func traducirDireccion(direccionLogica string) string {
	nroPagina, desplazamiento := separarDireccionLogica(direccionLogica)
	frame := obtenerFrame(nroPagina)

	return direccionFisica(frame, desplazamiento)
}

// E X E C U T E:
func executeInstruction(cpuState *CPUState, opcode string, params []string) {
	// LOG obligatorio: Ejecutando instrucción
	utils_general.LoggearMensaje("INFO", "## PID: %d - Ejecutando: %s - %v", cpuState.PID, opcode, params)

	switch opcode {
	case "NOOP":
		break

	case "WRITE":
		if cacheEstaActivada() {
			escribirEnCache(params[0], params[1])
		} else {
			// Traducir dirección lógica a física solo si no usamos caché
			dirFisica := traducirDireccion(params[0])
			escribirEnMemoria(dirFisica, params[1])
		}
	case "READ":
		var dato string
		if cacheEstaActivada() {
			dato = leerEnCache(params[0], params[1])
		} else {
			// Traducir dirección lógica a física solo si no usamos caché
			dirFisica := traducirDireccion(params[0])
			dato = leerEnMemoria(dirFisica, params[1])
		}
		fmt.Println("Dato leido: " + dato) // TODO: Revisar, creo que requerido por enunciado

	case "GOTO":
		saltar(cpuState, params[0])

	case "INIT_PROC":
		NotificarSyscallKernel(opcode, params, cpuState.PID)

	case "IO", "DUMP_MEMORY", "EXIT":
		NotificarSyscallKernel(opcode, params, cpuState.PID)
		liberarCPU(identificador, cpuState.PID, "syscall")

	default:
		utils_general.LoggearMensaje("DEBUG", "## PID: %d - ERROR - Instrucción desconocida: %s", cpuState.PID, opcode)
		cpuState.Running = false
	}
	if opcode != "GOTO" {
		cpuState.PC++ // TODO: No repetir lógica en cpuState.PC++
	}
}

func cacheEstaActivada() bool {
	return CPUConfig.Cache_entries > 0
}

// Busca una entrada en cache
func buscarEnCache(pid int, pagina int) int {
	mutexCACHE.Lock()
	defer mutexCACHE.Unlock()

	metricasCache.Accesos++

	for i, entrada := range cachePaginas {
		if entrada.PID == pid && entrada.Pagina == pagina {
			utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Hit - Pagina: %d", cpuState.PID, pagina)
			cachePaginas[i].Uso = true // Marcar como usada

			metricasCache.Hits++

			// Log de DEBUG para mostrar el estado de la cache
			logCacheStatus(pid)

			return i
		}
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Miss - Pagina: %d", cpuState.PID, pagina)

	metricasCache.Misses++

	// Log de DEBUG para mostrar el estado de la cache
	logCacheStatus(pid)

	return traerPaginaACache(cpuState.PID, pagina)
}

// Asume que se tomó mutexCACHE
func cacheEstaLlena() bool {
	return len(cachePaginas) >= CPUConfig.Cache_entries
}

func elegirVictima() int { // Devuelve el índice, no la entrada
	n := CPUConfig.Cache_entries

	if CPUConfig.Cache_replacement == "CLOCK" {
		// Primera pasada: buscar página no usada
		for i := 0; i < n; i++ {
			indice := (posicionPuntero + i) % n
			if !cachePaginas[indice].Uso {
				posicionPuntero = (indice + 1) % n
				return indice
			}
			cachePaginas[indice].Uso = false // Resetear bit de uso durante la primera pasada
		}
		// Si llegamos aquí, todas las páginas tenían Uso=true, pero ahora están resetadas
		// Tomar la primera (la más antigua desde posicionPuntero)
		victima := posicionPuntero
		posicionPuntero = (posicionPuntero + 1) % n
		return victima
	} else { // CLOCK-M
		// Primera pasada: buscar (0,0) - no usada, no modificada
		for i := 0; i < n; i++ {
			indice := (posicionPuntero + i) % n
			if !cachePaginas[indice].Uso && !cachePaginas[indice].Modificada {
				posicionPuntero = (indice + 1) % n
				return indice
			}
		}
		// Segunda pasada: buscar (0,1) - no usada, modificada mientras pisas bit uso
		for i := 0; i < n; i++ {
			indice := (posicionPuntero + i) % n
			if !cachePaginas[indice].Uso && cachePaginas[indice].Modificada {
				posicionPuntero = (indice + 1) % n
				return indice
			}
			cachePaginas[indice].Uso = false
		}
		return elegirVictima()
	}
}

// Asume que se tomó mutexCACHE
// Reemplaza una entrada de cache si está llena, y si está modificada la envía a memoria
func agregarPagina(nueva globals.EntradaCache) int {
	indice := len(cachePaginas)
	if cacheEstaLlena() {
		indice = elegirVictima()
		reemplazarPaginaVictimaEnIndice(indice, nueva)
	} else {
		cachePaginas = append(cachePaginas, nueva)
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Add - Pagina: %d", nueva.PID, nueva.Pagina)

	return indice
}

// Reemplaza la página en el índice dado por la nueva entrada
func reemplazarPaginaVictimaEnIndice(indice int, nueva globals.EntradaCache) {
	if cachePaginas[indice].Modificada {
		actualizarPaginaEnMemoria(cachePaginas[indice].PID, cachePaginas[indice].Pagina, cachePaginas[indice].Contenido)
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Replace - Página vieja: %d - Página nueva: %d",
		cachePaginas[indice].PID, cachePaginas[indice].Pagina, nueva.Pagina)

	metricasCache.Reemplazos++

	cachePaginas[indice] = nueva
}

// Asume que se tomó mutexCACHE
// Agrega una página a la cache (pidiéndola a memoria si es necesario)
func traerPaginaACache(pid int, pagina int) int {
	if pagina < 0 || pagina >= cpuState.CantPaginas {
		panic(fmt.Sprintf("Intento de acceso a página inválida: %d", pagina))
	}

	contenido := pedirPaginaAMemoria(pid, pagina)

	entrada := globals.EntradaCache{
		PID:        pid,
		Pagina:     pagina,
		Contenido:  make([]byte, MEMORIAInfo.TamanioPagina),
		Modificada: false,
		Uso:        true, // Marcar como usada al agregar
	}
	copy(entrada.Contenido, contenido)

	// Si la cache está llena, se logueará el reemplazo dentro de agregarPagina

	indice := agregarPagina(entrada)

	if len(cachePaginas) == 0 {
		panic("Error al agregar página a la caché: caché vacía")
	}

	return indice
}

// Pide una página a memoria y devuelve el contenido (array de bytes)
func pedirPaginaAMemoria(pid int, pagina int) []byte {
	direccion := obtenerFrame(pagina) * MEMORIAInfo.TamanioPagina

	params := []string{strconv.Itoa(pid), strconv.Itoa(direccion)}
	// params := []string{strconv.Itoa(pid), strconv.Itoa(pagina)} // TODO: Hacer traduccion de entradas
	respuesta, _ := comunicacion.LeerDeServidor[globals.PaginaMemoria](CPUConfig.Ip_memory, CPUConfig.Port_memory, "leerPagina", params...)

	contenidoLeido := comunicacion.LeerDatosBinarios(respuesta.Contenido)

	utils_general.LoggearMensaje("INFO", "## PID: %d - Acción: LEER - Dirección Física: %d - Valor: %s",
		cpuState.PID, direccion, contenidoLeido)

	return respuesta.Contenido // []byte
}

// Envía una página modificada a memoria
func actualizarPaginaEnMemoria(pid int, pagina int, contenido []byte) {
	frame := obtenerFrame(pagina)
	dirFisicaPagina := frame * MEMORIAInfo.TamanioPagina

	paquete := globals.Paquete{
		Tipo:    "actualizarPagina",
		Origen:  "cpu",
		Destino: "memoria",
		Payload: globals.PaginaMemoria{
			PID:       pid,
			Direccion: dirFisicaPagina,
			Contenido: contenido,
		},
	}
	contenidoEscrito := comunicacion.LeerDatosBinarios(contenido)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Memory Update - Página: %d - Frame: %d", pid, pagina, frame)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor: %s",
		cpuState.PID, dirFisicaPagina, contenidoEscrito)
	go comunicacion.EnviarPaquete(CPUConfig.Ip_memory, CPUConfig.Port_memory, "/actualizarPagina", paquete)
}

func retardoCache() {
	time.Sleep(time.Duration(CPUConfig.Cache_delay) * time.Millisecond)
	// utils_general.LoggearMensaje("DEBUG", "Retardo de cache aplicado: %d ms", CPUConfig.Cache_delay)
}

// Escribe en cache usando desplazamiento
func escribirEnCache(dirLogica string, dato string) {
	retardoCache()
	nroPagina, desplazamiento := separarDireccionLogica(dirLogica)
	indice := buscarEnCache(cpuState.PID, nroPagina)

	// Marcar la entrada como usada y modificada
	cachePaginas[indice].Uso = true
	cachePaginas[indice].Modificada = true
	utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Update - Página: %d", cpuState.PID, nroPagina)

	// Copiar el dato al buffer de la página
	copy(cachePaginas[indice].Contenido[desplazamiento:desplazamiento+len(dato)], []byte(dato))

	// Obtener la dirección física para el log (usando la tabla de páginas)
}

// Lee de cache usando desplazamiento
func leerEnCache(dirLogica string, tamanio string) string {
	retardoCache()
	nroPagina, desplazamiento := separarDireccionLogica(dirLogica)
	tam, _ := strconv.Atoi(tamanio)

	indice := buscarEnCache(cpuState.PID, nroPagina)
	// Marcar como usado
	cachePaginas[indice].Uso = true

	// Leer los datos de la caché
	fin := min(desplazamiento+tam, len(cachePaginas[indice].Contenido))
	valor := string(cachePaginas[indice].Contenido[desplazamiento:fin])

	// Obtener la dirección física para el log (usando la tabla de páginas)
	return valor
}

func leerEnMemoria(dirFisica string, tamanio string) string {
	pid := strconv.Itoa(cpuState.PID)
	respuesta, _ := comunicacion.LeerDeServidor[globals.Mensaje](CPUConfig.Ip_memory, CPUConfig.Port_memory, "leer", pid, dirFisica, tamanio)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Acción: LEER - Dirección Física: %s - Valor: %s", cpuState.PID, dirFisica, respuesta.Mensaje)
	return respuesta.Mensaje
}

func saltar(cpuState *CPUState, pcString string) {
	pc, err := strconv.Atoi(pcString)
	// ACLARACION: si el parametro esta vacio go lo toma como 0
	if err != nil || pc < 0 {
		utils_general.LoggearMensaje("DEBUG", "## PID: %d - ERROR - GOTO con parámetro inválido ", cpuState.PID)
		cpuState.Running = false
	}
	cpuState.PC = pc
}

func escribirEnMemoria(dirFisica string, dato string) {
	utils_general.LoggearMensaje("INFO", "## PID: %d - Acción: ESCRIBIR - Dirección Física: %s - Valor: %s", cpuState.PID, dirFisica, dato)
	dirFisicaInt, _ := strconv.Atoi(dirFisica)
	paquete := globals.Paquete{
		Tipo:    "mensaje",
		Origen:  "cpu",
		Destino: "memoria",
		Payload: globals.EscrituraMem{
			PID:             cpuState.PID,
			DireccionFisica: dirFisicaInt,
			Dato:            dato,
		},
	}
	comunicacion.EnviarPaquete(CPUConfig.Ip_memory, CPUConfig.Port_memory, "/escribir", paquete)
}

/*Manda paquete a kernel con la syscall y los parámetros a ejecutar*/
func NotificarSyscallKernel(keyWord string, params []string, pid int) {
	actualizarPC(identificador, cpuState.PID, cpuState.PC+1)
	syscall := globals.Paquete{
		Tipo:    "Syscall",
		Origen:  "cpu",
		Destino: "kernel",
		Payload: globals.Syscall{
			KeyWord: keyWord,
			Params:  params,
			PID:     pid,
		},
	}

	comunicacion.EnviarPaquete(CPUConfig.Ip_kernel, CPUConfig.Port_kernel, "/ejecucionSyscall", syscall)
}

func actualizarPC(identificador string, pid int, pc int) {
	paquete := globals.Paquete{
		Tipo:    "mensaje",
		Origen:  "cpu",
		Destino: "kernel",
		Payload: globals.ActualizacionPC{
			Identificador: identificador,
			PID:           pid,
			PC:            pc,
		},
	}
	utils_general.LoggearMensaje("DEBUG", "Actualizando PC")
	comunicacion.EnviarPaquete(CPUConfig.Ip_kernel, CPUConfig.Port_kernel, "/actualizarPC", paquete)
	utils_general.LoggearMensaje("DEBUG", "PC Actualizado")
}

// Muestra métricas de caché o TLB
func mostrarMetricas(tipo string, metricas globals.MetricasCPU) {
	// Calcular hitrate si hay accesos
	if metricas.Accesos > 0 {
		metricas.Hitrate = float64(metricas.Hits) / float64(metricas.Accesos) * 100.0
	}

	utils_general.LoggearMensaje("INFO",
		"## PID: %d - Métricas de %s: Accesos: %d, Hits: %d, Misses: %d, Hit Rate: %.2f%%, Reemplazos: %d",
		cpuState.PID, tipo, metricas.Accesos, metricas.Hits, metricas.Misses, metricas.Hitrate, metricas.Reemplazos)
}

// Envía todas las páginas de la cache del proceso a memoria y elimina sus entradas de la cache
func vaciarCache() {
	if !cacheEstaActivada() {
		return
	}
	paginasParaActualizar := make([]globals.EntradaCache, 0)
	for i := range cachePaginas {
		entrada := cachePaginas[i]
		if entrada.Modificada {
			paginasParaActualizar = append(paginasParaActualizar, entrada)
		}
	}
	for _, entrada := range paginasParaActualizar {
		actualizarPaginaEnMemoria(entrada.PID, entrada.Pagina, entrada.Contenido)
	}

	initCache()
	utils_general.LoggearMensaje("INFO", "## PID: %d - Cache Vaciada", cpuState.PID)
}

// cuando se desaloja un proceso, se limpia la TLB de todas las entradas asociadas a ese PID
func vaciarTLB() {
	initTLB()
	utils_general.LoggearMensaje("INFO", "## PID: %d - TLB Vaciada", cpuState.PID)
}

func initTLB() {
	TLB = make([]globals.EntradaTLB, 0, CPUConfig.TLB_entries)
	metricasTLB = globals.MetricasCPU{
		Accesos:    0,
		Reemplazos: 0,
		Hits:       0,
		Misses:     0,
		Hitrate:    0.0,
	}
}
func liberarCPU(identificador string, pid int, motivo string) {
	// Mostrar métricas y vaciar la cache y TLB con los mutexes apropiados

	utils_general.LoggearMensaje("INFO", "## PID: %d - Liberando CPU", pid)

	// Manejar cache
	mutexCACHE.Lock()
	if cacheEstaActivada() {
		mostrarMetricas("Cache", metricasCache)
		vaciarCache()
	}
	mutexCACHE.Unlock()

	// Manejar TLB
	mutexTLB.Lock()
	if !tlbDeshabilitada() {
		mostrarMetricas("TLB", metricasTLB)
		vaciarTLB()
	}
	mutexTLB.Unlock()

	cpuState.Running = false
	paquete := globals.Paquete{
		Tipo:    "mensaje",
		Origen:  "cpu",
		Destino: "kernel",
		Payload: globals.LiberacionCPU{
			Identificador: identificador,
			PID:           pid,
			Motivo:        motivo,
		},
	}
	comunicacion.EnviarPaquete(CPUConfig.Ip_kernel, CPUConfig.Port_kernel, "/liberacionCPU", paquete)
}

// C H E C K   I N T E R R U P T
func checkInterrupt(cpuState *CPUState) {
	//verifica una variable bool global que indica si hay una interrupcion

	if interrupciones.IsEmpty_ts() {
		utils_general.LoggearMensaje("DEBUG", "## PID: %d - No hay interrupciones pendientes", cpuState.PID)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "## PID: %d - Interrupción recibida, procesando...", cpuState.PID)
	interrupcion, _ := comunicacion.DecodePayloadAs[globals.Interrupt](interrupciones.Pop_ts())

	if interrupcion.Nombre == "" {
		return
	}

	if interrupcion.Nombre == "desalojoSRT" {
		actualizarPC(identificador, cpuState.PID, cpuState.PC)
		liberarCPU(identificador, cpuState.PID, interrupcion.Nombre)
		utils_general.LoggearMensaje("INFO", "## PID: %d - Desalojado", cpuState.PID)
		desalojoSRT <- struct{}{} // Notificar al ciclo de CPU que se debe desalojar
	}
}

func cpuCycle() {
	for {
		utils_general.LoggearMensaje("INFO", "## %s - Esperando proceso...", identificador)
		//semaforo que reciba el proceso
		<-notficarProcesoAEjecutar
		mutexCPU.Lock()
		for cpuState.Running {
			// Fetch
			inst := fetchInstruction(cpuState)

			// Decode
			opcode, params := decodeInstruction(inst)

			// Execute
			executeInstruction(cpuState, opcode, params)

			// Check Interrupt
			checkInterrupt(cpuState)
		}
		mutexCPU.Unlock()
	}
}

func procesoReadyHandler(w http.ResponseWriter, r *http.Request) {
	paquete := comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Error al recibir paquete desde Kernel")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	proceso, err := comunicacion.DecodePayloadAs[globals.Proceso](paquete.Payload)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Error al decodificar proceso: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "Recibido proceso PID %d desde Kernel", proceso.Pcb.PID)
	ejecutarProceso(proceso)
	w.WriteHeader(http.StatusOK)
}

func ejecutarProceso(proceso globals.Proceso) {
	mutexCPU.Lock()
	cpuState = &CPUState{
		PID:         proceso.Pcb.PID,
		PC:          proceso.Pcb.PC,
		CantPaginas: int(math.Ceil(float64(proceso.Tamanio) / float64(MEMORIAInfo.TamanioPagina))),
		Running:     true,
	}
	utils_general.LoggearMensaje("DEBUG", "## PID: %d - Proceso listo para ejecutar - PC: %d - Cantidad de páginas: %d", cpuState.PID, cpuState.PC, cpuState.CantPaginas)
	//notificar por semaforo al cpu cycle

	mutexCPU.Unlock()
	notficarProcesoAEjecutar <- struct{}{}
}

func recibirInterrupcionHandler(w http.ResponseWriter, r *http.Request) {
	paquete := comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "Error al recibir paquete desde Kernel")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	interrupcion, err := comunicacion.DecodePayloadAs[globals.Interrupt](paquete.Payload)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "Error al decodificar interrupción: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	utils_general.LoggearMensaje("INFO", "## Llega interrupción al puerto Interrupt")

	realizarInterrupcion(interrupcion)

	w.WriteHeader(http.StatusOK)

}

func realizarInterrupcion(interrupcion globals.Interrupt) {
	// Agregar a la cola de interrupciones
	interrupciones.Push_ts(interrupcion)

	utils_general.LoggearMensaje("DEBUG", "Desalojando proceso")
	<-desalojoSRT
	utils_general.LoggearMensaje("DEBUG", "Desalojo terminado")
}

func tlbDeshabilitada() bool {
	return CPUConfig.TLB_entries == 0
}

func tlbLlena() bool {
	return len(TLB) >= CPUConfig.TLB_entries
}

// Accede a la TLB para buscar un frame dado un PID y una página.
// Retorna el frame y un booleano indicando si fue un hit o no.
func accederTLB(pid int, pagina int) (int, bool) {
	mutexTLB.Lock()
	defer mutexTLB.Unlock()

	metricasTLB.Accesos++

	var frame int = -1
	var hit bool = false
	for i := range TLB {
		if TLB[i].PID == pid && TLB[i].Pagina == pagina {
			// Actualizar timestamp para LRU
			TLB[i].Timestamp = time.Now()
			frame = TLB[i].Frame
			hit = true
			if CPUConfig.TLB_replacement == "LRU" {
				// Mover la entrada al principio (más reciente)
				entrada := TLB[i]
				TLB = append(TLB[:i], TLB[i+1:]...)
				TLB = append([]globals.EntradaTLB{entrada}, TLB...)
				break
			}

		}
	}
	if hit {
		utils_general.LoggearMensaje("INFO", "## PID: %d - TLB HIT - Pagina: %d", pid, pagina)
		metricasTLB.Hits++
	} else {
		utils_general.LoggearMensaje("INFO", "## PID: %d - TLB MISS - Pagina: %d", pid, pagina)
		metricasTLB.Misses++
	}

	// Log de DEBUG para mostrar el orden de reemplazo
	logTLBStatus(pid)

	return frame, hit
}

// Elige la víctima para reemplazo (pop del último elemento, el más antiguo)
func elegirVictimaTLB() int {
	// Siempre el último elemento (más antiguo)
	return len(TLB) - 1
}

// Agrega una entrada a la TLB, manteniendo el orden de más reciente a más antiguo
func agregarEntradaTLB(pid int, pagina int, frame int) {
	mutexTLB.Lock()
	defer mutexTLB.Unlock()

	nuevaEntrada := globals.EntradaTLB{
		PID:       pid,
		Pagina:    pagina,
		Frame:     frame,
		Timestamp: time.Now(),
	}

	if tlbLlena() {
		victima := elegirVictimaTLB()
		utils_general.LoggearMensaje("INFO", "## PID: %d - TLB Replace - Página vieja: %d - Página nueva: %d", pid, TLB[victima].Pagina, pagina)
		// Eliminar la víctima (último elemento)
		TLB = TLB[:victima]
		metricasTLB.Reemplazos++
	}
	// Insertar nueva entrada al principio (más reciente)
	TLB = append([]globals.EntradaTLB{nuevaEntrada}, TLB...)
}

// Muestra el estado actual de la TLB ordenado de más reciente a más antiguo (según timestamp)
func logTLBStatus(pid int) {
	if tlbDeshabilitada() || len(TLB) == 0 {
		return
	}

	var statusMsg string = fmt.Sprintf("## PID: %d - TLB Status (orden reemplazo): [", pid)
	for i, entrada := range TLB {
		if i > 0 {
			statusMsg += ", "
		}
		statusMsg += strconv.Itoa(entrada.Pagina)
	}
	statusMsg += "]"
	utils_general.LoggearMensaje("INFO", "%s", statusMsg)
}

// Asume que se tomó mutexCACHE
func initCache() {
	cachePaginas = make([]globals.EntradaCache, 0, CPUConfig.Cache_entries)
	posicionPuntero = 0
	metricasCache = globals.MetricasCPU{
		Accesos:    0,
		Reemplazos: 0,
		Hits:       0,
		Misses:     0,
		Hitrate:    0.0,
	}
}

// Muestra el estado actual de la cache con bits de uso, modificado y posición del puntero
func logCacheStatus(pid int) {
	if !cacheEstaActivada() || len(cachePaginas) == 0 {
		return
	}

	var statusMsg string = fmt.Sprintf("## PID: %d - Cache Status: [", pid)

	for i, entrada := range cachePaginas {
		if i > 0 {
			statusMsg += ", "
		}

		// Convertir booleanos a 0/1
		usoInt := 0
		if entrada.Uso {
			usoInt = 1
		}

		modInt := 0
		if entrada.Modificada {
			modInt = 1
		}

		// Add arrow indicator if this is the entry the pointer points to
		if i == posicionPuntero {
			statusMsg += "> "
		}

		statusMsg += fmt.Sprintf("%d (U:%d, M:%d)", entrada.Pagina, usoInt, modInt)
	}
	statusMsg += "]"

	utils_general.LoggearMensaje("INFO", "%s", statusMsg)
}
