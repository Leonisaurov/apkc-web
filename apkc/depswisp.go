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
	"strconv"
	"strings"
	"time"
)

// Errores centinela de las allowlists. Se envuelven con el detalle del host o
// del destino rechazado, para que la interfaz pueda distinguir un rechazo de
// política de un fallo de red y enseñar el motivo sin interpretar el texto.
var (
	ErrRelayNoAutorizado   = errors.New("wisp: relay no autorizado")
	ErrDestinoNoAutorizado = errors.New("wisp: destino no autorizado")
	ErrCoordenadaInvalida  = errors.New("wisp: coordenada Maven inválida")
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

// destinoWISP es una pareja host/puerto que el camino seguro puede abrir.
type destinoWISP struct {
	Host   string
	Puerto uint16
}

// destinosWISP son los destinos que `AbrirTLSWISP` abre por defecto: los dos
// espejos de Maven Central que se midieron durante la investigación. El puerto es
// 443 porque encima del flujo WISP se monta TLS de punta a punta: el relay solo
// ve registros cifrados y no puede leer ni alterar los artefactos. Abrir el mismo
// host en un puerto en claro (80, o cualquier otro) delataría el tráfico al relay
// y rompería esa promesa, así que la lista autoriza la pareja host:puerto, no el
// host suelto.
var destinosWISP = []destinoWISP{
	{Host: "repo1.maven.org", Puerto: 443},
	{Host: "repo.maven.apache.org", Puerto: 443},
}

// DestinosWISP devuelve una copia de los destinos autorizados por defecto.
func DestinosWISP() []destinoWISP {
	return append([]destinoWISP(nil), destinosWISP...)
}

// listaDestinos autoriza esta pareja host/puerto si está en la lista base o, con
// `PermitirDestinosPersonalizados`, en `DestinosExtra`. La comparación no
// distingue mayúsculas ni un punto final en el host, que son la misma dirección
// DNS, pero exige el puerto exacto.
func (c ConfigWISP) listaDestinos(host string, puerto uint16) bool {
	normalizado := normalizarHost(host)
	for _, d := range destinosWISP {
		if normalizado == d.Host && puerto == d.Puerto {
			return true
		}
	}
	if !c.PermitirDestinosPersonalizados {
		return false
	}
	for _, entrada := range c.DestinosExtra {
		h, p, err := parseDestinoExtra(entrada)
		if err != nil {
			continue
		}
		if h == normalizado && p == puerto {
			return true
		}
	}
	return false
}

// normalizarHost deja el nombre comparable: minúsculas y sin el punto final que
// a veces se escribe para fijar la raíz DNS.
func normalizarHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// parseDestinoExtra exige la forma «host:puerto», con el puerto explícito y en
// decimal canónico: un destino sin puerto sería ambiguo (¿443 por defecto?) y un
// «0443» es la misma cifra escrita de otra forma, que no debe colarse por la
// lista. Además, la ampliación es para otros espejos de Maven: se rechazan los
// literales IP, el loopback y los comodines, porque apuntarían a la red del
// usuario a través del relay, y cualquier puerto que no sea 443, porque el camino
// seguro monta TLS de punta a punta.
func parseDestinoExtra(entrada string) (string, uint16, error) {
	host, textoPuerto, err := net.SplitHostPort(entrada)
	if err != nil {
		return "", 0, fmt.Errorf("wisp: destino extra %q inválido: se espera host:puerto: %w", entrada, err)
	}
	host = normalizarHost(host)
	if !hostMavenValido(host) {
		return "", 0, fmt.Errorf("wisp: destino extra %q: se espera un nombre DNS con punto; no se admiten IP, loopback ni comodines", entrada)
	}
	puerto, err := strconv.ParseUint(textoPuerto, 10, 16)
	if err != nil || puerto == 0 {
		return "", 0, fmt.Errorf("wisp: destino extra %q con puerto %q no válido (1-65535)", entrada, textoPuerto)
	}
	if textoPuerto != strconv.FormatUint(puerto, 10) {
		return "", 0, fmt.Errorf("wisp: destino extra %q: el puerto debe escribirse en decimal canónico, sin ceros a la izquierda", entrada)
	}
	if puerto != 443 {
		return "", 0, fmt.Errorf("wisp: destino extra %q: el camino seguro solo abre el puerto 443 (TLS)", entrada)
	}
	return host, uint16(puerto), nil
}

// hostMavenValido exige un nombre DNS con al menos un punto y etiquetas válidas.
// Deja fuera los literales IP, `localhost` y los comodines: no son espejos de
// Maven y convertirían la ampliación en una puerta hacia la red local.
func hostMavenValido(host string) bool {
	if host == "" || net.ParseIP(host) != nil || host == "localhost" {
		return false
	}
	if !strings.Contains(host, ".") {
		return false
	}
	for _, etiqueta := range strings.Split(host, ".") {
		if etiqueta == "" || len(etiqueta) > 63 {
			return false
		}
		if etiqueta[0] == '-' || etiqueta[len(etiqueta)-1] == '-' {
			return false
		}
		for i := 0; i < len(etiqueta); i++ {
			c := etiqueta[i]
			if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
				continue
			}
			return false
		}
	}
	return true
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

	// PermitirRelaysPersonalizados abre la allowlist de relays. Sin esto solo
	// valen las seis URLs verificadas por handshake: un relay ajeno vería a qué
	// hosts se conecta el usuario con su reputación. Se activa a mano, igual que
	// el módulo.
	PermitirRelaysPersonalizados bool

	// PermitirDestinosPersonalizados abre la allowlist de destinos y habilita
	// `DestinosExtra`. Sin esto, el camino seguro solo abre los dos espejos de
	// Maven Central en 443.
	PermitirDestinosPersonalizados bool

	// DestinosExtra amplía la allowlist cuando `PermitirDestinosPersonalizados`
	// está activo. Cada entrada es «host:puerto», con el puerto explícito.
	DestinosExtra []string
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

// Validar comprueba que el módulo está activado, que la URL del relay tiene la
// forma que exige WISP (WebSocket seguro y terminado en «/»), que el relay está
// en la allowlist y que los destinos extra, si los hay, están bien formados.
func (c ConfigWISP) Validar() error {
	if !c.Habilitado {
		return errors.New("wisp: el módulo está desactivado (ConfigWISP.Habilitado = false)")
	}
	u, err := url.Parse(c.Relay)
	if err != nil {
		return fmt.Errorf("wisp: URL de relay inválida: %w", err)
	}
	if err := validarFormaRelay(u, c.Relay); err != nil {
		return err
	}
	if err := c.validarRelayAutorizado(u); err != nil {
		return err
	}
	return c.validarDestinosExtra()
}

// validarFormaRelay comprueba la forma de la URL sin mirar la allowlist. Está
// separada para que las pruebas puedan falsificar que un relay ajeno pasa todos
// los controles de forma y solo lo rechaza la lista.
func validarFormaRelay(u *url.URL, relay string) error {
	// `url.Parse` no distingue un «#» o un «?» vacíos de su ausencia, pero el
	// navegador sí: `new WebSocket("…/wisp/#")` lanza SyntaxError. Se rechaza el
	// carácter literal antes de mirar el resto, y así el error nombra el motivo.
	if strings.ContainsAny(relay, "#?") {
		return fmt.Errorf("wisp: la URL del relay no admite «#» ni «?»; recibido %q", relay)
	}
	if u.User != nil {
		return fmt.Errorf("wisp: la URL del relay no admite credenciales de usuario; recibido %q", relay)
	}
	if u.Scheme != "wss" {
		if u.Scheme != "ws" || !esHostLocal(u.Hostname()) {
			return fmt.Errorf("wisp: el relay debe usar wss:// (o ws:// en localhost); recibido %q", relay)
		}
	}
	if u.Host == "" {
		return errors.New("wisp: la URL del relay no tiene host")
	}
	if !strings.HasSuffix(u.Path, "/") {
		return fmt.Errorf("wisp: la URL del relay debe acabar en «/»; recibido %q", relay)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("wisp: la URL del relay no admite query ni fragmento")
	}
	return nil
}

// validarRelayAutorizado exige que la URL sea una de las verificadas, salvo que
// el usuario haya activado `PermitirRelaysPersonalizados`. La comparación es por
// URL canónica (esquema, autoridad y ruta), no por host suelto: cambiar la ruta
// de un relay conocido también lo convierte en otro relay.
func (c ConfigWISP) validarRelayAutorizado(u *url.URL) error {
	if c.PermitirRelaysPersonalizados {
		return nil
	}
	canonica := relayCanonico(u)
	for _, autorizado := range relaysWISP {
		au, err := url.Parse(autorizado)
		if err != nil {
			continue
		}
		if relayCanonico(au) == canonica {
			return nil
		}
	}
	return fmt.Errorf("%w: el relay %q (host %q) no figura entre los %d verificados; activa ConfigWISP.PermitirRelaysPersonalizados para usar uno propio",
		ErrRelayNoAutorizado, c.Relay, u.Hostname(), len(relaysWISP))
}

// relayCanonico reduce una URL de relay a la forma comparable: minúsculas en el
// esquema y el host, sin el puerto que ya implica el esquema y sin el punto final
// del host. La ruta se compara con `EscapedPath`, no con `Path`: `/%77isp/` se
// decodifica a `/wisp/`, pero el navegador pide una ruta distinta y no debe
// colarse por la lista.
func relayCanonico(u *url.URL) string {
	esquema := strings.ToLower(u.Scheme)
	host := normalizarHost(u.Hostname())
	puerto := u.Port()
	if (esquema == "wss" && puerto == "443") || (esquema == "ws" && puerto == "80") {
		puerto = ""
	}
	autoridad := host
	if puerto != "" {
		autoridad = net.JoinHostPort(host, puerto)
	}
	return esquema + "://" + autoridad + u.EscapedPath()
}

// validarDestinosExtra comprueba la forma de la lista ampliada. Una lista puesta
// sin el interruptor es un error y no un campo ignorado: quien la escribió cree
// que autoriza destinos y no lo hace.
func (c ConfigWISP) validarDestinosExtra() error {
	if len(c.DestinosExtra) == 0 {
		return nil
	}
	if !c.PermitirDestinosPersonalizados {
		return errors.New("wisp: ConfigWISP.DestinosExtra exige PermitirDestinosPersonalizados = true")
	}
	for _, entrada := range c.DestinosExtra {
		if _, _, err := parseDestinoExtra(entrada); err != nil {
			return err
		}
	}
	return nil
}

// ValidarDestino decide si el camino seguro puede abrir `host:puerto`. Se llama
// antes de mandar el CONNECT, de modo que un destino prohibido no llega a tocar
// el relay. El mensaje nombra el destino y el motivo.
func (c ConfigWISP) ValidarDestino(host string, puerto uint16) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("%w: host vacío", ErrDestinoNoAutorizado)
	}
	if puerto == 0 {
		return fmt.Errorf("%w: %s:0 no es un puerto válido", ErrDestinoNoAutorizado, host)
	}
	if c.listaDestinos(host, puerto) {
		return nil
	}
	permitidos := make([]string, 0, len(destinosWISP)+len(c.DestinosExtra))
	for _, d := range destinosWISP {
		permitidos = append(permitidos, fmt.Sprintf("%s:%d", d.Host, d.Puerto))
	}
	if c.PermitirDestinosPersonalizados {
		permitidos = append(permitidos, c.DestinosExtra...)
	}
	return fmt.Errorf("%w: el destino %s:%d no está autorizado; el camino seguro solo abre %s",
		ErrDestinoNoAutorizado, host, puerto, strings.Join(permitidos, ", "))
}

