package main

import (
	"encoding/base64"
	"testing"
)

// Las dos clases del banco de `$PREFIX/tmp/e61`, en base64 y sin tocar disco:
//   - `Ver.class` es el `.class` que escribe **este fork** (`apkc compile` sobre el fuente de abajo),
//     con un método `static void cuenta()` que suma, compara y multiplica un estático de clase.
//   - `Arranque.class` es de javac 21 y su `main` llama a `cuenta()` **sin argumentos** dos veces,
//     lee `Ver.total` y cierra con `System.exit(total)`.
//
// El fuente del banco, para que el fixture no sea un misterio:
//
//	package p; public class Ver { static int total = 5;
//	  public static void cuenta() { total = total + 2; if (total > 10) { total = total * 3; } } }
//	package p; public class Arranque { public static void main(String[] a) {
//	  Ver.cuenta(); Ver.cuenta(); int t = Ver.total; System.exit(t); } }
//
// Ejecutado en el aparato con `app_process` (sin instalar nada) los dos dex —el de `d8` y el nuestro—
// salen con código de salida 9, igual que la JVM con las clases de javap. §61.
const verEmitido = "yv66vgAAADQAEwEABXAvVmVyBwABAQAQamF2YS9sYW5nL09iamVjdAcAAwEABXRvdGFs" +
	"AQABSQEABjxpbml0PgEAAygpVgwABwAICgAEAAkBAA9MaW5lTnVtYmVyVGFibGUBAARDb2RlDAAFAAYJAAIADQEABmN1" +
	"ZW50YQEACDxjbGluaXQ+AQAIVmVyLmphdmEBAApTb3VyY2VGaWxlACEAAgAEAAAAAQAIAAUABgAAAAMAAQAHAAgAAQAM" +
	"AAAAHQABAAEAAAAFKrcACrEAAAABAAsAAAAGAAEAAAADAAkADwAIAAEADAAAAD0AAgAAAAAAGbIADgVgswAOsgAOEAqk" +
	"AAuyAA4GaLMADrEAAAABAAsAAAASAAQAAAAHAAgACAAQAAkAGAALAAgAEAAIAAEADAAAAB0AAQAAAAAABQizAA6xAAAA" +
	"AQALAAAABgABAAAABAABABIAAAACABE="

const arranqueJavac = "yv66vgAAAEEAHgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+" +
	"AQADKClWCgAIAAkHAAoMAAsABgEABXAvVmVyAQAGY3VlbnRhCQAIAA0MAA4ADwEABXRvdGFsAQABSQoAEQASBwATDAAU" +
	"ABUBABBqYXZhL2xhbmcvU3lzdGVtAQAEZXhpdAEABChJKVYHABcBAApwL0FycmFucXVlAQAEQ29kZQEAD0xpbmVOdW1i" +
	"ZXJUYWJsZQEABG1haW4BABYoW0xqYXZhL2xhbmcvU3RyaW5nOylWAQAKU291cmNlRmlsZQEADUFycmFucXVlLmphdmEA" +
	"IQAWAAIAAAAAAAIAAQAFAAYAAQAYAAAAHQABAAEAAAAFKrcAAbEAAAABABkAAAAGAAEAAAADAAkAGgAbAAEAGAAAADcA" +
	"AQACAAAAD7gAB7gAB7IADDwbuAAQsQAAAAEAGQAAABYABQAAAAUAAwAGAAYABwAKAAgADgAJAAEAHAAAAAIAHQ=="

func leeFixture(t *testing.T, nombre, b64 string) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("%s no es base64: %v", nombre, err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("%s no leyó: %v", nombre, err)
	}
	return cf
}

