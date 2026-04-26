package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	globals "tp-2025-1c-FSociety/utils"
	"tp-2025-1c-FSociety/utils/comunicacion"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

// ============================================================================
// VARIABLES GLOBALES Y SEMÁFOROS
// ============================================================================

var (
	mutexMemoria     sync.Mutex
	mutexTablaFrames sync.Mutex
	mutexProcesos    sync.Mutex
	mutexTablaSwap   sync.Mutex
	mutexSwapOffset  sync.Mutex

	MEMORIAInfo    *globals.MemoriaInfo
	Memoria_config *globals.Memoria_config
	MemoriaFisica  []byte
	TablaFrames    []globals.Frame
	TablaSwap      []*globals.EntradaSwap
	Procesos       map[int]*globals.ProcesoMem
	nextSwapOffset int
)

const TAM_HEADER int = 16

// ============================================================================
// INICIALIZACIÓN DE MEMORIA
// ============================================================================

// Semáforos: mutexMemoria, mutexProcesos, mutexTablaFrames, mutexSwapfile (en InicializarSwapfile)
func iniciarMemoria() {
	mutexMemoria.Lock()
	MemoriaFisica = make([]byte, Memoria_config.Memory_size)
	mutexMemoria.Unlock()

	mutexProcesos.Lock()
	Procesos = make(map[int]*globals.ProcesoMem)
	mutexProcesos.Unlock()

	mutexTablaFrames.Lock()
	TablaFrames = make([]globals.Frame, MEMORIAInfo.CantFrames)
	for i := range MEMORIAInfo.CantFrames {
		TablaFrames[i] = globals.Frame{
			ID:      i,
			Inicio:  i * MEMORIAInfo.TamanioPagina,
			Ocupado: false,
		}
	}
	mutexTablaFrames.Unlock()

	InicializarSwapFile(Memoria_config.Swapfile_path)

	utils_general.LoggearMensaje("INFO", "Memoria iniciada")
	utils_general.LoggearMensaje("INFO", "Tamaño Físico: %d bytes, Tamaño de Página: %d, Frames: %d",
		Memoria_config.Memory_size, MEMORIAInfo.TamanioPagina, MEMORIAInfo.CantFrames)
	utils_general.LoggearMensaje("INFO", "Niveles de Paginación: %d, Entradas por Tabla: %d",
		MEMORIAInfo.Niveles, MEMORIAInfo.CantEntradasTabla)
	utils_general.LoggearMensaje("INFO", "Directorio de Instrucciones: %s", Memoria_config.Scripts_path)
	utils_general.LoggearMensaje("INFO", "Directorio de Dump: %s", Memoria_config.Dump_path)
	utils_general.LoggearMensaje("INFO", "Archivo de Swap: %s", Memoria_config.Swapfile_path)
	utils_general.LoggearMensaje("INFO", "Esperando peticiones...")
}

// Inicializa el archivo de swap si no existe (solo una vez al inicio)
func InicializarSwapFile(path string) error {
	// mutexSwapfile.Lock()
	// defer mutexSwapfile.Unlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo crear el archivo de swap: %s", err.Error())
		return err
	}
	utils_general.LoggearMensaje("DEBUG", "Archivo swap creado en ruta %s", path)
	return file.Close()
}

// ============================================================================
// FUNCIONES AUXILIARES
// ============================================================================

// Semáforos: ninguno
func retardoMemoria() {
	time.Sleep(time.Duration(Memoria_config.Memory_delay) * time.Millisecond)
}

// Semáforos: ninguno
func retardoSwapeo() {
	time.Sleep(time.Duration(Memoria_config.Swap_delay) * time.Millisecond)
}

// Semáforos: ninguno
func calcularCantPaginas(tamanio int) int {
	return int(math.Ceil(float64(tamanio) / float64(MEMORIAInfo.TamanioPagina)))
}

// Semáforos: ninguno
func calcularEntrada(nPag int, nivel int) int {
	exp := float64(MEMORIAInfo.Niveles - nivel)
	divisor := int(math.Pow(float64(MEMORIAInfo.CantEntradasTabla), exp))
	return (nPag / divisor) % MEMORIAInfo.CantEntradasTabla
}

// Semáforos: ninguno
func calcularIndices(nPag int) []int {
	indices := make([]int, MEMORIAInfo.Niveles)
	for nivel := 1; nivel <= MEMORIAInfo.Niveles; nivel++ {
		indices[nivel-1] = calcularEntrada(nPag, nivel)
	}
	return indices
}

// ============================================================================
// GESTIÓN DE MEMORIA Y FRAMES
// ============================================================================

// Semáforos: mutexTablaFrames (en hayFramesLibres)
func verificarSihayEspacio(tamanio int) bool {
	return hayFramesLibres(calcularCantPaginas(tamanio))
}

