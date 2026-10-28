package main

import (
	"encoding/base64"
	"testing"
)

// Lector del AXML y de la tabla de recursos, con los ficheros que aapt2 escribio sobre el proyecto
// del studio de fixture (recursosfixture_test.go). Las cuentas de abajo son las medidas en
// docs/evidencia.md §24; si el lector no las reproduce, es que esta leyendo un offset donde no era.

func axmlDelFixture(t *testing.T) *axml {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(axmlManifiesto)
	if err != nil {
		t.Fatalf("the manifest fixture is not the base64 it claims: %v", err)
	}
	if len(b) != 2000 {
		t.Fatalf("the manifest fixture is %d bytes, aapt2 wrote 2000", len(b))
	}
	a, err := leeAXML(b)
	if err != nil {
		t.Fatalf("reading aapt2's own manifest: %v", err)
	}
	return a
}

func tablaDelFixture(t *testing.T) *tablaRecursos {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(arscTabla)
	if err != nil {
		t.Fatalf("the table fixture is not base64: %v", err)
	}
	if len(b) != 1088 {
		t.Fatalf("the table fixture is %d bytes, aapt2 wrote 1088", len(b))
	}
	tr, err := leeTabla(b)
	if err != nil {
		t.Fatalf("reading aapt2's own resource table: %v", err)
	}
	return tr
}

// TestLectorDeAXMLReproduceLoMedido: 31 cadenas en UTF-16, 12 ids de atributo, 20 nodos y la
// cuenta exacta de atributos por etiqueta. Y el nombre que aapt2 deja en cada nodo, para que la
// comparacion no sea solo de numeros.
func TestLectorDeAXMLReproduceLoMedido(t *testing.T) {
	a := axmlDelFixture(t)
	if len(a.tabla.cadenas) != 31 {
		t.Errorf("%d strings in the pool, §24 measured 31", len(a.tabla.cadenas))
	}
	if a.tabla.utf8 {
		t.Error("the manifest pool is UTF-16 in the file (§24) and the reader says UTF-8")
	}
	if len(a.idsMapa) != 12 {
		t.Errorf("%d attribute ids in the resource map, measured 12", len(a.idsMapa))
	}
	if a.idsMapa[0] != 0x01010000 {
		t.Errorf("the first mapped id is %#08x, the file has 0x01010000", a.idsMapa[0])
	}
	if len(a.nodos) != 16 {
		t.Errorf("%d node chunks, §24's dump has sixteen (two namespaces and fourteen tags)", len(a.nodos))
	}

	// la raiz: <manifest> con los siete atributos que imprimio aapt2 dump, en su orden
	primero := a.nodos[1]
	if primero.tipo != trozoInicioEtiqueta || a.nombreDe(primero.nombre) != "manifest" {
		t.Errorf("the second node is %s %q, expected the <manifest> start tag",
			nombreTrozo(primero.tipo), a.nombreDe(primero.nombre))
	}
	if len(primero.attrs) != 7 {
		t.Fatalf("<manifest> has %d attributes, aapt2 wrote 7", len(primero.attrs))
	}
	nombres := []string{}
	for _, at := range primero.attrs {
		nombres = append(nombres, a.nombreDe(at.nombre))
	}
	esperados := []string{"versionCode", "versionName", "compileSdkVersion", "compileSdkVersionCodename",
		"package", "platformBuildVersionCode", "platformBuildVersionName"}
	for i, e := range esperados {
		if nombres[i] != e {
			t.Errorf("attribute #%d is %q, aapt2 dump says %q", i, nombres[i], e)
		}
	}
	// los dos tipos de valor que se midieron: entero decimal y cadena
	if primero.attrs[0].valor.tipo != 0x10 || primero.attrs[0].valor.datos != 1 {
		t.Errorf("versionCode reads as tipo %#02x datos %d, measured 0x10 and 1",
			primero.attrs[0].valor.tipo, primero.attrs[0].valor.datos)
	}
	if primero.attrs[1].valor.tipo != 0x03 || a.tabla.cadenas[primero.attrs[1].valor.datos] != "0.1" {
		t.Errorf("versionName: tipo %#02x, datos %d (%q)", primero.attrs[1].valor.tipo,
			primero.attrs[1].valor.datos, a.tabla.cadenas[primero.attrs[1].valor.datos])
	}
	// y el xmlns:android, que es el primer trozo de nodo del fichero
	if a.nodos[0].tipo != trozoInicioNS || a.nombreDe(a.nodos[0].nombre) != "http://schemas.android.com/apk/res/android" {
		t.Errorf("the first node is not the android namespace declaration")
	}
}

