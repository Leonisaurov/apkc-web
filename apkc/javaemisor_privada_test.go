package main

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

// fuentePrivada es la sonda del parrafo 72: un miembro privado de instancia, uno de referencia, uno
// estatico y uno protegido, un metodo privado de instancia, uno privado estatico y uno privado con
// vuelta, y una anonima que los toca todos. El orden de los accesores que escribe javac sale de los
// enunciados: primero el de escritura de `secreto`, despues el de lectura, y asi hasta seis.
const fuentePrivada = `public class P {
    private int secreto = 0;
    private String texto;
    private static int cuenta = 0;
    protected int abierto = 0;
    private void oculto() { secreto = secreto + 1; }
    private static void estatico() { cuenta = cuenta + 1; }
    private int lee() { return secreto; }
    public void publico() { secreto = secreto + 1; }
    protected void protegido() { abierto = abierto + 1; }
    public Runnable anonima() {
        return new Runnable() {
            public void run() {
                secreto = secreto + 2;
                texto = "x";
                cuenta = cuenta + 3;
                oculto();
                estatico();
                secreto = lee();
                publico();
                protegido();
            }
        };
    }
}
`

// privadaJavac: `javac --release 8 -g P.java` y `base64 -w0 P.class`.
const privadaJavac = "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"

// privadaJavacAnonima: `javac --release 8 -g P.java` y `base64 -w0 P$1.class`.
const privadaJavacAnonima = "yv66vgAAADQARQkAAgADBwAEDAAFAAYBAANQJDEBAAZ0aGlzJDABAANMUDsKAAgACQcACgwACwAMAQAQamF2YS9sYW5nL09iamVjdAEABjxpbml0PgEAAygpVgoADgAPBwAQDAARABIBAAFQAQAKYWNjZXNzJDAwMAEABihMUDspSQoADgAUDAAVABYBAAphY2Nlc3MkMDAyAQAHKExQO0kpSQgAGAEAAXgKAA4AGgwAGwAcAQAKYWNjZXNzJDEwMgEAKShMUDtMamF2YS9sYW5nL1N0cmluZzspTGphdmEvbGFuZy9TdHJpbmc7CgAOAB4MAB8AIAEACmFjY2VzcyQyMDABAAMoKUkKAA4AIgwAIwAkAQAKYWNjZXNzJDIwMgEABChJKUkKAA4AJgwAJwAoAQAKYWNjZXNzJDMwMAEABihMUDspVgoADgAqDAArAAwBAAphY2Nlc3MkNDAwCgAOAC0MAC4AEgEACmFjY2VzcyQ1MDAKAA4AMAwAMQAMAQAHcHVibGljbwoADgAzDAA0AAwBAAlwcm90ZWdpZG8HADYBABJqYXZhL2xhbmcvUnVubmFibGUBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQASTG9jYWxWYXJpYWJsZVRhYmxlAQAEdGhpcwEABUxQJDE7AQAQTWV0aG9kUGFyYW1ldGVycwEAA3J1bgEAClNvdXJjZUZpbGUBAAZQLmphdmEBAA9FbmNsb3NpbmdNZXRob2QMAEIAQwEAB2Fub25pbWEBABYoKUxqYXZhL2xhbmcvUnVubmFibGU7AQAMSW5uZXJDbGFzc2VzACAAAgAIAAEANQABEBAABQAGAAAAAgAAAAsAKAACADcAAAA+AAIAAgAAAAoqK7UAASq3AAexAAAAAgA4AAAABgABAAAADAA5AAAAFgACAAAACgA6ADsAAAAAAAoABQAGAAEAPAAAAAUBAACAEAABAD0ADAABADcAAACWAAMAAQAAAEwqtAABKrQAAbgADQVguAATVyq0AAESF7gAGVe4AB0GYLgAIVcqtAABuAAluAApKrQAASq0AAG4ACy4ABNXKrQAAbYALyq0AAG2ADKxAAAAAgA4AAAAJgAJAAAADgARAA8AGwAQACQAEQArABIALgATAD0AFABEABUASwAWADkAAAAMAAEAAABMADoAOwAAAAMAPgAAAAIAPwBAAAAABAAOAEEARAAAAAoAAQACAAAAAAAA"