// Semáforos: mutexTablaFrames
func hayFramesLibres(cantPedida int) bool {
	cantLibres := 0
	mutexTablaFrames.Lock()
	for i := range MEMORIAInfo.CantFrames {
		if !TablaFrames[i].Ocupado {
			cantLibres++
		}
	}
	mutexTablaFrames.Unlock()
	utils_general.LoggearMensaje("DEBUG", "Memoria tiene %d frames libres de %d pedidos", cantLibres, cantPedida)
	return cantLibres >= cantPedida
}

// Semáforos: mutexTablaFrames, mutexMemoria
func asignarFrame() int {
	mutexTablaFrames.Lock()
	defer mutexTablaFrames.Unlock()

	for i := range MEMORIAInfo.CantFrames {
		frameActual := &TablaFrames[i]
		if !frameActual.Ocupado {
			frameActual.Ocupado = true

			// Verificar que el frame esté vacío antes de asignarlo
			inicio := frameActual.ID * MEMORIAInfo.TamanioPagina
			fin := inicio + MEMORIAInfo.TamanioPagina
			mutexMemoria.Lock()
			for j := inicio; j < fin; j++ {
				if MemoriaFisica[j] != 0 {
					mutexMemoria.Unlock()
					panic(fmt.Sprintf("ERROR: Frame %d no está vacío pero se marcó como libre", frameActual.ID))
				}
			}
			mutexMemoria.Unlock()

			utils_general.LoggearMensaje("DEBUG", "Asignando frame %d", frameActual.ID)
			return frameActual.ID
		}
	}
	return -1
}

// Semáforos: mutexMemoria
func actualizarFrame(frame int, datos []byte) {
	inicio := frame * MEMORIAInfo.TamanioPagina
	fin := inicio + MEMORIAInfo.TamanioPagina
	if inicio < 0 || fin > Memoria_config.Memory_size {
		panic("rango de memoria física inválido")
	}
	mutexMemoria.Lock()
	copy(MemoriaFisica[inicio:fin], datos)
	mutexMemoria.Unlock()
}

// ============================================================================
// GESTION DE TABLAS DE PAGINAS
// ============================================================================

// Semáforos: mutexTablaFrames (en asignarFrame)
func crearTablaPaginaDesde(nivel int, paginasPorAsignar *int) *globals.TablaPaginacion {
	entradas := make([]globals.EntradaTabla, MEMORIAInfo.CantEntradasTabla)
	for i := range MEMORIAInfo.CantEntradasTabla {
		entrada := globals.EntradaTabla{
			Subtabla: nil,
			Frame:    -1,
			Presente: false,
		}

		if nivel < MEMORIAInfo.Niveles {
			entrada.Subtabla = crearTablaPaginaDesde(nivel+1, paginasPorAsignar)
		} else if *paginasPorAsignar > 0 {
			entrada.Frame = asignarFrame()
			entrada.Presente = true
			*paginasPorAsignar--
		}
		entradas[i] = entrada
	}
	tabla := globals.TablaPaginacion{
		Nivel:    nivel,
		Entradas: entradas,
	}
	return &tabla
}

// Semáforos: ninguno (NO TOMA proc.Mutex)
func obtenerEntradaDeProceso(proc *globals.ProcesoMem, indices []int, conMetricas bool) (*globals.EntradaTabla, error) {
	tablaActual := proc.TablaRaiz
	for nivel := 1; nivel <= MEMORIAInfo.Niveles; nivel++ {
		indice := indices[nivel-1]
		if indice < 0 || indice >= len(tablaActual.Entradas) {
			return nil, fmt.Errorf("índice fuera de rango en nivel %d", nivel)
		}

		entrada := &tablaActual.Entradas[indice]

		if conMetricas {
			retardoMemoria()
			proc.Metricas.AccesosTablasPaginas++
		}

		if nivel < MEMORIAInfo.Niveles {
			if entrada.Subtabla == nil {
				return nil, fmt.Errorf("subtabla inexistente en nivel %d", nivel)
			}
			tablaActual = entrada.Subtabla
		} else {
			if !entrada.Presente {
				return entrada, fmt.Errorf("página no presente en nivel %d", nivel)
			}
			return entrada, nil
		}
	}
	return nil, fmt.Errorf("no se encontró la entrada")
}

// Semáforos: proc.Mutex
func obtenerFrameDeProceso(proc *globals.ProcesoMem, indices []int, conMetricas bool) (int, error) {
	proc.Mutex.Lock()
	defer proc.Mutex.Unlock()
	entrada, err := obtenerEntradaDeProceso(proc, indices, conMetricas)
	if err != nil {
		return -1, fmt.Errorf("no se encontró el frame")
	}
	return entrada.Frame, nil
}

// ============================================================================
// GESTIÓN DE PROCESOS
// ============================================================================

// Semáforos: ninguno
func inicializaProceso(proceso globals.Proceso, instrucciones []string) *globals.ProcesoMem {
	return &globals.ProcesoMem{
		PID:           proceso.Pcb.PID,
		Tamanio:       proceso.Tamanio,
		CantPaginas:   calcularCantPaginas(proceso.Tamanio),
		Instrucciones: instrucciones,
		TablaRaiz:     nil,
		Metricas: globals.MetricasProceso{
			AccesosTablasPaginas:     0,
			InstruccionesSolicitadas: 0,
			BajadasASwap:             0,
			SubidasAMemoria:          0,
			LecturasMemoria:          0,
			EscriturasMemoria:        0,
		},
	}
}