func esHostLocal(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}

// maxLargoCampoMaven acota cada campo de la coordenada. Los nombres reales de
// Maven no pasan de unas decenas de caracteres; un campo enorme solo sirve para
// agotar memoria o para colar bytes raros al construir rutas.
const maxLargoCampoMaven = 128

// extensionesMaven son las únicas que el camino seguro acepta al nombrar el
// artefacto. Se listan en vez de aceptar cualquier sufijo para que la extensión
// tampoco sea una vía de escape.
var extensionesMaven = map[string]bool{"aar": true, "jar": true, "pom": true}

// campoMavenValido acepta letras ASCII, dígitos, punto, guion y guion bajo; el
// más (+) solo en la versión, donde Maven lo usa. Nada de «/», «\», «:», «..»,
// espacios, controles ni caracteres de otros alfabetos: cualquier cosa de esas
// podría cambiar la ruta que se construye después.
func campoMavenValido(campo string, permiteMas bool) bool {
	if campo == "" || len(campo) > maxLargoCampoMaven {
		return false
	}
	for i := 0; i < len(campo); i++ {
		c := campo[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		case permiteMas && c == '+':
		default:
			return false
		}
	}
	return true
}

// grupoMavenValido exige segmentos no vacíos separados por un punto, de modo que
// «com..example», «.com» o «com.» se rechazan y el grupo se puede convertir en
// directorios sin ambigüedad.
func grupoMavenValido(grupo string) bool {
	if !campoMavenValido(grupo, false) {
		return false
	}
	for _, segmento := range strings.Split(grupo, ".") {
		if segmento == "" {
			return false
		}
	}
	return true
}

