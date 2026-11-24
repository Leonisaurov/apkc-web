package main

// Pruebas de la condición F de la auditoría de seguridad de WISP: allowlist de
// relays, allowlist de destinos del camino seguro y validación de la coordenada
// Maven antes de construir una ruta bajo `deps/`.
//
// Cada lista se prueba en positivo (los valores autorizados pasan) y en negativo
// (un valor ajeno se rechaza con su motivo). Además, cada prueba de
// falsificación enseña que sin la lista se abriría el destino prohibido o se
// saldría de `deps/`, para que el verde no venga de un test que no mide nada.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// transporteGrabador implementa TransporteWISP y apunta cada mensaje enviado.
// Sirve para comprobar que un destino rechazado no llega a mandar el CONNECT y,
// en la falsificación, que la primitiva sin allowlist sí lo manda.
type transporteGrabador struct {
	mu       sync.Mutex
	enviados [][]byte

	entrada chan []byte
	cerrado chan struct{}
	unaVez  sync.Once
}

func nuevoTransporteGrabador() *transporteGrabador {
	t := &transporteGrabador{
		entrada: make(chan []byte, 8),
		cerrado: make(chan struct{}),
	}
	// El relay manda el CONTINUE inicial en cuanto conecta.
	t.entrada <- paqueteContinue(0, 128)
	return t
}

func (t *transporteGrabador) Enviar(b []byte) error {
	t.mu.Lock()
	t.enviados = append(t.enviados, append([]byte(nil), b...))
	t.mu.Unlock()
	return nil
}

func (t *transporteGrabador) Recibir() ([]byte, error) {
	select {
	case mensaje := <-t.entrada:
		return mensaje, nil
	case <-t.cerrado:
		return nil, net.ErrClosed
	}
}

func (t *transporteGrabador) Cerrar() error {
	t.unaVez.Do(func() { close(t.cerrado) })
	return nil
}

// conexiones devuelve los destinos de los CONNECT enviados.
func (t *transporteGrabador) conexiones(tb testing.TB) []destinoWISP {
	tb.Helper()
	t.mu.Lock()
	defer t.mu.Unlock()
	var destinos []destinoWISP
	for _, mensaje := range t.enviados {
		paquetes, err := decodificarPaquetes(mensaje)
		if err != nil {
			continue
		}
		for _, p := range paquetes {
			if p.tipo != tipoWISPConnect || len(p.carga) < 3 {
				continue
			}
			puerto := binary.LittleEndian.Uint16(p.carga[1:3])
			destinos = append(destinos, destinoWISP{Host: string(p.carga[3:]), Puerto: puerto})
		}
	}
	return destinos
}

// TestAllowlistRelaysConocidos es el positivo de la lista de relays: las seis
// URLs verificadas por handshake validan tal cual.
func TestAllowlistRelaysConocidos(t *testing.T) {
	relays := RelaysWISP()
	if len(relays) != 6 {
		t.Fatalf("se esperaban los 6 relays verificados, hay %d", len(relays))
	}
	for _, relay := range relays {
		cfg := ConfigWISP{Habilitado: true, Relay: relay}
		if err := cfg.Validar(); err != nil {
			t.Fatalf("el relay verificado %q fue rechazado: %v", relay, err)
		}
	}

	// RelaysWISP devuelve una copia: mutar el resultado no toca la lista del
	// núcleo, que es la que decide la allowlist.
	copia := RelaysWISP()
	copia[0] = "wss://mutado.test/wisp/"
	if RelaysWISP()[0] == "wss://mutado.test/wisp/" {
		t.Fatal("RelaysWISP permitió mutar la lista interna")
	}
}

// TestRelayAjenoRechazado es el negativo: un relay que no está entre los seis se
// rechaza nombrando su host, y cambiar la ruta de uno conocido también lo saca.
func TestRelayAjenoRechazado(t *testing.T) {
	const ajeno = "wss://relay-ajeno.test/wisp/"
	cfg := ConfigWISP{Habilitado: true, Relay: ajeno}
	err := cfg.Validar()
	if !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("Validar(%q) = %v, se esperaba ErrRelayNoAutorizado", ajeno, err)
	}
	for _, trozo := range []string{"relay-ajeno.test", "no autorizado", "PermitirRelaysPersonalizados"} {
		if !strings.Contains(err.Error(), trozo) {
			t.Fatalf("el mensaje %q no contiene %q", err.Error(), trozo)
		}
	}

	cfg.Relay = "wss://wisp.mercurywork.shop/otra-ruta/"
	if err := cfg.Validar(); !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("cambiar la ruta de un relay conocido debía sacarlo de la lista: %v", err)
	}
}