// Semáforos: mutexProcesos
func guardarNuevoProceso(proc *globals.ProcesoMem) {
	mutexProcesos.Lock()
	Procesos[proc.PID] = proc
	mutexProcesos.Unlock()
}

// Semáforos: mutexProcesos, proceso.Mutex
func proximaInstruccion(PID int, PC int) string {
	mutexProcesos.Lock()
	proceso, existe := Procesos[PID]
	mutexProcesos.Unlock()

	if !existe || PC < 0 {
		panic(fmt.Sprintf("ERROR - Proceso con PID %d no existe o PC %d fuera de rango", PID, PC))
	}

	proceso.Mutex.Lock()
	instruccion := proceso.Instrucciones[PC]
	proceso.Metricas.InstruccionesSolicitadas++
	proceso.Mutex.Unlock()

	return instruccion
}

// Liberar todos los frames físicos usados por el proceso
// Semáforos: proceso.Mutex, mutexTablaFrames, mutexMemoria
func liberarFramesDeProceso(proceso *globals.ProcesoMem) {
	utils_general.LoggearMensaje("DEBUG", "Liberando frames del proceso %d", proceso.PID)
	proceso.Mutex.Lock()

	var frames []int
	for nPag := range proceso.CantPaginas {
		indices := calcularIndices(nPag)
		entrada, err := obtenerEntradaDeProceso(proceso, indices, false)

		if err == nil && entrada.Presente {
			entrada.Presente = false
			frames = append(frames, entrada.Frame)
		}
	}
	proceso.Mutex.Unlock()

	cleanBuf := make([]byte, MEMORIAInfo.TamanioPagina)
	mutexTablaFrames.Lock()
	mutexMemoria.Lock()
	for _, frame := range frames {
		TablaFrames[frame].Ocupado = false
		inicio := frame * MEMORIAInfo.TamanioPagina
		fin := inicio + MEMORIAInfo.TamanioPagina
		copy(MemoriaFisica[inicio:fin], cleanBuf)
	}
	mutexMemoria.Unlock()
	mutexTablaFrames.Unlock()
	utils_general.LoggearMensaje("DEBUG", "%d frames liberados del proceso %d", len(frames), proceso.PID)
}

// Semáforos: mutexProcesos, proceso.Mutex, mutexTablaFrames, mutexMemoria, mutexTablaSwap (en LiberarSwapDeProceso)
func liberarEspacioAsociado(pid int) {
	mutexProcesos.Lock()
	proceso, existe := Procesos[pid]
	mutexProcesos.Unlock()
	if !existe {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se encontró el proceso con PID %d", pid)
		return
	}

	LiberarSwapDeProceso(pid)
	liberarFramesDeProceso(proceso)

	mutexProcesos.Lock()
	utils_general.LoggearMensaje("INFO",
		"## PID: %d - Proceso Destruido - Métricas - Acc.T.Pag: %d; Inst.Sol.: %d; SWAP: %d; Mem.Prin.: %d; Lec.Mem.: %d; Esc.Mem.: %d",
		pid,
		proceso.Metricas.AccesosTablasPaginas,
		proceso.Metricas.InstruccionesSolicitadas,
		proceso.Metricas.BajadasASwap,
		proceso.Metricas.SubidasAMemoria,
		proceso.Metricas.LecturasMemoria,
		proceso.Metricas.EscriturasMemoria,
	)
	delete(Procesos, pid)
	mutexProcesos.Unlock()

	utils_general.LoggearMensaje("DEBUG", "Proceso %d: liberación completa de frames y estructura de tablas", pid)
}

// ============================================================================
// OPERACIONES EN MEMORIA
// ============================================================================

// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria
func escribir(pid int, dirFisica int, dato []byte) bool {
	mutexProcesos.Lock()
	proceso, existe := Procesos[pid]
	mutexProcesos.Unlock()
	if !existe {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Proceso con PID %d no existe al intentar escribir en memoria", pid)
		return false
	}
	tamanio := len(dato)

	if dirFisica < 0 || dirFisica+tamanio > Memoria_config.Memory_size {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Intento de escritura fuera de los límites de memoria: dir=%d, tamaño=%d", dirFisica, tamanio)
		return false
	}
	mutexMemoria.Lock()
	copy(MemoriaFisica[dirFisica:dirFisica+tamanio], dato)
	mutexMemoria.Unlock()

	utils_general.LoggearMensaje("INFO", "## PID: %d - Escritura - Dir. Física: %d - Tamaño: %d", pid, dirFisica, tamanio)

	contenidoEscrito := comunicacion.LeerDatosBinarios(dato)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Contenido Escritura: %s", pid, contenidoEscrito)

	proceso.Mutex.Lock()
	proceso.Metricas.EscriturasMemoria++
	proceso.Mutex.Unlock()

	return true
}

