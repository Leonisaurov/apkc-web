package main

// Pruebas del endurecimiento posterior a la re-auditoría: no copiar para
// rechazar (A), contabilidad honesta de un flujo cerrado (B), drenaje con el
// plazo del Write (C), el CLOSE de desborde sin parar el reparto (D) y la
// entrada pública de WISP exigiendo la configuración validada (E).

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWISPPonerNoCopiaAlRechazar es la sonda del auditor: `poner` de 48 MiB
// devuelve ErrColaDesbordada sin que el heap crezca de forma significativa. La
// comprobación va antes de la copia, así que rechazar N bytes no cuesta N.
func TestWISPPonerNoCopiaAlRechazar(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("sinhueco.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)
	_ = relay

	const tam = 48 << 20
	trampa := make([]byte, tam)

	runtime.GC()
	var antes, despues runtime.MemStats
	runtime.ReadMemStats(&antes)
	errPoner := f.cola.poner(trampa)
	runtime.ReadMemStats(&despues)

	if !errors.Is(errPoner, ErrColaDesbordada) {
		t.Fatalf("poner 48 MiB = %v, se esperaba ErrColaDesbordada", errPoner)
	}
	dif := despues.TotalAlloc - antes.TotalAlloc
	if dif > 1<<20 {
		t.Fatalf("poner rechazó %d B pero asignó %d B (TotalAlloc)", tam, dif)
	}
	if enCola := bytesEnCola(f.cola); enCola != 0 {
		t.Fatalf("el bloque rechazado dejó %d B en la cola", enCola)
	}
	t.Logf("poner(%d B) rechazado con TotalAlloc +%d B", tam, dif)
}

// TestWISPComprobarMensajeAntesDeConvertir cubre la decisión que el transporte
// del navegador aplica sobre `byteLength` antes de crear el Uint8Array, más la
// lectura de la cabecera para saber qué flujo se cierra.
func TestWISPComprobarMensajeAntesDeConvertir(t *testing.T) {
	if err := comprobarMensajeWISP(48 << 20); !errors.Is(err, ErrColaDesbordada) {
		t.Fatalf("una trama de 48 MiB dio %v, se esperaba ErrColaDesbordada", err)
	}
	if err := comprobarMensajeWISP(topeMensajeWISP); err != nil {
		t.Fatalf("una trama de %d B se rechazó: %v", topeMensajeWISP, err)
	}
	if err := comprobarMensajeWISP(topeMensajeWISP + 1); err == nil {
		t.Fatal("una trama por encima del tope de mensaje pasó")
	}

	if flujo, ok := flujoDeTramaGrandeWISP(paqueteData(7, []byte("x"))); !ok || flujo != 7 {
		t.Fatalf("DATA del flujo 7 se leyó como (%d, %v)", flujo, ok)
	}
	coalescido := append(paqueteContinue(0, 128), paqueteData(9, nil)...)
	coalescido = append(paqueteClose(4, motivoCierreVoluntario), coalescido...)
	if flujo, ok := flujoDeTramaGrandeWISP(coalescido); !ok || flujo != 9 {
		t.Fatalf("cabecera coalescida se leyó como (%d, %v), se esperaba el DATA del flujo 9", flujo, ok)
	}
	if _, ok := flujoDeTramaGrandeWISP([]byte{0x02, 0x01}); ok {
		t.Fatal("una cabecera truncada no debía identificar flujo")
	}
}

// TestWISPContabilidadDeFlujoCerrado es la sonda B: un flujo cerrado con 272 KiB
// sin drenar mantiene esos bytes contados en el tope global hasta que se leen.
func TestWISPContabilidadDeFlujoCerrado(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("drenar.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	const trozos = 17
	trozo := bytes.Repeat([]byte{'z'}, 16<<10)
	esperado := trozos * len(trozo)
	for i := 0; i < trozos; i++ {
		relay.enviar(paqueteData(f.id, trozo))
	}
	esperarEnCola(t, f.cola, esperado)

	relay.enviar(paqueteClose(f.id, motivoCierreVoluntario))
	esperarColaCerrada(t, f.cola)
	if enUso := cliente.topeGlobal.enUso(); enUso != esperado {
		t.Fatalf("el global cuenta %d B con %d vivos en la cola de un flujo cerrado", enUso, esperado)
	}

	uno := make([]byte, 1)
	if n, err := conexion.Read(uno); n != 1 || err != nil {
		t.Fatalf("Read de 1 byte = (%d, %v)", n, err)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != esperado-1 {
		t.Fatalf("tras leer 1 byte el global cuenta %d B, se esperaban %d", enUso, esperado-1)
	}

	leidos := 1
	buf := make([]byte, 32<<10)
	for {
		n, errLectura := conexion.Read(buf)
		leidos += n
		if errLectura != nil {
			break
		}
	}
	if leidos != esperado {
		t.Fatalf("se drenaron %d B, se esperaban %d", leidos, esperado)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 0 {
		t.Fatalf("el global quedó en %d B tras drenar el flujo cerrado", enUso)
	}
	t.Logf("%d B siguieron contados hasta drenarse", esperado)
}

// TestWISPCloseSueltaPendientes comprueba el otro camino de salida: un Close
// propio descarta lo pendiente y devuelve su hueco de verdad.
func TestWISPCloseSueltaPendientes(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("abandonar.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	trozo := bytes.Repeat([]byte{'q'}, 8<<10)
	for i := 0; i < 4; i++ {
		relay.enviar(paqueteData(f.id, trozo))
	}
	esperarEnCola(t, f.cola, 4*len(trozo))

	if err := conexion.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if enCola := bytesEnCola(f.cola); enCola != 0 {
		t.Fatalf("Close dejó %d B en la cola", enCola)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 0 {
		t.Fatalf("Close dejó %d B contados en el tope global", enUso)
	}
}

// TestEsperarDrenajeRespetaPlazoDelWrite es la sonda C: el drenaje de
// bufferedAmount no puede pasarse del plazo del Write.
func TestEsperarDrenajeRespetaPlazoDelWrite(t *testing.T) {
	atascado := func() (int, bool) { return 2 << 20, true }

	inicio := time.Now()
	err := esperarDrenajeWISP(atascado, 1<<20, time.Now().Add(50*time.Millisecond), time.Now, time.Sleep)
	tardanza := time.Since(inicio)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("drenaje con plazo de 50 ms = %v, se esperaba os.ErrDeadlineExceeded", err)
	}
	if tardanza > 200*time.Millisecond {
		t.Fatalf("el drenaje tardó %v con un plazo de 50 ms", tardanza)
	}
	t.Logf("drenaje atascado cortado a los %v por el plazo del Write", tardanza)

	if err := esperarDrenajeWISP(atascado, 1<<20, time.Now().Add(-time.Second), time.Now, time.Sleep); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("un plazo ya vencido dio %v", err)
	}

	restantes := 3
	drena := func() (int, bool) {
		if restantes == 0 {
			return 0, true
		}
		restantes--
		return 2 << 20, true
	}
	if err := esperarDrenajeWISP(drena, 1<<20, time.Time{}, time.Now, func(time.Duration) {}); err != nil {
		t.Fatalf("con el búfer drenando el envío falló: %v", err)
	}
	if restantes != 0 {
		t.Fatalf("quedaron %d consultas sin hacer", restantes)
	}
}

// transporteTaponCLOSE bloquea el envío de CLOSE hasta que el test lo suelta,
// como un WebSocket con bufferedAmount alto.
type transporteTaponCLOSE struct {
	TransporteWISP
	bloqueado chan struct{}
	soltar    chan struct{}
	unaVez    sync.Once
}

func (t *transporteTaponCLOSE) Enviar(b []byte) error {
	if len(b) >= cabeceraWISP && b[0] == tipoWISPClose {
		t.unaVez.Do(func() { close(t.bloqueado) })
		select {
		case <-t.soltar:
		case <-time.After(3 * time.Second):
		}
	}
	return t.TransporteWISP.Enviar(b)
}

// TestWISPCLOSEDeDesbordeNoBloqueaElReparto es la sonda D: con el CLOSE atascado,
// los demás flujos siguen recibiendo datos.
func TestWISPCLOSEDeDesbordeNoBloqueaElReparto(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	tapon := &transporteTaponCLOSE{
		TransporteWISP: extremo,
		bloqueado:      make(chan struct{}),
		soltar:         make(chan struct{}),
	}
	cliente := NuevoClienteWISP(tapon)
	cliente.topeFlujo = 64 << 10
	defer cliente.Cerrar()
	defer close(tapon.soltar)

	desborda, err := cliente.abrirTCPCrudo("desborde.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP (desborde): %v", err)
	}
	vivo, err := cliente.abrirTCPCrudo("vivo.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP (vivo): %v", err)
	}
	fDesborda := desborda.(*flujoWISP)
	fVivo := vivo.(*flujoWISP)

	trozo := make([]byte, 16<<10)
	for i := 0; i < 8; i++ {
		relay.enviar(paqueteData(fDesborda.id, trozo))
	}
	esperarColaCerrada(t, fDesborda.cola)
	select {
	case <-tapon.bloqueado:
	case <-time.After(time.Second):
		t.Fatal("el cliente no intentó mandar el CLOSE de desborde")
	}

	relay.enviar(paqueteData(fVivo.id, []byte("vivo")))
	_ = vivo.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	leido := make([]byte, 4)
	if _, err := io.ReadFull(vivo, leido); err != nil {
		t.Fatalf("el reparto se paró esperando el CLOSE atascado: %v", err)
	}
	if string(leido) != "vivo" {
		t.Fatalf("el flujo vivo leyó %q", leido)
	}
}

// TestAbrirWebSocketWISPExigeConfigValidada es la sonda E: no hay puerta
// pública que se salte `ConfigWISP.Validar`.
func TestAbrirWebSocketWISPExigeConfigValidada(t *testing.T) {
	if _, err := AbrirWebSocketWISP(NuevaConfigWISP()); err == nil || !strings.Contains(err.Error(), "desactivado") {
		t.Fatalf("con el módulo desactivado dio %v", err)
	}
	cfg := NuevaConfigWISP()
	cfg.Habilitado = true
	cfg.Relay = "http://ejemplo.test/wisp/"
	if _, err := AbrirWebSocketWISP(cfg); err == nil || !strings.Contains(err.Error(), "wss") {
		t.Fatalf("con un relay inseguro dio %v", err)
	}
	cfg.Relay = RelaysWISP()[0]
	_, err := AbrirWebSocketWISP(cfg)
	if err == nil {
		t.Fatal("en nativo AbrirWebSocketWISP debía fallar tras validar")
	}
	if strings.Contains(err.Error(), "desactivado") || !strings.Contains(err.Error(), "GOOS=js") {
		t.Fatalf("la configuración válida no llegó al aviso de nativo: %v", err)
	}
}

// esperarEnCola espera a que la cola tenga exactamente `n` bytes.
func esperarEnCola(t *testing.T, q *colaBytes, n int) {
	t.Helper()
	limite := time.Now().Add(2 * time.Second)
	for time.Now().Before(limite) {
		if bytesEnCola(q) == n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("la cola tiene %d B, se esperaban %d", bytesEnCola(q), n)
}

// drenarCola lee hasta el error de cierre y comprueba el total.
func drenarCola(t *testing.T, conexion net.Conn, esperado int) {
	t.Helper()
	leidos := 0
	buf := make([]byte, 32<<10)
	for {
		n, err := conexion.Read(buf)
		leidos += n
		if err != nil {
			break
		}
	}
	if leidos != esperado {
		t.Fatalf("se drenaron %d B, se esperaban %d", leidos, esperado)
	}
}