// TestRelayAjenoSoloLoParaLaLista falsifica el control: la URL del relay ajeno
// pasa todas las comprobaciones de forma, así que el rechazo viene de la
// allowlist y no de que la URL estuviera mal escrita.
func TestRelayAjenoSoloLoParaLaLista(t *testing.T) {
	const ajeno = "wss://relay-ajeno.test/wisp/"
	u, err := url.Parse(ajeno)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", ajeno, err)
	}
	if err := validarFormaRelay(u, ajeno); err != nil {
		t.Fatalf("la forma del relay ajeno debía ser válida; solo la lista lo rechaza: %v", err)
	}
	if err := (ConfigWISP{Habilitado: true, Relay: ajeno}).Validar(); !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("con la forma válida, el rechazo debía ser de allowlist: %v", err)
	}
	t.Logf("falsificación: %q pasa la forma y solo la allowlist lo rechaza", ajeno)
}

// TestRelayPersonalizadoSoloConInterruptor prueba que el relay propio exige el
// interruptor explícito, también para un relay local en claro.
func TestRelayPersonalizadoSoloConInterruptor(t *testing.T) {
	propio := ConfigWISP{Habilitado: true, Relay: "wss://local.test/wisp/"}
	if err := propio.Validar(); !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("sin el interruptor, un relay propio debía rechazarse: %v", err)
	}
	propio.PermitirRelaysPersonalizados = true
	if err := propio.Validar(); err != nil {
		t.Fatalf("con el interruptor, el relay propio debía valer: %v", err)
	}

	local := ConfigWISP{Habilitado: true, Relay: "ws://localhost:8000/wisp/"}
	if err := local.Validar(); !errors.Is(err, ErrRelayNoAutorizado) {
		t.Fatalf("sin el interruptor, un relay local tampoco está en la lista: %v", err)
	}
	local.PermitirRelaysPersonalizados = true
	if err := local.Validar(); err != nil {
		t.Fatalf("con el interruptor, el relay local debía valer: %v", err)
	}
}

// TestAllowlistDestinosPorDefecto es el positivo de destinos: los dos espejos de
// Maven Central en 443, con el host normalizado.
func TestAllowlistDestinosPorDefecto(t *testing.T) {
	destinos := DestinosWISP()
	if len(destinos) != 2 {
		t.Fatalf("se esperaban 2 destinos por defecto, hay %d", len(destinos))
	}
	cfg := ConfigWISP{Habilitado: true, Relay: RelaysWISP()[0]}
	for _, d := range destinos {
		if err := cfg.ValidarDestino(d.Host, d.Puerto); err != nil {
			t.Fatalf("el destino %s:%d fue rechazado: %v", d.Host, d.Puerto, err)
		}
	}
	if err := cfg.ValidarDestino("REPO1.MAVEN.ORG.", 443); err != nil {
		t.Fatalf("mayúsculas y punto final son el mismo host: %v", err)
	}
}

// TestDestinoProhibido es el negativo: el mismo host en otro puerto, o cualquier
// host ajeno, se rechaza antes de abrir nada y el mensaje lo nombra.
func TestDestinoProhibido(t *testing.T) {
	cfg := ConfigWISP{Habilitado: true, Relay: RelaysWISP()[0]}
	casos := []struct {
		host   string
		puerto uint16
	}{
		{"repo1.maven.org", 25},
		{"repo1.maven.org", 80},
		{"repo.maven.apache.org", 22},
		{"smtp-ajeno.test", 25},
		{"redis-ajeno.test", 6379},
		{"memcached-ajeno.test", 11211},
	}
	for _, caso := range casos {
		err := cfg.ValidarDestino(caso.host, caso.puerto)
		if !errors.Is(err, ErrDestinoNoAutorizado) {
			t.Fatalf("ValidarDestino(%s:%d) = %v, se esperaba ErrDestinoNoAutorizado", caso.host, caso.puerto, err)
		}
		esperado := fmt.Sprintf("%s:%d", caso.host, caso.puerto)
		if !strings.Contains(err.Error(), esperado) {
			t.Fatalf("el mensaje %q no nombra el destino %q", err.Error(), esperado)
		}
	}
}