// Semáforos: mutexMemoria, mutexProcesos, proceso.Mutex
func leer(pid int, dirFisica int, tamanio int) []byte {
	if dirFisica < 0 || dirFisica >= Memoria_config.Memory_size {
		panic(fmt.Sprintf("Dirección inválida de lectura: %d", dirFisica))
	}

	final := min(dirFisica+tamanio, Memoria_config.Memory_size)

	mutexMemoria.Lock()
	datos := MemoriaFisica[dirFisica:final]
	mutexMemoria.Unlock()

	utils_general.LoggearMensaje("INFO", "## PID: %d - Lectura - Dir. Física: %d - Tamaño: %d", pid, dirFisica, tamanio)

	contenidoLeido := comunicacion.LeerDatosBinarios(datos)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Contenido Lectura: %s", pid, contenidoLeido)

	mutexProcesos.Lock()
	proceso, existe := Procesos[pid]
	mutexProcesos.Unlock()
	if !existe {
		panic(fmt.Sprintf("Proceso con PID %d no existe al intentar leer en memoria", pid))
	}
	proceso.Mutex.Lock()
	proceso.Metricas.LecturasMemoria++
	proceso.Mutex.Unlock()

	return datos
}

// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria (en leer)
func leerFrame(pid int, direccion int) []byte {
	return leer(pid, direccion, Memoria_config.Page_size)
}

// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria
func realizarDumpMemory(pid int) bool {
	mutexProcesos.Lock()
	proc, ok := Procesos[pid]
	mutexProcesos.Unlock()
	if !ok {
		utils_general.LoggearMensaje("DEBUG", "ERROR - PID: %d - Memory Dump fallido: proceso no existe", pid)
		return true
	}

	utils_general.LoggearMensaje("INFO", "## PID: %d - Memory Dump solicitado", pid)
	proc.Mutex.Lock()
	defer proc.Mutex.Unlock()

	dumpPath := Memoria_config.Dump_path
	os.MkdirAll(dumpPath, 0755)
	timestamp := time.Now().Format("20060102-150405")
	fileName := filepath.Join(dumpPath, fmt.Sprintf("%d-%s.dmp", pid, timestamp))
	file, err := os.Create(fileName)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - PID: %d - Memory Dump fallido: %s", pid, err.Error())
		return true
	}
	defer file.Close()
	utils_general.LoggearMensaje("DEBUG", "Archivo dump creado en %s", fileName)

	cantPaginas := proc.CantPaginas

	for pagina := 0; pagina < cantPaginas; pagina++ {
		indices := calcularIndices(pagina)
		entradaProceso, err := obtenerEntradaDeProceso(proc, indices, false)

		if err != nil || !entradaProceso.Presente {
			utils_general.LoggearMensaje("DEBUG", "ERROR - PID: %d - Memory Dump: frame no encontrado para página %d", pid, pagina)
			os.Remove(fileName)
			utils_general.LoggearMensaje("DEBUG", "Archivo dump eliminado por error: %s", fileName)
			return true
		}
		frame := entradaProceso.Frame
		inicio := frame * MEMORIAInfo.TamanioPagina
		fin := inicio + MEMORIAInfo.TamanioPagina
		if inicio >= Memoria_config.Memory_size || fin > Memoria_config.Memory_size {
			utils_general.LoggearMensaje("DEBUG", "ERROR - PID: %d - Memory Dump: rango de memoria inválido", pid)
			os.Remove(fileName)
			utils_general.LoggearMensaje("DEBUG", "Archivo dump eliminado por error: %s", fileName)
			return true
		}

		mutexMemoria.Lock()
		datos := MemoriaFisica[inicio:fin]
		mutexMemoria.Unlock()

		_, err = file.Write(datos)
		if err != nil {
			utils_general.LoggearMensaje("DEBUG", "ERROR - PID: %d - Memory Dump: error al escribir archivo: %s", pid, err.Error())
			os.Remove(fileName)
			utils_general.LoggearMensaje("DEBUG", "Archivo dump eliminado por error: %s", fileName)
			return true
		}
	}

	return false
}

// ============================================================================
// FUNCIONES DE SWAP
// ============================================================================

// Registra entrada de swap para un proceso (se llama al crear el proceso)
// Semáforos: mutexTablaSwap
func registrarEntradaSwap(pidInt int, cantPaginas int) *globals.EntradaSwap {
	mutexTablaSwap.Lock()
	defer mutexTablaSwap.Unlock()

	tamanio := cantPaginas*MEMORIAInfo.TamanioPagina + TAM_HEADER // Header adicional para "P: [PID]"
	mutexSwapOffset.Lock()
	nuevaEntrada := &globals.EntradaSwap{
		PID:      pidInt,
		Offset:   nextSwapOffset,
		Presente: false,
		Tamanio:  tamanio,
	}

	if tamanio > 0 {
		nextSwapOffset += tamanio + TAM_HEADER // Actualizar offset para la próxima entrada
	}
	mutexSwapOffset.Unlock()

	TablaSwap = append(TablaSwap, nuevaEntrada)

	utils_general.LoggearMensaje("INFO", "## PID: %d - Entrada swap registrada - Offset: %d - Tamaño con header: %d", pidInt, nuevaEntrada.Offset, TAM_HEADER+tamanio)
	return nuevaEntrada
}

