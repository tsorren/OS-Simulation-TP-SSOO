package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

var waitRutinaEscuchar sync.WaitGroup

func escucharCPU(port int, mux *http.ServeMux) {
	err := http.ListenAndServe(":"+strconv.Itoa(port), mux)
	if err != nil {
		panic(err)
	}
	waitRutinaEscuchar.Done()
}
func main() {
	CPUConfig = comunicacion.IniciarConfiguracion[globals.CPU_config]("cpu/cpu_config.json")

	utils_general.ConfigurarLogger(CPUConfig.Log_id, CPUConfig.Log_level, "cpu")
	if CPUConfig == nil {

		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo cargar la configuración")
	}
	if len(os.Args) < 2 {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Reintentar ingresando el identificador de la CPU por parámetro")
		return
	}
	identificador = os.Args[1]
	//CPUConfig.Port_cpu, _ = strconv.Atoi(os.Args[2])

	// Inicializar estado del CPU
	// cpuState = &CPUState{
	// 	PID:     1, // TODO: Obtener PID real del kernel
	// 	PC:      0,
	// 	Running: true,
	// }

	// Configurar servidor HTTP
	cpuInfo := globals.Paquete{
		Tipo:    "conexion",
		Origen:  "cpu",
		Destino: "kernel",
		Payload: globals.CPU_conectada{
			Ip:            comunicacion.GetLocalIP(),
			Puerto:        CPUConfig.Port_cpu,
			Identificador: identificador,
			Ejecutando:    false,
		},
	}

	initCPU()

	mux := http.NewServeMux()

	mux.HandleFunc("/procesoReady", procesoReadyHandler)
	mux.HandleFunc("/interrupt", recibirInterrupcionHandler)
	mux.HandleFunc("/finDeKernel", comunicacion.RecibirFinalizacionDeKernel)

	utils_general.LoggearMensaje("DEBUG", "Iniciando el módulo CPU en localhost: %d", CPUConfig.Port_cpu)

	waitRutinaEscuchar.Add(1)
	go escucharCPU(CPUConfig.Port_cpu, mux)
	go cpuCycle()

	comunicacion.EnviarPaquete(CPUConfig.Ip_kernel, CPUConfig.Port_kernel, "/nuevaConexionCPU", cpuInfo)
	// Iniciar ciclo del CPU en una goroutine
	waitRutinaEscuchar.Wait()
}
