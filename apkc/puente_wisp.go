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
// demonio: `wisp-get` es un comando de un solo disparo que lee la petición de
// `go.env`, recorre este mismo camino y escribe una línea de JSON; no se escribe
// nada en OPFS: el núcleo devuelve los bytes y la ruta canónica, y la página
// decide qué hacer con ellos.

import (
	"bufio"
	"crypto/sha256"
	"crypto/x509"
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

// abrirConexionPuente entrega una conexión nueva para una extensión. El que
// llama pone el deadline del GET: `AbrirTLSWISP` limpia el del handshake.
type abrirConexionPuente func() (net.Conn, error)

// obtenerConCfg es el camino común del puente vivo y del comando `wisp-get`:
// recibe la configuración ya validada y el almacén de CAs fijado, monta TLS sobre
// el relay y devuelve el artefacto entero. La allowlist se aplica aguas arriba,
// en `validarPeticionPuente`; aquí no se duplica ni se ensancha.
func obtenerConCfg(cfg ConfigWISP, pool *x509.CertPool, coord string) (respuestaPuenteWisp, error) {
	var vacia respuestaPuenteWisp
	cfg.RootCAs = pool
	cliente, err := ConectarWISP(cfg)
	if err != nil {
		return vacia, err
	}
	defer cliente.Cerrar()
	abrir := func() (net.Conn, error) {
		conn, err := AbrirTLSWISP(cliente, hostPuenteWisp, puertoPuenteWisp, cfg)
		if err != nil {
			return nil, err
		}
		// `AbrirTLSWISP` limpia el deadline del handshake; el GET lleva el suyo.
		if err := conn.SetDeadline(time.Now().Add(plazoGetPuente)); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
	return obtenerConConn(abrir, coord)
}

// obtenerConConn prueba las tres extensiones, cada una en su propia conexión.
// Ningún lector sobrevive a una respuesta: no hay reintentos sobre bytes sin
// consumir y un cuerpo mayor que su Content-Length no puede colarse en el GET
// siguiente. La fábrica abre un flujo nuevo por extensión porque el GET pide
// `Connection: close` y el servidor cierra tras responder.
func obtenerConConn(abrir abrirConexionPuente, coord string) (respuestaPuenteWisp, error) {
	var vacia respuestaPuenteWisp

	for _, extension := range extensionesPuenteWisp {
		ruta, err := RutaDependenciaMaven(coord, extension)
		if err != nil {
			return vacia, err
		}
		rutaWeb := prefijoMavenPuente + strings.TrimPrefix(ruta, "deps/")
		conn, err := abrir()
		if err != nil {
			return vacia, err
		}
		lector := bufio.NewReader(conn)
		cuerpo, err := getHTTPPuente(conn, lector, rutaWeb)
		conn.Close()
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

// getHTTPPuente manda un GET HTTP/1.1 con `Connection: close` y devuelve nil
// cuando la respuesta no es 200. El cuerpo se lee hasta el cierre, acotado por
// `topeCuerpoPuente`, y se compara con Content-Length: un cuerpo con más bytes
// de los declarados se rechaza entero, sin devolver un trozo ni dejar bytes
// para otra extensión. La codificación chunked se rechaza en las cabeceras.
func getHTTPPuente(conn net.Conn, lector *bufio.Reader, rutaWeb string) ([]byte, error) {
	peticion := "GET " + rutaWeb + " HTTP/1.1\r\n" +
		"Host: " + hostPuenteWisp + "\r\n" +
		"User-Agent: " + contratoPuenteWisp + "\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n\r\n"
	if _, err := io.WriteString(conn, peticion); err != nil {
		return nil, fmt.Errorf("puente wisp: no se pudo mandar el GET %q: %w", rutaWeb, err)
	}

	estado, largo, err := leerCabecerasPuente(lector)
	if err != nil {
		return nil, err
	}
	cuerpo, err := io.ReadAll(io.LimitReader(lector, int64(topeCuerpoPuente)+1))
	if err != nil {
		return nil, fmt.Errorf("puente wisp: no se pudo leer el cuerpo: %w", err)
	}
	if len(cuerpo) > topeCuerpoPuente {
		return nil, fmt.Errorf("puente wisp: cuerpo de %d bytes supera el tope de %d", len(cuerpo), topeCuerpoPuente)
	}
	if len(cuerpo) != largo {
		return nil, fmt.Errorf("puente wisp: cuerpo de %d bytes no coincide con Content-Length %d", len(cuerpo), largo)
	}
	if estado != 200 {
		return nil, nil
	}
	if largo == 0 {
		return nil, errors.New("puente wisp: el servidor devolvió 200 con cuerpo vacío")
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
