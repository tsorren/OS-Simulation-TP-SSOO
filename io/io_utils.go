package main

import (
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

var instruccionesPorProcesar = make(chan globals.InstruccionIO, 200)

var procesosFinalizados = []int{}
var mutexProcesosFinalizados sync.Mutex
var signalFin = make(chan os.Signal, 1)

//preguntar como manejar lo del tamaño del buffer. Si ponemos un numero grande para que no llegue a bloquearse u otra cosa
//porque si le ponemos tamaño 1 y el sleep tarda mucho, se va a bloquear el procesador de requests eventualmente
//segun leí GO maneja cada request que llega en una gorutine, por lo que si el buffer se llena solo deberia bloquearse esa gorutine que esta procesando

//1. Cada cpu tiene que tener su propia cache? cada cpu tiene su cache, mmu y todo. Traduce direcciones por cada cpu
//2. El check de interrupciones? sjf con desalojo manda interrupcion kernel. La interrupcion va praa el proceso que esta ejecutando
//3.Preguntar si cuando cpu ejecuta syscall, le avisa a kernel y este le manda una interrupcion para que desaloje

func ProcesarRequest(w http.ResponseWriter, r *http.Request) {
	utils_general.LoggearMensaje("DEBUG", "recibi request")
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la conexion de la CPU")
		return
	}

	var instruccion, error = comunicacion.DecodePayloadAs[globals.InstruccionIO](paquete.Payload)
	if error != nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al ProcesarRequest: %s", error.Error())
		return
	}
	instruccionesPorProcesar <- instruccion

	w.WriteHeader(http.StatusOK)
}

func Scheduler() {
	var instruccion globals.InstruccionIO
	for {
		instruccion = <-instruccionesPorProcesar
		ejecutarInstruccion(instruccion)
	}
}

func ejecutarInstruccion(instruccion globals.InstruccionIO) {
	if EstaFinalizado(instruccion.PID) {
		return
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Inicio de IO - Tiempo: %d", instruccion.PID, instruccion.Tiempo)
	time.Sleep(time.Duration(instruccion.Tiempo) * time.Millisecond)
	mutexCierreIO.Lock()
	defer mutexCierreIO.Unlock()
	if cierreIO {
		utils_general.LoggearMensaje("DEBUG", "Murió la IO antes de terminar su instrucción")
		return
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Fin de IO", instruccion.PID)

	//TODO: enviar de vuelta a kernel para que replanifique:
	//si estaba bloqueado, lo manda a ready
	//si estaba susp bloqueado, lo mando a susp ready
	paquete := globals.Paquete{
		Tipo:    "fin_io",
		Origen:  "io",
		Destino: "kernel",
		Payload: globals.FinalizacionIO{
			PID:    instruccion.PID,
			Nombre: nombreDispositivo,
			IP:     ip,
			Puerto: IOConfig.Port_io,
		},
	}
	// contiene PID y tiempo
	// Cambia la ruta por la que definas en el Kernel
	comunicacion.EnviarPaquete(IOConfig.Ip_kernel, IOConfig.Port_kernel, "/finalizarIO", paquete)
	utils_general.LoggearMensaje("DEBUG", "envie paquete a kernel al finalizar io - PID: %d, Nombre: %s, IP: %s, Puerto: %d",
		instruccion.PID, nombreDispositivo, ip, IOConfig.Port_io)
}

// recibe el pid de un proceso que fue finalizado para que ya no ejecute la io si tenia alguna pendiente encolada
func RecibirFinalizacionDeProceso(w http.ResponseWriter, r *http.Request) {
	var pid, _ = strconv.Atoi(r.PathValue("pid"))
	mutexProcesosFinalizados.Lock()
	procesosFinalizados = append(procesosFinalizados, pid)
	mutexProcesosFinalizados.Unlock()
	utils_general.LoggearMensaje("DEBUG", "recibi finalizacion del proceso %d", pid)
}

func EstaFinalizado(pid int) bool {
	mutexProcesosFinalizados.Lock()
	defer mutexProcesosFinalizados.Unlock()
	return slices.Contains(procesosFinalizados, pid)
}

// Notifica al kernel que este IO se está cerrando y debe matar procesos afectados
func NotificarCierreIO(ip string, puerto int, nombre string) {
	paquete := globals.Paquete{
		Tipo:    "desconexion io",
		Origen:  "io",
		Destino: "kernel",
		Payload: globals.IO_conectada{
			Ip:     ip,
			Nombre: nombre,
			Puerto: puerto,
		},
	}
	comunicacion.EnviarPaquete(IOConfig.Ip_kernel, IOConfig.Port_kernel, "/desconexionIO", paquete)
	utils_general.LoggearMensaje("DEBUG", "Notifiqué al kernel el cierre del IO %s:%d", nombre, puerto)
}

// Manejar Finalización
func manejarFinalizacion(ip string, puerto int, nombreDispositivo string) {
	sig := <-signalFin
	mutexCierreIO.Lock()
	cierreIO = true
	mutexCierreIO.Unlock()
	utils_general.LoggearMensaje("DEBUG", "IO recibiendo señal de cierre (%s), notificando a kernel...", sig.String())

	// Notificar al kernel sobre el cierre del IO
	NotificarCierreIO(ip, puerto, nombreDispositivo)

	// Realizar limpieza adicional si es necesario
	utils_general.LoggearMensaje("DEBUG", "Finalizando módulo IO de manera controlada...")

	// Salir del programa con código de éxito
	os.Exit(0)
}
