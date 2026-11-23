package main

// Los monitores, medidos con `javac`, `d8` y `dexdump`. `monitorenter` 0xc2 y `monitorexit` 0xc3
// del JVMS salen como `monitor-enter` 0x1d y `monitor-exit` 0x1e (11x) en DEX. La sonda `Moni` los
// escribe tanto en un método estático como en uno de instancia, y `d8` deja `1d01` al entrar y
// `1e01` al salir (más el `monitorexit` de la salida normal).

import (
	"encoding/base64"
	"testing"
)

// moniJavac: `javac 21 --release 8 -g:lines,source Moni.java` y `base64 -w0 Moni.class`.
const moniJavac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgACAAgMAAkACgEACGhhc2hDb2RlAQADKClJBwAMAQAETW9uaQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAARzaW5jAQAVKExqYXZhL2xhbmcvT2JqZWN0OylWAQANU3RhY2tNYXBUYWJsZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUBAAhzaW5jSW5zdAEAClNvdXJjZUZpbGUBAAlNb25pLmphdmEAIQALAAIAAAAAAAMAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgADwAQAAEADQAAAFsAAgADAAAAFCpZTMIqtgAHVyvDpwAITSvDLL+xAAIABAALAA4AAAAOABEADgAAAAIADgAAAAoAAgAAAAMAEwAEABEAAAAVAAL/AA4AAgcAAgcAAgABBwAS+gAEAAAAFAAQAAEADQAAAF4AAgAEAAAAFCpZTcIrtgAHVyzDpwAITizDLb+xAAIABAALAA4AAAAOABEADgAAAAIADgAAAAoAAgAAAAYAEwAHABEAAAAYAAL/AA4AAwcACwcAAgcAAgABBwAS+gAEAAEAFQAAAAIAFg=="

func claseMoniDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(moniJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexWalkJvmCodeAceptaMonitores: el caminante acepta `monitorenter` 0xc2 y `monitorexit` 0xc3,
// cada uno de un byte y sin operandos. Falla si se quita cualquiera de las dos entradas de
// `jvmMeasured`: `walkJvmCode` para en el byte con «opcode outside the table this fork measured».
func TestDexWalkJvmCodeAceptaMonitores(t *testing.T) {
	casos := []struct {
		op     byte
		nombre string
	}{{0xc2, "monitorenter"}, {0xc3, "monitorexit"}}
	for _, c := range casos {
		lista, err := walkJvmCode([]byte{c.op, 0xb1})
		if err != nil {
			t.Fatalf("%#02x: el caminante lo rechaza: %v", c.op, err)
		}
		if len(lista) != 2 || lista[0].op != c.op || lista[0].name != c.nombre || len(lista[0].ops) != 0 {
			t.Errorf("%#02x: esperaba %s de cero operandos, salió %+v", c.op, c.nombre, lista)
		}
	}
}

// TestDexWalkJvmCodeRechazaMonitorSinEntrada: la prueba contraria de la tabla. Quitando `monitorenter`
// en caliente —y devolviéndolo al terminar— el caminante vuelve a pararse en el 0xc2 con el error que
// destapó el jar. Deja la tabla atada a la prueba.
func TestDexWalkJvmCodeRechazaMonitorSinEntrada(t *testing.T) {
	guardada, habia := jvmMeasured[0xc2]
	delete(jvmMeasured, 0xc2)
	defer func() {
		if habia {
			jvmMeasured[0xc2] = guardada
		}
	}()
	if _, err := walkJvmCode([]byte{0xc2, 0xb1}); err == nil {
		t.Fatal("sin la entrada de monitorenter, el caminante lo aceptó como si estuviera medida")
	}
}

// TestDexMonitoresDeUnClassDeJar: el dexer acepta el `.class` de javac y baja los dos monitores a
// `monitor-enter` 0x1d y `monitor-exit` 0x1e. La prueba muere si el dispatch de `monitor` o las dos
// entradas del caminante desaparecen.
func TestDexMonitoresDeUnClassDeJar(t *testing.T) {
	clase := claseMoniDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	for _, nombre := range []string{"sinc", "sincInst"} {
		m, ok := buscaMetodo(clase, nombre)
		if !ok {
			t.Fatalf("%s: no está en el fixture de javac", nombre)
		}
		ci, err := td.codeItemDe(clase, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := map[byte]bool{}
		for _, u := range ci.insns {
			visto[byte(u)] = true
		}
		if !visto[opMonitorEntra] {
			t.Errorf("%s: no aparece monitor-enter %#02x en %04x", nombre, opMonitorEntra, ci.insns)
		}
		if !visto[opMonitorSale] {
			t.Errorf("%s: no aparece monitor-exit %#02x en %04x", nombre, opMonitorSale, ci.insns)
		}
	}
}
