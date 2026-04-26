package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	globals "tp-2025-1c-FSociety/utils"
	queues "tp-2025-1c-FSociety/utils/Queues"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

func RecibirConexionCPU(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la conexion de la CPU")
		return
	}

	var cpu_conexion, error = comunicacion.DecodePayloadAs[globals.CPU_conectada](paquete.Payload)
	if error != nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirConexionCPU: %s", error.Error())
		return
	}

	utils_general.LoggearMensaje("DEBUG", "handshake con CPU %d finalizado", len(cpusConectadas))
	mutexCpusConectadas.Lock()
	cpusConectadas = append(cpusConectadas, cpu_conexion)
	mutexCpusConectadas.Unlock()
	NotificarCPUDisponible <- struct{}{}

	w.WriteHeader(http.StatusOK)
}

func RecibirActualizacionPC(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir paquete de CPU")
		return
	}
	actualizacionPC, errorLiberacion := comunicacion.DecodePayloadAs[globals.ActualizacionPC](paquete.Payload)

	if errorLiberacion != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirActualizacionPC: %s", errorLiberacion.Error())
		return
	}

	ActualizarPC(actualizacionPC.PID, actualizacionPC.PC)

	w.WriteHeader(http.StatusOK)
}

func RecibirLiberacionCPU(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir paquete de CPU")
		return
	}
	liberacionCPU, errorLiberacion := comunicacion.DecodePayloadAs[globals.LiberacionCPU](paquete.Payload)

	if errorLiberacion != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirLiberacionCPU: %s", errorLiberacion.Error())
		return
	}

	utils_general.LoggearMensaje("DEBUG", "recibi liberacion de %s", liberacionCPU.Identificador)

	liberarCPU(liberacionCPU)

	w.WriteHeader(http.StatusOK)
}

func liberarCPU(lib globals.LiberacionCPU) {
	mutexCpusConectadas.Lock()
	now := time.Now()
	defer mutexCpusConectadas.Unlock()

	for i := range cpusConectadas {
		if cpusConectadas[i].Identificador == lib.Identificador {
			pid := cpusConectadas[i].Pid_ejecutando
			tiempoEjecucion := now.Sub(cpusConectadas[i].Inicio_ejecucion).Milliseconds()

			// Manejar la actualización de estimaciones si corresponde
			if algoritmoUsaEstimacion() {
				manejarEstimacionesProceso(pid, lib, float64(tiempoEjecucion), lib.Identificador)
			}

			// Liberar la CPU si el motivo es syscall
			if lib.Motivo == "syscall" {
				marcarCPULibre(&cpusConectadas[i])
			}

			break
		}
	}
}

// Verifica si el algoritmo de planificación es SJF o SRT
func algoritmoUsaEstimacion() bool {
	return KernelConfig.Scheduler_algorithm == "SJF" || KernelConfig.Scheduler_algorithm == "SRT"
}

// Actualiza las estimaciones del proceso según el motivo de liberación
func manejarEstimacionesProceso(pid int, lib globals.LiberacionCPU, tiempoEjecutado float64, identificadorCPU string) {
	mutex_estimaciones.Lock()
	defer mutex_estimaciones.Unlock()

	estimacionProceso, ok := estimaciones[pid]
	if !ok {
		return
	}

	estimacionProceso.RealEjecutado += tiempoEjecutado

	switch lib.Motivo {
	case "syscall":
		actualizarEstimacionProceso(&estimacionProceso, pid, identificadorCPU)
	case "desalojoSRT":
		manejarDesalojoSRT(pid)
	}
}

// Actualiza la estimación de un proceso usando la fórmula
func actualizarEstimacionProceso(estimacionProceso *globals.EstimacionProceso, pid int, identificadorCPU string) {
	alpha := KernelConfig.Alpha
	estimacionProceso.Estimacion = alpha*estimacionProceso.RealEjecutado + (1-alpha)*estimacionProceso.Estimacion
	estimaciones[pid] = *estimacionProceso

	utils_general.LoggearMensaje("DEBUG", "CPU %s, PID %d, Estimación actualizada: %f",
		identificadorCPU, pid, estimacionProceso.Estimacion)
}

// Maneja el caso de desalojo por SRT
func manejarDesalojoSRT(pid int) {
	PasarAEstado(globals.READY, pid)
	utils_general.LoggearMensaje("INFO", "## (%d) - Desalojado por algoritmo SJF/SRT", pid)
	NotificarNuevoProcesoEnReady <- struct{}{}
}

// Libera los recursos de la CPU
func marcarCPULibre(cpu *globals.CPU_conectada) {
	cpu.Ejecutando = false
	cpu.Pid_ejecutando = -1
	NotificarCPUDisponible <- struct{}{}
	utils_general.LoggearMensaje("DEBUG", "%s liberada", cpu.Identificador)
}