// TestLectorDeTablaReproduceLoMedido: pool global de una cadena en UTF-8, paquete 127 con su nombre
// de tres tipos y tres claves, cuatro bloques de tipo y las entradas con su clave y su valor.
func TestLectorDeTablaReproduceLoMedido(t *testing.T) {
	tr := tablaDelFixture(t)
	if len(tr.global.cadenas) != 1 || tr.global.cadenas[0] != "res/layout/main.xml" {
		t.Errorf("the global pool is %v, §24 measured one string: res/layout/main.xml", tr.global.cadenas)
	}
	if !tr.global.utf8 {
		t.Error("the global pool is UTF-8 in the file and the reader says otherwise")
	}
	if tr.idPaquete != 127 || tr.nombrePaquete != "com.hola.apkcstudio" {
		t.Errorf("package %d %q, aapt2 wrote 127 %q", tr.idPaquete, tr.nombrePaquete, "com.hola.apkcstudio")
	}
	if len(tr.nombresTipo) != 3 || len(tr.claves) != 3 {
		t.Fatalf("%d type names and %d keys, measured 3 and 3", len(tr.nombresTipo), len(tr.claves))
	}
	if len(tr.bloques) != 4 {
		t.Fatalf("%d type blocks, §24 counted four (one of them twice, for two configurations)", len(tr.bloques))
	}

	// id / layout / style, con su entrada y su valor
	primero := tr.bloques[0]
	if primero.nombre != "id" || len(primero.entradas) != 1 {
		t.Errorf("the first block is type %q with %d entries", primero.nombre, len(primero.entradas))
	}
	e := primero.entradas[0]
	if tr.clave(e) != "pulsar" {
		t.Errorf("the entry of type id is keyed %q, the file says pulsar", tr.clave(e))
	}
	if e.flags != 0x04 || e.tam != 8 {
		t.Errorf("pulsar: tam %d flags %#04x, §24's dump says 8 and 0x04", e.tam, e.flags)
	}
	if len(primero.config) != 64 {
		t.Errorf("the ResTable_config of this aapt2 is %d bytes, §24 measured 64", len(primero.config))
	}

	seg := tr.bloques[1]
	if seg.nombre != "layout" || tr.clave(seg.entradas[0]) != "main" {
		t.Errorf("the second block is %q keyed %q", seg.nombre, tr.clave(seg.entradas[0]))
	}
	v := seg.entradas[0].valor
	if v.tipo != 0x03 || tr.global.cadenas[int(v.datos)] != "res/layout/main.xml" {
		t.Errorf("layout main: tipo %#02x datos %d, expected a string reference into the global pool",
			v.tipo, v.datos)
	}

	ter := tr.bloques[2]
	if ter.nombre != "style" || tr.clave(ter.entradas[0]) != "AppTheme" {
		t.Errorf("the third block is %q keyed %q", ter.nombre, tr.clave(ter.entradas[0]))
	}
	cuatro := ter.entradas[0]
	if !cuatro.compleja {
		t.Error("a style must read as a complex value")
	}
	if cuatro.padre != 0x0103012b {
		t.Errorf("the style's parent reads as %#08x, the bytes say 0x0103012b (an android style id)", cuatro.padre)
	}
	// El valor complejo NO se interpreta todavia: se guarda el bloque de registros de doce bytes y
	// se compara su forma. Lo que queda por medir es el campo de recuento, que en los dos bloques
	// dice 4 mientras el segundo trae cinco registros (parrafo 26).
	if len(cuatro.crudo) != 48 || len(cuatro.crudo)/12 != 4 {
		t.Errorf("the first style block carries %d bytes of records, the measured 48 (four of twelve)", len(cuatro.crudo))
	}
	if cuatro.recuentos != 4 {
		t.Errorf("the declared count is %d, the file says 4", cuatro.recuentos)
	}
	segundo := tr.bloques[3].entradas[0]
	if len(segundo.crudo) != 60 {
		t.Errorf("the second style block carries %d bytes of records, §24 has 60 (five of twelve)", len(segundo.crudo))
	}
	t.Logf("AppTheme: dos configuraciones, %d y %d registros de 12; el recuento declarado es %d en ambas",
		len(cuatro.crudo)/12, len(segundo.crudo)/12, cuatro.recuentos)
	if tr.bloques[3].nombre != "style" || tr.clave(tr.bloques[3].entradas[0]) != "AppTheme" {
		t.Errorf("the fourth block should be the second configuration of the same style")
	}
}

// TestLectoresRechazanBytesMal: un byte cambiado en un desplazamiento de cadenas tiene que dar un
// error dicho por su nombre, no un pool raro ni un panic. Es la misma precaucion que en el lector
// de dex: si el fichero del fork vecino no cuadra, se para.
func TestLectoresRechazanBytesMal(t *testing.T) {
	b, _ := base64.StdEncoding.DecodeString(axmlManifiesto)

	// la cabecera XML mintirosa: file size que no es el del fichero
	malo := append([]byte{}, b...)
	malo[4] = 0xf0
	if _, err := leeAXML(malo); err == nil {
		t.Error("a manifest claiming a different size was accepted")
	}

	// el offset de la primera cadena, fuera del area del pool
	malo = append([]byte{}, b...)
	malo[28+3] = 0x7f
	if _, err := leeAXML(malo); err == nil {
		t.Error("a string offset past the end of its pool was accepted")
	}

	// un nombre de etiqueta fuera de la pool
	malo = append([]byte{}, b...)
	malo[1216+3] = 0x7f
	if _, err := leeAXML(malo); err == nil {
		t.Error("an unknown chunk type was accepted")
	}

	t2, _ := base64.StdEncoding.DecodeString(arscTabla)
	malo = append([]byte{}, t2...)
	malo[8] = 3 // packageCount
	if _, err := leeTabla(malo); err == nil {
		t.Error("a table declaring three packages was accepted")
	}
	malo = append([]byte{}, t2...)
	malo[68+2] = 200 // headerSize del paquete
	if _, err := leeTabla(malo); err == nil {
		t.Error("a package with a made-up header size was accepted")
	}
	malo = append([]byte{}, t2...)
	malo[860+6] = 0x3f // clave de la entrada de estilo, fuera de la tabla de claves
	if _, err := leeTabla(malo); err == nil {
		t.Error("an entry naming a key that is not in the table was accepted")
	}
}
