package main

import (
	"math"
	"time"
	globals "tp-2025-1c-FSociety/utils"
	queues "tp-2025-1c-FSociety/utils/Queues"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

/*planifica el pasaje de procesos a la cola ready*/
func planificadorColaReady() {
	<-NotificarInicioPlanificadorLargoPlazo
	for {
		var proceso globals.Proceso
		hay_procesos := true
		estabaEnSwap := false
		select {
		case <-NotificarProcesoANewOSuspReady: // Recibir proceso de la cola NEW

		case <-notificarLiberacionEspacio: // Alguien notificó que puede haber espacio
			utils_general.LoggearMensaje("DEBUG", "Planificando por liberacion de espacio")
		}

		colaNEW.Sem_mutex.Lock()
		colaSuspReady.Sem_mutex.Lock()

		if colaNEW.IsEmpty() && colaSuspReady.IsEmpty() {
			colaNEW.Sem_mutex.Unlock()
			colaSuspReady.Sem_mutex.Unlock()
			continue
		}
		proceso, estabaEnSwap = sacarCopiaDeNewOSuspReady() //saca una copia de la cola NEW o SuspReady. Lo que antes era el switch con logica repetida

		colaSuspReady.Sem_mutex.Unlock()
		colaNEW.Sem_mutex.Unlock()

		hay_procesos = true // !colaNEW.IsEmpty_ts() || !colaSuspReady.IsEmpty_ts()

		utils_general.LoggearMensaje("DEBUG", "Planificando proceso %d, %d", proceso.Pcb.PID, proceso.Tamanio)
		var hay_espacio = preguntarSiHayEspacio(proceso.Tamanio)

		//este segundo bucle es para que vaya metiendo procesos mientras haya espacio
		for hay_espacio && hay_procesos {
			if estabaEnSwap {
				enviarDeswapearProceso(proceso.Pcb.PID)
			} else {
				paquete := globals.Paquete{
					Tipo:    "mensaje",
					Origen:  "kernel",
					Destino: "memoria",
					Payload: proceso,
				}
				utils_general.LoggearMensaje("DEBUG", "(%d) Cargando proceso en memoria", proceso.Pcb.PID)
				respuesta, err := comunicacion.PostYLeerRespuesta[globals.Mensaje](
					KernelConfig.Ip_memory, KernelConfig.Port_memory, "/cargarProceso", paquete)
				if err != nil || respuesta.Mensaje != "OK" {
					utils_general.LoggearMensaje("DEBUG", "Error al cargar proceso en memoria: %v", err)
					break
				}
			}

			PasarAEstado(globals.READY, proceso.Pcb.PID)
			NotificarNuevoProcesoEnReady <- struct{}{}

			hay_procesos = !colaNEW.IsEmpty_ts() || !colaSuspReady.IsEmpty_ts()
			if !hay_procesos {
				break
			}

			colaNEW.Sem_mutex.Lock()
			colaSuspReady.Sem_mutex.Lock()

			proceso, estabaEnSwap = sacarCopiaDeNewOSuspReady()

			colaSuspReady.Sem_mutex.Unlock()
			colaNEW.Sem_mutex.Unlock()

			hay_espacio = preguntarSiHayEspacio(proceso.Tamanio)

		}
	}
}

func sacarCopiaDeNewOSuspReady() (globals.Proceso, bool) {
	var proceso globals.Proceso
	var estabaEnSwap bool

	switch KernelConfig.Ready_ingress_algorithm {
	case "FIFO":
		if colaSuspReady.IsEmpty() {
			proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](colaNEW.Peek())
			estabaEnSwap = false
		} else {
			proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](colaSuspReady.Peek())
			estabaEnSwap = true
		}
	case "PMCP":
		if colaSuspReady.IsEmpty() {
			proceso = ObtenerProcesoMasPequenio(colaNEW)
			estabaEnSwap = false
		} else {
			proceso = ObtenerProcesoMasPequenio(colaSuspReady)
			estabaEnSwap = true
		}
	}

	return proceso, estabaEnSwap
}

/*planifica el pasaje de procesos a la cola exec*/
func planificadorColaExec() {
	switch KernelConfig.Scheduler_algorithm {
	case "FIFO":
		algoritmoFIFOColaExec()
	case "SJF":
		algoritmoSJFColaExec()
	case "SRT":
		algoritmoSRTColaExec()

	}

}

func algoritmoFIFOColaExec() {
	var proceso globals.Proceso
	var indiceCPU int
	var hayCPU bool
	for {
		select {
		case <-NotificarCPUDisponible:
		case <-NotificarNuevoProcesoEnReady:
		}
		// Cada vez que hay un cambio, despachar todos los procesos posibles
		for {
			if colaREADY.IsEmpty_ts() {
				utils_general.LoggearMensaje("DEBUG", "No hay procesos en READY para ejecutar")
				break
			}
			proceso, _ = comunicacion.DecodePayloadAs[globals.Proceso](colaREADY.Peek_ts())
			if proceso.Pcb == nil {
				utils_general.LoggearMensaje("DEBUG", "Proceso en READY sin PCB, saltando")
				break
			}
			indiceCPU, hayCPU = tomarCpuDisponible(proceso.Pcb.PID)
			if !hayCPU {
				break
			}
			PasarAEstado(globals.EXEC, proceso.Pcb.PID)
			mandarProcesoAEjecutar(proceso, indiceCPU)
		}
	}
}

