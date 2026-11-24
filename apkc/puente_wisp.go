package main

// Camino mínimo del puente WISP entre la página y el núcleo.
//
// Aquí viven las piezas puras, sin `syscall/js`, para que se puedan falsar con
// `net.Pipe()` en la suite nativa. El registro de los globales y el manejo de
// `js.Value` están en `puente_wisp_js.go`.
//
// El opt-in de extremo a extremo se mantiene por construcción: `init()` solo
// registra dos funciones, no abre ningún socket, y `Habilitado` solo se pone a
// true dentro de `validarPeticionPuente`, que únicamente se alcanza cuando la
// página llama a `apkcWisp.obtener` después de la decisión de la persona. No hay
// demonio, no hay CLI nuevo y no se escribe nada en OPFS: el núcleo devuelve los
// bytes y la ruta canónica, y la página decide qué hacer con ellos.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// contratoPuenteWisp es el identificador congelado en `studio/wisp.js:18`. Si la
// página y el núcleo no dicen exactamente esto, no se habla: no se negocia una
// forma a medias.
const contratoPuenteWisp = "apkc-wisp/1"

const (
	hostPuenteWisp     = "repo1.maven.org"
	puertoPuenteWisp   = 443
	prefijoMavenPuente = "/maven2/"
	topeLineaPuente    = 32 << 10 // cabeceras: 32 KiB en total, nunca más
	topeCuerpoPuente   = 64 << 20 // artefacto: 64 MiB, rechazado antes de copiar
	plazoGetPuente     = 30 * time.Second
)

// extensionesPuenteWisp son las tres que `RutaDependenciaMaven` autoriza, en el
// orden en que se prueba cada una sobre la misma conexión TLS. Sin índice no se
// sabe si el artefacto es `.jar`, `.aar` o `.pom`, así que se prueban todas y
// solo se acepta la primera que devuelva 200.
var extensionesPuenteWisp = []string{"jar", "aar", "pom"}

// respuestaPuenteWisp es lo que devuelve el puente: los bytes íntegros, su
// sha256 y la ruta canónica bajo `/deps/`. Nunca se devuelve un cuerpo a medias.
type respuestaPuenteWisp struct {
	Bytes  []byte
	SHA256 string
	Ruta   string
}

// validarPeticionPuente comprueba el contrato y devuelve la configuración ya
// permitida junto con la carpeta canónica bajo `/deps/`. `ConfigWISP.Validar` es
// la única allowlist: no se duplica aquí ninguna lista de relays ni de destinos.
// `destino` se compara con la carpeta que sale de la coordenada para que un
// llamador no pueda pedir un artefacto y declarar otro sitio distinto.
func validarPeticionPuente(version, relay, coord, destino string) (ConfigWISP, string, error) {
	if version != contratoPuenteWisp {
		return ConfigWISP{}, "", fmt.Errorf("puente wisp: contrato %q no reconocido; se esperaba %q", version, contratoPuenteWisp)
	}

	cfg := NuevaConfigWISP()
	// Solo se llega aquí desde `apkcWisp.obtener`, es decir, después del opt-in
	// explícito de la página. Ningún otro camino activa el módulo por su cuenta.
	cfg.Habilitado = true
	cfg.Relay = relay
	// Sin PermitirRelaysPersonalizados ni PermitirDestinosPersonalizados: la
	// allowlist de `Validar` queda intacta.
	if err := cfg.Validar(); err != nil {
		return ConfigWISP{}, "", err
	}

	grupo, artefacto, versionCoord, err := ValidarCoordenadaMaven(coord)
	if err != nil {
		return ConfigWISP{}, "", err
	}
	carpeta := "/deps/" + strings.ReplaceAll(grupo, ".", "/") + "/" + artefacto + "/" + versionCoord + "/"
	if destino != carpeta {
		return ConfigWISP{}, "", fmt.Errorf("puente wisp: destino %q no corresponde a la coordenada %q; se esperaba %q", destino, coord, carpeta)
	}
	// La misma allowlist de rutas que usa el resto del núcleo: rechaza extensiones
	// ajenas y cualquier traversal antes de construir una petición HTTP.
	if _, err := RutaDependenciaMaven(coord, "jar"); err != nil {
		return ConfigWISP{}, "", err
	}
	return cfg, carpeta, nil
}

// obtenerConConn hace los GET necesarios sobre una conexión TLS ya autenticada.
// El llamador le pone el deadline: `AbrirTLSWISP` limpia el del handshake.
func obtenerConConn(conn net.Conn, coord string) (respuestaPuenteWisp, error) {
	var vacia respuestaPuenteWisp
	lector := bufio.NewReader(conn)

	for _, extension := range extensionesPuenteWisp {
		ruta, err := RutaDependenciaMaven(coord, extension)
		if err != nil {
			return vacia, err
		}
		rutaWeb := prefijoMavenPuente + strings.TrimPrefix(ruta, "deps/")
		cuerpo, err := getHTTPPuente(conn, lector, rutaWeb)
		if err != nil {
			return vacia, err
		}
		if cuerpo == nil { // 404 u otro rechazo: se prueba la extensión siguiente
			continue
		}
		suma := sha256.Sum256(cuerpo)
		return respuestaPuenteWisp{
			Bytes:  cuerpo,
			SHA256: hex.EncodeToString(suma[:]),
			Ruta:   "/" + ruta,
		}, nil
	}
	return vacia, fmt.Errorf("puente wisp: %q no está en ninguna de las extensiones permitidas (jar, aar, pom)", coord)
}

