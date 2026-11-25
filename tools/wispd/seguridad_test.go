package main

// Falsaciones de la auditoría: destinos, memoria y tabla de motivos de cierre.

import (
	"context"
	"net"
	"net/http/httptest"
	"runtime/debug"
	"strconv"
	"testing"
	"time"
)

// Política inyectada solo desde los tests: permite el loopback de los servidores
// eco. No hay ninguna bandera que pueda activarla en producción.
var politicaPrueba = politicaDestinos{
	host: func(host string, puerto uint16, tipo byte) byte {
		if host == "127.0.0.1" || host == "::1" {
			return 0
		}
		return motivoDestinoBloqueado
	},
	ip: func(ip net.IP, puerto uint16, tipo byte) byte {
		if ip != nil && ip.IsLoopback() {
			return 0
		}
		return motivoDestinoBloqueado
	},
}

// cierreDeDestino conecta con un Origin válido y devuelve el primer paquete,
// que debe ser un CLOSE del destino denegado.
func cierreDeDestino(t *testing.T, url string, id uint32, tipo byte, host string, puerto uint16) paquete {
	t.Helper()
	cl := abrirCliente(t, url, origenProduccion)
	cl.leer() // CONTINUE inicial
	cl.connect(id, tipo, host, puerto)
	p := cl.leer()
	if p.tipo != tipoClose || p.streamID != id {
		t.Fatalf("destino %s:%d: tipo=0x%02x stream=%d, quería CLOSE en %d",
			host, puerto, p.tipo, p.streamID, id)
	}
	return p
}

func TestDestinoLoopbackDenegado(t *testing.T) {
	_, url := servidorPruebaProduccion(t)
	p := cierreDeDestino(t, url, 1, tipoFlujoTCP, "127.0.0.1", 443)
	if p.carga[0] != motivoInfoInvalida {
		t.Fatalf("127.0.0.1: motivo=0x%02x, quería 0x41", p.carga[0])
	}
}

func TestDestinoPrivadoDenegado(t *testing.T) {
	_, url := servidorPruebaProduccion(t)
	for i, host := range []string{"10.1.2.3", "172.16.5.4", "192.168.1.1", "169.254.1.1", "::1", "fd00::1"} {
		p := cierreDeDestino(t, url, uint32(i+1), tipoFlujoTCP, host, 443)
		if p.carga[0] != motivoInfoInvalida {
			t.Fatalf("%s: motivo=0x%02x, quería 0x41", host, p.carga[0])
		}
	}
}

func TestDestinoAjenoDenegado(t *testing.T) {
	_, url := servidorPruebaProduccion(t)
	p := cierreDeDestino(t, url, 1, tipoFlujoTCP, "mal.example", 443)
	if p.carga[0] != motivoDestinoBloqueado {
		t.Fatalf("destino ajeno: motivo=0x%02x, quería 0x48", p.carga[0])
	}
}

func TestPuertoDistintoDe443Denegado(t *testing.T) {
	_, url := servidorPruebaProduccion(t)
	p := cierreDeDestino(t, url, 1, tipoFlujoTCP, "repo1.maven.org", 80)
	if p.carga[0] != motivoInfoInvalida {
		t.Fatalf("puerto 80: motivo=0x%02x, quería 0x41", p.carga[0])
	}
}

func TestUDPSinDestinoAutorizado(t *testing.T) {
	_, url := servidorPruebaProduccion(t)
	p := cierreDeDestino(t, url, 1, tipoFlujoUDP, "repo1.maven.org", 443)
	if p.carga[0] != motivoDestinoBloqueado {
		t.Fatalf("UDP a 443: motivo=0x%02x, quería 0x48", p.carga[0])
	}
}