// TestDestinosExtraSoloConInterruptor prueba la ampliación explícita: la lista
// sola no basta, y una entrada mal formada se detecta en Validar.
func TestDestinosExtraSoloConInterruptor(t *testing.T) {
	cfg := ConfigWISP{
		Habilitado:    true,
		Relay:         RelaysWISP()[0],
		DestinosExtra: []string{"maven.ejemplo.test:443"},
	}
	if err := cfg.Validar(); err == nil || !strings.Contains(err.Error(), "PermitirDestinosPersonalizados") {
		t.Fatalf("DestinosExtra sin interruptor debía ser un error explícito: %v", err)
	}
	if err := cfg.ValidarDestino("maven.ejemplo.test", 443); !errors.Is(err, ErrDestinoNoAutorizado) {
		t.Fatalf("sin interruptor, el destino extra debía rechazarse: %v", err)
	}

	cfg.PermitirDestinosPersonalizados = true
	if err := cfg.Validar(); err != nil {
		t.Fatalf("con interruptor, la configuración debía validar: %v", err)
	}
	if err := cfg.ValidarDestino("maven.ejemplo.test", 443); err != nil {
		t.Fatalf("con interruptor, el destino extra debía autorizarse: %v", err)
	}
	if err := cfg.ValidarDestino("maven.ejemplo.test", 80); !errors.Is(err, ErrDestinoNoAutorizado) {
		t.Fatalf("el puerto extra también cuenta: 80 debía rechazarse: %v", err)
	}
	if err := cfg.ValidarDestino("repo1.maven.org", 443); err != nil {
		t.Fatalf("la lista base sigue valiendo al ampliarla: %v", err)
	}

	malFormado := ConfigWISP{
		Habilitado:                     true,
		Relay:                          RelaysWISP()[0],
		PermitirDestinosPersonalizados: true,
		DestinosExtra:                  []string{"maven.ejemplo.test"},
	}
	if err := malFormado.Validar(); err == nil {
		t.Fatal("un destino extra sin puerto debía rechazarse")
	}
}

// TestDestinoProhibidoNoTocaElRelay demuestra que la comprobación ocurre antes
// de mandar el CONNECT: el transporte grabador no recibe ningún mensaje.
func TestDestinoProhibidoNoTocaElRelay(t *testing.T) {
	transporte := nuevoTransporteGrabador()
	cliente := NuevoClienteWISP(transporte)
	defer cliente.Cerrar()

	cfg := ConfigWISP{Habilitado: true, Relay: RelaysWISP()[0]}
	if _, err := AbrirTLSWISP(cliente, "smtp-ajeno.test", 25, cfg); !errors.Is(err, ErrDestinoNoAutorizado) {
		t.Fatalf("AbrirTLSWISP con destino prohibido = %v, se esperaba ErrDestinoNoAutorizado", err)
	}
	if destinos := transporte.conexiones(t); len(destinos) != 0 {
		t.Fatalf("el destino prohibido llegó a mandar %d CONNECT: %v", len(destinos), destinos)
	}
}

// TestFalsificacionDestinoProhibidoSeAbreSinLaLista es la falsificación de la
// allowlist de destinos: la primitiva de protocolo, sin la comprobación, manda
// el CONNECT al puerto prohibido. Solo la lista lo impide.
func TestFalsificacionDestinoProhibidoSeAbreSinLaLista(t *testing.T) {
	transporte := nuevoTransporteGrabador()
	cliente := NuevoClienteWISP(transporte)
	defer cliente.Cerrar()

	cfg := ConfigWISP{Habilitado: true, Relay: RelaysWISP()[0]}
	if _, err := cliente.AbrirTCPAutorizado(cfg, "smtp-ajeno.test", 25); !errors.Is(err, ErrDestinoNoAutorizado) {
		t.Fatalf("el camino seguro debía rechazar smtp-ajeno.test:25: %v", err)
	}

	conexion, err := cliente.AbrirTCP("smtp-ajeno.test", 25)
	if err != nil {
		t.Fatalf("AbrirTCP crudo: %v", err)
	}
	defer conexion.Close()

	destinos := transporte.conexiones(t)
	if len(destinos) != 1 {
		t.Fatalf("se esperaba un CONNECT, hubo %d: %v", len(destinos), destinos)
	}
	if destinos[0].Host != "smtp-ajeno.test" || destinos[0].Puerto != 25 {
		t.Fatalf("el CONNECT crudo salió a %s:%d, no al destino prohibido", destinos[0].Host, destinos[0].Puerto)
	}
	t.Logf("falsificación: sin allowlist el CONNECT salió a %s:%d", destinos[0].Host, destinos[0].Puerto)
}