func ActualizarPC(pid int, pc int) {
	//mutexProcesos.Lock()
	//defer mutexProcesos.Unlock()
	utils_general.LoggearMensaje("DEBUG", "Actualizando PC de proceso %d a %d", pid, pc)
	colaExec.Sem_mutex.Lock()
	defer colaExec.Sem_mutex.Unlock()
	for i := range colaExec.Payload_items {
		var proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](colaExec.Payload_items[i])
		if proceso.Pcb.PID == pid {
			proceso.Pcb.PC = pc
			colaExec.Payload_items[i] = proceso
			utils_general.LoggearMensaje("DEBUG", "pc de proceso %d actualizado a %d", pid, pc)
		}
	}
}

func RecibirConexionIO(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la conexion de la CPU")
		return
	}

	var io_conexion, error = comunicacion.DecodePayloadAs[globals.IO_conectada](paquete.Payload)
	if error != nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirConexionIO: %s", error.Error())
		return
	}

	utils_general.LoggearMensaje("DEBUG", "handshake con IO %d finalizado, puerto %d", len(iosConectadas), io_conexion.Puerto)
	mutexIosConectadas.Lock()
	iosConectadas = append(iosConectadas, io_conexion)
	mutexIosConectadas.Unlock()

	mutexColaBloqueadosPorIOs.Lock()
	_, existe := colaBloqueadosPorIOs[io_conexion.Nombre] // si no existe la entrada en el map entonces lo crea
	if !existe {
		colaBloqueadosPorIOs[io_conexion.Nombre] = &queues.Queue{
			Payload_items: make([]interface{}, 0),
			Sem_mutex:     sync.Mutex{},
		}
	}
	mutexColaBloqueadosPorIOs.Unlock()
	w.WriteHeader(http.StatusOK)
	NotificarLiberacionIO <- io_conexion.Nombre
}

func preguntarSiHayEspacio(tamanio_proceso int) bool {
	tam := strconv.Itoa(tamanio_proceso)
	respuesta, _ := comunicacion.LeerDeServidor[globals.Mensaje](KernelConfig.Ip_memory, KernelConfig.Port_memory, "haySuficienteEspacio", tam)

	respuestaBooleana := respuesta.Mensaje == "OK"

	utils_general.LoggearMensaje("DEBUG", "Memoria tiene %d bytes libres? %t", tamanio_proceso, respuestaBooleana)
	return respuestaBooleana
}

func crearProceso(archivo_pseudocodigo string, tamanio_proceso int) {
	utils_general.LoggearMensaje("DEBUG", "Creando proceso, %s, %d", archivo_pseudocodigo, tamanio_proceso)

	mutexProcesosVivos.Lock()
	procesosVivos += 1
	mutexProcesosVivos.Unlock()

	proceso := globals.Proceso{Path: archivo_pseudocodigo, Tamanio: tamanio_proceso}
	var pcb = globals.PCB{Estado: globals.NEW}
	pcb.ConteoEstado = map[globals.Estado_proceso]int{
		globals.NEW:        0,
		globals.READY:      0,
		globals.EXEC:       0,
		globals.BLOQUEADO:  0,
		globals.SUSP_BLOQ:  0,
		globals.SUSP_READY: 0,
		globals.EXIT:       0}
	pcb.TiempoEstado = map[globals.Estado_proceso]time.Duration{
		globals.NEW:        0,
		globals.READY:      0,
		globals.EXEC:       0,
		globals.BLOQUEADO:  0,
		globals.SUSP_BLOQ:  0,
		globals.SUSP_READY: 0,
		globals.EXIT:       0}
	pcb.TimestampCambio = time.Now()
	proceso.Pcb = &pcb
	asignarPID(&proceso) // TODO Volver hacia atrás cuando se mata un proceso

	if algoritmoUsaEstimacion() {
		var estimacion = globals.EstimacionProceso{Estimacion: KernelConfig.Initial_estimate, RealEjecutado: 0}
		mutex_estimaciones.Lock()
		estimaciones[proceso.Pcb.PID] = estimacion
		mutex_estimaciones.Unlock()
	}

	colaNEW.Push_ts(proceso)

	utils_general.LoggearMensaje("INFO", "## (%d) Se crea el proceso - Estado: NEW", proceso.Pcb.PID)

	NotificarProcesoANewOSuspReady <- struct{}{}

}

func asignarPID(proceso *globals.Proceso) {
	mutexAsignarPID.Lock()
	proceso.Pcb.PID = pidCounter
	pidCounter = pidCounter + 1
	mutexAsignarPID.Unlock()
}

