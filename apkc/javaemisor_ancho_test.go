package main

import (
	"encoding/base64"
	"math"
	"testing"
)

// La sonda del párrafo 82: `javac --release 8 -g:lines,source Ancho.java` y `base64 -w0`, con la
// forma que destapó tres defectos ejecutando en ART —un `double` y un `float` vivos a la vez, un
// `instanceof` sobre un tipo (`CharSequence`) que no vuelve a nombrarse en todo el fichero, y un
// `long` en un local por encima del 3, que javac lee y escribe con las formas indexadas—:
//
//	public class Ancho {
//	    public static int uno(int n) {
//	        double d = n * 2.5 + 1.5f;
//	        long l = (long) d;
//	        Object o = "cadena";
//	        if (o instanceof String) { l = l + 1; }
//	        return (int) l;
//	    }
//	    public static int dos(int n) { … con `instanceof CharSequence` de más … }
//	}
const anchoJavac = "yv66vgAAADQALQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBkAEAAAAAAAABj/4AAAAAAAACAAMAQAGY2FkZW5hBwAOAQAQamF2YS9sYW5nL1N0cmluZwcAEAEAFmphdmEvbGFuZy9DaGFyU2VxdWVuY2UJABIAEwcAFAwAFQAWAQAQamF2YS9sYW5nL1N5c3RlbQEAA291dAEAFUxqYXZhL2lvL1ByaW50U3RyZWFtOwoAGAAZBwAaDAAbABwBAARwL1cyAQADdW5vAQAEKEkpSQoAHgAfBwAgDAAhACIBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYKABgAJAwAJQAcAQADZG9zAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEADVN0YWNrTWFwVGFibGUBAARtYWluAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEAClNvdXJjZUZpbGUBAAdXMi5qYXZhACEAGAACAAAAAAAEAAEABQAGAAEAJgAAAB0AAQABAAAABSq3AAGxAAAAAQAnAAAABgABAAAAAgAJACUAHAABACYAAABqAAQABgAAAC0ahxQAB2sUAAljSCePQhILOgUZBcEADZkAByEKYUIZBcEAD5kAByEKYUIhiKwAAAACACcAAAAaAAYAAAAEAAsABQAOAAYAEgAHAB4ACAAqAAkAKAAAAAsAAv4AHgMEBwACCwAJABsAHAABACYAAABZAAQABgAAACEahxQAB2sUAAljSCePQhILOgUZBcEADZkAByEKYUIhiKwAAAACACcAAAAWAAUAAAAMAAsADQAOAA4AEgAPAB4AEAAoAAAACgAB/gAeAwQHAAIACQApACoAAQAmAAAANQACAAEAAAAVsgARBbgAF7YAHbIAEQW4ACO2AB2xAAAAAQAnAAAADgADAAAAEwAKABQAFAAVAAEAKwAAAAIALA=="

