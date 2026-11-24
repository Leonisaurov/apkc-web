package main

// El emisor de `.class` escribía siempre `ldc` (0x12), con el índice del pool en un byte, y cuando el
// pool pasaba de 255 se negaba a seguir: «el pool de share/apps/util/MainActivity pasa de 255 entradas
// y `ldc_w` no esta medido en este emisor». Lo destapó ApkShare-main al cerrar `Intent.createChooser`,
// que añadió constantes a `MainActivity` hasta pasar el límite (medido con el binario de e3a46a1).
// JVMS 6.5: a partir de 256 la forma es `ldc_w` (0x13), con dos bytes de índice, y la instrucción ocupa
// uno más; el byte de más corre las posiciones que se anotan después y los saltos se resuelven luego.
//
// El oráculo es el de siempre en este fork: javac 17 `--release 8 -g:lines,source` y el dex byte a
// byte. Además se exige la regla del límite sobre nuestro `.class` —`ldc` solo con índice ≤ 255 y
// `ldc_w` solo por encima—, porque el dex no distingue las dos formas: una `ldc_w` con índice corto
// dexaría igual y estaría mal. Los `max_stack`/`max_locals` se comparan método a método.

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuentePoolGrande arma una clase con más de 256 constantes distintas. Cada método devuelve un literal
// que no se repite: javac mete cada cadena como `CONSTANT_String` con su `Utf8` y cada entero grande
// como `CONSTANT_Integer`, así que con doscientos métodos el pool pasa de largo el límite y el último
// literal solo cabe en `ldc_w`. Los enteros son mayores que 32767 para que no quepan en `sipush`.
func fuentePoolGrande(cadenas, enteros int) string {
	var b strings.Builder
	b.WriteString("package p;\npublic class PoolGrande {\n")
	for i := 0; i < cadenas; i++ {
		fmt.Fprintf(&b, "    static String c%d() { return \"cadena-del-pool-%03d\"; }\n", i, i)
	}
	for i := 0; i < enteros; i++ {
		fmt.Fprintf(&b, "    static int n%d() { return %d; }\n", i, 40000+i)
	}
	// Los dos de abajo cargan sus literales con el pool ya pasado de 255 y además tienen ramas y una
	// tabla de excepciones: el byte de más del `ldc_w` corre las posiciones que se anotan después, y
	// una fila de línea o un manejador movido rompen el dex, que lleva `debug_info` y los `try`.
	b.WriteString("    static int conSalto(int n) {\n")
	b.WriteString("        if (n > 3) { return 40099; }\n")
	b.WriteString("        return 40098;\n")
	b.WriteString("    }\n")
	b.WriteString("    static String conTry(String s) {\n")
	b.WriteString("        try { return s; } catch (RuntimeException e) { return \"cadena-del-fallo\"; }\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}

// compilaConJavac17: el `.class` de referencia de la sonda. Se compara después con el nuestro.
func compilaConJavac17(t *testing.T, raiz, fuente string) classFull {
	t.Helper()
	javac := javac17(t)
	jar := jarDelSDK(t)
	pon(t, raiz, "PoolGrande.java", fuente)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "PoolGrande.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó la sonda: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejó %d clases, una", len(deJavac))
	}
	return deJavac[0]
}

// nuestraClase: emite la sonda con el emisor del fork y la vuelve a leer.
func nuestraClase(t *testing.T, fuente string) classFull {
	t.Helper()
	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "PoolGrande.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "p/PoolGrande" {
		t.Fatalf("salieron %d clases (%s), y una p/PoolGrande", len(nuestras), nombresDe(nuestras))
	}
	cf, err := readClassFull(nuestras[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se relee: %v", err)
	}
	return cf
}

// mismoDex: el oráculo estricto del repo, el dex entero byte a byte. El `.class` puede diferir en el
// orden del pool; el dex no, y un índice truncado carga otra constante y lo rompe.
func mismoDex(t *testing.T, nuestra, suya classFull) {
	t.Helper()
	dexNuestro, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	dexJavac, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x", i, dexNuestro[i], dexJavac[i])
		}
	}
}