func start_kernel(archivo_pseudocodigo string, tamanio_proceso string) {
	utils_general.LoggearMensaje("INFO", "Iniciando el kernel...")
	utils_general.LoggearMensaje("INFO", "Planificador de Largo Plazo: Algoritmo %s", KernelConfig.Ready_ingress_algorithm)
	utils_general.LoggearMensaje("INFO", "Planificador de Corto Plazo: Algoritmo %s", KernelConfig.Scheduler_algorithm)
	if algoritmoUsaEstimacion() {
		utils_general.LoggearMensaje("INFO", "Planificador de Corto Plazo: Estimación inicial: %f - Alpha: %f", KernelConfig.Initial_estimate, KernelConfig.Alpha)
	}
	utils_general.LoggearMensaje("INFO", "Presione ENTER para iniciar la planificación de largo plazo...")
	os.Stdin.Read(make([]byte, 1))
	utils_general.LoggearMensaje("DEBUG", "Se recibió ENTER, se inicia el planificador de largo plazo")

	var tamanio_proceso_int, _ = strconv.Atoi(tamanio_proceso)

	crearProceso(archivo_pseudocodigo, tamanio_proceso_int)
	NotificarInicioPlanificadorLargoPlazo <- struct{}{}

	<-NotificarFinDeEjecucion
	stop_kernel()
}

func stop_kernel() {
	time.Sleep(1 * time.Second) // Esperar un segundo para asegurarse de que todos los procesos hayan terminado
	utils_general.LoggearMensaje("INFO", "Finalizando el kernel...")

	utils_general.LoggearMensaje("INFO", "Finalizando IOs...")
	mutexIosConectadas.Lock()
	for _, io := range iosConectadas {
		stop_module(io.Ip, io.Puerto)
	}
	mutexIosConectadas.Unlock()

	// Liberar recursos, cerrar conexiones, etc.
	utils_general.LoggearMensaje("INFO", "Finalizando CPUs...")
	mutexCpusConectadas.Lock()
	for _, cpu := range cpusConectadas {
		stop_module(cpu.Ip, cpu.Puerto)
	}
	mutexCpusConectadas.Unlock()

	utils_general.LoggearMensaje("INFO", "Finalizando Memoria")

	stop_module(KernelConfig.Ip_memory, KernelConfig.Port_memory)

	utils_general.LoggearMensaje("INFO", "Kernel finalizado correctamente")

	time.Sleep(1 * time.Second)
	os.Exit(0)
}

// Envía un mensaje de finalización a una CPU conectada
func stop_module(ip string, puerto int) {
	utils_general.LoggearMensaje("INFO", "Enviando mensaje de finalización al módulo en %s:%d", ip, puerto)
	paquete := globals.Paquete{
		Tipo:    "fin_kernel",
		Origen:  "kernel",
		Destino: "modulo",
		Payload: "fin",
	}
	comunicacion.EnviarPaquete(ip, puerto, "/finDeKernel", paquete)
}

func init_queues() {
	colaNEW.Payload_items = make([]interface{}, 0)
	colaREADY.Payload_items = make([]interface{}, 0)
	colaExec.Payload_items = make([]interface{}, 0)
	colaBlocked.Payload_items = make([]interface{}, 0)
	colaExit.Payload_items = make([]interface{}, 0)
	colaSuspBlocked.Payload_items = make([]interface{}, 0)
	colaSuspReady.Payload_items = make([]interface{}, 0)
}

func desalojarCPU(indice int) {
	mutexCpusConectadas.Lock()
	cpuConectada := cpusConectadas[indice]
	mutexCpusConectadas.Unlock()
	paquete := globals.Paquete{
		Tipo:    "interrupcion",
		Origen:  "kernel",
		Destino: "cpu",
		Payload: globals.Interrupt{Nombre: "desalojoSRT", PID: cpuConectada.Pid_ejecutando}, //si despues tenemos mas interrupciones las podemos clasificar por el nombre
	}

	utils_general.LoggearMensaje("DEBUG", "Enviando interrupción de desalojo a CPU %s", cpuConectada.Identificador)
	comunicacion.EnviarPaquete(cpuConectada.Ip, cpuConectada.Puerto, "/interrupt", paquete)
}

// esta funcion no te devuelve el proceso posta, sino que te devuelve una copia
func ObtenerProcesoPorPidDe(queue *queues.Queue, pid int) (proceso globals.Proceso, encontrado bool) {
	for _, item := range queue.Payload_items {
		var p, _ = comunicacion.DecodePayloadAs[globals.Proceso](item)
		if p.Pcb.PID == pid {
			proceso = p
			encontrado = true
			break
		}
	}
	return
}

// ******** END SCHEDULERS ******** //

