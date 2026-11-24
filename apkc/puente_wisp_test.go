package main

// Falsaciones nativas del camino mínimo del puente WISP. No hay `js.Value` ni
// red: `net.Pipe()` hace de servidor HTTP para que se midan el GET, el límite de
// cuerpo, el sha256 y la ruta canónica.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

const (
	coordPuentePrueba   = "com.ejemplo:artefacto:1.0"
	destinoPuentePrueba = "/deps/com/ejemplo/artefacto/1.0/"
)

func TestPuenteWispValidarPeticion(t *testing.T) {
	cfg, carpeta, err := validarPeticionPuente(contratoPuenteWisp, RelaysWISP()[0], coordPuentePrueba, destinoPuentePrueba)
	if err != nil {
		t.Fatalf("la petición del contrato fue rechazada: %v", err)
	}
	if !cfg.Habilitado || cfg.Relay != RelaysWISP()[0] {
		t.Fatalf("la configuración no quedó activada con el relay pedido: %+v", cfg)
	}
	if cfg.PermitirRelaysPersonalizados || cfg.PermitirDestinosPersonalizados || len(cfg.DestinosExtra) != 0 {
		t.Fatalf("la validación ensanchó la allowlist: %+v", cfg)
	}
	if err := cfg.Validar(); err != nil {
		t.Fatalf("la configuración devuelta no pasa Validar: %v", err)
	}
	if carpeta != destinoPuentePrueba {
		t.Fatalf("carpeta canónica %q, se esperaba %q", carpeta, destinoPuentePrueba)
	}

	casos := []struct {
		nombre  string
		version string
		relay   string
		coord   string
		destino string
	}{
		{"contrato ajeno", "apkc-wisp/2", RelaysWISP()[0], coordPuentePrueba, destinoPuentePrueba},
		{"relay fuera de la allowlist", contratoPuenteWisp, "wss://relay-ajeno.test/wisp/", coordPuentePrueba, destinoPuentePrueba},
		{"coordenada con traversal", contratoPuenteWisp, RelaysWISP()[0], "../fuera:artefacto:1.0", destinoPuentePrueba},
		{"coordenada sin versión", contratoPuenteWisp, RelaysWISP()[0], "com.ejemplo:artefacto", destinoPuentePrueba},
		{"destino de otra coordenada", contratoPuenteWisp, RelaysWISP()[0], coordPuentePrueba, "/deps/otro/sitio/1.0/"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if _, _, err := validarPeticionPuente(caso.version, caso.relay, caso.coord, caso.destino); err == nil {
				t.Fatal("la petición debía rechazarse")
			}
		})
	}

	_, _, err = validarPeticionPuente(contratoPuenteWisp, "wss://relay-ajeno.test/wisp/", coordPuentePrueba, destinoPuentePrueba)
	if !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("un relay ajeno debe rechazarlo la allowlist de Validar: %v", err)
	}
}

// servirUnaPuente responde a un GET con el estado, la cabecera de longitud y el
// cuerpo que reciba la prueba. Devuelve la línea de petición para comprobarla.
func servirUnaPuente(conn net.Conn, estado, cuerpo string) string {
	defer conn.Close()
	lector := bufio.NewReader(conn)
	linea, _ := lector.ReadString('\n')
	for {
		l, err := lector.ReadString('\n')
		if err != nil || l == "\r\n" || l == "\n" {
			break
		}
	}
	fmt.Fprintf(conn, "HTTP/1.1 %s\r\nContent-Length: %d\r\n\r\n%s", estado, len(cuerpo), cuerpo)
	return linea
}

