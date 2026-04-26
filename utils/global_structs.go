package globals

import (
	"sync"
	"time"
)

type Paquete struct {
	Tipo      string      `json:"tipo"`
	Origen    string      `json:"origen"`
	Destino   string      `json:"destino"`
	Payload   interface{} `json:"payload"`
	Has_error bool        `json:"error"`
}

type CPU_config struct {
	Ip_memory         string `json:"ip_memory"`
	Port_memory       int    `json:"port_memory"`
	Ip_kernel         string `json:"ip_kernel"`
	Port_kernel       int    `json:"port_kernel"`
	Port_cpu          int    `json:"port_cpu"`
	TLB_entries       int    `json:"tlb_entries"`
	TLB_replacement   string `json:"tlb_replacement"`
	Cache_entries     int    `json:"cache_entries"`
	Cache_replacement string `json:"cache_replacement"`
	Cache_delay       int    `json:"cache_delay"`
	Log_id            string `json:"log_id"`
	Log_level         string `json:"log_level"`
}

type IO_config struct {
	Ip_kernel   string `json:"ip_kernel"`
	Port_kernel int    `json:"port_kernel"`
	Port_io     int    `json:"port_io"`
	Log_id      string `json:"log_id"`
	Log_level   string `json:"log_level"`
}
type Kernel_config struct {
	Ip_memory               string  `json:"ip_memory"`
	Port_memory             int     `json:"port_memory"`
	Scheduler_algorithm     string  `json:"scheduler_algorithm"`
	Ready_ingress_algorithm string  `json:"ready_ingress_algorithm"`
	Alpha                   float64 `json:"alpha"`
	Initial_estimate        float64 `json:"initial_estimate"`
	Suspension_time         int     `json:"suspension_time"`
	Log_level               string  `json:"log_level"`
	Port_kernel             int     `json:"port_kernel"`
}

type Memoria_config struct {
	Port_memory      int    `json:"port_memory"`
	Memory_size      int    `json:"memory_size"`
	Page_size        int    `json:"page_size"`
	Entries_per_page int    `json:"entries_per_page"`
	Number_of_levels int    `json:"number_of_levels"`
	Memory_delay     int    `json:"memory_delay"`
	Swapfile_path    string `json:"swapfile_path"`
	Swap_delay       int    `json:"memory__delay"`
	Dump_path        string `json:"dump_path"`
	Scripts_path     string `json:"scripts_path"`
	Log_level        string `json:"log_level"`
}

type CPU_conectada struct {
	Ip               string    `json:"ip"`
	Puerto           int       `json:"puerto"`
	Identificador    string    `json:"identificador"`
	Ejecutando       bool      `json:"ejecutando"`
	Pid_ejecutando   int       `json:"pid_ejecutando"`   // PID del proceso que está ejecutando, -1 si no está ejecutando ninguno
	Inicio_ejecucion time.Time `json:"inicio_ejecucion"` // Marca de tiempo del inicio de la ejecución
}

type IO_conectada struct {
	Ip      string `json:"ip"`
	Puerto  int    `json:"puerto"`
	Nombre  string `json:"nombre"`
	Ocupada bool   `json:"ocupada"`
}

type Estado_proceso string //es lo que va a reflejar el estado del proceso con el enum de abajo

const (
	NEW        Estado_proceso = "NEW"
	READY      Estado_proceso = "READY"
	EXEC       Estado_proceso = "EXEC"
	BLOQUEADO  Estado_proceso = "BLOCKED"
	EXIT       Estado_proceso = "EXIT"
	SUSP_READY Estado_proceso = "SUSP_READY"
	SUSP_BLOQ  Estado_proceso = "SUSP_BLOCKED"
)

type Proceso struct {
	Path    string `json:"path"`
	Tamanio int    `json:"tamanio"`
	Pcb     *PCB   `json:"pcb"`
}

type PCB struct {
	PID    int            `json:"pid"`
	PC     int            `json:"pc"`
	Estado Estado_proceso `json:"estado"`
	// Métricas de estado
	ConteoEstado    map[Estado_proceso]int           // p.ej. "NEW": 3, "READY": 5, …
	TiempoEstado    map[Estado_proceso]time.Duration // tiempo acumulado en cada estado
	TimestampCambio time.Time                        // último cambio de estado
}

type Mensaje struct {
	Mensaje string `json:"mensaje"`
}