// TestCoordenadaMavenValida es el positivo: la coordenada se convierte en una
// ruta bajo deps/ con el diseño de Maven Central.
func TestCoordenadaMavenValida(t *testing.T) {
	casos := []struct {
		coordenada string
		extension  string
		ruta       string
	}{
		{"com.example:app:1.0.0", "aar", "deps/com/example/app/1.0.0/app-1.0.0.aar"},
		{"androidx.core:core:1.12.0-SNAPSHOT", "jar", "deps/androidx/core/core/1.12.0-SNAPSHOT/core-1.12.0-SNAPSHOT.jar"},
		{"a_b-c.d:art_1:2.0+build", "pom", "deps/a_b-c/d/art_1/2.0+build/art_1-2.0+build.pom"},
	}
	for _, caso := range casos {
		ruta, err := RutaDependenciaMaven(caso.coordenada, caso.extension)
		if err != nil {
			t.Fatalf("RutaDependenciaMaven(%q): %v", caso.coordenada, err)
		}
		if ruta != caso.ruta {
			t.Fatalf("RutaDependenciaMaven(%q) = %q, se esperaba %q", caso.coordenada, ruta, caso.ruta)
		}
		if !strings.HasPrefix(ruta, "deps/") || contieneTraversal(ruta) {
			t.Fatalf("la ruta %q no está limpia bajo deps/", ruta)
		}
	}
}

// TestCoordenadaMavenHostil es el negativo: ni barras, ni URL, ni traversal, ni
// campos vacíos, ni caracteres fuera del alfabeto permitido.
func TestCoordenadaMavenHostil(t *testing.T) {
	casos := []string{
		"../../x",
		"http://servidor.test:app:1.0",
		"a/b:c:d",
		"com.example:../../x:1.0",
		"com.example:../../../../../x:1.0",
		"com.example:app:../../etc/passwd",
		"com..example:app:1.0",
		".com.example:app:1.0",
		"com.example.:app:1.0",
		":app:1.0",
		"com.example::1.0",
		"com.example:app:",
		"com.example:app:1.0:extra",
		"com.example:..:1.0",
		"com.example:app:..",
		" com.example:app:1.0",
		"com.example:app:1.0\n",
		"com.example:app:1.0%00",
		"com.example/ap:1.0",
	}
	for _, coordenada := range casos {
		if _, _, _, err := ValidarCoordenadaMaven(coordenada); !errors.Is(err, ErrCoordenadaInvalida) {
			t.Fatalf("ValidarCoordenadaMaven(%q) = %v, se esperaba ErrCoordenadaInvalida", coordenada, err)
		}
		if _, err := RutaDependenciaMaven(coordenada, "aar"); !errors.Is(err, ErrCoordenadaInvalida) {
			t.Fatalf("RutaDependenciaMaven(%q) = %v, se esperaba ErrCoordenadaInvalida", coordenada, err)
		}
	}
}

// TestExtensionMavenRechazada cierra la otra mitad de la ruta: la extensión no
// puede ser cualquier sufijo.
func TestExtensionMavenRechazada(t *testing.T) {
	for _, extension := range []string{"", "so", "aar/..", "../aar", "AAR", "aar\x00"} {
		if _, err := RutaDependenciaMaven("com.example:app:1.0", extension); !errors.Is(err, ErrCoordenadaInvalida) {
			t.Fatalf("extensión %q = %v, se esperaba ErrCoordenadaInvalida", extension, err)
		}
	}
}

// TestFalsificacionRutaSinValidacionSeSaleDeDeps es la falsificación de la
// coordenada: la misma unión ingenua que haría un llamador sin validar se sale
// de deps/, mientras que RutaDependenciaMaven la rechaza antes de unir.
func TestFalsificacionRutaSinValidacionSeSaleDeDeps(t *testing.T) {
	const coordenada = "com.example:../../../../../x:1.0"
	if _, err := RutaDependenciaMaven(coordenada, "aar"); !errors.Is(err, ErrCoordenadaInvalida) {
		t.Fatalf("la coordenada hostil debía rechazarse: %v", err)
	}

	partes := strings.Split(coordenada, ":")
	ingenua := filepath.Join("deps", strings.ReplaceAll(partes[0], ".", "/"),
		partes[1], partes[2], partes[1]+"-"+partes[2]+".aar")
	if strings.HasPrefix(ingenua, "deps/") {
		t.Fatalf("la unión ingenua no se salió de deps/: %q", ingenua)
	}
	t.Logf("falsificación: sin validar, %q se resuelve en %q", coordenada, ingenua)
}