// TestLlamadaSinArgumentosSaleTresCincoceta: `d8` escribe el invoke-static de cero argumentos con la
// forma 35c de siempre —medido en el fichero del aparato: `7100 0600 0000`, es decir recuento a cero
// en el byte alto de la primera unidad, el índice del método detras y la tercera unidad en cero—.
// Este fork la rechazaba (`n < 1`), y el rechazo saltó en cuanto el banco de pruebas empezó a *ejecutar*
// los dex en lugar de solo compararlos.
func TestLlamadaSinArgumentosSaleTresCincoceta(t *testing.T) {
	ver := leeFixture(t, "Ver", verEmitido)
	arranque := leeFixture(t, "Arranque", arranqueJavac)
	clases := []classFull{ver, arranque}
	td := newDexTables()
	if err := td.collectClasses(clases); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	b, err := td.armaDex(clases)
	if err != nil {
		t.Fatalf("ensamblando el dex: %v", err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("el lector rechaza lo que escribimos: %v", err)
	}
	verificaLayout(t, b, "el dex de la llamada sin argumentos")

	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("leyendo class_data: %v", err)
	}
	cuentaOff := uint32(0)
	for _, m := range append(append([]encodedMethod{}, porClase["Lp/Arranque;"].directs...),
		porClase["Lp/Arranque;"].virtuals...) {
		if d.methods[m.methodIdx].name == "main" {
			cuentaOff = m.codeOff
		}
	}
	if cuentaOff == 0 {
		t.Fatal("no encontró el main de Lp/Arranque;")
	}
	ci, err := d.readCodeItem(cuentaOff)
	if err != nil {
		t.Fatalf("leyendo el code_item: %v", err)
	}
	// la cabecera que publicó `dexdump -d`: un registro, uno de entrada (el String[], aunque no se
	// use) y una salida. El nuestro declara **tres** registros porque los temporales se colocan por
	// encima de la cima de pila que midió javap (parrafo 50) y `d8` reutiliza el hueco del argumento:
	// es la diferencia de numeración ya aceptada en el 2G.4, y aquí queda medida otra vez
	if ci.insSize != 1 || ci.outsSize != 1 || ci.registersSize < 1 {
		t.Errorf("registers/ins/outs = %d/%d/%d, d8 escribe 1/1/1 y el nuestro 3/1/1 por el borde de temporales",
			ci.registersSize, ci.insSize, ci.outsSize)
	}
	if len(ci.insns) != 12 {
		t.Fatalf("%d palabras, d8 escribe 12", len(ci.insns))
	}

	// las dos llamadas sin argumentos: el recuento de argumentos es el **nibble alto** del segundo byte
	for k, en := range []int{0, 3} {
		op := ci.insns[en] & 0xff
		recuento := ci.insns[en] >> 12
		if op != 0x71 || recuento != 0 {
			t.Errorf("llamada #%d: %#04x, se esperaba 0x71 con recuento 0 (%#04x)", k, ci.insns[en], 0x0071)
		}
		if ci.insns[en+2] != 0 {
			t.Errorf("llamada #%d: la tercera unidad es %#04x, con cero argumentos va en cero", k, ci.insns[en+2])
		}
		m := d.methods[ci.insns[en+1]]
		if m.name != "cuenta" || m.owner != "Lp/Ver;" {
			t.Errorf("llamada #%d: nombra %s.%s, se esperaba Lp/Ver;.cuenta", k, m.owner, m.name)
		}
		if descriptorDe(m.proto) != "()V" {
			t.Errorf("llamada #%d: descriptor %s, se esperaba ()V", k, descriptorDe(m.proto))
		}
	}
	// el `sget v0, Lp/Ver;.total:I` de la unidad 6 y el `invoke-static {v0}, System.exit` de la 7
	if op := ci.insns[6] & 0xff; op != 0x60 {
		t.Errorf("unidad 6: opcode %#02x, se esperaba 0x60 (sget)", op)
	}
	if f := d.fields[ci.insns[7]]; f.name != "total" || f.typeOf != "I" {
		t.Errorf("sget nombra %s:%s, se esperaba total:I", f.name, f.typeOf)
	}
	if op := ci.insns[8] & 0xff; op != 0x71 || ci.insns[8]>>12 != 1 {
		t.Errorf("unidad 8: %#04x, se esperaba invoke-static con un argumento", ci.insns[8])
	}
	// el registro del argumento: en cero porque nuestro `sget` escribe en el mismo registro que lee
	// `exit`, aunque `d8` lo ponga en v0 (son 3 registros contra su 1: la diferencia de numeración)
	if m := d.methods[ci.insns[9]]; m.name != "exit" || m.owner != "Ljava/lang/System;" {
		t.Errorf("unidad 9: nombra %s.%s, se esperaba Ljava/lang/System;.exit", m.owner, m.name)
	}
	if op := ci.insns[11] & 0xff; op != 0x0e {
		t.Errorf("última palabra %#04x, se esperaba return-void (0x000e)", ci.insns[11])
	}
}

// TestNuestrasClasesEmitidasSeDexanYLeyen: el banco también deja constancia de que el `.class` que
// escribe **este fork** entra en el dexer y sale con las tres palabras medidas para la llamada a
// `cuenta()`. Es la mitad que no cubría el oráculo de `javap`: que lo nuestro sea consumible por el
// traductor y ejecute en ART (el `app_process` del §61 da código de salida 9, el mismo que la JVM).
func TestNuestrasClasesEmitidasSeDexanYLeyen(t *testing.T) {
	ver := leeFixture(t, "Ver", verEmitido)
	td := newDexTables()
	if err := td.collectClasses([]classFull{ver}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	b, err := td.armaDex([]classFull{ver})
	if err != nil {
		t.Fatalf("ensamblando: %v", err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatalf("el lector rechaza lo que escribimos: %v", err)
	}
	verificaLayout(t, b, "el dex de nuestra clase emitida")
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("class_data: %v", err)
	}
	encontrado := false
	for _, m := range append(append([]encodedMethod{}, porClase["Lp/Ver;"].directs...),
		porClase["Lp/Ver;"].virtuals...) {
		if d.methods[m.methodIdx].name != "cuenta" {
			continue
		}
		encontrado = true
		ci, err := d.readCodeItem(m.codeOff)
		if err != nil {
			t.Fatalf("cuenta: %v", err)
		}
		if ci.registersSize < 1 || ci.insSize != 0 || ci.outsSize != 0 {
			t.Errorf("cuenta: %d/%d/%d, un método static sin argumentos y sin salida es */0/0",
				ci.registersSize, ci.insSize, ci.outsSize)
		}
	}
	if !encontrado {
		t.Error("el class_data no anuncia el método cuenta")
	}
}