// NO registra métricas ya que también se usa para borrar de swapfile
// Semáforos: ninguno
func EscribirEnSwap(offset int, datos []byte) error {
	file, err := os.OpenFile(Memoria_config.Swapfile_path, os.O_RDWR, 0666)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo abrir el archivo de swap para escribir: %s", err.Error())
		return err
	}
	defer file.Close()

	_, err = file.Seek(int64(offset), 0)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo hacer seek en el archivo de swap: %s", err.Error())
		return err
	}
	_, err = file.Write(datos)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo escribir en el archivo de swap: %s", err.Error())
		return err
	}
	return nil
}

// Semáforos: entradaSwap.Mutex
func LeerPaginaDeSwap(entradaSwap *globals.EntradaSwap, pagina int) ([]byte, error) {
	utils_general.LoggearMensaje("DEBUG", "Intentando leer página de swap: PID=%d, Pagina=%d, OffsetBase=%d", entradaSwap.PID, pagina, entradaSwap.Offset)
	file, err := os.OpenFile(Memoria_config.Swapfile_path, os.O_RDWR, 0666)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo abrir el archivo de swap para leer: %s", err.Error())
		return nil, err
	}
	defer file.Close()

	// Saltear heade y calcular offset de la página
	offset := int64(entradaSwap.Offset + TAM_HEADER + pagina*MEMORIAInfo.TamanioPagina)

	utils_general.LoggearMensaje("DEBUG", "Realizando seek en swap: Offset=%d (PID=%d, Pagina=%d)", offset, entradaSwap.PID, pagina)
	buf := make([]byte, Memoria_config.Page_size)
	_, err = file.Seek(offset, 0)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo hacer seek en el archivo de swap: %s", err.Error())
		return nil, err
	}
	_, err = file.Read(buf)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo leer del archivo de swap: %s", err.Error())
	} else {
		utils_general.LoggearMensaje("DEBUG", "Lectura exitosa de swap: PID=%d, Pagina=%d, BytesLeidos=%d", entradaSwap.PID, pagina, len(buf))
	}

	return buf, err
}

// Semáforos: mutexTablaSwap, entrada.Mutex
// Semáforos: mutexTablaSwap
func encontrarEntradaSwap(pid int) (*globals.EntradaSwap, error) {
	mutexTablaSwap.Lock()
	defer mutexTablaSwap.Unlock()
	/*
		for _, entrada := range TablaSwap {
			entrada.Mutex.Lock()
			if entrada.PID == pid {
				entrada.Mutex.Unlock()
				return entrada, nil
			}
			entrada.Mutex.Unlock()
		}
	*/
	if TablaSwap[pid].PID == pid {
		return TablaSwap[pid], nil
	}
	return nil, fmt.Errorf("proceso %d no encontrado en swap", pid)
}

// Semáforos: mutexProcesos, mutexTablaFrames (en asignarFrame), mutexMemoria (en actualizarFrame), proc.Mutex
func cargarPaginaEnMemoriaPrincipal(pid int, pagina int, datos []byte) error {
	mutexProcesos.Lock()
	proc, existe := Procesos[pid]
	mutexProcesos.Unlock()

	if !existe {
		return fmt.Errorf("proceso %d no existe", pid)
	}

	frame := asignarFrame()
	if frame == -1 {
		return fmt.Errorf("no hay frames libres disponibles")
	}

	actualizarFrame(frame, datos)

	proc.Mutex.Lock()
	defer proc.Mutex.Unlock()

	indices := calcularIndices(pagina)
	entrada, err := obtenerEntradaDeProceso(proc, indices, false)

	if !entrada.Presente {
		entrada.Frame = frame
		entrada.Presente = true
		proc.Metricas.EscriturasMemoria++
	} else if err != nil {
		return fmt.Errorf("no se pudo obtener la entrada de la tabla de páginas: %w", err)
	}

	return nil
}

// Semáforos: mutexTablaSwap, entrada.Mutex, mutexProcesos, proceso.Mutex
func LiberarSwapDeProceso(pid int) {
	utils_general.LoggearMensaje("DEBUG", "Liberando swap para el proceso %d", pid)

	entradaSwap, err := encontrarEntradaSwap(pid)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Liberar proceso - no se encontró entrada de swap para PID %d: %s", pid, err.Error())
		return
	}
	entradaSwap.Mutex.Lock()
	defer entradaSwap.Mutex.Unlock()
	LimpiarContenidoSwap(entradaSwap)

	mutexProcesos.Lock()
	proceso, existe := Procesos[pid]
	mutexProcesos.Unlock()

	if existe {
		proceso.Mutex.Lock()
		proceso.Metricas.SubidasAMemoria++
		proceso.Mutex.Unlock()
	}
}