// TestLaAnonimaTocaLoPrivadoPorLosAccess: la clase exterior y su anonima salen con los mismos
// campos, los mismos metodos en el mismo orden (los seis `access$NNN` incluidos), los mismos flags y
// los mismos bytes de `Code` que las de javac. No se compara el fichero entero porque el pool puede
// ordenar distinto; el `Code` de cada metodo es donde vive la forma medida.
func TestLaAnonimaTocaLoPrivadoPorLosAccess(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/P.java", contenido: fuentePrivada}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestras := map[string]classFull{}
	for _, c := range salida {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", c.interno, err)
		}
		nuestras[c.interno] = cf
	}
	esperadas := map[string]string{"P": privadaJavac, "P$1": privadaJavacAnonima}
	for nombre, b64 := range esperadas {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("%s: el fixture no es base64: %v", nombre, err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatalf("%s: el fixture no se lee: %v", nombre, err)
		}
		nuestra, ok := nuestras[nombre]
		if !ok {
			t.Errorf("%s: no se emitio", nombre)
			continue
		}
		if nuestra.Access != suya.Access {
			t.Errorf("%s: los flags de clase son %#04x y los de javac %#04x", nombre, nuestra.Access, suya.Access)
		}
		if len(nuestra.Fields) != len(suya.Fields) {
			t.Errorf("%s: %d campos y javac %d", nombre, len(nuestra.Fields), len(suya.Fields))
		} else {
			for i := range suya.Fields {
				a, b := nuestra.Fields[i], suya.Fields[i]
				if a.Name != b.Name || a.Desc != b.Desc || a.Access != b.Access {
					t.Errorf("%s: el campo %d es %s:%s %#04x y el de javac %s:%s %#04x",
						nombre, i, a.Name, a.Desc, a.Access, b.Name, b.Desc, b.Access)
				}
			}
		}
		if len(nuestra.Methods) != len(suya.Methods) {
			t.Errorf("%s: %d metodos y javac %d", nombre, len(nuestra.Methods), len(suya.Methods))
			for _, m := range nuestra.Methods {
				t.Logf("  nuestro %s %s %#04x", m.Name, m.Desc, m.Access)
			}
			for _, m := range suya.Methods {
				t.Logf("  javac   %s %s %#04x", m.Name, m.Desc, m.Access)
			}
			continue
		}
		for i := range suya.Methods {
			a, b := nuestra.Methods[i], suya.Methods[i]
			if a.Name != b.Name || a.Desc != b.Desc || a.Access != b.Access {
				t.Errorf("%s: el metodo %d es %s%s %#04x y el de javac %s%s %#04x",
					nombre, i, a.Name, a.Desc, a.Access, b.Name, b.Desc, b.Access)
				continue
			}
			if a.HasCode != b.HasCode {
				t.Errorf("%s.%s: uno lleva Code y el otro no", nombre, a.Name)
				continue
			}
			if a.HasCode {
				nuestraForma := formaDeMetodo(t, nuestra, a)
				suyaForma := formaDeMetodo(t, suya, b)
				if nuestraForma != suyaForma {
					t.Errorf("%s.%s:\n  nuestro: %s\n  javac:   %s", nombre, a.Name, nuestraForma, suyaForma)
				}
			}
		}
	}
}

// formaDeMetodo: pc, mnemotecnico y, cuando la instruccion nombra el pool, el miembro resuelto por su
// nombre. Comparar el Code crudo entre dos compiladores no vale: el pool se ordena distinto y los
// indices son otros; esto compara las mismas instrucciones sobre los mismos miembros, que es lo que la
// medicion afirma.
func formaDeMetodo(t *testing.T, cf classFull, m classMethod) string {
	t.Helper()
	lista, err := walkJvmCode(m.Code)
	if err != nil {
		t.Fatalf("%s.%s: el Code no se recorre: %v", cf.Name, m.Name, err)
	}
	x := poolIndex{cf.p}
	fila := []string{}
	for _, in := range lista {
		texto := itoa(in.pc) + " " + in.name
		switch in.name {
		case "getstatic", "putstatic", "getfield", "putfield",
			"invokevirtual", "invokespecial", "invokestatic", "invokeinterface":
			dueno, nombre, desc, _ := x.refOf(u2De(in.ops, 0))
			texto += " " + dueno + "." + nombre + ":" + desc
		case "new", "checkcast", "instanceof", "anewarray":
			texto += " " + x.classDescriptor(u2De(in.ops, 0))
		case "ldc", "ldc_w", "ldc2_w":
			idx := int(in.ops[0])
			if in.name != "ldc" {
				idx = u2De(in.ops, 0)
			}
			texto += " " + constanteDelPool(x, idx)
		case "bipush":
			texto += " " + itoa(int(int8(in.ops[0])))
		case "sipush":
			texto += " " + itoa(int(int16(u2De(in.ops, 0))))
		}
		fila = append(fila, texto)
	}
	return strings.Join(fila, " | ")
}

// constanteDelPool: la entrada nombrada por un `ldc`/`ldc_w`/`ldc2_w` renderizada como el valor que es,
// no como el indice. Comparar indices entre dos compiladores no vale porque el pool se ordena distinto,
// y leerla siempre como entero ocultaba el valor: con la sonda del párrafo 85, un `ldc` de 3.0f y uno de
// 5.0f salían con la misma palabra.
func constanteDelPool(x poolIndex, idx int) string {
	switch x.p.tag[idx] {
	case cpString:
		return "\"" + x.stringValue(idx) + "\""
	case cpInteger:
		return itoa(int(x.intOf(idx)))
	case cpFloat:
		return strconv.FormatFloat(float64(x.floatOf(idx)), 'g', -1, 32) + "f"
	case cpLong:
		return strconv.FormatInt(x.longOf(idx), 10) + "l"
	case cpDouble:
		return strconv.FormatFloat(x.doubleOf(idx), 'g', -1, 64) + "d"
	}
	// El `tag` es un mapa y solo guarda las entradas que no son `Utf8` ni `Class`, asi que la ausencia
	// dice el indice: un `ldc` apuntando a un nombre o a un tipo es un defecto que hay que ver.
	return "pool?" + itoa(idx)
}

// TestUnaClaseAnidadaPrivadaSigueRechazada: la frontera del parrafo 72. Una anidada `private` pide en
// javac un constructor sintetico y una clase marcador, y una `protected` con nombre y sin `static` pide
// la captura `this$0`: las dos salen por su nombre en vez de escribir un ClassFile que no es el suyo.
func TestUnaClaseAnidadaPrivadaSigueRechazada(t *testing.T) {
	casos := []struct{ nombre, fuente, pista string }{
		{"private", "package p;\npublic class A { private static class Oculta { int x = 0; } public int f() { return 1; } }\n", "private"},
		{"protected", "package p;\npublic class A { protected class Media { int y = 0; } public int f() { return 1; } }\n", "protected"},
	}
	for _, c := range casos {
		_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/A.java", contenido: c.fuente}}, jarDelSDK(t))
		if err == nil {
			t.Errorf("%s: se acepto una clase anidada que no esta medida", c.nombre)
			continue
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.pista)) {
			t.Errorf("%s: el mensaje no nombra %q: %v", c.nombre, c.pista, err)
		}
	}
}