// formasDelLdc devuelve los índices que nuestro `.class` escribe con cada forma. La regla del límite
// es lo que se comprueba: `ldc` (0x12) solo puede llevar índices de un byte y `ldc_w` (0x13) solo
// índices que no caben en él. Es la mitad que el dex no ve.
func formasDelLdc(t *testing.T, cf classFull) (cortas, largas []int) {
	t.Helper()
	for _, m := range cf.Methods {
		if !m.HasCode {
			continue
		}
		lista, err := walkJvmCode(m.Code)
		if err != nil {
			t.Fatalf("%s: el Code no se recorre: %v", m.Name, err)
		}
		for _, in := range lista {
			switch in.name {
			case "ldc":
				if len(in.ops) != 1 {
					t.Fatalf("%s: `ldc` con %d operandos", m.Name, len(in.ops))
				}
				cortas = append(cortas, int(in.ops[0]))
			case "ldc_w":
				if len(in.ops) != 2 {
					t.Fatalf("%s: `ldc_w` con %d operandos", m.Name, len(in.ops))
				}
				largas = append(largas, u2De(in.ops, 0))
			}
		}
	}
	return cortas, largas
}

// TestElLdcWEscribeLaFormaLargaYCoincideConJavac17: el pool de la sonda pasa de 255 y nuestro `.class`
// tiene que resolverlo como javac: forma corta donde cabe y larga donde no, con el mismo dex byte a
// byte y las mismas pilas declaradas. Antes del arreglo esta compilación se rechazaba con el error del
// usuario y no salía ningún `.class`.
func TestElLdcWEscribeLaFormaLargaYCoincideConJavac17(t *testing.T) {
	fuente := fuentePoolGrande(200, 80)
	nuestra := nuestraClase(t, fuente)
	suya := compilaConJavac17(t, t.TempDir(), fuente)

	// premisa: la sonda de javac de verdad pasa de 255 y usa `ldc_w` para lo que no cabe.
	_, largasJavac := formasDelLdc(t, suya)
	if len(largasJavac) == 0 {
		t.Fatalf("el `.class` de javac no trae `ldc_w`: la sonda no pasó de 255 entradas")
	}
	for _, idx := range largasJavac {
		if idx <= 255 {
			t.Errorf("javac escribe `ldc_w` con el índice %d: la premisa de la sonda no se cumple", idx)
		}
	}

	cortas, largas := formasDelLdc(t, nuestra)
	if len(largas) == 0 {
		t.Fatalf("nuestro `.class` no trae `ldc_w` con el pool por encima de 255 (%d `ldc` cortos)", len(cortas))
	}
	for _, idx := range cortas {
		if idx > 255 {
			t.Errorf("`ldc` con el índice %d: no cabe en un byte y JVMS 6.5 pide `ldc_w`", idx)
		}
	}
	for _, idx := range largas {
		if idx <= 255 {
			t.Errorf("`ldc_w` con el índice %d: cabe en un byte y javac usa `ldc`", idx)
		}
		if idx > 0xffff {
			t.Errorf("`ldc_w` con el índice %d: no cabe en dos bytes", idx)
		}
	}

	// las pilas declaradas, método a método: la forma larga no cambia ni una palabra de la pila.
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el método %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if !esperado.HasCode {
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s: max_stack %d y locals %d, javac declara %d y %d", esperado.Name,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
	}

	mismoDex(t, nuestra, suya)
}

// TestElPoolCortoSeQuedaEnLdc es el control: con menos de 256 entradas no hay ni una `ldc_w` y el
// `.class` sigue siendo el mismo de antes. Si el emisor escribiera la forma larga siempre, el dex
// seguiría coincidiendo y solo esta prueba lo vería.
func TestElPoolCortoSeQuedaEnLdc(t *testing.T) {
	fuente := fuentePoolGrande(3, 2)
	nuestra := nuestraClase(t, fuente)
	suya := compilaConJavac17(t, t.TempDir(), fuente)

	cortas, largas := formasDelLdc(t, nuestra)
	if len(cortas) == 0 {
		t.Fatalf("la sonda no cargó ni una constante corta: no mide nada")
	}
	if len(largas) != 0 {
		t.Errorf("con un pool corto salen %d `ldc_w` (%v)", len(largas), largas)
	}
	mismoDex(t, nuestra, suya)
}