func LimpiarContenidoSwap(entradaSwap *globals.EntradaSwap) {
	entradaSwap.Presente = false
	cleanBuf := make([]byte, TAM_HEADER+entradaSwap.Tamanio)
	//copy(cleanBuf, "LIMPIADO")
	err := EscribirEnSwap(entradaSwap.Offset, cleanBuf)
	if err != nil {
		panic(err)
	}
	utils_general.LoggearMensaje("INFO", "## PID: %d - Contenido de swap limpiado - Offset: %d", entradaSwap.PID, entradaSwap.Offset)
}

// ============================================================================
// ENDPOINTS PARA CPU
// ============================================================================
// Info básica de memoria
// Semáforos: ninguno
func RecibirConsultaInfoBasica(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MEMORIAInfo)
}

// Siguiente instrucción
// Semáforos: mutexProcesos, proceso.Mutex
func RecibirConsultaProximaInstruccion(w http.ResponseWriter, r *http.Request) {
	PID := r.PathValue("pid")
	if PID == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el PID ")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el PID "))
		return
	}

	PIDInt, err := strconv.Atoi(PID)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir el PID a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir el PID"))
		return
	}

	PC := r.PathValue("pc")
	if PC == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el PC ")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el PC "))
		return
	}

	PCInt, err := strconv.Atoi(PC)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir el PC a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir el PC"))
		return
	}

	retardoMemoria()
	var instruccion = proximaInstruccion(PIDInt, PCInt)

	respuesta := globals.RespuestaInstruccion{Instruccion: instruccion}
	encoder := json.NewEncoder(w)
	error_respuesta := encoder.Encode(respuesta)
	if error_respuesta != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al codificar la respuesta al cliente: %s", error_respuesta.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "memoria respondió a cpu %s", respuesta.Instruccion)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Obtener instrucción: %d - Instrucción: %s", PIDInt, PCInt, instruccion)
}

// Escritura de memoria
// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria
func RecibirEscritura(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir escritura de CPU")
		return
	}

	var escritura, error = comunicacion.DecodePayloadAs[globals.EscrituraMem](paquete.Payload)
	if error != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirEscritura %s", error.Error())
		return
	}
	retardoMemoria()

	escribir(escritura.PID, escritura.DireccionFisica, []byte(escritura.Dato))

	w.WriteHeader(http.StatusOK)
}

// Lectura de memoria
// Semáforos: mutexMemoria, mutexProcesos, proceso.Mutex
func RecibirLectura(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")
	if pid == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el PID")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el PID"))
		return
	}

	dirFisica := r.PathValue("dir")
	if dirFisica == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la direccion fisica")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir la direccion fisica "))
		return
	}

	tamanio := r.PathValue("tamanio")
	if dirFisica == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el tamanio ")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el tamanio"))
		return
	}

	pidInt, err := strconv.Atoi(pid)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir el PID a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir el PID"))
		return
	}

	dirFisicaInt, err := strconv.Atoi(dirFisica)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir la direccion fisica a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir la direccion fisica"))
		return
	}

	tamanioInt, err := strconv.Atoi(tamanio)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir la direccion fisica a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir la direccion fisica"))
		return
	}
	retardoMemoria()
	dato := string(leer(pidInt, dirFisicaInt, tamanioInt))

	var respuesta globals.Mensaje = globals.Mensaje{Mensaje: dato}
	encoder := json.NewEncoder(w)
	error_respuesta := encoder.Encode(respuesta)
	if error_respuesta != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al codificar la respuesta al cliente: %s", error_respuesta.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "memoria respondió a cpu %s", respuesta.Mensaje)
}

// Consulta de marco
// Semáforos: mutexProcesos, proceso.Mutex
func RecibirConsultaDeMarco(w http.ResponseWriter, r *http.Request) {
	entradas := make([]int, MEMORIAInfo.Niveles)
	var val int
	var str string

	for i := range MEMORIAInfo.Niveles {
		str = strconv.Itoa(i)
		val, _ = strconv.Atoi(r.PathValue("e" + str))
		entradas[i] = val
	}

	pid, err := strconv.Atoi(r.PathValue("pid"))
	if err != nil {
		http.Error(w, "PID inválido", http.StatusBadRequest)
		return
	}
	mutexProcesos.Lock()
	proc, existe := Procesos[pid]
	mutexProcesos.Unlock()
	if !existe {
		http.Error(w, "PID no existe", http.StatusBadRequest)
		return
	}

	frame, err := obtenerFrameDeProceso(proc, entradas, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respuesta := globals.RespuestaFrame{Frame: frame}

	utils_general.LoggearMensaje("DEBUG", "PID: %d - OBTENER MARCO - Marco: %d", pid, respuesta.Frame)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respuesta)
}

