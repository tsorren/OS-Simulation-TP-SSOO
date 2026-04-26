package utils_general

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// ============================================================================
// CONFIGURACIÓN Y VARIABLES GLOBALES
// ============================================================================

// Constante de configuración - poner en false para deshabilitar salida en terminal
const MOSTRAR_LOGS_EN_TERMINAL = false

var logger *slog.Logger

// Códigos de color ANSI
const (
	ColorReset        = "\033[0m"
	ColorRed          = "\033[31m"
	ColorYellow       = "\033[33m"
	ColorGreen        = "\033[32m"
	ColorGray         = "\033[37m"
	ColorBold         = "\033[1m"
	ColorPurple       = "\033[35m"
	ColorCyan         = "\033[36m"
	ColorWhite        = "\033[97m"
	ColorOrange       = "\033[91m"
	ColorMagenta      = "\033[95m"
	ColorBrightYellow = "\033[93m"
)

// Colores para estados - colores representativos para cada estado de proceso
var stateColors = map[string]string{
	"NEW":          ColorPurple,       // Azul
	"READY":        ColorGreen,        // Verde
	"EXEC":         ColorMagenta,      // Magenta
	"BLOCKED":      ColorYellow,       // Amarillo
	"SUSP_BLOCKED": ColorBrightYellow, // Amarillo brillante
	"SUSP_READY":   ColorCyan,         // Cian
	"EXIT":         ColorRed,          // Rojo
}

// Compilar regex una vez para mejor rendimiento
var stateRegex = regexp.MustCompile(`\b(NEW|READY|EXEC|BLOCKED|SUSP_BLOCKED|SUSP_READY|EXIT)\b`)
var pidRegex = regexp.MustCompile(`(?:PID: (\d+)|\## \((\d+)\))`)

// Colores para PID - 7 colores distintos para PID % 7
var pidColors = []string{
	ColorRed,     // 0
	ColorGreen,   // 1
	ColorPurple,  // 2
	ColorYellow,  // 3
	ColorMagenta, // 4
	ColorCyan,    // 5
	ColorWhite,   // 6
}

// ============================================================================
// AUXILIARES
// ============================================================================

type ColorHandler struct {
	handler slog.Handler
	writer  io.Writer
}

// Crea un nuevo handler con colores
func NuevoColorHandler(w io.Writer, opciones *slog.HandlerOptions) *ColorHandler {
	return &ColorHandler{
		handler: slog.NewTextHandler(w, opciones),
		writer:  w,
	}
}

// Verifica si el handler está habilitado para el nivel de log
func (h *ColorHandler) Enabled(ctx context.Context, nivel slog.Level) bool {
	return h.handler.Enabled(ctx, nivel)
}

// Agrega atributos al handler
func (h *ColorHandler) WithAttrs(atributos []slog.Attr) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithAttrs(atributos),
		writer:  h.writer,
	}
}

// Agrupa logs bajo un nombre específico
func (h *ColorHandler) WithGroup(nombre string) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithGroup(nombre),
		writer:  h.writer,
	}
}

// ============================================================================
// MANEJO DE LOGS CON COLORES
// ============================================================================

// Procesa y formatea un registro de log con colores
func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {
	// Formatear timestamp
	timestamp := r.Time.Format("15:04:05.000")

	// Obtener color del nivel
	var levelColor string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = ColorYellow
	case slog.LevelInfo:
		levelColor = ColorPurple
	case slog.LevelWarn:
		levelColor = ColorBrightYellow
	case slog.LevelError:
		levelColor = ColorRed
	default:
		levelColor = ColorPurple
	}

	// Colorea los estados y PIDs en el mensaje
	message := colorearEstados(r.Message)
	message = colorearPIDs(message)

	// Formatea el mensaje de log con el nivel entre corchetes y agrega padding después
	levelStr := strings.ToUpper(r.Level.String())

	// Calcula el padding necesario - "DEBUG" es el más largo (5 caracteres)
	paddingSize := 6 - len(levelStr) // 5 caracteres + 1 espacio mínimo
	padding := strings.Repeat(" ", paddingSize)

	formattedMsg := fmt.Sprintf("[%s] %s[%s]%s%s%s\n",
		timestamp,
		levelColor, levelStr, ColorReset,
		padding, // Agrega padding después del corchete
		message,
	)

	// Escribe en el writer
	_, err := h.writer.Write([]byte(formattedMsg))
	return err
}

// ============================================================================
// FUNCIONES DE COLORES
// ============================================================================

