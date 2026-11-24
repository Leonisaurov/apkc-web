//go:build js && wasm

package main

// La cota del apk en la pestaña (R6). El hospedador de Node es donde se puede medir qué hace un
// binario wasm con un fichero grande, así que esta prueba vive aquí y no en la suite nativa: el
// apk sintético es un fichero disperso de 300 MiB (`truncate`, sin datos reales) que en nativo no
// llega al tope de 1 GiB y por tanto no ejercita la rama.
//
// Lo medido: con la cota de wasm que fija `init` (256 MiB) el verificador rechaza el apk por su
// tamaño y no lo lee; con la cota nativa (1 GiB) el mismo apk ya no se rechaza y el veredicto pasa
// al `.idsig`, que aquí declara más de lo que su tope permite. Esa segunda mitad es la
// falsificación de `init`: si el ajuste desapareciera, el motivo volvería a ser el del apk que se
// intenta leer entero.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// apkGrandeV4 cae por encima de la cota de wasm (256 MiB) y por debajo de la nativa (1 GiB): esa
// ventana es justo la que separa las dos ramas. Un apk de la pestaña son decenas de MB.
const apkGrandeV4 = 300 << 20

// idsigGrandeV4 declara más de lo que deja `topeIdsigParaV4(300 MiB)` (~66 MiB): el verificador
// tropieza en su tope, no en la lectura, cuando el apk ya pasó el suyo.
const idsigGrandeV4 = 80 << 20

// ficheroDisperso crea `ruta` con `tamano` bytes declarados y nada escrito.
func ficheroDisperso(t *testing.T, ruta string, tamano int64) {
	t.Helper()
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(tamano); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestVerificarV4RespetaLaCotaDeWasm: el apk de 300 MiB y su `.idsig` de 80 MiB ni se abren; y con
// la cota devuelta a 1 GiB el apk deja de rechazarse, que es lo que se vería sin el `init`.
func TestVerificarV4RespetaLaCotaDeWasm(t *testing.T) {
	// `init` ya bajó la cota; se afirma aquí para que la falsificación no esté midiendo otra cosa
	if topeApkParaV4 != 256<<20 {
		t.Fatalf("en wasm el tope del apk es %d; se esperaba %d", topeApkParaV4, 256<<20)
	}
	apk := filepath.Join(t.TempDir(), "app-grande.apk")
	ficheroDisperso(t, apk, apkGrandeV4)
	ficheroDisperso(t, apk+".idsig", idsigGrandeV4)

	v := &verificacion{}
	v.verificarV4(apk)
	if !v.v4Presente {
		t.Fatal("el .idsig no se anunció presente")
	}
	motivo := strings.Join(v.v4Motivos, " | ")
	if !strings.Contains(motivo, "app-grande.apk mide") || !strings.Contains(motivo, "no se lee") {
		t.Fatalf("el apk de 300 MiB no se rechazó por el tope del apk: %v", v.v4Motivos)
	}
	if strings.Contains(motivo, ".idsig") {
		t.Errorf("el veredicto habla del .idsig: el apk no se rechazó antes de mirarlo: %v", v.v4Motivos)
	}

	t.Run("falsificación: con la cota en 1 GiB el apk deja de rechazarse", func(t *testing.T) {
		anterior := topeApkParaV4
		topeApkParaV4 = 1 << 30
		defer func() { topeApkParaV4 = anterior }()

		v := &verificacion{}
		v.verificarV4(apk)
		motivo := strings.Join(v.v4Motivos, " | ")
		if strings.Contains(motivo, "app-grande.apk mide") {
			t.Fatalf("con la cota en 1 GiB el apk de 300 MiB se sigue rechazando por su tamaño: %v", v.v4Motivos)
		}
		if !strings.Contains(motivo, ".idsig") || !strings.Contains(motivo, "no se lee") {
			t.Fatalf("con la cota en 1 GiB el veredicto no pasó al tope del .idsig: %v", v.v4Motivos)
		}
	})
}