// Lectura de página
// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria (en leerFrame)
func RecibirLecturaDePagina(w http.ResponseWriter, r *http.Request) {
	PID := r.PathValue("pid")
	if PID == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el PID ")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el PID "))
		return
	}

	PIDInt, err := strconv.Atoi(PID)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir el PID a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir el PID"))
		return
	}
	dirFisica := r.PathValue("dir")
	if PID == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir la direccion fisica")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir la direccion fisica "))
		return
	}

	dirFisicaInt, err := strconv.Atoi(dirFisica)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir la direccion fisica a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir la direccion fisica"))
		return
	}

	retardoMemoria()
	contenidoPagina := leerFrame(PIDInt, dirFisicaInt)
	respuesta := globals.PaginaMemoria{
		PID:       PIDInt,
		Contenido: contenidoPagina,
	}
	encoder := json.NewEncoder(w)
	error_respuesta := encoder.Encode(respuesta)
	if error_respuesta != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al codificar la respuesta al cliente: %s", error_respuesta.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Actualización de página
// Semáforos: mutexMemoria, mutexProcesos, proceso.Mutex
func RecibirActualizacionDePagina(w http.ResponseWriter, r *http.Request) {
	paquete := comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir actualización de página de CPU")
		return
	}
	paginaMem, err := comunicacion.DecodePayloadAs[globals.PaginaMemoria](paquete.Payload)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirActualizacionDePagina %s", err.Error())
		return
	}
	retardoMemoria()
	escribir(paginaMem.PID, paginaMem.Direccion, paginaMem.Contenido)

	w.WriteHeader(http.StatusOK)
}

// ============================================================================
// ENDPOINTS PARA KERNEL
// ============================================================================
// Consultar si hay suficiente espacio
// Semáforos: mutexTablaFrames
func RecibirTamanio(w http.ResponseWriter, r *http.Request) {
	tamanio_proceso := r.PathValue("tamanio")
	if tamanio_proceso == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el tamanio del proceso")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el tamanio del proceso"))
		return
	}

	tamanioInt, err := strconv.Atoi(tamanio_proceso)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al convertir el tamanio del proceso a entero: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al convertir el tamanio del proceso a entero"))
		return
	}

	var hay_espacio = verificarSihayEspacio(tamanioInt)

	var msj string
	if hay_espacio {
		msj = "OK"
	} else {
		msj = "NO"
	}

	var respuesta globals.Mensaje = globals.Mensaje{Mensaje: msj}
	encoder := json.NewEncoder(w)
	error_respuesta := encoder.Encode(respuesta)
	if error_respuesta != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al codificar la respuesta al cliente: %s", error_respuesta.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "memoria respondió a kernel %s", respuesta.Mensaje)
}

// Crear proceso
// Semáforos: proceso.Mutex, mutexProcesos, mutexTablaFrames, mutexSwapfile, mutexTablaSwap
func RecibirProceso(w http.ResponseWriter, r *http.Request) {
	var paquete = comunicacion.RecibirPaquete(w, r)
	if paquete.Has_error {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir proceso de CPU")
		return
	}

	var proceso, error = comunicacion.DecodePayloadAs[globals.Proceso](paquete.Payload)
	if error != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al decodificar el payload al RecibirProceso %s", error.Error())
		return
	}

	instrucciones, err := comunicacion.LeerTxt(Memoria_config.Scripts_path + proceso.Path)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo leer el archivo de instrucciones: %s", err.Error())
		return
	}

	proc := inicializaProceso(proceso, instrucciones)
	utils_general.LoggearMensaje("DEBUG", "creando tabla de paginas de proceso %d", proc.PID)

	proc.Mutex.Lock()
	paginasPorAsignar := proc.CantPaginas
	proc.TablaRaiz = crearTablaPaginaDesde(1, &paginasPorAsignar)

	// Registrar entrada de swap para el proceso

	guardarNuevoProceso(proc)

	utils_general.LoggearMensaje("INFO", "## PID: %d - Proceso Creado - Tamaño: %d", proc.PID, proc.Tamanio)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Instrucciones obtenidas de %s: %d", proc.PID, proceso.Path, len(instrucciones))
	registrarEntradaSwap(proc.PID, proc.CantPaginas)
	proc.Mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(globals.Mensaje{Mensaje: "OK"})
}

// Liberar espacio asociado
// Semáforos: mutexProcesos, proceso.Mutex, mutexTablaFrames, mutexMemoria, mutexTablaSwap
func RecibirLiberacionProceso(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")
	if pid == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el pid para liberar espacio")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el pid "))
		return
	}

	pidInt, _ := strconv.Atoi(pid)

	utils_general.LoggearMensaje("DEBUG", "liberando espacio asociado al proceso %d", pidInt)
	liberarEspacioAsociado(pidInt)
	utils_general.LoggearMensaje("DEBUG", "se liberó espacio asociado al proceso %d", pidInt)
	w.WriteHeader(http.StatusOK)
}