// solo envia el paquete a la cpu para que lo ejecute
func mandarProcesoAEjecutar(proceso globals.Proceso, indiceCPU int) {
	paquete := globals.Paquete{
		Tipo:    "execute",
		Origen:  "kernel",
		Destino: "cpu",
		Payload: proceso,
	}
	mutexCpusConectadas.Lock()
	cpuConectada := &cpusConectadas[indiceCPU]
	cpuConectada.Inicio_ejecucion = time.Now()
	utils_general.LoggearMensaje("INFO", "## (%d) - Ejecutando en %s", proceso.Pcb.PID, cpuConectada.Identificador)
	comunicacion.EnviarPaquete(cpuConectada.Ip, cpuConectada.Puerto, "/procesoReady", paquete)
	mutexCpusConectadas.Unlock()

	if algoritmoUsaEstimacion() {
		mutex_estimaciones.Lock()
		estimacion := estimaciones[proceso.Pcb.PID].Estimacion
		estimaciones[proceso.Pcb.PID] = globals.EstimacionProceso{
			Estimacion:    estimacion,
			RealEjecutado: 0, // Reiniciar el tiempo real ejecutado al iniciar la ejecución
		}
		mutex_estimaciones.Unlock()
	}
}

// tomarCpuDisponible busca y devuelve la primera CPU disponible que encuentra.
//
// Retorna:
//   - el índice de la CPU conectada si está disponible
//     la pone como en ejecucion, el timer arranca cuando se le manda el proceso a ejecutar.

func tomarCpuDisponible(pid int) (int, bool) {
	mutexCpusConectadas.Lock()
	defer mutexCpusConectadas.Unlock()
	for i := range cpusConectadas {
		if !cpusConectadas[i].Ejecutando {
			// Esto debería ir en mandarProcesoAEjecutar?
			cpusConectadas[i].Ejecutando = true
			cpusConectadas[i].Pid_ejecutando = pid

			// Configurar la CPU para ejecución

			// Guardar la CPU seleccionada

			return i, true
		}
	}

	utils_general.LoggearMensaje("DEBUG", "No hay CPUs disponibles para ejecutar el proceso")
	return -1, false
}

func ObtenerProcesoMasPequenio(queue *queues.Queue) globals.Proceso {
	masChico, _ := comunicacion.DecodePayloadAs[globals.Proceso](queue.Peek())

	for _, item := range queue.Payload_items {
		var proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](item)
		if proceso.Tamanio < masChico.Tamanio {
			masChico = proceso
		}
	}
	return masChico
}

func RecibirSyscall(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la conexion de la CPU")
		return
	}

	var syscall, error = comunicacion.DecodePayloadAs[globals.Syscall](paquete.Payload)
	if error != nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al recibirSyscall: %s", error.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	utils_general.LoggearMensaje("INFO", "## (%d) - Solicitó syscall: %s", syscall.PID, syscall.KeyWord)
	utils_general.LoggearMensaje("DEBUG", "Parámetros de syscall: %s", strings.Join(syscall.Params, " "))
	EjecutarSyscall(syscall)

}

func EjecutarSyscall(syscall globals.Syscall) {
	switch syscall.KeyWord {
	case "IO":
		ejecutarSyscallIO(syscall.PID, syscall.Params[0], syscall.Params[1])

	case "INIT_PROC":
		var tamanio_proceso, _ = strconv.Atoi(syscall.Params[1])
		crearProceso(syscall.Params[0], tamanio_proceso)
		return

	case "DUMP_MEMORY":
		PasarAEstado(globals.BLOQUEADO, syscall.PID)
		//comunicacion.EnviarMensaje(KernelConfig.Ip_memory, KernelConfig.Port_memory, "/dumpMemory/{"+strconv.Itoa(pid)+"}")
		var finalizadoConError = pedirDumpMemory(syscall.PID)

		if finalizadoConError {
			matarProceso(syscall.PID)
		} else {
			// PasarAEstado(globals.READY, pid)
			// NotificarNuevoProcesoEnReady <- struct{}{}
			DesbloquearProceso(syscall.PID)
		}
		return

	case "EXIT":
		matarProceso(syscall.PID)
		return
	}

}

func ejecutarSyscallIO(pid int, nombre string, tiempoStr string) {
	// mutexSyscallIO.Lock()
	// defer mutexSyscallIO.Unlock()
	//la posicion 0 de params es el nombre de la IO y la 1 el tiempo

	mutexIosConectadas.Lock()
	var ios = ObtenerIOsPorNombre(nombre)

	if len(ios) <= 0 {
		mutexIosConectadas.Unlock()
		utils_general.LoggearMensaje("DEBUG", "No esta la IO %s conectada", nombre)
		matarProceso(pid)
		return
	}
	mutexIosConectadas.Unlock()

	PasarAEstado(globals.BLOQUEADO, pid)
	utils_general.LoggearMensaje("INFO", "## (%d) - Bloqueado por IO: %s", pid, nombre)

	tiempo, _ := strconv.Atoi(tiempoStr)
	instruccionIO := globals.InstruccionIO{PID: pid, Tiempo: tiempo}

	utils_general.LoggearMensaje("DEBUG", "Encolando instrucción IO para PID %d en IO %s", pid, nombre)
	// Encolar IO
	mutexColaBloqueadosPorIOs.Lock()
	cola, existe := colaBloqueadosPorIOs[nombre]
	mutexColaBloqueadosPorIOs.Unlock()
	if !existe {
		utils_general.LoggearMensaje("DEBUG", "IO %s desconectada durante syscall, matando proceso %d", nombre, pid)
		matarProceso(pid)
		return
	}
	cola.Push_ts(instruccionIO) // La cola se crea al conectar IO
	utils_general.LoggearMensaje("DEBUG", "Notificando encolado IO para PID %d en IO %s", pid, nombre)
	NotficarEncoladoIO <- nombre
	// TODO: Ver si ya funciona
}

