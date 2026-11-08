package main

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"sort"
	"testing"
)

// fuenteBase y fuenteCtor son la sonda del párrafo 91: un constructor con `super(x)`, otro que delega
// con `this(2)` y un campo con inicializador. `Ctor()` no baja `a = 1`; `Ctor(int)` sí, después del
// `super`.
const fuenteBase = `public class Base {
    int v;
    Base(int x) {
        v = x;
    }
}
`

const fuenteCtor = `public class Ctor extends Base {
    int a = 1;
    int b;
    Ctor() {
        this(2);
    }
    Ctor(int x) {
        super(x);
        b = x + 1;
    }
    int suma() {
        return a + b + v;
    }
}
`

// baseJavac y ctorJavac: `javac --release 8 -g:lines,source Base.java Ctor.java` y `base64 -w0`.
const baseJavac = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABEJhc2UBAAF2AQABSQEABChJKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAKU291cmNlRmlsZQEACUJhc2UuamF2YQAhAAgAAgAAAAEAAAALAAwAAAABAAAABQANAAEADgAAACoAAgACAAAACiq3AAEqG7UAB7EAAAABAA8AAAAOAAMAAAADAAQABAAJAAUAAQAQAAAAAgAR"
const ctorJavac = "yv66vgAAADQAGwoAAgADBwAEDAAFAAYBAARDdG9yAQAGPGluaXQ+AQAEKEkpVgoACAADBwAJAQAEQmFzZQkAAgALDAAMAA0BAAFhAQABSQkAAgAPDAAQAA0BAAFiCQACABIMABMADQEAAXYBAAMoKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEc3VtYQEAAygpSQEAClNvdXJjZUZpbGUBAAlDdG9yLmphdmEAIQACAAgAAAACAAAADAANAAAAAAAQAA0AAAADAAAABQAUAAEAFQAAACIAAgABAAAABioFtwABsQAAAAEAFgAAAAoAAgAAAAUABQAGAAAABQAGAAEAFQAAADYAAwACAAAAEiobtwAHKgS1AAoqGwRgtQAOsQAAAAEAFgAAABIABAAAAAgABQACAAoACQARAAoAAAAXABgAAQAVAAAAJwACAAEAAAAPKrQACiq0AA5gKrQAEWCsAAAAAQAWAAAABgABAAAADAABABkAAAACABo="

var metodosBase = []string{"<init>"}
var metodosCtor = []string{"<init>()V", "<init>(I)V", "suma()I"}

// Sonda del `super()` implicito: `Simple` no escribe constructor de padre; javac mete la llamada a
// `Base2.<init>()` en la linea de la firma y baja `a = 1` despues.
const fuenteBase2 = `public class Base2 {
    int v;
    Base2() {
        v = 7;
    }
}
`

const fuenteSimple = `public class Simple extends Base2 {
    int a = 1;
    Simple() {
        b = 2;
    }
    int b;
    int suma() {
        return a + b + v;
    }
}
`

const base2Javac = "yv66vgAAADQAEQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABUJhc2UyAQABdgEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAKU291cmNlRmlsZQEACkJhc2UyLmphdmEAIQAIAAIAAAABAAAACwAMAAAAAQAAAAUABgABAA0AAAArAAIAAQAAAAsqtwABKhAHtQAHsQAAAAEADgAAAA4AAwAAAAMABAAEAAoABQABAA8AAAACABA="
const simpleJavac = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBAAVCYXNlMgEABjxpbml0PgEAAygpVgkACAAJBwAKDAALAAwBAAZTaW1wbGUBAAFhAQABSQkACAAODAAPAAwBAAFiCQAIABEMABIADAEAAXYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEc3VtYQEAAygpSQEAClNvdXJjZUZpbGUBAAtTaW1wbGUuamF2YQAhAAgAAgAAAAIAAAALAAwAAAAAAA8ADAAAAAIAAAAFAAYAAQATAAAAMwACAAEAAAAPKrcAASoEtQAHKgW1AA2xAAAAAQAUAAAAEgAEAAAAAwAEAAIACQAEAA4ABQAAABUAFgABABMAAAAnAAIAAQAAAA8qtAAHKrQADWAqtAAQYKwAAAABABQAAAAGAAEAAAAIAAEAFwAAAAIAGA=="

func TestElConstructorImplicitoSeEscribeComoJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/Base2.java", contenido: fuenteBase2},
		{nombre: "src/Simple.java", contenido: fuenteSimple},
	}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestras := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase no se lee: %v", err)
		}
		nuestras = append(nuestras, cf)
	}
	var suyas []classFull
	for _, b64 := range []string{base2Javac, simpleJavac} {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("el fixture no es base64: %v", err)
		}
		cf, err := readClassFull(datos)
		if err != nil {
			t.Fatalf("el fixture de javac no se lee: %v", err)
		}
		suyas = append(suyas, cf)
	}
	sort.Slice(nuestras, func(i, j int) bool { return nuestras[i].Name < nuestras[j].Name })
	sort.Slice(suyas, func(i, j int) bool { return suyas[i].Name < suyas[j].Name })
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
	casos := []struct {
		clase  int
		nombre string
		desc   string
	}{{0, "<init>", "()V"}, {1, "<init>", "()V"}, {1, "suma", "()I"}}
	for _, c := range casos {
		a, ok := buscaMetodoDesc(nuestras[c.clase], c.nombre, c.desc)
		if !ok {
			t.Fatalf("%s.%s: no está en nuestra clase", nuestras[c.clase].Name, c.nombre)
		}
		b, ok := buscaMetodoDesc(suyas[c.clase], c.nombre, c.desc)
		if !ok {
			t.Fatalf("%s.%s: no está en la de javac", suyas[c.clase].Name, c.nombre)
		}
		if x, y := sinPool(formaDeMetodo(t, nuestras[c.clase], a)), sinPool(formaDeMetodo(t, suyas[c.clase], b)); x != y {
			t.Errorf("%s.%s:\nnuestro  %s\njavac    %s", nuestras[c.clase].Name, c.nombre, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s.%s: las filas nuestras son %q y las de javac %q", nuestras[c.clase].Name, c.nombre, x, y)
		}
	}
}