// Swappear proceso
// Semáforos: mutexSwapfile, mutexProcesos, proceso.Mutex, mutexTablaSwap
func RecibirSwapearProceso(w http.ResponseWriter, r *http.Request) {
	// mutexSwapfile.Lock()
	// defer mutexSwapfile.Unlock()
	utils_general.LoggearMensaje("DEBUG", "Recibiendo solicitud de swapeo de proceso")
	pid := r.PathValue("pid")
	pidInt, _ := strconv.Atoi(pid)

	mutexProcesos.Lock()
	proceso := Procesos[pidInt]
	mutexProcesos.Unlock()

	// Encontrar la entrada de swap existente y marcarla como presente
	entradaSwap, err := encontrarEntradaSwap(pidInt)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	entradaSwap.Mutex.Lock()
	retardoSwapeo()
	entradaSwap.Presente = true
	cantPaginas := entradaSwap.Tamanio / MEMORIAInfo.TamanioPagina
	defer entradaSwap.Mutex.Unlock()

	var paginas = make([]struct {
		Frame     int
		Contenido []byte
	}, cantPaginas)

	// Crear header "P: [PID]" de TAM_HEADER bytes
	header := make([]byte, TAM_HEADER)
	headerString := "PID: " + strconv.Itoa(pidInt)
	copy(header, []byte(headerString))

	//err = EscribirEnSwap(entradaSwap.Offset+TAM_HEADER, []byte("Datos"))
	err = EscribirEnSwap(entradaSwap.Offset, header)
	if err != nil {
		panic(err)
	}

	// El tamaño total incluye el header más el contenido de las páginas
	datos := make([]byte, entradaSwap.Tamanio)

	for nPag := range paginas {
		frame, err := obtenerFrameDeProceso(proceso, calcularIndices(nPag), false)

		if err != nil {
			continue
		}

		paginas[nPag].Frame = frame
		paginas[nPag].Contenido = leerFrame(pidInt, frame*MEMORIAInfo.TamanioPagina)
	}

	// Copiar el contenido de las páginas después del header
	for nPag := range paginas {
		inicioDatos := nPag * MEMORIAInfo.TamanioPagina // Comenzar después del header
		finDatos := inicioDatos + MEMORIAInfo.TamanioPagina
		copy(datos[inicioDatos:finDatos], paginas[nPag].Contenido)
	}

	err = EscribirEnSwap(entradaSwap.Offset+TAM_HEADER, datos)
	if err != nil {
		panic(err)
	}

	liberarFramesDeProceso(proceso)
	utils_general.LoggearMensaje("INFO", "## PID: %d - Swapeado", pidInt)

	proceso.Mutex.Lock()
	proceso.Metricas.BajadasASwap++
	proceso.Mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

// Deswappear proceso
// Semáforos: mutexSwapfile, mutexTablaSwap, proceso.Mutex, mutexProcesos, mutexTablaFrames, mutexMemoria
func RecibirDeswapearProceso(w http.ResponseWriter, r *http.Request) {
	// mutexSwapfile.Lock()
	// defer mutexSwapfile.Unlock()
	pid := r.PathValue("pid")
	pidInt, _ := strconv.Atoi(pid)

	entradaSwap, err := encontrarEntradaSwap(pidInt)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "ERROR - %s", err.Error())
		return
	}

	entradaSwap.Mutex.Lock()
	retardoSwapeo()
	cantPaginas := entradaSwap.Tamanio / MEMORIAInfo.TamanioPagina
	defer entradaSwap.Mutex.Unlock()

	for i := range cantPaginas {
		datosPagina, err := LeerPaginaDeSwap(entradaSwap, i)
		if err != nil {
			utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo leer la página %d del proceso %d desde swap: %s", i, pidInt, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = cargarPaginaEnMemoriaPrincipal(pidInt, i, datosPagina)
		if err != nil {
			utils_general.LoggearMensaje("DEBUG", "ERROR - No se pudo cargar la página %d en memoria principal: %s", i, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	LimpiarContenidoSwap(entradaSwap)

	utils_general.LoggearMensaje("INFO", "## PID: %d - Deswapeado", pidInt)

	w.WriteHeader(http.StatusOK)
}

// Dump de memoria
// Semáforos: mutexProcesos, proceso.Mutex, mutexMemoria, mutexTablaFrames
func RecibirDumpeoProceso(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")
	if pid == "" {
		utils_general.LoggearMensaje("DEBUG", "ERROR - error al recibir el pid para realizar dump")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al recibir el pid "))
		return
	}

	pidInt, _ := strconv.Atoi(pid)

	retardoMemoria()
	err := realizarDumpMemory(pidInt)

	if err {
		utils_general.LoggearMensaje("DEBUG", "ERROR - Error al realizar el dump de memoria para el proceso %d", pidInt)
	} else {
		utils_general.LoggearMensaje("DEBUG", "Memoria realizó el dump de memoria para el proceso %d", pidInt)
	}

	w.WriteHeader(http.StatusOK)
	var respuesta globals.Mensaje = globals.Mensaje{Mensaje: strconv.FormatBool(err)}
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(respuesta)
}