// NOTA: Esta función asume que mutexIosConectadas ya está tomado
func TomarIOLibre(ios []*globals.IO_conectada) (globals.IO_conectada, bool) {
	for _, io := range ios {
		if !io.Ocupada {
			io.Ocupada = true
			return *io, true
		}
	}
	return globals.IO_conectada{}, false
}

func pedirIO(io globals.IO_conectada, instruccionIO globals.InstruccionIO) {
	utils_general.LoggearMensaje("DEBUG", "Pidiendo IO %s:%d para PID %d - %dms", io.Ip, io.Puerto, instruccionIO.PID, instruccionIO.Tiempo)
	mutexProcesosIO.Lock()
	// Asociar datos de IO al proceso antes de bloquearlo
	procIO := globals.ProcesoIO{
		PID:           instruccionIO.PID,
		DispositivoIO: io.Nombre,
		IpIO:          io.Ip,
		PuertoIO:      io.Puerto,
	}
	procesosIO = append(procesosIO, procIO)

	paquete := globals.Paquete{
		Tipo:    "instruccion io",
		Origen:  "kernel",
		Destino: "io",
		Payload: instruccionIO,
	}
	utils_general.LoggearMensaje("DEBUG", " (%d) Enviando instrucción IO a %s:%d - %dms", instruccionIO.PID, io.Ip, io.Puerto, instruccionIO.Tiempo)

	comunicacion.EnviarPaquete(io.Ip, io.Puerto, "/request", paquete)
	mutexProcesosIO.Unlock()
}

func pedirDumpMemory(pid int) bool {
	pidStr := strconv.Itoa(pid)
	respuesta, _ := comunicacion.LeerDeServidor[globals.Mensaje](KernelConfig.Ip_memory, KernelConfig.Port_memory, "dump", pidStr)

	respuestaBooleana, _ := strconv.ParseBool(respuesta.Mensaje)

	return respuestaBooleana
}

// NOTA: Esta función asume que mutexIosConectadas ya está tomado
func ObtenerIOsPorNombre(nombreBuscado string) []*globals.IO_conectada {
	ios := make([]*globals.IO_conectada, 0)
	for i := range iosConectadas {
		if iosConectadas[i].Nombre == nombreBuscado {
			ios = append(ios, &iosConectadas[i])
		}
	}
	return ios
}

func BuscarProcesoPorPID(pid int) (bool, globals.Proceso, *queues.Queue) {
	mutexMapaColas.Lock()
	defer mutexMapaColas.Unlock()
	// Siempre usar el mutex de procesos
	//mutexProcesos.Lock()
	//defer mutexProcesos.Unlock()

	// Recorrer todas las colas buscando el proceso
	for _, cola := range colas {
		// Siempre usar el mutex de la cola para evitar race conditions
		cola.Sem_mutex.Lock()

		// Buscar el proceso en la cola actual
		for _, item := range cola.Payload_items {
			var proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](item)
			if proceso.Pcb.PID == pid {
				// Desbloquear el mutex antes de retornar
				cola.Sem_mutex.Unlock()
				return true, proceso, cola
			}
		}

		// Desbloquear el mutex de la cola actual
		cola.Sem_mutex.Unlock()
	}

	// No se encontró el proceso
	return false, globals.Proceso{}, &queues.Queue{}
}

// ******** TIMER de SUSPENSION  ******** //
// Esta función inicia un timer que suspende el proceso después de un tiempo determinado
func iniciarTimerSuspension(pid int) {
	time.Sleep(time.Duration(KernelConfig.Suspension_time) * time.Millisecond)
	if estaEnColaBlocked(pid) {
		PasarAEstado(globals.SUSP_BLOQ, pid)
		// Notificar a memoria para swappear el proceso
		enviarSwapearProceso(pid)
		// Notificar replanificación
		utils_general.LoggearMensaje("DEBUG", "Proceso %d suspendido por tiempo de suspensión", pid)
		notificarLiberacionEspacio <- struct{}{}

	}
}

// Esta función verifica si un proceso está en la cola de bloqueados
// Retorna true si el proceso está en la cola de bloqueados, false en caso contrario

