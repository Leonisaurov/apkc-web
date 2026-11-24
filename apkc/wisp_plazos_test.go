package main

// Pruebas de que los plazos interrumpen la E/S pendiente, como exige net.Conn:
// fijar un plazo sobre un Read o un Write ya bloqueado tiene que despertarlo.

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestWISPSetReadDeadlineDespiertaLectura(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 8, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("mudo.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	listo := make(chan error, 1)
	go func() {
		_, errLectura := conexion.Read(make([]byte, 8))
		listo <- errLectura
	}()
	time.Sleep(30 * time.Millisecond)

	inicio := time.Now()
	if err := conexion.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	select {
	case errLectura := <-listo:
		tardanza := time.Since(inicio)
		if !errors.Is(errLectura, os.ErrDeadlineExceeded) {
			t.Fatalf("el Read devolvió %v, se esperaba os.ErrDeadlineExceeded", errLectura)
		}
		if tardanza > 200*time.Millisecond {
			t.Fatalf("el Read bloqueado tardó %v en despertar, más de 200 ms", tardanza)
		}
		t.Logf("el Read bloqueado despertó %v después del SetReadDeadline", tardanza)
	case <-time.After(time.Second):
		t.Fatal("el Read sigue bloqueado un segundo después de SetReadDeadline")
	}

	// Sin plazo, la conexión sigue viva: un DATA que llega se lee.
	if err := conexion.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("limpiar SetReadDeadline: %v", err)
	}
	relay.enviar(paqueteData(f.id, []byte("hola")))
	_ = conexion.SetReadDeadline(time.Now().Add(2 * time.Second))
	leido := make([]byte, 4)
	if _, err := io.ReadFull(conexion, leido); err != nil {
		t.Fatalf("Read tras limpiar el plazo: %v", err)
	}
	if string(leido) != "hola" {
		t.Fatalf("se leyó %q", leido)
	}
}

func TestWISPSetDeadlineDespiertaLectura(t *testing.T) {
	_, extremo := nuevoRelayPrueba(t, 8, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("mudo2.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}

	listo := make(chan error, 1)
	go func() {
		_, errLectura := conexion.Read(make([]byte, 8))
		listo <- errLectura
	}()
	time.Sleep(30 * time.Millisecond)

	inicio := time.Now()
	if err := conexion.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	select {
	case errLectura := <-listo:
		if !errors.Is(errLectura, os.ErrDeadlineExceeded) {
			t.Fatalf("el Read devolvió %v, se esperaba os.ErrDeadlineExceeded", errLectura)
		}
		if tardanza := time.Since(inicio); tardanza > 200*time.Millisecond {
			t.Fatalf("el Read bloqueado tardó %v en despertar", tardanza)
		}
	case <-time.After(time.Second):
		t.Fatal("el Read sigue bloqueado un segundo después de SetDeadline")
	}
}

func TestWISPSetWriteDeadlineDespiertaEscritura(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 0, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.abrirTCPCrudo("sincredito.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	listo := make(chan error, 1)
	go func() {
		_, errEscritura := conexion.Write([]byte("x"))
		listo <- errEscritura
	}()
	time.Sleep(30 * time.Millisecond)

	inicio := time.Now()
	if err := conexion.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	select {
	case errEscritura := <-listo:
		if !errors.Is(errEscritura, os.ErrDeadlineExceeded) {
			t.Fatalf("el Write devolvió %v, se esperaba os.ErrDeadlineExceeded", errEscritura)
		}
		if tardanza := time.Since(inicio); tardanza > 200*time.Millisecond {
			t.Fatalf("el Write bloqueado tardó %v en despertar", tardanza)
		}
	case <-time.After(time.Second):
		t.Fatal("el Write sigue bloqueado un segundo después de SetWriteDeadline")
	}

	// El flujo no quedó roto: con plazo limpio y crédito, el Write pasa.
	if err := conexion.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatalf("limpiar SetWriteDeadline: %v", err)
	}
	relay.enviar(paqueteContinue(f.id, 1))
	if _, err := conexion.Write([]byte("y")); err != nil {
		t.Fatalf("Write tras limpiar el plazo: %v", err)
	}
}