// ValidarCoordenadaMaven comprueba que la coordenada tiene exactamente la forma
// `grupo:artefacto:version` y que cada campo solo lleva caracteres permitidos.
// Rechaza `/`, `..`, esquemas de URL y cualquier otro adorno antes de que la
// coordenada se use para construir una ruta.
func ValidarCoordenadaMaven(coordenada string) (grupo, artefacto, version string, err error) {
	partes := strings.Split(coordenada, ":")
	if len(partes) != 3 {
		return "", "", "", fmt.Errorf("%w: %q no tiene la forma grupo:artefacto:version", ErrCoordenadaInvalida, coordenada)
	}
	grupo, artefacto, version = partes[0], partes[1], partes[2]
	if !grupoMavenValido(grupo) {
		return "", "", "", fmt.Errorf("%w: el grupo %q no es válido", ErrCoordenadaInvalida, grupo)
	}
	if !campoMavenValido(artefacto, false) || artefacto == "." || artefacto == ".." {
		return "", "", "", fmt.Errorf("%w: el artefacto %q no es válido", ErrCoordenadaInvalida, artefacto)
	}
	if !campoMavenValido(version, true) || version == "." || version == ".." {
		return "", "", "", fmt.Errorf("%w: la versión %q no es válida", ErrCoordenadaInvalida, version)
	}
	return grupo, artefacto, version, nil
}