func estaEnColaBlocked(pid int) bool {
	colaBlocked.Sem_mutex.Lock() //utilizo el mutex para evitar condiciones de carrera
	defer colaBlocked.Sem_mutex.Unlock()
	for _, item := range colaBlocked.Payload_items {
		proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](item)
		if proceso.Pcb.PID == pid {
			return true
		}
	}
	return false
}

func enviarSwapearProceso(pid int) {
	comunicacion.EnviarMensaje(KernelConfig.Ip_memory, KernelConfig.Port_memory, "/swapearProceso/"+strconv.Itoa(pid))
	utils_general.LoggearMensaje("DEBUG", "Proceso %d enviado a swap", pid)
}

func enviarDeswapearProceso(pid int) {
	// Hace una petición a Memoria para que traiga el proceso de swap a memoria principal
	comunicacion.EnviarMensaje(KernelConfig.Ip_memory, KernelConfig.Port_memory, "/deswapearProceso/"+strconv.Itoa(pid))
	utils_general.LoggearMensaje("DEBUG", "Proceso %d des-swapeado", pid)
}

func registrarMetricas(proceso globals.Proceso) {
	duracion := time.Since(proceso.Pcb.TimestampCambio)
	proceso.Pcb.TiempoEstado[proceso.Pcb.Estado] += duracion
	proceso.Pcb.ConteoEstado[proceso.Pcb.Estado]++
	proceso.Pcb.TimestampCambio = time.Now()
}

func PasarAEstado(estado globals.Estado_proceso, pid int) {
	mutexPasarEstado.Lock()
	defer mutexPasarEstado.Unlock()

	encontrado, procesoInterface, cola := BuscarProcesoPorPID(pid)
	if !encontrado {
		panic("No se encontró el proceso con PID: " + strconv.Itoa(pid))
	}
	proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](procesoInterface)

	// Log de cambio de estado
	utils_general.LoggearMensaje("INFO", "## (%d) Pasa del estado %s al estado %s", pid, proceso.Pcb.Estado, estado)

	// Registrar métricas de estado SOLO si el estado cambia
	registrarMetricas(proceso)
	proceso.Pcb.Estado = estado

	eliminarProcesoDeCola(proceso.Pcb.PID, cola)

	mutexMapaColas.Lock()
	nuevaCola, ok := colas[estado]
	mutexMapaColas.Unlock()

	if !ok {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Estado no reconocido")
		return
	}

	nuevaCola.Push_ts(proceso)
	if estado == globals.BLOQUEADO {
		go iniciarTimerSuspension(proceso.Pcb.PID)
	}

}

func eliminarProcesoDeCola(pid int, cola *queues.Queue) {
	// El parámetro usarMutex es variádico y se puede pasar directamente como true o false

	// Siempre usar el mutex para evitar race conditions
	// La opción usarMutexValue=false debe usarse solo cuando ya se tiene el lock

	cola.Sem_mutex.Lock()
	defer cola.Sem_mutex.Unlock()

	// Hacer una copia segura antes de modificar
	newItems := make([]interface{}, 0, len(cola.Payload_items))
	encontrado := false

	for i := 0; i < len(cola.Payload_items); i++ {
		proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](cola.Payload_items[i])
		if proceso.Pcb.PID == pid {
			encontrado = true
			// No incluir este proceso en la copia
		} else {
			// Incluir todos los demás procesos
			newItems = append(newItems, cola.Payload_items[i])
		}
	}

	if encontrado {
		// Reemplazar el slice completo en lugar de modificar en el medio
		cola.Payload_items = newItems
	}
}

// funcion que mata un proceso
func loggearMetricasEstadosYPromedios(pid int, conteo map[globals.Estado_proceso]int, tiempo map[globals.Estado_proceso]time.Duration) {
	estados := []globals.Estado_proceso{
		globals.NEW, globals.READY, globals.EXEC, globals.BLOQUEADO, globals.SUSP_BLOQ, globals.SUSP_READY, globals.EXIT,
	}
	nombres := []string{"NEW", "READY", "EXEC", "BLOCKED", "SUSP_BLOCKED", "SUSP_READY", "EXIT"}

	// Métricas de estado
	msg := "## (%d) - Métricas de estado: "
	args := []interface{}{pid}
	for i, estado := range estados {
		c := conteo[estado]
		t := tiempo[estado].Milliseconds()
		msg += nombres[i] + " (%d) (%dms)"
		if i < len(estados)-1 {
			msg += ", "
		}
		args = append(args, c, t)
	}
	utils_general.LoggearMensaje("INFO", msg, args...)

	// Tiempos promedio
	msgProm := "## (%d) - Tiempo promedio: "
	argsProm := []interface{}{pid}
	for i, estado := range estados {
		c := conteo[estado]
		t := tiempo[estado].Milliseconds()
		promedio := int64(0)
		if c > 0 {
			promedio = t / int64(c)
		}
		msgProm += nombres[i] + " (%dms)"
		if i < len(estados)-1 {
			msgProm += ", "
		}
		argsProm = append(argsProm, promedio)
	}
	utils_general.LoggearMensaje("INFO", msgProm, argsProm...)
}

