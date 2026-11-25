package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildDosVecesEnWasmMideEtapas es la medición 3 llevada al repo (§ de la documentación del
// incremental web). Dos `_build` por modo en el MISMO proceso de wasm: el segundo exige que el
// hospedador sepa vaciar y reconstruir `build/res` (shim/navegador-sim.js:285-307), que era el
// defecto que hacía ir reproducible la medición solo desde una copia de trabajo de /tmp.
//
// El proyecto es el de banco/gen-proyecto-grande.py, montado por banco/incremental-wasm.sh en
// /proyecto. Las etapas las escribe `timeIt` (apkc/log.go:16) en la salida; las etiquetas de este
// test sirven para atribuir cada línea `timing` a su modo y a su vuelta. El cierre compara los dos
// apks entrada por entrada, igual que banco/incremental.sh:118-128, para que el ahorro no sea un
// cambio de comportamiento.
func TestBuildDosVecesEnWasmMideEtapas(t *testing.T) {
	jar := jarDelSDK(t)
	if _, err := os.Stat(filepath.Join("/proyecto", "AndroidManifest.xml")); err != nil {
		t.Skipf("el proyecto grande no está montado en /proyecto: %v", err)
	}

	// Un keystore propio del proceso, escrito por el propio apkc sin keytool ni ~/.android.
	keystore := "/debug.keystore"
	if err := escribirDebugKeystore(keystore, "android", "androiddebugkey", 10000); err != nil {
		t.Fatalf("escribirDebugKeystore: %v", err)
	}
	antesV4, antesSal := escribirV4, v4SaltHex
	t.Cleanup(func() { escribirV4, v4SaltHex = antesV4, antesSal })

	var apks [2][]byte
	for i, modo := range []string{"plain", "inc"} {
		setupProject(t)
		copiarArbol(t, "/proyecto", ".")

		opciones := buildOpts{
			incremental: modo == "inc",
			linker:      linkerGo,
			compiler:    compilerGo,
			signer:      signerGo,
			keyStore:    keystore,
			storePass:   "android",
			keyAlias:    "androiddebugkey",
			jar:         jar,
			sdkVersion:  34,
			codename:    "14",
			v4:          true,
		}

		fmt.Printf("\n=== modo %s, vuelta fría (build %d) ===\n", modo, 2*i+1)
		_build(opciones)

		// La edición de un solo fuente, como en banco/incremental.sh:66-68, obliga a la segunda
		// vuelta a reenlazar, recompilar, dexar y volver a firmar dentro del mismo proceso.
		fmt.Printf("=== modo %s, vuelta caliente (build %d): editar Mod037.java ===\n", modo, 2*i+2)
		editarFuente(t)
		_build(opciones)

		apk, err := os.ReadFile(filepath.Join("build", "app.apk"))
		if err != nil {
			t.Fatalf("leer build/app.apk del modo %s: %v", modo, err)
		}
		apks[i] = apk
		v, err := verificarApk(filepath.Join("build", "app.apk"))
		if err != nil {
			t.Fatalf("verificarApk del modo %s: %v", modo, err)
		}
		if !v.v1 || !v.v2 {
			t.Fatalf("el apk del modo %s no verifica: v1=%v v2=%v", modo, v.v1, v.v2)
		}
	}

	comparaEntradas(t, apks[0], apks[1])
}

// copiarArbol mete el proyecto montado en el directorio de trabajo de la vuelta. El fs del
// hospedador es plano y solo monta ficheros sueltos (--montar), así que el árbol se reconstruye
// aquí con la misma API de os que usa el build.
func copiarArbol(t *testing.T, origen, destino string) {
	t.Helper()
	entradas, err := os.ReadDir(origen)
	if err != nil {
		t.Fatalf("leer %s: %v", origen, err)
	}
	for _, e := range entradas {
		de := filepath.Join(origen, e.Name())
		dd := filepath.Join(destino, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dd, 0o755); err != nil {
				t.Fatal(err)
			}
			copiarArbol(t, de, dd)
			continue
		}
		datos, err := os.ReadFile(de)
		if err != nil {
			t.Fatalf("leer %s: %v", de, err)
		}
		if err := os.WriteFile(dd, datos, 0o644); err != nil {
			t.Fatalf("escribir %s: %v", dd, err)
		}
	}
}

// editarFuente cambia el mismo literal que banco/incremental.sh:67 cambia con sed. La cuenta
// tiene que dar uno: si el generador cambiara, la medición no sería la misma.
func editarFuente(t *testing.T) {
	t.Helper()
	ruta := filepath.Join("src", "com", "example", "big", "Mod037.java")
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	viejo, nuevo := []byte("fBase + 37;"), []byte("fBase + 137;")
	if bytes.Count(datos, viejo) != 1 {
		t.Fatalf("%s: esperaba una sola aparición de %q", ruta, viejo)
	}
	if err := os.WriteFile(ruta, bytes.Replace(datos, viejo, nuevo, 1), 0o644); err != nil {
		t.Fatalf("escribir %s: %v", ruta, err)
	}
}

// comparaEntradas es la comprobación de banco/incremental.sh:118-128 hecha dentro del test, sin
// exportar apks ni depender de Python: mismos nombres en el mismo orden y mismos bytes por entrada.
func comparaEntradas(t *testing.T, a, b []byte) {
	t.Helper()
	za, err := zip.NewReader(bytes.NewReader(a), int64(len(a)))
	if err != nil {
		t.Fatalf("zip del apk plain: %v", err)
	}
	zb, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("zip del apk inc: %v", err)
	}
	if len(za.File) != len(zb.File) {
		t.Fatalf("los apks tienen %d y %d entradas", len(za.File), len(zb.File))
	}
	distintas := []string{}
	for i := range za.File {
		if za.File[i].Name != zb.File[i].Name {
			t.Fatalf("entrada %d: %q contra %q", i, za.File[i].Name, zb.File[i].Name)
		}
		fa, err := za.File[i].Open()
		if err != nil {
			t.Fatal(err)
		}
		da, err := io.ReadAll(fa)
		fa.Close()
		if err != nil {
			t.Fatal(err)
		}
		fb, err := zb.File[i].Open()
		if err != nil {
			t.Fatal(err)
		}
		db, err := io.ReadAll(fb)
		fb.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(da, db) {
			distintas = append(distintas, za.File[i].Name)
		}
	}
	fmt.Printf("comparación entrada por entrada: %d entradas, %d con bytes distintos %v\n",
		len(za.File), len(distintas), distintas)
	if len(distintas) != 0 {
		t.Fatalf("el apk --inc difiere del plain entrada por entrada: %v", distintas)
	}
}
