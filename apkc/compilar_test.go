package main

// Tests de `apkc compile`: el camino de disco, que es el que usa la página. El oráculo otra vez no necesita
// el JDK: las seis clases de javac viajan como base64 (classreader_test.go) y lo que escribimos desde los
// mismos fuentes tiene que dar el mismo dex byte a byte.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileEscribeLasClasesDelProyecto(t *testing.T) {
	jar := jarDelSDK(t)
	proyecto := t.TempDir()
	src := filepath.Join(proyecto, "src", "com", "hola", "apkcstudio")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for nombre, contenido := range map[string]string{
		"MainActivity.java": fuenteMainActivityStudio,
		"R.java":            fuenteRStudio,
	} {
		if err := os.WriteFile(filepath.Join(src, nombre), []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fuentes, err := fuentesJava([]string{filepath.Join(proyecto, "src")})
	if err != nil {
		t.Fatalf("leer el arbol: %v", err)
	}
	if len(fuentes) != 2 {
		t.Fatalf("%d fuentes leidos, medidos dos", len(fuentes))
	}
	for _, f := range fuentes {
		if filepath.Base(f.nombre) != "MainActivity.java" && filepath.Base(f.nombre) != "R.java" {
			t.Errorf("el nombre del fuente sale con ruta de proyecto: %q", f.nombre)
		}
	}

	clases, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el compile rechazo el proyecto: %v", err)
	}
	out := filepath.Join(proyecto, "build", "classes")
	if err := escribirClases(out, clases); err != nil {
		t.Fatalf("escribir las clases: %v", err)
	}

	// el orden del vector es el de las rutas: `apkc dex` sobre este directorio tiene que leer las seis
	// listas en el orden en que javac las dejo
	vuelta, err := leeClases([]string{out})
	if err != nil {
		t.Fatalf("el dexer no leeria lo que escribimos: %v", err)
	}
	if len(vuelta) != 6 {
		t.Fatalf("%d clases releidas, medidas seis", len(vuelta))
	}
	dexNuestro, err := dexDeClases(vuelta)
	if err != nil {
		t.Fatalf("dexando nuestras clases desde disco: %v", err)
	}
	dexJavac, err := dexDeClases(leeLasSeis(t))
	if err != nil {
		t.Fatalf("dexando las clases de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("el dex desde disco mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("el dex desde disco difiere en el byte %d: %#02x frente a %#02x", i, dexNuestro[i], dexJavac[i])
		}
	}

	// y el fichero se llama como lo nombra javac: con el `$` de la anidada en el nombre interno
	if _, err := os.Stat(filepath.Join(out, "com", "hola", "apkcstudio", "MainActivity$1.class")); err != nil {
		t.Errorf("falta la anonima escrita en su propio fichero: %v", err)
	}
}

func TestCompileRechazaSinEscribirNada(t *testing.T) {
	jar := jarDelSDK(t)
	proyecto := t.TempDir()
	src := filepath.Join(proyecto, "src", "p")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "C.java"), []byte("package p;\npublic class C {\n    void f() { if (true) return; }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fuentes, err := fuentesJava([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(fuentes) != 1 {
		t.Fatalf("%d fuentes", len(fuentes))
	}
	clases, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatalf("el compile acepto un `if` y devolvio %d clases", len(clases))
	}
	out := filepath.Join(proyecto, "build", "classes")
	if err := escribirClases(out, clases); err != nil {
		t.Fatalf("escribir sin clases deberia ser un no-op: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("se creo el arbol de salida pese al rechazo")
	}
}