func matarProceso(pid int) {
	// Antes de liberar el PCB, registrar el tiempo final en el estado actual y loggear métricas
	utils_general.LoggearMensaje("DEBUG", "Inicio matar proceso %d ", pid)
	PasarAEstado(globals.EXIT, pid)
	encontrado, procesoInterface, _ := BuscarProcesoPorPID(pid)
	if !encontrado {
		return
	}

	proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](procesoInterface)

	utils_general.LoggearMensaje("INFO", "## (%d) - Finaliza el proceso", proceso.Pcb.PID)
	mutex_estimaciones.Lock()
	delete(estimaciones, proceso.Pcb.PID) // Eliminar la estimación del proceso
	mutex_estimaciones.Unlock()

	liberarMemoriaAsociadaA(proceso.Pcb.PID) // Ver esto

	avisarAIOsFinaliacionDe(proceso.Pcb.PID) // Log de fin de proceso

	registrarMetricas(proceso)

	loggearMetricasEstadosYPromedios(proceso.Pcb.PID, proceso.Pcb.ConteoEstado, proceso.Pcb.TiempoEstado)

	mutexProcesosVivos.Lock()
	procesosVivos -= 1
	if procesosVivos <= 0 {
		NotificarFinDeEjecucion <- struct{}{}
	}
	mutexProcesosVivos.Unlock()
}

// le hace una reqesta a memoria para liberar el espacio asociado al pid
func liberarMemoriaAsociadaA(pid int) {
	comunicacion.EnviarMensaje(KernelConfig.Ip_memory, KernelConfig.Port_memory, "liberarEspacioAsociado/"+strconv.Itoa(pid))
	notificarLiberacionEspacio <- struct{}{}
}

// le avisa a los dispositivos de E/S que el proceso fue finalizado. Para que no ejecuten las peticiones que tenia pendientes el finalizado
func avisarAIOsFinaliacionDe(pid int) {
	mutexIosConectadas.Lock()
	defer mutexIosConectadas.Unlock()
	for _, io := range iosConectadas {
		comunicacion.EnviarMensaje(io.Ip, io.Puerto, "finalizacionDeProceso/"+strconv.Itoa(pid))
	}
}

// RecibirFinalizarIO recibe la finalización de una instrucción de E/S y actualiza el estado del proceso

func RecibirFinalizarIO(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)

	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir finalizacion de intruccion IO")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var finalizacionIO, err = comunicacion.DecodePayloadAs[globals.FinalizacionIO](paquete.Payload)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar instruccion IO: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	utils_general.LoggearMensaje("DEBUG", "(%d) Kernel recibió finalización de IO", finalizacionIO.PID)

	var nombreIO string

	mutexIosConectadas.Lock()
	for i := range iosConectadas {
		if iosConectadas[i].Ip == finalizacionIO.IP && iosConectadas[i].Puerto == finalizacionIO.Puerto {
			iosConectadas[i].Ocupada = false
			nombreIO = iosConectadas[i].Nombre
			break
		}
	}
	mutexIosConectadas.Unlock()
	utils_general.LoggearMensaje("INFO", "## (%d) finalizó IO y pasa a READY", finalizacionIO.PID)
	// Eliminar el proceso de la lista de procesos en IO
	sacarProcesoIO(finalizacionIO.PID)

	DesbloquearProceso(finalizacionIO.PID)
	utils_general.LoggearMensaje("DEBUG", "Notificacion Liberacion IO (%s) enviada a Planificador IO", nombreIO)
	NotificarLiberacionIO <- nombreIO
	w.WriteHeader(http.StatusOK)
}

func DesbloquearProceso(pid int) {
	// Crear un mutex para esta función para evitar race conditions
	var mutexDesbloquearProceso sync.Mutex
	mutexDesbloquearProceso.Lock()
	defer mutexDesbloquearProceso.Unlock()

	var estadoActual globals.Estado_proceso

	// Buscar de forma thread-safe en qué cola está el proceso
	// Primero en colaBlocked
	colaBlocked.Sem_mutex.Lock()
	found := false
	for _, item := range colaBlocked.Payload_items {
		proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](item)
		if proceso.Pcb.PID == pid {
			estadoActual = proceso.Pcb.Estado
			found = true
			break
		}
	}
	colaBlocked.Sem_mutex.Unlock()

	// Si no lo encontramos, buscar en colaSuspBlocked
	if !found {
		colaSuspBlocked.Sem_mutex.Lock()
		for _, item := range colaSuspBlocked.Payload_items {
			proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](item)
			if proceso.Pcb.PID == pid {
				estadoActual = proceso.Pcb.Estado
				break
			}
		}
		colaSuspBlocked.Sem_mutex.Unlock()
	}

	switch estadoActual {
	case globals.BLOQUEADO:
		// Usar el mutex interno de PasarAEstado
		PasarAEstado(globals.READY, pid)
		NotificarNuevoProcesoEnReady <- struct{}{}

	case globals.SUSP_BLOQ:
		// Usar el mutex interno de PasarAEstado
		PasarAEstado(globals.SUSP_READY, pid)
		NotificarProcesoANewOSuspReady <- struct{}{}

	default:
		utils_general.LoggearMensaje("DEBUG", "Proceso %d no estaba en estado BLOQUEADO ni SUSP_BLOQ", pid)
	}
}