func algoritmoSJFColaExec() {
	var proceso globals.Proceso
	var indiceCPU int
	var hayCPU bool
	var encontrado bool
	for {
		select {
		case <-NotificarCPUDisponible:
		case <-NotificarNuevoProcesoEnReady:
		}
		// Cada vez que hay un cambio, despachar todos los procesos posibles
		for {
			if colaREADY.IsEmpty_ts() {
				utils_general.LoggearMensaje("DEBUG", "No hay procesos en READY para ejecutar")
				break
			}

			proceso, _, encontrado = ObtenerProcesoConMenorEstimacion(colaREADY)
			if !encontrado {
				panic("No se encontró un proceso con menor estimación en la cola READY")
			}

			if proceso.Pcb == nil {
				utils_general.LoggearMensaje("DEBUG", "Proceso en READY sin PCB, saltando")
				break
			}
			indiceCPU, hayCPU = tomarCpuDisponible(proceso.Pcb.PID)
			if !hayCPU {
				break
			}
			PasarAEstado(globals.EXEC, proceso.Pcb.PID)
			mandarProcesoAEjecutar(proceso, indiceCPU)
		}
	}
}

func ObtenerProcesoConMenorEstimacion(queue *queues.Queue) (globals.Proceso, globals.EstimacionProceso, bool) {
	first := true
	var menorEstimacion globals.EstimacionProceso
	var PidProcesoConMenorEstimacion int
	var proceso globals.Proceso
	queue.Sem_mutex.Lock()
	defer queue.Sem_mutex.Unlock()
	mutex_estimaciones.Lock()
	for indice := range queue.Payload_items {
		proceso, _ := comunicacion.DecodePayloadAs[globals.Proceso](queue.Payload_items[indice])
		estimacion := estimaciones[proceso.Pcb.PID]
		if first {
			menorEstimacion = estimacion
			PidProcesoConMenorEstimacion = proceso.Pcb.PID
			first = false
		} else if estimacion.Estimacion < menorEstimacion.Estimacion {
			menorEstimacion = estimacion
			PidProcesoConMenorEstimacion = proceso.Pcb.PID
		}
	}
	mutex_estimaciones.Unlock()

	utils_general.LoggearMensaje("DEBUG", "PID con menor estimación: %d, Estimación: %f", PidProcesoConMenorEstimacion, menorEstimacion.Estimacion)

	proceso, encontrado := ObtenerProcesoPorPidDe(queue, PidProcesoConMenorEstimacion)

	utils_general.LoggearMensaje("DEBUG", "Proceso con menor estimación encontrado: %d, Encontrado: %t", proceso.Pcb.PID, encontrado)
	return proceso, menorEstimacion, encontrado
}

func algoritmoSRTColaExec() {
	var proceso globals.Proceso
	var menorEstimacion globals.EstimacionProceso
	var indiceCPU int
	var hayCPU bool
	var encontrado bool
	var huboDesalojo bool
	for {
		select {
		case <-NotificarCPUDisponible:
		case <-NotificarNuevoProcesoEnReady:
		}
		// Cada vez que hay un cambio, despachar todos los procesos posibles
		for {
			huboDesalojo = false
			if colaREADY.IsEmpty_ts() {
				utils_general.LoggearMensaje("DEBUG", "No hay procesos en READY para ejecutar")
				break
			}

			proceso, menorEstimacion, encontrado = ObtenerProcesoConMenorEstimacion(colaREADY)
			if !encontrado {
				panic("No se encontró un proceso con menor estimación en la cola READY")
			}

			if proceso.Pcb == nil {
				utils_general.LoggearMensaje("DEBUG", "Proceso en READY sin PCB, saltando")
				break
			}
			indiceCPU, hayCPU = tomarCpuDisponible(proceso.Pcb.PID)
			if !hayCPU {
				utils_general.LoggearMensaje("DEBUG", "## (%d) Intentando desalojar CPU por estimación: %f", proceso.Pcb.PID, menorEstimacion.Estimacion)
				indiceCPU, huboDesalojo = elegirCPUADesalojarPorEstimacionMenor(menorEstimacion.Estimacion)

				if !huboDesalojo {
					break
				}
				desalojarCPU(indiceCPU)
				mutexCpusConectadas.Lock()
				cpusConectadas[indiceCPU].Ejecutando = true
				cpusConectadas[indiceCPU].Pid_ejecutando = proceso.Pcb.PID
				mutexCpusConectadas.Unlock()
			}
			PasarAEstado(globals.EXEC, proceso.Pcb.PID)
			mandarProcesoAEjecutar(proceso, indiceCPU)
		}
	}
}