func claseAncha(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(anchoJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	return cf
}

func tablaAncha(t *testing.T) (*dexTables, classFull) {
	t.Helper()
	cf := claseAncha(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	return td, cf
}

// TestElTipoDelInstanceOfSeInterna: la clase del `instanceof` es un `type_id` aunque no vuelva a
// nombrarse en ninguna otra parte del fichero. Medido en el aparato (Android 15, `app_process`,
// 2026-11-03): sin el internado, el traductor escribía el cero del mapa ausente y el aparato cortaba
// el método con `instance-of on unexpected class Integer`; con él, `dos` da 8.
func TestElTipoDelInstanceOfSeInterna(t *testing.T) {
	td, cf := tablaAncha(t)
	m, ok := buscaMetodo(cf, "dos")
	if !ok {
		t.Fatal("`dos` no está en la sonda")
	}
	ci, err := td.codeItemDe(cf, m)
	if err != nil {
		t.Fatalf("dexando `dos`: %v", err)
	}
	vistos := []string{}
	for i := 0; i+1 < len(ci.insns); i++ {
		if byte(ci.insns[i]) != opInstanceDex {
			continue
		}
		idx := int(ci.insns[i+1])
		if idx >= len(td.types) {
			t.Fatalf("el `instance-of` de la unidad %d apunta al tipo %d y la tabla tiene %d", i, idx, len(td.types))
		}
		vistos = append(vistos, td.types[idx])
	}
	queridos := []string{"Ljava/lang/String;", "Ljava/lang/CharSequence;"}
	if len(vistos) != len(queridos) {
		t.Fatalf("los `instance-of` de `dos` apuntan a %q, y se esperaban %q", vistos, queridos)
	}
	for i, q := range queridos {
		if vistos[i] != q {
			t.Errorf("el `instance-of` %d apunta a %q y tenía que ser %q", i, vistos[i], q)
		}
	}
}

// TestLaFrameAnchaCabe: los dos valores anchos vivos y el `long` del local 4 no pueden llevar el marco
// a v16. Antes de reutilizar el par consecutivo más bajo de la bolsa, `dos` pedía 17 registros y el
// traductor lo rechazaba (o lo escribía con un local en v16, que no cabe en cuatro bits).
func TestLaFrameAnchaCabe(t *testing.T) {
	td, cf := tablaAncha(t)
	for _, nombre := range []string{"uno", "dos"} {
		m, ok := buscaMetodo(cf, nombre)
		if !ok {
			t.Fatalf("`%s` no está en la sonda", nombre)
		}
		ci, err := td.codeItemDe(cf, m)
		if err != nil {
			t.Fatalf("dexando `%s`: %v", nombre, err)
		}
		if ci.registersSize > 16 {
			t.Errorf("`%s` pide %d registros y los formatos de cuatro bits llegan a v15", nombre, ci.registersSize)
		}
		if int(ci.insSize) > int(ci.registersSize) {
			t.Errorf("`%s`: ins %d en un marco de %d", nombre, ci.insSize, ci.registersSize)
		}
	}
}

// TestElCaminanteConoceLosAnchosIndexados: `lload` (0x16) y `lstore` (0x37) con el índice de un byte
// son la forma que javac escribe cuando el local pasa del 3 —medido con `javap -c` sobre la sonda del
// párrafo 82, que tiene el `long` en el 4—. Sin ellos el caminante paraba en el primer `lstore 4`.
func TestElCaminanteConoceLosAnchosIndexados(t *testing.T) {
	lista, err := walkJvmCode([]byte{0x16, 0x04, 0x37, 0x04, 0xb1})
	if err != nil {
		t.Fatalf("el caminante no conoce las formas indexadas: %v", err)
	}
	if len(lista) != 3 {
		t.Fatalf("el cuerpo tiene %d instrucciones y se esperaban 3", len(lista))
	}
	if lista[0].name != "lload" || len(lista[0].ops) != 1 || lista[0].ops[0] != 4 {
		t.Errorf("la primera no es `lload 4`: %s %v", lista[0].name, lista[0].ops)
	}
	if lista[1].name != "lstore" || len(lista[1].ops) != 1 || lista[1].ops[0] != 4 {
		t.Errorf("la segunda no es `lstore 4`: %s %v", lista[1].name, lista[1].ops)
	}
}

// estaticosJavac: `javac --release 8 -g:lines,source Fd.java` y `base64 -w0` de la sonda con
// `static final double FD = 2.5; static final float FF = 1.5f; static double d = 3.5;`. Las dos
// primeras llevan `ConstantValue` y javac NO las asigna en el `<clinit>`; la tercera no es `final`,
// así que su valor viaja en el `<clinit>`.
const estaticosJavac = "yv66vgAAADQAHwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAEcC9GZAZAEAAAAAAAAAkABwAMDAANAA4BAAFkAQABRAZADAAAAAAAAAEAAkZEAQANQ29uc3RhbnRWYWx1ZQZABAAAAAAAAAEAAkZGAQABRgQ/wAAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA3VzbwEAAygpRAEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEAB0ZkLmphdmEAIQAHAAIAAAADABgAEQAOAAEAEgAAAAIAEwAYABUAFgABABIAAAACABcACAANAA4AAAADAAEABQAGAAEAGAAAAB0AAQABAAAABSq3AAGxAAAAAQAZAAAABgABAAAAAgAJABoAGwABABgAAAAgAAQAAAAAAAgUAAmyAAtjrwAAAAEAGQAAAAYAAQAAAAYACAAcAAYAAQAYAAAAHwACAAAAAAAHFAAPswALsQAAAAEAGQAAAAYAAQAAAAUAAQAdAAAAAgAe"

// cmpJavac: `javac --release 8 -g:lines,source Cmp.java` y `base64 -w0`, con un `long`, un `double` y
// un `float` vivos y cuatro comparaciones encadenadas. Es el método que pedía más temporales en la
// pasada que escribe las palabras que en la de medida (`Ver.compara` del banco, después del §81).
const cmpJavac = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBj/4AAAAAAAABkAAAAAAAAAABECAAAAGQFkAAAAAAAAHAA8BAANDbXABAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAHY29tcGFyYQEABChJKUkBAA1TdGFja01hcFRhYmxlAQAKU291cmNlRmlsZQEACENtcC5qYXZhACEADgACAAAAAAACAAEABQAGAAEAEAAAAB0AAQABAAAABSq3AAGxAAAAAQARAAAABgABAAAAAQAJABIAEwABABAAAACHAAQABgAAAD4ahUAfCZSeAAcaBGA7GocUAAdrSikUAAmXngAHGgRgOxqGOAUXBRILlZsABxoEYDspFAAMmJwABxoEYDsarAAAAAIAEQAAACIACAAAAAMAAwAEAA0ABQAUAAYAIAAHACQACAAwAAkAPAAKABQAAAAPAAT8AA0E/AASA/wADwILAAEAFQAAAAIAFg=="

// TestElMarcoSeReintentaCuandoLaPasadaFinalPideMas: con el long, el double y el float vivos a la vez,
// la pasada que escribe las palabras pide un registro más que el pico medido. Antes el traductor lo
// rechazaba (`it needs more temporaries than the max stack javap measured`) y el banco no dexaba;
// ahora la base sube al pico de esa pasada y se repite sola, acotada por los registros de ocho bits.
func TestElMarcoSeReintentaCuandoLaPasadaFinalPideMas(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(cmpJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	m, ok := buscaMetodo(cf, "compara")
	if !ok {
		t.Fatal("`compara` no está en la sonda")
	}
	ci, err := td.codeItemDe(cf, m)
	if err != nil {
		t.Fatalf("dexando `compara`: %v", err)
	}
	if len(ci.insns) == 0 {
		t.Fatal("`compara` salió sin cuerpos")
	}
	if ci.registersSize > 16 {
		t.Errorf("`compara` pide %d registros y los formatos de cuatro bits llegan a v15", ci.registersSize)
	}
}

// TestLosEstaticosAnchosLlevanSuTipo: el `encoded_value` de un estático tiene que ser del TIPO del
// campo, y cuando el `.class` trae `ConstantValue` lleva sus bits. Medido en el aparato (Android 15,
// `dexdump -d`, 2026-11-03): con `valueNull` en un `static double` sin `ConstantValue`, ART cortaba
// la clase con `unexpected static field initial value type: 'L' vs 'D'`. `d8` propaga la constante
// del `<clinit>` al valor estático; nosotros escribimos el cero del tipo, que el `<clinit>`
// sobrescribe al correr.
func TestLosEstaticosAnchosLlevanSuTipo(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(estaticosJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	dex, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	file, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	off := uint32(0)
	encontrada := false
	for _, c := range file.classes {
		if c.name == "Lp/Fd;" {
			off = c.staticValuesOff
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatal("la clase no está en el dex")
	}
	vals, err := file.valoresEstaticos(off)
	if err != nil {
		t.Fatalf("leyendo los estáticos: %v", err)
	}
	// el orden de `field_ids` es el de la tabla: FD, FF y d. El array trae un elemento por campo
	// estático (la cuenta que el lector tiene que cuadrar, §55); el de `d`, sin `ConstantValue`, es un
	// cero del tipo que el `<clinit>` sobrescribe al correr.
	if len(vals) != 3 {
		t.Fatalf("la sonda tiene %d valores estáticos y se esperaban 3 (FD, FF y d)", len(vals))
	}
	if vals[0].valueType != valueDouble || vals[0].value != math.Float64bits(2.5) {
		t.Errorf("FD sale con tipo %#02x y valor %#x; se esperaba %#02x y %#x",
			vals[0].valueType, vals[0].value, valueDouble, math.Float64bits(2.5))
	}
	if vals[1].valueType != valueFloat || vals[1].value != uint64(math.Float32bits(1.5)) {
		t.Errorf("FF sale con tipo %#02x y valor %#x; se esperaba %#02x y %#x",
			vals[1].valueType, vals[1].value, valueFloat, uint64(math.Float32bits(1.5)))
	}
	if vals[2].valueType != valueDouble || vals[2].value != 0 {
		t.Errorf("d sale con tipo %#02x y valor %#x; se esperaba el cero de un `double` (%#02x)",
			vals[2].valueType, vals[2].value, valueDouble)
	}
}