// servidorPruebaInyectado sustituye DNS y dial por falsificaciones: no consulta
// la red real, pero mantiene la política de producción del destino.
func servidorPruebaInyectado(t *testing.T,
	resolver func(context.Context, string) ([]net.IPAddr, error),
	dialer func(context.Context, string, string) (net.Conn, error),
) (*httptest.Server, string) {
	t.Helper()
	allow, err := nuevaAllowlist(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := nuevoServidor(allow, 64, 8)
	srv.resolver = resolver
	srv.dialer = dialer
	s := httptest.NewServer(srv)
	t.Cleanup(s.Close)
	return s, s.URL
}

// TestDestinoAutorizadoPermitido falsifica el caso que la auditoría encontró
// roto: los dos hosts autorizados resuelven a una IP pública y el CONNECT debe
// llegar a Dial, sin CLOSE, y hacer eco en el socket local del test.
func TestDestinoAutorizadoPermitido(t *testing.T) {
	hostLocal, puertoLocal := ecoTCP(t)
	local := net.JoinHostPort(hostLocal, strconv.Itoa(int(puertoLocal)))
	const ipPublica = "93.184.216.34"

	type llamada struct{ red, dir string }
	llamadas := make(chan llamada, 4)
	resolver := func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP(ipPublica)}}, nil
	}
	dialer := func(ctx context.Context, red, dir string) (net.Conn, error) {
		llamadas <- llamada{red: red, dir: dir}
		return net.DialTimeout("tcp", local, 2*time.Second)
	}

	_, url := servidorPruebaInyectado(t, resolver, dialer)
	cl := abrirCliente(t, url, origenProduccion)
	ch := lectorFondo(cl.ws)
	if p := esperarPaquete(t, ch); p.tipo != tipoContinue || p.streamID != 0 {
		t.Fatalf("CONTINUE inicial: tipo=0x%02x stream=%d", p.tipo, p.streamID)
	}

	for i, host := range []string{"repo1.maven.org", "repo.maven.apache.org"} {
		id := uint32(i + 1)
		cl.connect(id, tipoFlujoTCP, host, puertoDestino)
		cl.enviar(tipoData, id, []byte("hola"))

		p := esperarPaquete(t, ch)
		if p.tipo == tipoClose {
			motivo := byte(0)
			if len(p.carga) > 0 {
				motivo = p.carga[0]
			}
			t.Fatalf("%s: CLOSE inesperado con motivo 0x%02x", host, motivo)
		}
		if p.tipo != tipoData || p.streamID != id || string(p.carga) != "hola" {
			t.Fatalf("%s: paquete tipo=0x%02x stream=%d carga=%q", host, p.tipo, p.streamID, p.carga)
		}

		select {
		case ll := <-llamadas:
			if ll.red != "tcp" || ll.dir != ipPublica+":443" {
				t.Fatalf("%s: Dial(%s, %s), quería tcp y %s:443", host, ll.red, ll.dir, ipPublica)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no se llegó a Dial", host)
		}

		select {
		case lleg := <-ch:
			if lleg.err == nil && lleg.p.tipo == tipoClose && lleg.p.streamID == id {
				t.Fatalf("%s: CLOSE posterior al eco", host)
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestSoloDosDestinosEnProduccion(t *testing.T) {
	if len(destinosProduccion) != 2 ||
		destinosProduccion["repo1.maven.org"] != puertoDestino ||
		destinosProduccion["repo.maven.apache.org"] != puertoDestino {
		t.Fatalf("destinos de producción: %v", destinosProduccion)
	}
}

func TestLimiteMemoriaActivo(t *testing.T) {
	anterior := debug.SetMemoryLimit(limiteMemoria)
	t.Cleanup(func() { debug.SetMemoryLimit(anterior) })

	fijarLimiteMemoria()
	if actual := debug.SetMemoryLimit(limiteMemoria); actual != limiteMemoria {
		t.Fatalf("límite de memoria activo=%d, quería %d", actual, limiteMemoria)
	}
}

// La tabla la publica la spec v1.2 en
// https://github.com/MercuryWorkshop/wisp-protocol/blob/v1/protocol.md
func TestMotivosCierreSegunSpec(t *testing.T) {
	tabla := map[string]struct {
		valor    byte
		esperado byte
	}{
		"sin especificar":   {motivoCierreGenerico, 0x01},
		"voluntario":        {motivoCierreVoluntario, 0x02},
		"error de red":      {motivoErrorRed, 0x03},
		"info inválida":     {motivoInfoInvalida, 0x41},
		"host inalcanzable": {motivoHostInalcanzable, 0x42},
		"tiempo agotado":    {motivoTiempoAgotado, 0x43},
		"rechazado":         {motivoConexionRechazada, 0x44},
		"transferencia":     {motivoTransferenciaAgotada, 0x47},
		"bloqueado":         {motivoDestinoBloqueado, 0x48},
		"limitado":          {motivoLimitado, 0x49},
	}
	for nombre, entrada := range tabla {
		if entrada.valor != entrada.esperado {
			t.Errorf("%s: 0x%02x, la spec dice 0x%02x", nombre, entrada.valor, entrada.esperado)
		}
	}
}
