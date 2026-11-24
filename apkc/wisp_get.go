package main

// wisp-get es el comando de un solo disparo para la página en reposo. No es un
// demonio: la petición llega en `go.env` (APKC_WISP_REQ), se valida con las mismas
// piezas que el puente vivo y se escribe una sola línea con prefijo de marca y un
// JSON {bytes:base64,sha256,ruta}. El hospedador silencia esa línea en la consola
// de la página; los errores van a stderr con salida 1. No hay argv, no hay OPFS y
// no hay allowlist propia: la de `validarPeticionPuente` es la única.

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// marcaWispGet delimita la única línea de stdout que es el contrato del comando.
// La página la busca con startsWith antes de parsear el JSON; el base64 no puede
// contener los dos puntos de la marca.
const marcaWispGet = "APKC_WISP_GET:"

type peticionWispGet struct {
	Version    string `json:"version"`
	Relay      string `json:"relay"`
	Coordenada string `json:"coordenada"`
	Destino    string `json:"destino"`
	PEM        string `json:"pem"`
	Huella     string `json:"huella"`
}

type salidaWispGet struct {
	Bytes  string `json:"bytes"`
	SHA256 string `json:"sha256"`
	Ruta   string `json:"ruta"`
}

// obtenerWispGetFunc es la costura de la prueba: el comando pasa `obtenerConCfg`
// y la falsificación pasa un `net.Pipe()` con `servirUnaPuente`.
type obtenerWispGetFunc func(ConfigWISP, *x509.CertPool, string) (respuestaPuenteWisp, error)

// resolverWispGet valida la petición y las CAs antes de tocar la red. Se devuelve
// el error sin bytes parciales: el núcleo luego no escribe ninguna línea de marca.
func resolverWispGet(peticion peticionWispGet, obtener obtenerWispGetFunc) (respuestaPuenteWisp, error) {
	var vacia respuestaPuenteWisp
	cfg, _, err := validarPeticionPuente(peticion.Version, peticion.Relay, peticion.Coordenada, peticion.Destino)
	if err != nil {
		return vacia, err
	}
	decodificada, err := hex.DecodeString(peticion.Huella)
	if err != nil || len(decodificada) != sha256.Size {
		return vacia, fmt.Errorf("wisp-get: la huella debe ser sha256 de 64 dígitos hexadecimales; recibida %q",
			peticion.Huella)
	}
	var huella [sha256.Size]byte
	copy(huella[:], decodificada)
	pool, err := CargarCAsPEMFijadas([]byte(peticion.PEM), huella)
	if err != nil {
		return vacia, err
	}
	return obtener(cfg, pool, peticion.Coordenada)
}

// wispGetDesdeEnv lee la petición de `go.env` y devuelve la línea completa, con
// marca incluida. Separarla del `os.Exit` del comando permite falsificarla con
// `t.Setenv` y un `net.Pipe()` sin que la prueba se cierre a sí misma.
func wispGetDesdeEnv(obtener obtenerWispGetFunc) (string, error) {
	crudo := os.Getenv("APKC_WISP_REQ")
	if crudo == "" {
		return "", fmt.Errorf("wisp-get: falta APKC_WISP_REQ en go.env")
	}
	var peticion peticionWispGet
	if err := json.Unmarshal([]byte(crudo), &peticion); err != nil {
		return "", fmt.Errorf("wisp-get: APKC_WISP_REQ no es JSON válido: %w", err)
	}
	respuesta, err := resolverWispGet(peticion, obtener)
	if err != nil {
		return "", err
	}
	linea, err := json.Marshal(salidaWispGet{
		Bytes:  base64.StdEncoding.EncodeToString(respuesta.Bytes),
		SHA256: respuesta.SHA256,
		Ruta:   respuesta.Ruta,
	})
	if err != nil {
		return "", err
	}
	return marcaWispGet + string(linea), nil
}

func comandoWispGet() {
	linea, err := wispGetDesdeEnv(obtenerConCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wisp-get:", err)
		os.Exit(1)
	}
	fmt.Println(linea)
}
