package main

// Módulo WISP opt-in para las descargas de `apkc-deps` en la pestaña.
//
// El problema que resuelve: Maven Central y Google Maven no mandan
// `Access-Control-Allow-Origin`, así que un `fetch` desde la página falla por
// CORS. WISP abre TCP sobre un WebSocket contra un relay, y sobre ese TCP se
// monta TLS en Go (`crypto/tls`) para que el relay vea solo bytes cifrados y no
// pueda leer ni alterar los artefactos.
//
// El módulo está desactivado por defecto: `ConfigWISP.Habilitado` empieza en
// false y `Validar`/`AbrirTLSWISP` rechazan la conexión mientras siga así. La
// interfaz debe enseñar `AvisoWISP` y pedir confirmación antes de activarlo.
//
// Atribución del protocolo (CC-BY-4.0), como exige su licencia:
//
//	WISP v1.2, Copyright (c) ading2210, CC-BY-4.0
//	https://github.com/MercuryWorkshop/wisp-protocol
//
// Esta implementación no importa `wisp-js` (LGPL-3.0), ni `libcurl.js`, ni
// `epoxy`: el framing, el control de flujo y el TLS son código de este repo.

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// AtribucionWISP es el crédito que exige la licencia CC-BY-4.0 de la spec.
const AtribucionWISP = "WISP v1.2, Copyright (c) ading2210, CC-BY-4.0, https://github.com/MercuryWorkshop/wisp-protocol"

// AvisoWISP es el texto que la interfaz debe mostrar antes de activar el módulo.
// Dice lo que cambia y lo que cuesta, sin prometer que el relay sea de fiar: el
// TLS impide que lea el contenido, pero el relay sigue viendo a qué host se
// conecta y cuándo.
const AvisoWISP = `WISP permite abrir conexiones TCP desde esta pestaña para saltarse CORS al ` +
	`descargar dependencias Maven. Para ello usa un relay de terceros: se descargarán ` +
	`archivos extra y el relay verá a qué servidor te conectas, aunque el contenido viaja ` +
	`cifrado con TLS propio. El módulo está desactivado por defecto; actívalo solo si ` +
	`aceptas ese tráfico y confías en el relay elegido.`

// relaysWISP son los relays públicos que se verificaron por handshake durante la
// investigación. Sirven de lista inicial; la interfaz puede permitir editar la
// URL, y en el futuro se prevé un WISP local propio.
var relaysWISP = []string{
	"wss://wisp.mercurywork.shop/wisp/",
	"wss://terbiumon.top/wisp/",
	"wss://anura.pro/wisp/",
	"wss://lichology.com/wisp/",
	"wss://wisp.waved.site/w/",
	"wss://gointospace.app/wisp/",
}

// RelaysWISP devuelve una copia de la lista de relays conocidos.
func RelaysWISP() []string {
	return append([]string(nil), relaysWISP...)
}

// ConfigWISP configura una conexión. `Habilitado` es el interruptor opt-in y
// nace en false. `RootCAs` es obligatorio en `js/wasm`, donde no hay almacén del
// sistema; en nativo, si es nil se usa el del sistema.
//
// El almacén de `js/wasm` tiene que salir de un PEM local cuya huella sha256 esté
// fijada en la interfaz (o en este repo), no de una descarga: si el bundle
// llegara por el relay, el relay pasaría a ser una CA y podría firmar el
// certificado de cualquier host. `CargarCAsPEMFijadas` es la vía recomendada
// cuando se conoce la huella.
type ConfigWISP struct {
	Habilitado bool
	Relay      string
	RootCAs    *x509.CertPool

	// VerificacionInsegura desactiva la verificación del certificado. Solo para
	// pruebas locales: con esto, el relay podría hacer MITM.
	VerificacionInsegura bool

	// Plazo acota el handshake TLS. Cero usa el valor por defecto.
	Plazo time.Duration
}

// NuevaConfigWISP devuelve la configuración por defecto: desactivada y con el
// primer relay conocido como propuesta, para que activar sea un acto explícito.
func NuevaConfigWISP() ConfigWISP {
	return ConfigWISP{Relay: relaysWISP[0]}
}

// CargarCAsPEM construye un almacén a partir de uno o más bundles PEM. Es la vía
// para inyectar CAs en `js/wasm`, donde `PoolDeSistema` no existe.
//
// Es estricta a propósito: un bundle con un bloque que no sea `CERTIFICATE`, un
// certificado X.509 que no parsea o bytes que no son PEM es un error, no algo
// que se salte en silencio. Añadir un certificado de más al almacén equivale a
// confiar en quien lo puso; si el bundle viniera del relay, ese relay podría
// firmar el certificado de Maven o de Google. Nada se descarga aquí: el PEM lo
// pasa el llamador desde un fichero local verificado.
func CargarCAsPEM(bundles ...[]byte) (*x509.CertPool, error) {
	if len(bundles) == 0 {
		return nil, errors.New("wisp: no se recibió ningún bundle PEM")
	}
	pool := x509.NewCertPool()
	total := 0
	for i, bundle := range bundles {
		n, err := anadirCAsPEM(pool, bundle)
		if err != nil {
			return nil, fmt.Errorf("wisp: bundle PEM %d: %w", i+1, err)
		}
		total += n
	}
	if total == 0 {
		return nil, errors.New("wisp: no se recibió ningún certificado X.509 válido")
	}
	return pool, nil
}

