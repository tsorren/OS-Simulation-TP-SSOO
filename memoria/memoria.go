package main

import (
	"net/http"
	"strconv"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

func main() {
	Memoria_config = comunicacion.IniciarConfiguracion[globals.Memoria_config]("memoria/memoria_config.json")

	if Memoria_config != nil {
		MEMORIAInfo = &globals.MemoriaInfo{
			TamanioPagina:     Memoria_config.Page_size,
			Niveles:           Memoria_config.Number_of_levels,
			CantEntradasTabla: Memoria_config.Entries_per_page,
			CantFrames:        Memoria_config.Memory_size / Memoria_config.Page_size,
		}
		utils_general.ConfigurarLogger("memoria.log", Memoria_config.Log_level, "memoria")
	} else {

		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo cargar la configuración")
		return
	}

	// Verificar que el archivo de swap exista si no se crea uno nuevo
	if err := InicializarSwapFile(Memoria_config.Swapfile_path); err != nil {
		return
	}

	puerto := strconv.Itoa(Memoria_config.Port_memory) // este es el puerto del kernel

	entradas := ""
	for i := range MEMORIAInfo.Niveles {
		str := strconv.Itoa(i)
		entradas += "/{e" + str + "}"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /infoBasica", RecibirConsultaInfoBasica)
	mux.HandleFunc("GET /proximaInstruccion/{pid}/{pc}", RecibirConsultaProximaInstruccion)
	mux.HandleFunc("POST /escribir", RecibirEscritura)
	mux.HandleFunc("GET /leer/{pid}/{dir}/{tamanio}", RecibirLectura)
	mux.HandleFunc("GET /marco/{pid}"+entradas, RecibirConsultaDeMarco)
	mux.HandleFunc("GET /leerPagina/{pid}/{dir}", RecibirLecturaDePagina)
	mux.HandleFunc("POST /actualizarPagina", RecibirActualizacionDePagina)
	mux.HandleFunc("GET /haySuficienteEspacio/{tamanio}", RecibirTamanio)
	mux.HandleFunc("POST /cargarProceso", RecibirProceso)
	mux.HandleFunc("POST /liberarEspacioAsociado/{pid}", RecibirLiberacionProceso)
	mux.HandleFunc("GET /swapearProceso/{pid}", RecibirSwapearProceso)
	mux.HandleFunc("GET /deswapearProceso/{pid}", RecibirDeswapearProceso)
	mux.HandleFunc("GET /dump/{pid}", RecibirDumpeoProceso)
	mux.HandleFunc("/finDeKernel", comunicacion.RecibirFinalizacionDeKernel)

	utils_general.LoggearMensaje("DEBUG", "Iniciando el módulo Memoria en localhost: %s", puerto)
	iniciarMemoria()

	comunicacion.Escuchar(Memoria_config.Port_memory, mux)
}