// getHTTPPuente manda un GET HTTP/1.1 sobre la conexión y devuelve nil cuando la
// respuesta no es 200. Cualquier cuerpo se lee entero o se descarta; nunca se
// devuelve un trozo. El tamaño se decide por Content-Length, con tope, y se
// rechaza la codificación chunked en vez de interpretarla a medias.
func getHTTPPuente(conn net.Conn, lector *bufio.Reader, rutaWeb string) ([]byte, error) {
	peticion := "GET " + rutaWeb + " HTTP/1.1\r\n" +
		"Host: " + hostPuenteWisp + "\r\n" +
		"User-Agent: " + contratoPuenteWisp + "\r\n" +
		"Accept: */*\r\n\r\n"
	if _, err := io.WriteString(conn, peticion); err != nil {
		return nil, fmt.Errorf("puente wisp: no se pudo mandar el GET %q: %w", rutaWeb, err)
	}

	estado, largo, err := leerCabecerasPuente(lector)
	if err != nil {
		return nil, err
	}
	if estado != 200 {
		// Se consume el cuerpo de la respuesta rechazada para que la conexión
		// siga siendo utilizable con la extensión siguiente.
		if _, err := io.CopyN(io.Discard, lector, int64(largo)); err != nil {
			return nil, fmt.Errorf("puente wisp: no se pudo descartar la respuesta %d: %w", estado, err)
		}
		return nil, nil
	}
	if largo == 0 {
		return nil, errors.New("puente wisp: el servidor devolvió 200 con cuerpo vacío")
	}
	cuerpo := make([]byte, largo)
	if _, err := io.ReadFull(lector, cuerpo); err != nil {
		return nil, fmt.Errorf("puente wisp: cuerpo incompleto (%d de %d bytes): %w", len(cuerpo), largo, err)
	}
	return cuerpo, nil
}

// leerCabecerasPuente lee la línea de estado y las cabeceras con un tope estricto.
// Devuelve el código y el Content-Length. Lee byte a byte a propósito: así no se
// pierden bytes que un `bufio.Reader` ajeno hubiera adelantado para la respuesta.
func leerCabecerasPuente(lector *bufio.Reader) (estado, largo int, err error) {
	consumido := 0
	leerLinea := func() (string, error) {
		var linea []byte
		for {
			if consumido >= topeLineaPuente {
				return "", errors.New("puente wisp: cabeceras HTTP demasiado largas")
			}
			b, e := lector.ReadByte()
			if e != nil {
				return "", fmt.Errorf("puente wisp: cabeceras HTTP cortadas: %w", e)
			}
			consumido++
			if b == '\n' {
				return strings.TrimSuffix(string(linea), "\r"), nil
			}
			linea = append(linea, b)
		}
	}

	linea, err := leerLinea()
	if err != nil {
		return 0, 0, err
	}
	partes := strings.Fields(linea)
	if len(partes) < 2 || (partes[0] != "HTTP/1.1" && partes[0] != "HTTP/1.0") {
		return 0, 0, fmt.Errorf("puente wisp: línea de estado HTTP no válida: %q", linea)
	}
	estado, err = strconv.Atoi(partes[1])
	if err != nil || estado < 100 || estado > 599 {
		return 0, 0, fmt.Errorf("puente wisp: código HTTP no válido: %q", partes[1])
	}

	vistoLargo := false
	for {
		linea, err = leerLinea()
		if err != nil {
			return 0, 0, err
		}
		if linea == "" {
			break
		}
		clave, valor, ok := strings.Cut(linea, ":")
		if !ok {
			return 0, 0, fmt.Errorf("puente wisp: cabecera HTTP sin «:»: %q", linea)
		}
		clave = strings.TrimSpace(clave)
		valor = strings.TrimSpace(valor)
		if strings.EqualFold(clave, "Transfer-Encoding") {
			return 0, 0, errors.New("puente wisp: Transfer-Encoding chunked no está admitido")
		}
		if strings.EqualFold(clave, "Content-Length") {
			if vistoLargo {
				return 0, 0, errors.New("puente wisp: Content-Length repetido")
			}
			largo, err = strconv.Atoi(valor)
			if err != nil || largo < 0 || largo > topeCuerpoPuente {
				return 0, 0, fmt.Errorf("puente wisp: Content-Length %q fuera de 0..%d bytes", valor, topeCuerpoPuente)
			}
			vistoLargo = true
		}
	}
	if !vistoLargo {
		return 0, 0, errors.New("puente wisp: la respuesta no trae Content-Length; no se lee hasta el cierre")
	}
	if estado != 200 && largo == 0 {
		return estado, 0, nil
	}
	return estado, largo, nil
}