// anadirCAsPEM recorre un bundle bloque a bloque. Devuelve cuántos certificados
// añadió y el primer defecto que encuentre, en vez de ignorarlo.
func anadirCAsPEM(pool *x509.CertPool, bundle []byte) (int, error) {
	resto := bundle
	añadidos := 0
	for {
		inicio := bytes.Index(resto, []byte("-----BEGIN "))
		if inicio < 0 {
			if len(bytes.TrimSpace(resto)) > 0 {
				return añadidos, fmt.Errorf("%d bytes al final que no son PEM", len(bytes.TrimSpace(resto)))
			}
			return añadidos, nil
		}
		if len(bytes.TrimSpace(resto[:inicio])) > 0 {
			return añadidos, fmt.Errorf("%d bytes antes del primer bloque que no son PEM", len(bytes.TrimSpace(resto[:inicio])))
		}
		bloque, siguiente := pem.Decode(resto[inicio:])
		if bloque == nil {
			return añadidos, errors.New("bloque PEM ilegible")
		}
		if bloque.Type != "CERTIFICATE" {
			return añadidos, fmt.Errorf("bloque PEM de tipo %q; solo se aceptan CERTIFICATE", bloque.Type)
		}
		cert, err := x509.ParseCertificate(bloque.Bytes)
		if err != nil {
			return añadidos, fmt.Errorf("certificado X.509 inválido: %w", err)
		}
		pool.AddCert(cert)
		añadidos++
		resto = siguiente
	}
}

// HuellaCAsPEM devuelve el sha256 del bundle tal cual, para fijarlo en el código
// y comprobarlo antes de construir el almacén.
func HuellaCAsPEM(bundle []byte) [sha256.Size]byte {
	return sha256.Sum256(bundle)
}

// CargarCAsPEMFijadas exige que el bundle tenga la huella esperada y después
// aplica las mismas comprobaciones que CargarCAsPEM. La huella se fija en el
// código o en la interfaz y el PEM se lee de un fichero local: así ni un cambio
// en el fichero ni un bundle servido por el relay pasan inadvertidos.
func CargarCAsPEMFijadas(bundle []byte, huella [sha256.Size]byte) (*x509.CertPool, error) {
	if real := HuellaCAsPEM(bundle); real != huella {
		return nil, fmt.Errorf("wisp: la huella sha256 del bundle de CAs no coincide: %x != %x", real, huella)
	}
	return CargarCAsPEM(bundle)
}

// Validar comprueba que el módulo está activado y que la URL del relay tiene la
// forma que exige WISP: WebSocket seguro y terminado en «/».
func (c ConfigWISP) Validar() error {
	if !c.Habilitado {
		return errors.New("wisp: el módulo está desactivado (ConfigWISP.Habilitado = false)")
	}
	u, err := url.Parse(c.Relay)
	if err != nil {
		return fmt.Errorf("wisp: URL de relay inválida: %w", err)
	}
	if u.Scheme != "wss" {
		if u.Scheme != "ws" || !esHostLocal(u.Hostname()) {
			return fmt.Errorf("wisp: el relay debe usar wss:// (o ws:// en localhost); recibido %q", c.Relay)
		}
	}
	if u.Host == "" {
		return errors.New("wisp: la URL del relay no tiene host")
	}
	if !strings.HasSuffix(u.Path, "/") {
		return fmt.Errorf("wisp: la URL del relay debe acabar en «/»; recibido %q", c.Relay)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("wisp: la URL del relay no admite query ni fragmento")
	}
	return nil
}

func esHostLocal(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}

// ConectarWISP abre el WebSocket del relay y devuelve un cliente multiplexado.
// Valida la configuración antes de tocar la red.
func ConectarWISP(cfg ConfigWISP) (*ClienteWISP, error) {
	if err := cfg.Validar(); err != nil {
		return nil, err
	}
	transporte, err := AbrirWebSocketWISP(cfg.Relay)
	if err != nil {
		return nil, err
	}
	return NuevoClienteWISP(transporte), nil
}

// AbrirTLSWISP abre un flujo hacia `host:puerto` y monta TLS dentro del cliente:
// el handshake termina aquí y lo único que viaja por el WebSocket son registros
// TLS cifrados. Devuelve el `net.Conn` ya autenticado.
func AbrirTLSWISP(cliente *ClienteWISP, host string, puerto uint16, cfg ConfigWISP) (net.Conn, error) {
	if err := cfg.Validar(); err != nil {
		return nil, err
	}
	raices := cfg.RootCAs
	if raices == nil {
		var err error
		raices, err = PoolDeSistema()
		if err != nil {
			return nil, err
		}
	}

	bruto, err := cliente.AbrirTCP(host, puerto)
	if err != nil {
		return nil, fmt.Errorf("wisp: no se pudo abrir el flujo hacia %s:%d: %w", host, puerto, err)
	}

	plazo := cfg.Plazo
	if plazo <= 0 {
		plazo = 30 * time.Second
	}
	_ = bruto.SetDeadline(time.Now().Add(plazo))

	conf := &tls.Config{
		ServerName:         host,
		RootCAs:            raices,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.VerificacionInsegura,
	}
	conexion := tls.Client(bruto, conf)
	if err := conexion.Handshake(); err != nil {
		_ = bruto.Close()
		return nil, fmt.Errorf("wisp: handshake TLS con %s:%d: %w", host, puerto, err)
	}
	_ = conexion.SetDeadline(time.Time{})
	return conexion, nil
}