// selecciona la cpu que tiene el proceso con mayor tiempo por ejecutar aun
func elegirCPUADesalojarPorEstimacionMenor(menorEstimacion float64) (int, bool) {

	var restanteDeEjecucionMasGrande float64 = -1.0              //lo que le resta a un proceso para terminar de ejecutar (estimado - real ejecutado)
	var cpuDondeEstaElProcesoQueMasLeQueda globals.CPU_conectada //cpu donde está ejecutando el proceso que mas le queda por ejecutar
	var now = time.Now()                                         //para calcular el tiempo que le queda a un proceso
	var indiceCPU int
	//utils_general.LoggearMensaje("DEBUG", "Buscando CPU a desalojar por estimación menor: %f", menorEstimacion)

	mutexCpusConectadas.Lock()
	//este for obtiene la cpu que tiene el proceso con mayor tiempo restante por ejecutar (se puede abstraer a una funcion)
	for i, cpuConectada := range cpusConectadas {
		if !cpuConectada.Ejecutando { // Por si acaso
			continue
		}
		mutex_estimaciones.Lock()
		estimacion := estimaciones[cpuConectada.Pid_ejecutando].Estimacion
		realEjecutado := estimaciones[cpuConectada.Pid_ejecutando].RealEjecutado
		mutex_estimaciones.Unlock()
		tiempoEjecutado := realEjecutado + float64(now.Sub(cpuConectada.Inicio_ejecucion).Milliseconds())
		loQueLeQuedaPorEjecutar := math.Max(0, estimacion-tiempoEjecutado)

		utils_general.LoggearMensaje("DEBUG", "## PID %d: %s - Tiempo Restante: %.2f",
			cpuConectada.Pid_ejecutando, cpuConectada.Identificador, loQueLeQuedaPorEjecutar)

		if restanteDeEjecucionMasGrande < loQueLeQuedaPorEjecutar {
			restanteDeEjecucionMasGrande = loQueLeQuedaPorEjecutar
			cpuDondeEstaElProcesoQueMasLeQueda = cpuConectada
			indiceCPU = i
			utils_general.LoggearMensaje("DEBUG", "CPU %s es la que tiene el proceso con mayor tiempo restante: %f", cpuConectada.Identificador, restanteDeEjecucionMasGrande)

		}
	}
	mutexCpusConectadas.Unlock()
	if restanteDeEjecucionMasGrande >= 0 && menorEstimacion < restanteDeEjecucionMasGrande { // Esto explotaba porque menorEstimacion podia ser negativa
		utils_general.LoggearMensaje("DEBUG", "DESALOJO: CPU %s con PID %d tiene el proceso con mayor tiempo restante: %f",
			cpuDondeEstaElProcesoQueMasLeQueda.Identificador, cpuDondeEstaElProcesoQueMasLeQueda.Pid_ejecutando, restanteDeEjecucionMasGrande)

		return indiceCPU, true
	}

	utils_general.LoggearMensaje("DEBUG", "No se encontró una CPU con un proceso que le quede más tiempo por ejecutar")
	return -1, false //si no hay cpu con un proceso que le quede mas tiempo por ejecutar, no se desaloja a nadie
}

func planificadorIO() {
	var io globals.IO_conectada
	var nombreIO string
	var hayIO bool
	for {
		select {
		case nombreIO = <-NotificarLiberacionIO:
		case nombreIO = <-NotficarEncoladoIO:
		}

		utils_general.LoggearMensaje("DEBUG", "Planificador IO - Procesando IO: %s", nombreIO)
		mutexColaBloqueadosPorIOs.Lock()
		cola, existe := colaBloqueadosPorIOs[nombreIO]
		mutexColaBloqueadosPorIOs.Unlock()

		cola.Sem_mutex.Lock()

		if !existe || cola.IsEmpty() {
			cola.Sem_mutex.Unlock()
			continue
		}

		mutexIosConectadas.Lock()
		iosConEseNombre := ObtenerIOsPorNombre(nombreIO)
		mutexIosConectadas.Unlock()

		for {

			if cola.IsEmpty() {
				utils_general.LoggearMensaje("DEBUG", "Planificador IO - No hay instrucciones en cola %s", nombreIO)
				break
			}

			mutexIosConectadas.Lock()
			io, hayIO = TomarIOLibre(iosConEseNombre)
			mutexIosConectadas.Unlock()

			if !hayIO {
				utils_general.LoggearMensaje("DEBUG", "Planificador IO - No hay IO disponible para %s", nombreIO)
				break
			}

			instruccionIO, _ := comunicacion.DecodePayloadAs[globals.InstruccionIO](cola.Pop())

			pedirIO(io, instruccionIO)
		}
		cola.Sem_mutex.Unlock()
	}
}