func RecibirDesconexionIO(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "Error al recibir DesconexionIO")
		return
	}
	var io, err = comunicacion.DecodePayloadAs[globals.IO_conectada](paquete.Payload)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "Error al decodificar IO_conectada en DesconexionIO")
		return
	}
	utils_general.LoggearMensaje("DEBUG", "IO caído: %s %s:%d", io.Nombre, io.Ip, io.Puerto)
	// Buscar y matar procesos bloqueados en ese IO

	// mutexDesconexionIO.Lock()
	// defer mutexDesconexionIO.Unlock()
	var pidsAMatar []int

	pid, encontro := BuscarPIDEnIO(io.Nombre, io.Ip, io.Puerto)
	if encontro {
		utils_general.LoggearMensaje("DEBUG", "Matando proceso %d por caída de IO %s", pid, io.Nombre)

		pidsAMatar = append(pidsAMatar, pid)
	}
	mutexIosConectadas.Lock()

	var instanciasIO int = 0
	var indiceIo int = -1
	for i := range iosConectadas {
		if iosConectadas[i].Nombre == io.Nombre {
			instanciasIO++
			if iosConectadas[i].Ip == io.Ip && iosConectadas[i].Puerto == io.Puerto {
				// Almacenar indice de IO para eliminar de la lista de IOs conectadas
				indiceIo = i
				instanciasIO--
			}
		}
	}

	utils_general.LoggearMensaje("DEBUG", "IO %s %s:%d eliminada de la lista de IOs conectadas", io.Nombre, io.Ip, io.Puerto)
	iosConectadas = append(iosConectadas[:indiceIo], iosConectadas[indiceIo+1:]...)
	mutexIosConectadas.Unlock()

	if instanciasIO == 0 {
		utils_general.LoggearMensaje("DEBUG", "Matando procesos encolados por caída de IO %s", io.Nombre)

		mutexColaBloqueadosPorIOs.Lock()
		cola := colaBloqueadosPorIOs[io.Nombre]
		mutexColaBloqueadosPorIOs.Unlock()

		cola.Sem_mutex.Lock()
		for _, item := range cola.Payload_items {
			instruccionIO, _ := comunicacion.DecodePayloadAs[globals.InstruccionIO](item)
			utils_general.LoggearMensaje("DEBUG", "Agregando proceso %d a cola %s para matar", instruccionIO.PID, io.Nombre)
			pidsAMatar = append(pidsAMatar, instruccionIO.PID)
		}
		cola.Sem_mutex.Unlock()

		mutexColaBloqueadosPorIOs.Lock()
		delete(colaBloqueadosPorIOs, io.Nombre) // Eliminar la cola de IOs
		mutexColaBloqueadosPorIOs.Unlock()
		utils_general.LoggearMensaje("DEBUG", "Cola de IO %s eliminada", io.Nombre)
	}

	for _, pidAMatar := range pidsAMatar {
		utils_general.LoggearMensaje("DEBUG", "Matando proceso %d por caída de IO %s", pidAMatar, io.Nombre)
		matarProceso(pidAMatar)
	}
	w.WriteHeader(http.StatusOK)
}

func BuscarPIDEnIO(nombre string, ip string, puerto int) (int, bool) {
	mutexProcesosIO.Lock()
	defer mutexProcesosIO.Unlock()
	for _, procesoIO := range procesosIO {
		if procesoIO.DispositivoIO == nombre && procesoIO.IpIO == ip && procesoIO.PuertoIO == puerto { // Solo considerar procesos que están usando el IO
			return procesoIO.PID, true
		}

	}
	return -1, false
}

func sacarProcesoIO(pid int) {
	mutexProcesosIO.Lock()
	defer mutexProcesosIO.Unlock()
	for i := 0; i < len(procesosIO); i++ {
		if procesosIO[i].PID == pid {
			procesosIO = append(procesosIO[:i], procesosIO[i+1:]...)
			break
		}
	}
	utils_general.LoggearMensaje("DEBUG", "Proceso %d eliminado de la lista de IOs", pid)
}