type Syscall struct {
	KeyWord string   `json:"keyWord"`
	Params  []string `json:"params"`
	PID     int      `json:"pid"`
}

type InstruccionIO struct {
	Tiempo int `json:"tiempo"`
	PID    int `json:"pid"`
}

type FinalizacionIO struct {
	PID    int    `json:"pid"`
	Nombre string `json:"nombre"`
	IP     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type MemoriaInfo struct {
	TamanioPagina     int `json:"tamanioPagina"`
	Niveles           int `json:"niveles"`
	CantEntradasTabla int `json:"cantEntradasTabla"`
	CantFrames        int `json:"cantFrames"`
}

type Interrupt struct {
	Nombre string `json:"nombre"`
	PID    int    `json:"pid"`
}

type RespuestaDump struct {
	Pid   int  `json:"pid"`
	Error bool `json:"error"`
}

type EstimacionProceso struct {
	Estimacion    float64 `json:"estimacion"`    // Tiemp estimado que ejecutará el proceso
	RealEjecutado float64 `json:"realEjecutado"` // lo que realmente ejecuto el proceso
}
type RespuestaInstruccion struct {
	Instruccion string `json:"instruccion"`
}

type RespuestaFrame struct {
	Frame int `json:"frame"`
}

type Frame struct {
	ID      int
	Inicio  int // índice de inicio en MemoriaFisica
	Ocupado bool
}

type EntradaTabla struct {
	Subtabla *TablaPaginacion
	Frame    int
	Presente bool
}
type TablaPaginacion struct {
	Nivel    int
	Entradas []EntradaTabla
}

type MetricasProceso struct {
	AccesosTablasPaginas     int
	InstruccionesSolicitadas int
	BajadasASwap             int
	SubidasAMemoria          int
	LecturasMemoria          int
	EscriturasMemoria        int
}

type ProcesoMem struct {
	PID           int
	Tamanio       int
	CantPaginas   int
	TablaRaiz     *TablaPaginacion
	Instrucciones []string
	Metricas      MetricasProceso
	Mutex         sync.Mutex // Mutex para proteger el acceso a las estructuras del proceso
}
type EscrituraMem struct {
	PID             int    `json:"pid"`
	DireccionFisica int    `json:"direccion_fisica"` // Dirección física en memoria
	Dato            string `json:"dato"`             // Datos a escribir
}

type LecturaMem struct {
	PID             int `json:"pid"`
	DireccionFisica int `json:"direccion_fisica"` // Dirección física en memoria
	Tamanio         int `json:"tamanio"`          // Cantidad de bytes a leer
}
type ActualizacionPC struct {
	Identificador string `json:"identificador"`
	PID           int    `json:"pid"`
	PC            int    `json:"pc"` // PC del proceso que se está liberando
}

type LiberacionCPU struct {
	Identificador string `json:"identificador"`
	PID           int    `json:"pid"`
	Motivo        string `json:"motivo"` // Motivo de la liberación, p.ej. "desalojoSRT", "syscall", etc.
}

// EntradaSwap representa una entrada en el archivo de swap, que contiene el PID del proceso,
// el número de página y el offset dentro del archivo swap.
type EntradaSwap struct {
	PID      int
	Offset   int  // Offset dentro del archivo swap
	Presente bool // Indica si el proceso está actualmente en swap
	Tamanio  int  // Tamaño total del proceso en bytes
	Mutex    sync.Mutex
}

// Estructura de entrada de la cache de páginas de CPU
type EntradaCache struct {
	PID        int
	Pagina     int
	Contenido  []byte // ahora es un slice de bytes del tamaño de página
	Modificada bool
	Uso        bool
}

// Estructura para transferencia de páginas entre CPU y Memoria
// Usada para leer o actualizar una página completa
// (PID, número de página, contenido de la página)
type PaginaMemoria struct {
	PID       int
	Direccion int
	Contenido []byte
}

type EntradaTLB struct {
	PID       int
	Pagina    int
	Frame     int
	Timestamp time.Time // Para FIFO tiempo de insercion, para LRU tiempo ultimo uso
}
type ProcesoIO struct {
	PID           int
	DispositivoIO string
	PuertoIO      int
	IpIO          string
}

type MetricasCPU struct {
	Accesos    int
	Reemplazos int
	Hits       int
	Misses     int
	Hitrate    float64
}