// TestElLdcEligeLaFormaEnElLimite255 fija el límite 255/256 en los bytes que escribe el emisor, sin
// depender de cómo quede el pool de una compilación entera: 255 va en `ldc`, 256 ya pide `ldc_w`, el
// mayor índice de dos bytes también, y uno que no cabe en dos bytes se rechaza por nombre.
func TestElLdcEligeLaFormaEnElLimite255(t *testing.T) {
	casos := []struct {
		idx     int
		espera  []byte
		cortado bool
	}{
		{0, []byte{opLdc, 0x00}, false},
		{254, []byte{opLdc, 0xfe}, false},
		{255, []byte{opLdc, 0xff}, false},
		{256, []byte{opLdcW, 0x01, 0x00}, false},
		{257, []byte{opLdcW, 0x01, 0x01}, false},
		{0x7fff, []byte{opLdcW, 0x7f, 0xff}, false},
		{0xffff, []byte{opLdcW, 0xff, 0xff}, false},
		{0x10000, []byte{opLdc, 0x00}, true},
	}
	for _, c := range casos {
		e := &emisor{}
		e.ldc(c.idx)
		if !bytes.Equal(e.codigo, c.espera) {
			t.Errorf("ldc(%d) escribe %#02x y se espera %#02x", c.idx, e.codigo, c.espera)
		}
		if e.truncado != c.cortado {
			t.Errorf("ldc(%d) deja truncado=%v y se espera %v", c.idx, e.truncado, c.cortado)
		}
	}
}

// TestElInterruptorFuerzaLaFormaCorta reproduce la regresión de la falsificación: con
// `ldcSinFormaLarga` el emisor vuelve a escribir `ldc` corto para cualquier índice y la sonda del pool
// grande se rechaza con el aviso del usuario. Sin el arreglo, este es el camino que tomaba ApkShare.
func TestElInterruptorFuerzaLaFormaCorta(t *testing.T) {
	ldcSinFormaLarga = true
	defer func() { ldcSinFormaLarga = false }()

	_, err := compilarProyectoJava([]fuenteJava{{nombre: "PoolGrande.java", contenido: fuentePoolGrande(200, 80)}}, jarDelSDK(t))
	if err == nil {
		t.Fatalf("con el interruptor encendido el pool grande se aceptó: la sonda no mide el defecto")
	}
	if !strings.Contains(err.Error(), "pasa de 255 entradas") {
		t.Fatalf("el rechazo no nombra el pool: %v", err)
	}
}

// TestElLdcWNoCambiaElMaxStack: la forma larga no cambia el `max_stack`. Se compara el último método
// `return <entero>` de la sonda larga (índice por encima de 255) con el de la sonda corta: las dos
// declaran la misma pila y el mismo local, y el cuerpo difiere en el byte del índice, nada más.
func TestElLdcWNoCambiaElMaxStack(t *testing.T) {
	larga := nuestraClase(t, fuentePoolGrande(200, 80))
	corta := nuestraClase(t, fuentePoolGrande(3, 2))

	deLarga, ok := metodoDeLaClase(larga, "n79")
	if !ok {
		t.Fatal("la sonda larga no tiene `n79`")
	}
	deCorta, ok := metodoDeLaClase(corta, "n1")
	if !ok {
		t.Fatal("la sonda corta no tiene `n1`")
	}
	if deLarga.MaxStack != deCorta.MaxStack || deLarga.LocalCount != deCorta.LocalCount {
		t.Errorf("max_stack %d y locals %d con el pool largo, %d y %d con el corto",
			deLarga.MaxStack, deLarga.LocalCount, deCorta.MaxStack, deCorta.LocalCount)
	}
	if len(deLarga.Code) != len(deCorta.Code)+1 {
		t.Errorf("el Code de `return 40079` mide %d con `ldc_w` y %d con `ldc`: se espera un byte más",
			len(deLarga.Code), len(deCorta.Code))
	}
	forma := func(m classMethod) string {
		lista, err := walkJvmCode(m.Code)
		if err != nil || len(lista) == 0 {
			t.Fatalf("%s: el Code no se recorre: %v", m.Name, err)
		}
		return lista[0].name
	}
	if f := forma(deLarga); f != "ldc_w" {
		t.Errorf("el último método de la sonda larga carga su entero con %q y se espera `ldc_w`", f)
	}
	if f := forma(deCorta); f != "ldc" {
		t.Errorf("el último método de la sonda corta carga su entero con %q y se espera `ldc`", f)
	}
}
