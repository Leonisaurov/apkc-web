package main

// Pruebas de la firma con nombres hostiles: el plan de copia de un paquete que se vuelve a firmar
// conservaba los nombres tal cual, y uno con ".." o barra inicial salía del directorio al que se
// extraiga. Además, una entrada META-INF que acabara en .MF/.SF/.RSA/.DSA/.EC se descartaba en
// silencio, aunque fuera contenido como META-INF/services/x.MF: ahora se avisa.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSignRechazaNombreQueSaleDelPaquete: el mismo nombre que el PoC de la auditoría. Se rechaza
// antes de escribir nada y el error nombra la entrada.
func TestSignRechazaNombreQueSaleDelPaquete(t *testing.T) {
	dir := t.TempDir()
	entrada := filepath.Join(dir, "traversal.apk")
	if err := os.WriteFile(entrada,
		zipDeEntradas(t, map[string][]byte{"../../evil.txt": []byte("x")}), 0o644); err != nil {
		t.Fatal(err)
	}

	key, cert := signingPair(t)
	salida := filepath.Join(dir, "firmado.apk")
	_, err := resignWithKey(entrada, salida, key, cert)
	if err == nil {
		t.Fatal("se firmó un paquete con una entrada que sale del directorio")
	}
	if !strings.Contains(err.Error(), "../../evil.txt") {
		t.Errorf("el error no nombra la entrada: %v", err)
	}
	if _, statErr := os.Stat(salida); !os.IsNotExist(statErr) {
		t.Error("el paquete de salida se escribió pese al nombre hostil")
	}
}

// TestSignAvisaAlDescartarUnaFirmaEscondida: META-INF/services/x.MF es contenido, y la regla de
// sufijos lo confunde con la firma. Descartarlo es lo que hay que hacer —no puede viajar en un
// paquete firmado— pero no en silencio: el aviso nombra la entrada.
func TestSignAvisaAlDescartarUnaFirmaEscondida(t *testing.T) {
	dir := t.TempDir()
	entrada := filepath.Join(dir, "escondida.apk")
	if err := os.WriteFile(entrada, zipDeEntradas(t, map[string][]byte{
		"res/raw/ok.txt":         []byte("contenido que sí viaja"),
		"META-INF/services/x.MF": []byte("contenido que se descarta"),
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	key, cert := signingPair(t)
	salida := filepath.Join(dir, "firmado.apk")
	aviso := capturarSalida(t, func() {
		if _, err := resignWithKey(entrada, salida, key, cert); err != nil {
			t.Fatalf("no se pudo firmar el paquete: %v", err)
		}
	})
	if !strings.Contains(aviso, "META-INF/services/x.MF") {
		t.Errorf("el descarte no se avisó: %q", aviso)
	}
}

// TestOwnSignatureEntry: lo que una firma JAR deja directamente bajo META-INF se descarta sin ruido,
// porque el re-firmado lo escribe de nuevo; lo que está más abajo, no.
func TestOwnSignatureEntry(t *testing.T) {
	for _, e := range []struct {
		name string
		own  bool
	}{
		{"META-INF/MANIFEST.MF", true},
		{"META-INF/ANDROID.SF", true},
		{"META-INF/ANDROIDD.RSA", true},
		{"META-INF/CERT.DSA", true},
		{"META-INF/ECLIPSE_.EC", true},
		{"META-INF/services/x.MF", false},
		{"META-INF/versions/9/x.SF", false},
		{"res/raw/x.MF", false},
	} {
		if got := ownSignatureEntry(e.name); got != e.own {
			t.Errorf("ownSignatureEntry(%q) = %v, want %v", e.name, got, e.own)
		}
	}
}

// capturarSalida ejecuta fn con os.Stdout redirigido a una tubería y devuelve todo lo escrito, que
// es donde Log imprime. Las pruebas de este paquete no corren en paralelo, así que el cambio de
// os.Stdout no pisa a otra.
func capturarSalida(t *testing.T, fn func()) string {
	t.Helper()
	viejo := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	leido := make(chan string, 1)
	go func() {
		datos, _ := io.ReadAll(r)
		leido <- string(datos)
	}()
	fn()
	w.Close()
	os.Stdout = viejo
	return <-leido
}
