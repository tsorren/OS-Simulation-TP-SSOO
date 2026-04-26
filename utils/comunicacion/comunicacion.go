package comunicacion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	globals "tp-2025-1c-FSociety/utils"
	utils_general "tp-2025-1c-FSociety/utils/logs"
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "" // o algún error si no se encuentra una IP válida
}

func IniciarConfiguracion[T any](filePath string) *T {
	var config *T
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

func EnviarPaquete(ip string, puerto int, ruta string, paquete globals.Paquete) {
	body, err := json.Marshal(paquete)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "error codificando mensajes: %s", err.Error())
		return
	}

	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "error enviando mensajes a ip:%s puerto:%d", ip, puerto)
		return
	}

	utils_general.LoggearMensaje("DEBUG", "respuesta del servidor: %s", resp.Status)
}

func RecibirPaquete(w http.ResponseWriter, r *http.Request) globals.Paquete {
	decoder := json.NewDecoder(r.Body)
	var paquete globals.Paquete
	err := decoder.Decode(&paquete)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "error al decodificar paquete: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al decodificar paquete"))
		paquete.Has_error = true
		return paquete
	}

	utils_general.LoggearMensaje("DEBUG", "me llego un paquete de un cliente: %+v", paquete)

	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("ok"))

	return paquete
}

func DecodePayloadAs[T any](payload interface{}) (T, error) {
	var result T
	data, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

// LeerDeServidor envía un GET a la URL construida a partir de ip, puerto, ruta y params.
// Deserializa la respuesta JSON en el tipo genérico T y lo devuelve.
// En caso de error (GET falló o StatusCode != 200, o JSON inválido), hace log y retorna error.
func LeerDeServidor[T any](ip string, puerto int, ruta string, params ...string) (T, error) {
	var cero T

	// Construir URL base: "http://<ip>:<puerto>/<ruta>"
	base := fmt.Sprintf("http://%s:%d/%s", ip, puerto, ruta)
	// Si hay parámetros adicionales, los agregamos con "/"
	for _, p := range params {
		base += "/" + p
	}

	resp, err := http.Get(base)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG",
			"error enviando mensajes a ip:%s puerto:%d ruta:%s",
			ip, puerto, ruta,
		)
		return cero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils_general.LoggearMensaje("DEBUG",
			"respuesta inesperada del servidor para ruta %s: código %d",
			base, resp.StatusCode,
		)
		return cero, fmt.Errorf("status code %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG",
			"error leyendo cuerpo de respuesta ruta:%s/%v err:%s",
			ruta, params, err.Error(),
		)
		return cero, err
	}

	var resultado T
	if err := json.Unmarshal(raw, &resultado); err != nil {
		utils_general.LoggearMensaje("DEBUG",
			"error deserializando JSON de ruta:%s/%v err:%s",
			ruta, params, err.Error(),
		)
		return cero, err
	}

	return resultado, nil
}

// Envía un POST con el paquete y espera un JSON de respuesta, devolviendo el tipo T.
func PostYLeerRespuesta[T any](ip string, puerto int, ruta string, paquete globals.Paquete) (T, error) {
	var cero T
	body, err := json.Marshal(paquete)
	if err != nil {
		return cero, fmt.Errorf("error al serializar paquete: %w", err)
	}
	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return cero, fmt.Errorf("error al hacer POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return cero, fmt.Errorf("status code %d: %s", resp.StatusCode, string(raw))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return cero, fmt.Errorf("error al leer respuesta: %w", err)
	}
	if len(raw) == 0 {
		return cero, fmt.Errorf("respuesta vacía del servidor")
	}
	var resultado T
	if err := json.Unmarshal(raw, &resultado); err != nil {
		return cero, fmt.Errorf("error al decodificar respuesta: %w. Respuesta: %s", err, string(raw))
	}
	return resultado, nil
}

func Escuchar(port int, mux *http.ServeMux) {
	err := http.ListenAndServe(":"+strconv.Itoa(port), mux)
	if err != nil {
		panic(err)
	}
}

func LeerTxt(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo: %w", err)
	}
	defer file.Close()

	var lineas []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lineas = append(lineas, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error al leer el archivo: %w", err)
	}

	return lineas, nil
}

func EnviarMensaje(ip string, puerto int, ruta string) {
	body, err := json.Marshal("")
	if err != nil {
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, ruta)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "error enviando mensaje a ip:%s puerto:%d", ip, puerto)
	}
	utils_general.LoggearMensaje("DEBUG", "se envió mensaje a la ruta %s", ruta)
	utils_general.LoggearMensaje("DEBUG", "respuesta del servidor: %s", resp.Status)
	resp.Body.Close()
}

func RecibirMensaje(w http.ResponseWriter, r *http.Request, mensaje_de_respuesta string) {
	decoder := json.NewDecoder(r.Body)
	var mensaje globals.Mensaje
	err := decoder.Decode(&mensaje)
	if err != nil {
		utils_general.LoggearMensaje("DEBUG", "Error al decodificar mensaje: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error al decodificar mensaje"))

	}

	log.Println("Me llego un mensaje de un cliente")
	utils_general.LoggearMensaje("DEBUG", "%+v", mensaje)

	//respuesta
	var respuesta globals.Mensaje = globals.Mensaje{Mensaje: mensaje_de_respuesta}
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(respuesta)

	w.WriteHeader(http.StatusOK)
	// w.Write([]byte("true"))

}

func LeerDatosBinarios(datos []byte) string {
	return strings.TrimSpace(strings.Trim(string(datos), "\x00"))
}

func RecibirFinalizacionDeKernel(w http.ResponseWriter, r *http.Request) {
	utils_general.LoggearMensaje("INFO", "Deteniendo ejecucion por finalización de kernel")
	w.WriteHeader(http.StatusOK)
	os.Exit(0)
}