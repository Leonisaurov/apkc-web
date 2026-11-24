package main

// Pruebas de TLS sobre el net.Conn WISP. El servidor TLS es local y el relay
// captura todo lo que viaja por el WebSocket simulado: el test comprueba que el
// handshake termina dentro del cliente y que el relay solo ve registros TLS.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// certificadoDePrueba genera una CA autofirmada que también hace de servidor,
// con el hostname `ejemplo.test`.
func certificadoDePrueba(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generar clave: %v", err)
	}
	plantilla := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ejemplo.test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"ejemplo.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatalf("crear certificado: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsear certificado: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: clave}, pool
}

// servirTLSDevuelveEco arranca un servidor TLS sobre un pipe y devuelve el lado
// que el relay debe bombear. Todo lo que el cliente escriba se le devuelve.
func servirTLSDevuelveEco(t *testing.T, cert tls.Certificate) func(uint32) net.Conn {
	return func(uint32) net.Conn {
		ladoRelay, ladoServidor := net.Pipe()
		go func() {
			defer ladoServidor.Close()
			servidor := tls.Server(ladoServidor, &tls.Config{Certificates: []tls.Certificate{cert}})
			if err := servidor.Handshake(); err != nil {
				return
			}
			buf := make([]byte, 16<<10)
			for {
				n, err := servidor.Read(buf)
				if n > 0 {
					if _, errEscritura := servidor.Write(buf[:n]); errEscritura != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()
		return ladoRelay
	}
}

func configTLSDePrueba(pool *x509.CertPool) ConfigWISP {
	return ConfigWISP{
		Habilitado: true,
		Relay:      "wss://ejemplo.test/wisp/",
		RootCAs:    pool,
		Plazo:      5 * time.Second,
	}
}

func TestConfigWISPOptInPorDefecto(t *testing.T) {
	config := NuevaConfigWISP()
	if config.Habilitado {
		t.Fatal("el módulo WISP debe nacer desactivado")
	}
	if err := config.Validar(); err == nil {
		t.Fatal("Validar debía rechazar la configuración desactivada")
	}

	config.Habilitado = true
	if err := config.Validar(); err != nil {
		t.Fatalf("la configuración por defecto, una vez activada, no valida: %v", err)
	}
	if AvisoWISP == "" {
		t.Fatal("el aviso que debe ver el usuario está vacío")
	}
	relays := RelaysWISP()
	if len(relays) < 6 {
		t.Fatalf("se esperaban al menos 6 relays conocidos, hay %d", len(relays))
	}
	for _, relay := range relays {
		if !strings.HasPrefix(relay, "wss://") || !strings.HasSuffix(relay, "/") {
			t.Fatalf("relay con forma inválida: %q", relay)
		}
	}

	config.Relay = "wss://ejemplo.test/wisp"
	if err := config.Validar(); err == nil {
		t.Fatal("Validar debía exigir la barra final en la URL")
	}
	config.Relay = "http://ejemplo.test/wisp/"
	if err := config.Validar(); err == nil {
		t.Fatal("Validar debía rechazar un esquema que no es WebSocket")
	}
}

func TestWISPTLSCifraElFlujo(t *testing.T) {
	cert, raices := certificadoDePrueba(t)
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, servirTLSDevuelveEco(t, cert))
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()

	conexion, err := AbrirTLSWISP(cliente, "ejemplo.test", 443, configTLSDePrueba(raices))
	if err != nil {
		t.Fatalf("AbrirTLSWISP: %v", err)
	}
	defer conexion.Close()

	marcador := []byte("GET /privado/artefacto.aar HTTP/1.1\r\nHost: maven\r\n\r\n")
	if _, err := conexion.Write(marcador); err != nil {
		t.Fatalf("Write TLS: %v", err)
	}
	leido := make([]byte, len(marcador))
	if _, err := io.ReadFull(conexion, leido); err != nil {
		t.Fatalf("Read TLS: %v", err)
	}
	if !bytes.Equal(leido, marcador) {
		t.Fatalf("el eco TLS no coincide: %q", leido)
	}

	// El relay puede haber capturado el último registro después del eco; se le da
	// un instante antes de mirar.
	time.Sleep(50 * time.Millisecond)
	visto := relay.capturadoBytes()
	if len(visto) == 0 {
		t.Fatal("el relay no capturó ningún byte")
	}
	t.Logf("el relay vio %d bytes; primeros: % x", len(visto), visto[:minInt(len(visto), 16)])
	if bytes.Contains(visto, marcador) {
		t.Fatalf("el relay vio el plaintext en el flujo WISP (%d bytes)", len(visto))
	}
	if visto[0] != 0x16 {
		t.Fatalf("el primer byte del flujo es %#x, no un registro TLS (0x16)", visto[0])
	}
}

func TestWISPSinTLSElPlaintextAparece(t *testing.T) {
	// Falsificación del test anterior: sin TLS, el mismo marcador sí está en el
	// flujo. Así se demuestra que la comprobación detecta el plaintext de verdad.
	relay, extremo := nuevoRelayPrueba(t, 1<<20, true, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()

	conexion, err := cliente.AbrirTCP("ejemplo.test", 443)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	defer conexion.Close()

	marcador := []byte("GET /privado/artefacto.aar HTTP/1.1\r\nHost: maven\r\n\r\n")
	if _, err := conexion.Write(marcador); err != nil {
		t.Fatalf("Write: %v", err)
	}
	leido := make([]byte, len(marcador))
	if _, err := io.ReadFull(conexion, leido); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(leido, marcador) {
		t.Fatalf("el eco no coincide: %q", leido)
	}
	if !bytes.Contains(relay.capturadoBytes(), marcador) {
		t.Fatal("sin TLS el relay debía ver el plaintext y no lo vio")
	}
	t.Logf("sin TLS el relay vio el marcador de %d bytes en claro", len(marcador))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestWISPTLSRechazaCertificadoAjeno(t *testing.T) {
	cert, _ := certificadoDePrueba(t)
	_, extremo := nuevoRelayPrueba(t, 1<<20, false, servirTLSDevuelveEco(t, cert))
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()

	// Un almacén vacío: el certificado del servidor no es de fiar y el handshake
	// tiene que fallar dentro del cliente, antes de mandar datos.
	_, err := AbrirTLSWISP(cliente, "ejemplo.test", 443, configTLSDePrueba(x509.NewCertPool()))
	if err == nil {
		t.Fatal("el handshake aceptó un certificado que no estaba en el almacén")
	}
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "certificado") {
		t.Fatalf("error inesperado del handshake: %v", err)
	}
}