func TestFuentesJavaIgnoraLoQueNoEsJava(t *testing.T) {
	raiz := t.TempDir()
	for nombre, contenido := range map[string]string{
		"src/p/A.java":        "package p;\npublic class A {}\n",
		"src/p/B.kt":          "package p\n",
		"src/p/notas.txt":     "hola\n",
		"src/p/sub/C.java":    "package p.sub;\npublic class C {}\n",
		"build/p/D.java":      "package p;\npublic class D {}\n",
		"UnFicheroSuelt.java": "package p;\npublic class E {}\n",
	} {
		ruta := filepath.Join(raiz, filepath.FromSlash(nombre))
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fuentes, err := fuentesJava([]string{filepath.Join(raiz, "src"), filepath.Join(raiz, "UnFicheroSuelt.java")})
	if err != nil {
		t.Fatal(err)
	}
	if len(fuentes) != 3 {
		t.Fatalf("%d fuentes leidos: %v", len(fuentes), nombres(fuentes))
	}
	// un directorio que no existe se nombra, no se calla
	if _, err := fuentesJava([]string{filepath.Join(raiz, "noexiste")}); err == nil {
		t.Errorf("acepto una raiz que no existe")
	}
}

func nombres(f []fuenteJava) []string {
	out := []string{}
	for _, x := range f {
		out = append(out, x.nombre)
	}
	return out
}

// TestPorteroDelCompilerNombraLoQueNoMide cubre las negaciones de `--compiler go`: un portero que solo
// vive dentro de un LogF no se puede probar, y aqui si.
func TestPorteroDelCompilerNombraLoQueNoMide(t *testing.T) {
	casos := []struct {
		nombre string
		monta  func(t *testing.T, raiz string)
		useAAB bool
		espera string
	}{
		{"kotlin en src", func(_ *testing.T, raiz string) {
			pon(t, raiz, "src/p/A.kt", "package p\n")
		}, false, "Kotlin"},
		{"jars de dependencia", func(_ *testing.T, raiz string) {
			pon(t, raiz, "jar/dep.jar", "PK")
		}, false, "dependencias"},
		{"bundle pedido", func(_ *testing.T, raiz string) {
			pon(t, raiz, "src/p/A.java", "package p;\npublic class A {}\n")
		}, true, "bundles"},
		{"proyecto medido", func(_ *testing.T, raiz string) {
			pon(t, raiz, "src/p/A.java", "package p;\npublic class A {}\n")
		}, false, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			raiz := t.TempDir()
			c.monta(t, raiz)
			t.Chdir(raiz)
			err := revisarCompilerGo(buildOpts{compiler: compilerGo, useAAB: c.useAAB})
			if c.espera == "" {
				if err != nil {
					t.Fatalf("el portero rechazo un proyecto que si mide: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("el portero acepto %s en silencio", c.nombre)
			}
			if !strings.Contains(err.Error(), c.espera) {
				t.Errorf("el error no nombra la construccion (%q)", err)
			}
		})
	}
}

// TestEtapasDeBuildEnGo es el camino de `apkc build --compiler go` sin pasar por el resto de la
// herramienta: las dos etapas, una detras de otra, sobre un proyecto en disco. El oraculo vuelve a ser
// el dex de las seis clases que javac dejo para estos mismos fuentes.
func TestEtapasDeBuildEnGo(t *testing.T) {
	androidJar = jarDelSDK(t)
	raiz := t.TempDir()
	pon(t, raiz, "src/com/hola/apkcstudio/MainActivity.java", fuenteMainActivityStudio)
	pon(t, raiz, "src/com/hola/apkcstudio/R.java", fuenteRStudio)
	t.Chdir(raiz)

	fuente, ran := compilarFuentesEnGo()
	if !ran {
		t.Fatal("la etapa de compilacion no corro")
	}
	if len(fuente) != 2 {
		t.Errorf("%d fuentes paso la etapa, medidos dos", len(fuente))
	}
	dexarClasesEnGo()
	dato, err := os.ReadFile(filepath.Join("build", "classes.dex"))
	if err != nil {
		t.Fatalf("no hay build/classes.dex: %v", err)
	}
	esperado, err := dexDeClases(leeLasSeis(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(dato) != len(esperado) {
		t.Fatalf("el dex de las etapas mide %d bytes y el de javac %d", len(dato), len(esperado))
	}
	for i := range dato {
		if dato[i] != esperado[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x", i, dato[i], esperado[i])
		}
	}
}

func pon(t *testing.T, raiz, ruta, contenido string) {
	t.Helper()
	full := filepath.Join(raiz, filepath.FromSlash(ruta))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}