func buscaMetodoDesc(cf classFull, nombre, desc string) (classMethod, bool) {
	for _, m := range cf.Methods {
		if m.Name == nombre && m.Desc == desc {
			return m, true
		}
	}
	return classMethod{}, false
}

func dosClasesCtor(t *testing.T) ([]classFull, []classFull) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{
		{nombre: "src/Base.java", contenido: fuenteBase},
		{nombre: "src/Ctor.java", contenido: fuenteCtor},
	}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestras := make([]classFull, 0, len(salida))
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("nuestra clase %s no se lee: %v", c.interno, err)
		}
		nuestras = append(nuestras, cf)
	}
	fixtures := []string{baseJavac, ctorJavac}
	suyas := make([]classFull, 0, len(fixtures))
	for _, b64 := range fixtures {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("el fixture no es base64: %v", err)
		}
		cf, err := readClassFull(datos)
		if err != nil {
			t.Fatalf("el fixture de javac no se lee: %v", err)
		}
		suyas = append(suyas, cf)
	}
	sort.Slice(nuestras, func(i, j int) bool { return nuestras[i].Name < nuestras[j].Name })
	sort.Slice(suyas, func(i, j int) bool { return suyas[i].Name < suyas[j].Name })
	return nuestras, suyas
}

// TestElConstructorSeEscribeComoJavac: los dos `<init>` de `Ctor` (uno delega con `this`, el otro llama
// a `super` y baja el inicializador del campo) y el de `Base`, con pila, huecos y filas de línea.
func TestElConstructorSeEscribeComoJavac(t *testing.T) {
	nuestras, suyas := dosClasesCtor(t)
	sinPool := func(s string) string { return regexp.MustCompile(`pool\?\d+`).ReplaceAllString(s, "pool?") }
	casos := []struct {
		clase  int
		nombre string
		desc   string
	}{
		{0, "<init>", "(I)V"},
		{1, "<init>", "()V"},
		{1, "<init>", "(I)V"},
		{1, "suma", "()I"},
	}
	for _, c := range casos {
		a, ok := buscaMetodoDesc(nuestras[c.clase], c.nombre, c.desc)
		if !ok {
			t.Fatalf("%s.%s%s: no está en nuestra clase", nuestras[c.clase].Name, c.nombre, c.desc)
		}
		b, ok := buscaMetodoDesc(suyas[c.clase], c.nombre, c.desc)
		if !ok {
			t.Fatalf("%s.%s%s: no está en la de javac", suyas[c.clase].Name, c.nombre, c.desc)
		}
		if a.MaxStack != b.MaxStack || a.LocalCount != b.LocalCount {
			t.Errorf("%s.%s%s: pila %d y locales %d, y los de javac %d y %d",
				nuestras[c.clase].Name, c.nombre, c.desc, a.MaxStack, a.LocalCount, b.MaxStack, b.LocalCount)
		}
		if x, y := sinPool(formaDeMetodo(t, nuestras[c.clase], a)), sinPool(formaDeMetodo(t, suyas[c.clase], b)); x != y {
			t.Errorf("%s.%s%s:\nnuestro  %s\njavac    %s", nuestras[c.clase].Name, c.nombre, c.desc, x, y)
		}
		if x, y := filasDeLinea(a), filasDeLinea(b); x != y {
			t.Errorf("%s.%s%s: las filas nuestras son %q y las de javac %q", nuestras[c.clase].Name, c.nombre, c.desc, x, y)
		}
	}
}

// TestLosConstructoresSobrecargadosTienenCodeItemPropio: dos `<init>` con descriptores distintos son
// dos code_item. La clave de `areaCodigo` era el nombre a secas y los dos apuntaban al mismo cuerpo;
// el dex pasaba el doble oráculo (nuestras clases y las de javac colisionaban igual) pero ART lo
// cortaba. Medido en `Ver` con `Ver()` y `Ver(int)` del párrafo 91.
func TestLosConstructoresSobrecargadosTienenCodeItemPropio(t *testing.T) {
	nuestras, _ := dosClasesCtor(t)
	td := newDexTables()
	if err := td.collectClasses(nuestras); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	_, rels, _, _, err := td.areaCodigo(nuestras)
	if err != nil {
		t.Fatalf("armando el área de código: %v", err)
	}
	cuantos := 0
	for _, cf := range nuestras {
		for _, m := range cf.Methods {
			if m.HasCode {
				cuantos++
			}
		}
	}
	if len(rels) != cuantos {
		t.Errorf("%d code_item para %d métodos con cuerpo: una clave colisionó", len(rels), cuantos)
	}
	for _, clave := range [][3]string{
		{"LCtor;", "<init>", "()V"},
		{"LCtor;", "<init>", "(I)V"},
	} {
		if _, ok := rels[clave]; !ok {
			t.Errorf("no hay code_item para %v", clave)
		}
	}
}

// TestElConstructorSeDexaIgualQueJavac: el dex de las dos clases, byte a byte.
func TestElConstructorSeDexaIgualQueJavac(t *testing.T) {
	nuestras, suyas := dosClasesCtor(t)
	deUno, err := dexDeClases(nuestras)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases(suyas)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	t.Logf("el dex de la sonda del párrafo 91: %d bytes", len(deUno))
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex nuestro son %d bytes y el de javac %d", len(deUno), len(deOtro))
	}
}
