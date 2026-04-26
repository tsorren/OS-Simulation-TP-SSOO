package main

import (
	"net/http"
	"os"
	"strconv"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

// //******MAIN*********////
func main() {
	KernelConfig = comunicacion.IniciarConfiguracion[globals.Kernel_config]("kernel/kernel_config.json")
	utils_general.ConfigurarLogger("kernel.log", KernelConfig.Log_level, "kernel")
	if len(os.Args) < 3 {

		utils_general.LoggearMensaje("DEBUG", "ERROR - Reintentar ingresando el archivo y tamanio por parámetro")
		return
	}

	var archivo_pseudocodigo = os.Args[1]
	var tamanio_proceso = os.Args[2]

	if KernelConfig == nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo cargar la configuración")
		return
	}
	puerto := strconv.Itoa(KernelConfig.Port_kernel) // este es el puerto del kernel

	mux := http.NewServeMux()

	mux.HandleFunc("/nuevaConexionCPU", RecibirConexionCPU)
	mux.HandleFunc("/actualizarPC", RecibirActualizacionPC)
	mux.HandleFunc("/liberacionCPU", RecibirLiberacionCPU)
	mux.HandleFunc("/nuevaConexionIO", RecibirConexionIO)
	mux.HandleFunc("/ejecucionSyscall", RecibirSyscall)
	mux.HandleFunc("/finalizarIO", RecibirFinalizarIO)
	mux.HandleFunc("/desconexionIO", RecibirDesconexionIO)
	//mux.HandleFunc("/seLiberoEspacio", RecibirLiberacionEspacio)

	utils_general.LoggearMensaje("DEBUG", "Iniciando el módulo Kernel en localhost: %s", puerto)

	//inicia las 7 colas de estados
	init_queues()

	go start_kernel(archivo_pseudocodigo, tamanio_proceso)

	// Iniciar planificadores
	go planificadorColaReady()
	go planificadorColaExec()
	go planificadorIO()

	comunicacion.Escuchar(KernelConfig.Port_kernel, mux)
}
