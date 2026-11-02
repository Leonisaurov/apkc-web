package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// Las clases de javac 21 para los dos fuentes del bucle (parrafo 63), sin tocar disco:
//   - `bucleJavac`:      `while (i < n) { total = total + i; i = i + 1; }`
//   - `paraJavac`:       `for (int i = 0; i < n; i = i + 1) { total = total + i; }`
//
// Las dos con `static int total = 0;`.
const bucleJavac = "yv66vgAAAEEAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAB3AvQnVjbGUBAAV0b3RhbAEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAFY29ycmUBAAQoSSlWAQANU3RhY2tNYXBUYWJsZQEACDxjbGluaXQ+AQAKU291cmNlRmlsZQEACkJ1Y2xlLmphdmEAIQAIAAIAAAABAAgACwAMAAAAAwABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAIACQAPABAAAQANAAAATAACAAIAAAAXAzwbGqIAErIABxtgswAHGwRgPKf/77EAAAACAA4AAAAWAAUAAAAFAAIABgAHAAcADwAIABYACgARAAAABwAC/AACARMACAASAAYAAQANAAAAHQABAAAAAAAFA7MAB7EAAAABAA4AAAAGAAEAAAADAAEAEwAAAAIAFA=="

const paraJavac = "yv66vgAAAEEAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABnAvUGFyYQEABXRvdGFsAQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAVjb3JyZQEABChJKVYBAA1TdGFja01hcFRhYmxlAQAIPGNsaW5pdD4BAApTb3VyY2VGaWxlAQAJUGFyYS5qYXZhACEACAACAAAAAQAIAAsADAAAAAMAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAACAAkADwAQAAEADQAAAEoAAgACAAAAFwM8GxqiABKyAAcbYLMABxsEYDyn/++xAAAAAgAOAAAAEgAEAAAABQAHAAYADwAFABYACAARAAAACQAC/AACAfoAEwAIABIABgABAA0AAAAdAAEAAAAAAAUDswAHsQAAAAEADgAAAAYAAQAAAAMAAQATAAAAAgAU"

// fuenteBucle y fuentePara son los mismos fuentes que produjeron esos fixtures, sin `+` de cadenas
// (javac 21 lo baja con `invokedynamic`, que el dexer rechaza por nombre: plan 5.22).
const fuenteBucle = `package p;
public class Bucle {
    static int total = 0;
    public static void corre(int n) {
        int i = 0;
        while (i < n) {
            total = total + i;
            i = i + 1;
        }
    }
}
`

const fuentePara = `package p;
public class Para {
    static int total = 0;
    public static void corre(int n) {
        for (int i = 0; i < n; i = i + 1) {
            total = total + i;
        }
    }
}
`

// TestElBucleSeDexaIgualQueElDeJavac: el oráculo doble, aplicado al bucle. El `.class` que escribe este
// fork, dexado, tiene que dar **el mismo fichero** que el `javac` del mismo fuente dexado igual. Si la
// forma del `while` o la del desdoblamiento del `for` se aparta de javac, esto lo dice.
func TestElBucleSeDexaIgualQueElDeJavac(t *testing.T) {
	casos := []struct{ nombre, clase, fuente, javac string }{
		{"while", "Bucle", fuenteBucle, bucleJavac},
		{"for", "Para", fuentePara, paraJavac},
	}
	for _, c := range casos {
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/" + c.clase + ".java", contenido: c.fuente}}, jarDelSDK(t))
		if err != nil {
			t.Fatalf("%s: el emisor rechazó el fuente: %v", c.nombre, err)
		}
		if len(salida) != 1 {
			t.Fatalf("%s: %d clases, una", c.nombre, len(salida))
		}
		nuestra, err := readClassFull(salida[0].datos)
		if err != nil {
			t.Fatalf("%s: nuestra clase no se lee: %v", c.nombre, err)
		}
		datos, err := base64.StdEncoding.DecodeString(c.javac)
		if err != nil {
			t.Fatalf("%s: el fixture no es base64: %v", c.nombre, err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatalf("%s: el fixture no se lee: %v", c.nombre, err)
		}
		deUno, err := dexDeClases([]classFull{nuestra})
		if err != nil {
			t.Fatalf("%s: dexando lo nuestro: %v", c.nombre, err)
		}
		deOtro, err := dexDeClases([]classFull{suya})
		if err != nil {
			t.Fatalf("%s: dexando lo de javac: %v", c.nombre, err)
		}
		if !bytes.Equal(deUno, deOtro) {
			t.Errorf("%s: el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
				c.nombre, len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
		}
		// y el cuerpo, palabra a palabra, es el de la forma medida
		d, err := readDex(deUno)
		if err != nil {
			t.Fatalf("%s: el dex no se relee: %v", c.nombre, err)
		}
		ci, err := codeItemDeMetodo(d, "corre")
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if len(ci.insns) != 12 {
			t.Errorf("%s: %d palabras, la forma medida tiene 12", c.nombre, len(ci.insns))
		}
		if ci.insns[1]&0xff != 0x35 {
			t.Errorf("%s: la condición no es el `if-ge` (0x35) medido: %#04x", c.nombre, ci.insns[1])
		}
		if int(int16(ci.insns[2])) != 10 {
			t.Errorf("%s: el salto de salida no es +10 unidades: %#04x", c.nombre, ci.insns[2])
		}
		if ci.insns[10]&0xff != 0x28 {
			t.Errorf("%s: el `goto` de la vuelta no es el 10t (0x28): %#04x", c.nombre, ci.insns[10])
		}
		if back := int(int8(ci.insns[10] >> 8)); back != -9 {
			t.Errorf("%s: el `goto` de la vuelta salta %d unidades y la medida es -9", c.nombre, back)
		}
		if ci.insns[11]&0xff != 0x0e {
			t.Errorf("%s: la última palabra no es return-void: %#04x", c.nombre, ci.insns[11])
		}
	}
}

func primerByteDistinto(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return len(a)
}

// codeItemDeMetodo: el code_item del método `nombre` en el único class_def del fichero.
func codeItemDeMetodo(d *dexFile, nombre string) (codeItem, error) {
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		return codeItem{}, err
	}
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			if d.methods[m.methodIdx].name == nombre {
				return d.readCodeItem(m.codeOff)
			}
		}
	}
	return codeItem{}, errors.New("el método no aparece en el class_data")
}

// TestDoYTtrySiguenRechazados: la frontera escrita. Al añadir `while`, `for` y `switch` no se abrió
// nada más: lo que no está medido sigue saliendo por su nombre.
func TestDoYTtrySiguenRechazados(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"do-while", "package p;\npublic class A { public static void f(int n) { do { n = n + 1; } while (n < 3); } }\n", "do"},
		{"try", "package p;\npublic class A { public static void f() { try { f(); } catch (RuntimeException e) { f(); } } }\n", "try"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se aceptó una sentencia que no está medida", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.pista) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