// Colorea los estados en el mensaje
func colorearEstados(mensaje string) string {
	return stateRegex.ReplaceAllStringFunc(mensaje, func(estado string) string {
		if color, existe := stateColors[estado]; existe {
			return color + estado + ColorReset
		}
		return estado
	})
}

// Colorea los PIDs en el mensaje
func colorearPIDs(mensaje string) string {
	return pidRegex.ReplaceAllStringFunc(mensaje, func(coincidencia string) string {
		// Extrae el número de PID del match
		coincidencias := pidRegex.FindStringSubmatch(coincidencia)
		var pidStr string

		// Verifica qué grupo hizo match (formato PID: o formato ##)
		if coincidencias[1] != "" {
			pidStr = coincidencias[1]
		} else if coincidencias[2] != "" {
			pidStr = coincidencias[2]
		}

		if pidStr != "" {
			// Convertir a int para operación módulo
			pid := 0
			fmt.Sscanf(pidStr, "%d", &pid)

			// Obtener color basado en PID % 7
			colorIndex := pid % 7
			pidColor := pidColors[colorIndex]

			// Reemplazar solo la parte del número con versión coloreada
			if coincidencias[1] != "" {
				// Formato PID:
				return fmt.Sprintf("PID: %s%s%s", pidColor, pidStr, ColorReset)
			} else {
				// Formato ##
				return fmt.Sprintf("## (%s%s%s)", pidColor, pidStr, ColorReset)
			}
		}

		return coincidencia
	})
}

// ============================================================================
// CONFIGURACIÓN DEL LOGGER
// ============================================================================

// Configura el logger principal del sistema
func ConfigurarLogger(nombreArchivo string, logLevel string, modulo string) {
	logFilePath := fmt.Sprintf("%s/%s", modulo, nombreArchivo)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	// Para salida a archivo con el mismo formato pero sin colores
	fileHandler := NuevoColorHandler(logFile, &slog.HandlerOptions{
		Level: obtenerNivelLog(logLevel),
	})

	var handler slog.Handler

	if MOSTRAR_LOGS_EN_TERMINAL {
		// Para salida a consola con colores
		consoleHandler := NuevoColorHandler(os.Stdout, &slog.HandlerOptions{
			Level: obtenerNivelLog(logLevel),
		})

		// Crear un multi-handler que escriba logs formateados tanto a consola como a archivo
		handler = &MultiHandler{
			console: consoleHandler,
			file:    fileHandler,
		}
	} else {
		// Solo escribir a archivo
		handler = fileHandler
	}

	logger = slog.New(handler).With("module", modulo)
}

// Convierte string de nivel a tipo slog.Level
func obtenerNivelLog(nivelLog string) slog.Level {
	switch nivelLog {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ============================================================================
// MULTI-HANDLER PARA CONSOLA Y ARCHIVO
// ============================================================================

type MultiHandler struct {
	console slog.Handler
	file    slog.Handler
}

// Handler compuesto que permite escribir logs tanto en consola como en archivo
func (m *MultiHandler) Enabled(ctx context.Context, nivel slog.Level) bool {
	return m.console.Enabled(ctx, nivel) || m.file.Enabled(ctx, nivel)
}

// Maneja el registro de logs en ambos destinos
func (m *MultiHandler) Handle(ctx context.Context, registro slog.Record) error {
	if err := m.console.Handle(ctx, registro); err != nil {
		return err
	}
	return m.file.Handle(ctx, registro)
}

// Agrega atributos al handler compuesto
func (m *MultiHandler) WithAttrs(atributos []slog.Attr) slog.Handler {
	return &MultiHandler{
		console: m.console.WithAttrs(atributos),
		file:    m.file.WithAttrs(atributos),
	}
}

// Agrupa los logs bajo un mismo nombre en ambos destinos
func (m *MultiHandler) WithGroup(nombre string) slog.Handler {
	return &MultiHandler{
		console: m.console.WithGroup(nombre),
		file:    m.file.WithGroup(nombre),
	}
}

// ============================================================================
// FUNCIÓN PRINCIPAL DE LOGGING
// ============================================================================

// Función principal para loguear mensajes en el sistema
func LoggearMensaje(nivel string, formato string, args ...interface{}) {
	if logger == nil {
		panic("No se configuro el Logger, intenta configurar el logger nuevamente.")
	}
	mensaje := fmt.Sprintf(formato, args...)
	switch nivel {
	case "INFO":
		logger.Info(mensaje)
	case "WARN":
		logger.Warn(mensaje)
	case "ERROR":
		logger.Error(mensaje)
	case "DEBUG":
		logger.Debug(mensaje)
	default:
		logger.Info(mensaje)
	}
}
