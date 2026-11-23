package main

// La negación unaria, medida con `javac`, `d8` y `dexdump`. El `android-support-v4.jar` paró el dexer en el byte 0x76
// de un `.class`, que en el JVMS es `fneg`; esta sonda cubre los cuatro anchos sobre un `.class` de
// javac —el camino real de una dependencia, que llega ya compilada y solo se dexa— y comprueba que
// salen como el 12x de DEX que escribe `d8`: `neg-int` 0x7b, `neg-long` 0x7d, `neg-float` 0x7f y
// `neg-double` 0x80, negando en el sitio del operando cuando muere.
//
// El emisor de fuente propio todavía no baja la negación de algo que no sea un literal («la negacion
// de algo que no es un literal entero no esta medida»); por eso la prueba va contra el `.class` de
// javac y no contra `compilarProyectoJava`. Esa es la frontera que el jar no pisa: una dependencia no
// se compila aquí, se dexea.

import (
	"encoding/base64"
	"testing"
)

// negJavac: `javac 21 --release 8 -g:lines,source Neg.java` y `base64 -w0 Neg.class`, con una
// negación de cada ancho (`ni` int, `nl` long, `nf` float, `nd` double).
const negJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQADTmVnAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAm5pAQAEKEkpSQEAAm5sAQAEKEopSgEAAm5mAQAEKEYpRgEAAm5kAQAEKEQpRAEAClNvdXJjZUZpbGUBAAhOZWcuamF2YQAhAAcAAgAAAAAABQABAAUABgABAAkAAAAdAAEAAQAAAAUqtwABsQAAAAEACgAAAAYAAQAAAAEACAALAAwAAQAJAAAAGwABAAEAAAADGnSsAAAAAQAKAAAABgABAAAAAgAIAA0ADgABAAkAAAAbAAIAAgAAAAMeda0AAAABAAoAAAAGAAEAAAADAAgADwAQAAEACQAAABsAAQABAAAAAyJ2rgAAAAEACgAAAAYAAQAAAAQACAARABIAAQAJAAAAGwACAAIAAAADJnevAAAAAQAKAAAABgABAAAABQABABMAAAACABQ="

// claseNegDeJavac: el `.class` del fixture, como el que llega dentro de un jar.
func claseNegDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(negJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexWalkJvmCodeAceptaLaNegacion: el caminante del `.class` acepta `ineg` 0x74, `lneg` 0x75,
// `fneg` 0x76 y `dneg` 0x77 con su nombre y cero operandos. Es la prueba que falla si se quita
// cualquiera de las cuatro entradas de `jvmMeasured`: sin ellas `walkJvmCode` para en el primer byte
// con el error que destapó el jar («opcode outside the table this fork measured»).
func TestDexWalkJvmCodeAceptaLaNegacion(t *testing.T) {
	casos := []struct {
		op     byte
		nombre string
	}{{0x74, "ineg"}, {0x75, "lneg"}, {0x76, "fneg"}, {0x77, "dneg"}}
	for _, c := range casos {
		// la negación seguida de `return` (0xb1): dos instrucciones de un byte
		lista, err := walkJvmCode([]byte{c.op, 0xb1})
		if err != nil {
			t.Fatalf("%#02x: el caminante la rechaza: %v", c.op, err)
		}
		if len(lista) != 2 || lista[0].op != c.op || lista[0].name != c.nombre || len(lista[0].ops) != 0 {
			t.Errorf("%#02x: esperaba %s de cero operandos, salió %+v", c.op, c.nombre, lista)
		}
	}
}

// TestDexWalkJvmCodeRechazaFnegSinEntrada: la prueba contraria de la tabla. Quitando la entrada de
// `fneg` en caliente —y devolviéndola al terminar— el caminante vuelve a pararse en el 0x76 con el
// error que destapó el jar. Así la tabla queda atada a la prueba: si alguien borra la fila, este
// caso deja de tener sentido y `TestDexWalkJvmCodeAceptaLaNegacion` falla con el error medido.
func TestDexWalkJvmCodeRechazaFnegSinEntrada(t *testing.T) {
	guardada, habia := jvmMeasured[0x76]
	delete(jvmMeasured, 0x76)
	defer func() {
		if habia {
			jvmMeasured[0x76] = guardada
		}
	}()
	if _, err := walkJvmCode([]byte{0x76, 0xb1}); err == nil {
		t.Fatal("sin la entrada de fneg, el caminante lo aceptó como si estuviera medida")
	}
}

// TestDexNegacionUnariaDeUnClassDeJar: el dexer acepta la negación de un `.class` ya compilado y la
// baja al 12x de `d8`. El opcode medido con `d8`/`dexdump` sobre esta misma sonda es `7b00`, `7d00`,
// `7f00` y `8000`: la instrucción con el destino y la fuente en el mismo registro. Se comprueba que
// el opcode aparece y que los dos nibbles del registro casan (negación en el sitio); la prueba muere
// si el dispatch de `negacion` o la entrada del caminante desaparecen.
func TestDexNegacionUnariaDeUnClassDeJar(t *testing.T) {
	clase := claseNegDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{"ni": 0x7b, "nl": 0x7d, "nf": 0x7f, "nd": 0x80}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(clase, nombre)
		if !ok {
			t.Fatalf("%s: no está en el fixture de javac", nombre)
		}
		ci, err := td.codeItemDe(clase, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := false
		for _, u := range ci.insns {
			if byte(u) != opcode {
				continue
			}
			destino := (u >> 8) & 0xf
			fuente := (u >> 12) & 0xf
			if destino != fuente {
				t.Errorf("%s: el %#02x no es en el sitio (destino v%d, fuente v%d) en %04x",
					nombre, opcode, destino, fuente, ci.insns)
			}
			visto = true
			break
		}
		if !visto {
			t.Errorf("%s: no aparece el opcode %#02x en %04x", nombre, opcode, ci.insns)
		}
	}
}
