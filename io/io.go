package main

import (
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

var IOConfig *globals.IO_config
var ip string
var nombreDispositivo string

var mutexCierreIO sync.Mutex
var cierreIO bool = false // Variable para indicar si el IO está en proceso de cierre

func main() {
	IOConfig = comunicacion.IniciarConfiguracion[globals.IO_config]("io/io_config.json")
	utils_general.ConfigurarLogger(IOConfig.Log_id, IOConfig.Log_level, "io")
	if len(os.Args) < 2 {

		utils_general.LoggearMensaje("DEBUG", "ERROR - Reintentar ingresando nombre del dispositivo por parámetro")
		return
	}
	// var semaforo sync.RWMutex
	nombreDispositivo = os.Args[1]

	if IOConfig == nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo cargar la configuración")
	}
	puerto := strconv.Itoa(IOConfig.Port_io) // este es el puerto del kernel

	// Manejo de señales para avisar al kernel si el IO se cierra
	signal.Notify(signalFin, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)

	ip = comunicacion.GetLocalIP()

	IOInfo := globals.Paquete{
		Tipo:    "conexion",
		Origen:  "io",
		Destino: "kernel",
		Payload: globals.IO_conectada{
			Ip:     ip,
			Puerto: IOConfig.Port_io,
			Nombre: nombreDispositivo,
		},
	}

	utils_general.LoggearMensaje("DEBUG", "Puerto: %d", IOConfig.Port_io)
	comunicacion.EnviarPaquete(IOConfig.Ip_kernel, IOConfig.Port_kernel, "/nuevaConexionIO", IOInfo)

	mux := http.NewServeMux()
	mux.HandleFunc("/request", ProcesarRequest)                                       //ponemos semaforo para asegurarnos que cada proceso duerma lo que tiene que dormir
	mux.HandleFunc("POST /finalizacionDeProceso/{pid}", RecibirFinalizacionDeProceso) //ponemos semaforo para asegurarnos que cada proceso duerma lo que tiene que dormir
	mux.HandleFunc("/finDeKernel", comunicacion.RecibirFinalizacionDeKernel)

	utils_general.LoggearMensaje("DEBUG", "Iniciando el módulo IO en localhost: %s", puerto)
	utils_general.LoggearMensaje("INFO", "Esperando peticiones...")
	go manejarFinalizacion(ip, IOConfig.Port_io, nombreDispositivo)
	go Scheduler()
	comunicacion.Escuchar(IOConfig.Port_io, mux)

}
