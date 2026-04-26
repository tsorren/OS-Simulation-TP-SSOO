package main

import (
	"sync"
	globals "tp-2025-1c-FSociety/utils"
	queues "tp-2025-1c-FSociety/utils/Queues"
)

var mutexAsignarPID sync.Mutex
var mutexPasarEstado sync.Mutex
var mutexMapaColas sync.Mutex
var mutexProcesosVivos sync.Mutex

var cpusConectadas []globals.CPU_conectada // Slice para almacenar las conexiones de CPU
var mutexCpusConectadas sync.Mutex         // Mutex para proteger el acceso a cpusConectadas

var iosConectadas []globals.IO_conectada // Slice para almacenar las conexiones de IO
var mutexIosConectadas sync.Mutex        // Mutex para proteger el acceso a iosConectadas

var colaBloqueadosPorIOs = make(map[string]*queues.Queue) // Mapa para almacenar las colas de IO por nombre de dispositivo
var mutexColaBloqueadosPorIOs sync.Mutex                  // Mutex para proteger el acceso a colaBloqueadosPorIOs

var procesosIO []globals.ProcesoIO // Slice para almacenar los procesos que están en IO
var mutexProcesosIO sync.Mutex     // Mutex para proteger el acceso a procesosIO

var KernelConfig *globals.Kernel_config // Estructura para almacenar la configuración del kernel
var pidCounter int = 0                  // Variable para almacenar el PID global
var procesosVivos int = 0
var (
	colaNEW         = &queues.Queue{}
	colaREADY       = &queues.Queue{}
	colaExec        = &queues.Queue{}
	colaBlocked     = &queues.Queue{}
	colaExit        = &queues.Queue{}
	colaSuspReady   = &queues.Queue{}
	colaSuspBlocked = &queues.Queue{}
)

var (
	estimaciones       = make(map[int]globals.EstimacionProceso)
	mutex_estimaciones sync.Mutex
)

var colas = map[globals.Estado_proceso]*queues.Queue{
	globals.NEW:        colaNEW,
	globals.READY:      colaREADY,
	globals.EXEC:       colaExec,
	globals.BLOQUEADO:  colaBlocked,
	globals.EXIT:       colaExit,
	globals.SUSP_READY: colaSuspReady,
	globals.SUSP_BLOQ:  colaSuspBlocked,
}

var (
	NotificarProcesoANewOSuspReady        = make(chan struct{}, 100)
	NotificarCPUDisponible                = make(chan struct{}, 100)
	notificarLiberacionEspacio            = make(chan struct{}, 100)
	NotificarNuevoProcesoEnReady          = make(chan struct{}, 100)
	NotificarProcesoEnExit                = make(chan globals.Proceso, 100)
	NotificarLiberacionIO                 = make(chan string, 100)
	NotficarEncoladoIO                    = make(chan string, 100)
	NotificarInicioPlanificadorLargoPlazo = make(chan struct{}, 1)
	NotificarFinDeEjecucion               = make(chan struct{}, 1)
)

// var ioRegistry = struct {
// 	sync.RWMutex
// 	devices map[string]*ioDevice
// }{devices: make(map[string]*ioDevice)}