func TestPuenteWispObtenerConConn(t *testing.T) {
	cliente, servidor := net.Pipe()
	peticion := make(chan string, 1)
	go func() {
		peticion <- servirUnaPuente(servidor, "200 OK", "hola!")
	}()

	respuesta, err := obtenerConConn(func() (net.Conn, error) { return cliente, nil }, coordPuentePrueba)
	cliente.Close()
	if err != nil {
		t.Fatalf("obtenerConConn falló: %v", err)
	}
	if linea := <-peticion; !strings.HasPrefix(linea, "GET /maven2/com/ejemplo/artefacto/1.0/artefacto-1.0.jar HTTP/1.1") {
		t.Fatalf("la petición no es el GET esperado: %q", linea)
	}
	if string(respuesta.Bytes) != "hola!" {
		t.Fatalf("los bytes devueltos son %q", respuesta.Bytes)
	}
	suma := sha256.Sum256([]byte("hola!"))
	if respuesta.SHA256 != hex.EncodeToString(suma[:]) {
		t.Fatalf("sha256 %q no coincide con el del cuerpo", respuesta.SHA256)
	}
	if respuesta.Ruta != "/deps/com/ejemplo/artefacto/1.0/artefacto-1.0.jar" {
		t.Fatalf("ruta canónica %q no válida", respuesta.Ruta)
	}
}

func TestPuenteWispObtenerConConnPruebaExtensiones(t *testing.T) {
	respuestas := []struct {
		estado string
		cuerpo string
	}{
		{"404 Not Found", ""},
		{"404 Not Found", ""},
		{"200 OK", "pom"},
	}
	esperadas := []string{
		"GET /maven2/com/ejemplo/artefacto/1.0/artefacto-1.0.jar HTTP/1.1",
		"GET /maven2/com/ejemplo/artefacto/1.0/artefacto-1.0.aar HTTP/1.1",
		"GET /maven2/com/ejemplo/artefacto/1.0/artefacto-1.0.pom HTTP/1.1",
	}
	indice := 0
	lineas := make([]<-chan string, 0, len(esperadas))
	respuesta, err := obtenerConConn(func() (net.Conn, error) {
		cliente, servidor := net.Pipe()
		linea := make(chan string, 1)
		lineas = append(lineas, linea)
		respuesta := respuestas[indice]
		indice++
		go func() {
			linea <- servirUnaPuente(servidor, respuesta.estado, respuesta.cuerpo)
		}()
		return cliente, nil
	}, coordPuentePrueba)
	if err != nil {
		t.Fatalf("obtenerConConn falló tras probar extensiones: %v", err)
	}
	if string(respuesta.Bytes) != "pom" ||
		respuesta.Ruta != "/deps/com/ejemplo/artefacto/1.0/artefacto-1.0.pom" {
		t.Fatalf("respuesta inesperada: %q en %q", respuesta.Bytes, respuesta.Ruta)
	}
	for i, esperada := range esperadas {
		if linea := <-lineas[i]; !strings.HasPrefix(linea, esperada) {
			t.Fatalf("la petición %d no es el GET esperado: %q", i, linea)
		}
	}
}

func TestPuenteWispObtenerConConnSinRespuestaParcial(t *testing.T) {
	casos := []struct {
		nombre   string
		cabecera string
		cuerpo   string
	}{
		{"sin 200", "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n", ""},
		{"cuerpo por encima del tope", "HTTP/1.1 200 OK\r\nContent-Length: 67108865\r\n\r\n", ""},
		{"cuerpo cortado", "HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\n", "abc"},
		{"chunked", "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nabc\r\n0\r\n\r\n", ""},
		{"sin Content-Length", "HTTP/1.1 200 OK\r\n\r\n", "abc"},
		{"estado inventado", "HTTP/1.1 999 ?\r\nContent-Length: 0\r\n\r\n", ""},
		{"CL 3 / cuerpo pommas", "HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\n", "pommas"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			respuesta, err := obtenerConConn(func() (net.Conn, error) {
				cliente, servidor := net.Pipe()
				go func() {
					defer servidor.Close()
					lector := bufio.NewReader(servidor)
					lector.ReadString('\n')
					for {
						l, err := lector.ReadString('\n')
						if err != nil || l == "\r\n" || l == "\n" {
							break
						}
					}
					io.WriteString(servidor, caso.cabecera+caso.cuerpo)
				}()
				return cliente, nil
			}, coordPuentePrueba)
			if err == nil {
				t.Fatal("la respuesta debía rechazarse sin devolver bytes")
			}
			if respuesta.Bytes != nil {
				t.Fatalf("se devolvieron %d bytes de una respuesta rechazada", len(respuesta.Bytes))
			}
		})
	}
}
