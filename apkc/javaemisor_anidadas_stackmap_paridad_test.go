//go:build !js

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_anidadas_stackmap_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_anidadas_stackmap_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// formasDeLaSonda compila `fuenteAnidadaStackMap` por los dos caminos y devuelve, por clase y método,
// la forma del atributo en cada lado y si ese método lleva tabla.
func formasDeLaSonda(t *testing.T) (nuestras, deJavac map[string]string) {
	t.Helper()
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{{nombre: "src/p/Anidada.java", contenido: fuenteAnidadaStackMap}}

	salida, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	nuestras = map[string]string{}
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			forma, _, hay, err := tablaYFormaDelMetodo(c.datos, m.Name)
			if err != nil {
				t.Fatalf("%s.%s: %v", c.interno, m.Name, err)
			}
			if !hay {
				forma = "<sin tabla>"
			}
			nuestras[c.interno+"."+m.Name] = forma
		}
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Anidada.java", fuenteAnidadaStackMap)
	deJavacClases := compilaConJavac(t, javac, jar, raiz, fuentes)
	deJavac = map[string]string{}
	for _, cf := range deJavacClases {
		interno := cf.Name[1 : len(cf.Name)-1]
		datos, err := os.ReadFile(filepath.Join(raiz, "javac-out", interno+".class"))
		if err != nil {
			t.Fatalf("%s: %v", interno, err)
		}
		for _, m := range cf.Methods {
			if !m.HasCode {
				continue
			}
			forma, _, hay, err := tablaYFormaDelMetodo(datos, m.Name)
			if err != nil {
				t.Fatalf("%s.%s: %v", interno, m.Name, err)
			}
			if !hay {
				forma = "<sin tabla>"
			}
			deJavac[interno+"."+m.Name] = forma
		}
	}
	return nuestras, deJavac
}

// TestElStackMapDeLaAnidadaEsElDeJavac17: cada método con cuerpo de la sonda lleva las mismas formas en
// las mismas posiciones que javac 17, con los mismos tipos y las mismas pilas. La comparación es de
// formas, no de bytes: los índices del pool están ordenados distinto y no significan nada.
func TestElStackMapDeLaAnidadaEsElDeJavac17(t *testing.T) {
	nuestras, deJavac := formasDeLaSonda(t)
	if len(deJavac) == 0 {
		t.Fatal("javac no dejó ningún método: la sonda no mide nada")
	}
	var conTabla int
	for clave, esperado := range deJavac {
		nuestra, ok := nuestras[clave]
		if !ok {
			t.Errorf("falta el método %s que javac escribe", clave)
			continue
		}
		if esperado != "<sin tabla>" {
			conTabla++
		}
		if nuestra != esperado {
			t.Errorf("%s: el StackMapTable no es el de javac\nnuestro: %s\njavac:   %s", clave, nuestra, esperado)
		}
	}
	if conTabla < 5 {
		t.Fatalf("solo %d métodos llevan StackMapTable: la sonda no cubre las formas que se comprimen", conTabla)
	}
}

// TestElStackMapDeLaAnidadaSeComprimeDeVerdad es la falsificación: con `marcosSinComprimir` encendido
// (el emisor de antes de este frente) las formas dejan de coincidir. Sin este control, una sonda que
// no ejercitara la compresión pasaría la prueba de arriba sin medir nada.
func TestElStackMapDeLaAnidadaSeComprimeDeVerdad(t *testing.T) {
	antes := marcosSinComprimir
	marcosSinComprimir = true
	t.Cleanup(func() { marcosSinComprimir = antes })

	nuestras, deJavac := formasDeLaSonda(t)
	var distintos []string
	for clave, esperado := range deJavac {
		if esperado == "<sin tabla>" {
			continue
		}
		if nuestras[clave] != esperado {
			distintos = append(distintos, clave)
		}
	}
	if len(distintos) == 0 {
		t.Fatalf("con `full_frame` en cada comienzo las formas siguen siendo las de javac: la sonda no " +
			"ejercita ninguna forma comprimida")
	}
	for _, clave := range distintos {
		t.Logf("con el emisor de antes se desvía %s", clave)
	}
}