// RutaDependenciaMaven construye la ruta del artefacto bajo `deps/`, con el
// diseño de Maven Central: `deps/<grupo>/<artefacto>/<versión>/<artefacto>-<versión>.<ext>`.
// La validación va antes de unir nada: un campo hostil no llega a formar parte de
// la ruta y, por si acaso, el resultado se comprueba otra vez contra el traversal.
func RutaDependenciaMaven(coordenada, extension string) (string, error) {
	if !extensionesMaven[extension] {
		return "", fmt.Errorf("%w: la extensión %q no está permitida (aar, jar o pom)", ErrCoordenadaInvalida, extension)
	}
	grupo, artefacto, version, err := ValidarCoordenadaMaven(coordenada)
	if err != nil {
		return "", err
	}
	ruta := "deps/" + strings.ReplaceAll(grupo, ".", "/") + "/" + artefacto + "/" + version + "/" + artefacto + "-" + version + "." + extension
	if contieneTraversal(ruta) {
		return "", fmt.Errorf("%w: la ruta %q se sale de deps/", ErrCoordenadaInvalida, ruta)
	}
	return ruta, nil
}

// contieneTraversal busca un componente «..» en una ruta con separador «/».
// Es la última red: la validación de campos ya lo impide, pero un cambio futuro
// en `campoMavenValido` no debe convertir esta función en una puerta abierta.
func contieneTraversal(ruta string) bool {
	for _, parte := range strings.Split(ruta, "/") {
		if parte == ".." {
			return true
		}
	}
	return false
}

// ConectarWISP abre el WebSocket del relay y devuelve un cliente multiplexado.
// Valida la configuración antes de tocar la red.
func ConectarWISP(cfg ConfigWISP) (*ClienteWISP, error) {
	if err := cfg.Validar(); err != nil {
		return nil, err
	}
	transporte, err := AbrirWebSocketWISP(cfg)
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
	// El destino se comprueba antes de tocar el almacén de CAs y antes de mandar
	// el CONNECT: un puerto prohibido no debe llegar al relay ni siquiera como
	// intento.
	if err := cfg.ValidarDestino(host, puerto); err != nil {
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

	bruto, err := cliente.AbrirTCPAutorizado(cfg, host, puerto)
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
