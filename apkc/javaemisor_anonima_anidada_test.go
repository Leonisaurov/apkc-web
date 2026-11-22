package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Sonda de una anonima dentro de otra, compilada con javac 17 (`--release 8 -g:lines,source`).
// Capa$1 se crea en un metodo static (no lleva this$0) y captura `m` en val$m; Capa$1$1 se crea
// dentro de run() de Capa$1, captura this$0:LCapa$1; y val$m, y su constructor es (LCapa$1;I)V
// con Signature ()V y sin MethodParameters (medido con `javap -c -v -p`).
const anonimaAnidadaFuente = `public class Capa {
  static Runnable a() {
    return new Runnable() {
      public void run() {
        int m = getM();
        Runnable inner = new Runnable() {
          public void run() { System.out.println(m); }
        };
        inner.run();
      }
    };
  }
  static int getM() { return 7; }
}
`

const anonimaCapaJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAGQ2FwYSQxCgAHAAMHAAsBAARDYXBhAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAWEBABYoKUxqYXZhL2xhbmcvUnVubmFibGU7AQAEZ2V0TQEAAygpSQEAClNvdXJjZUZpbGUBAAlDYXBhLmphdmEBAAxJbm5lckNsYXNzZXMAIQAKAAIAAAAAAAMAAQAFAAYAAQAMAAAAHQABAAEAAAAFKrcAAbEAAAABAA0AAAAGAAEAAAABAAgADgAPAAEADAAAACAAAgAAAAAACLsAB1m3AAmwAAAAAQANAAAABgABAAAAAwAIABAAEQABAAwAAAAbAAEAAAAAAAMQB6wAAAABAA0AAAAGAAEAAAANAAIAEgAAAAIAEwAUAAAACgABAAcAAAAAAAA="
const anonimaCapa1Javac = "yv66vgAAADQAIgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEABENhcGEBAARnZXRNAQADKClJBwAOAQAIQ2FwYSQxJDEKAA0AEAwABQARAQAMKExDYXBhJDE7SSlWCwATABQHABUMABYABgEAEmphdmEvbGFuZy9SdW5uYWJsZQEAA3J1bgcAGAEABkNhcGEkMQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAApTb3VyY2VGaWxlAQAJQ2FwYS5qYXZhAQAPRW5jbG9zaW5nTWV0aG9kDAAfACABAAFhAQAWKClMamF2YS9sYW5nL1J1bm5hYmxlOwEADElubmVyQ2xhc3NlcwAgABcAAgABABMAAAACAAAABQAGAAEAGQAAAB0AAQABAAAABSq3AAGxAAAAAQAaAAAABgABAAAAAwABABYABgABABkAAAA5AAQAAwAAABW4AAc8uwANWSobtwAPTSy5ABIBALEAAAABABoAAAASAAQAAAAFAAQABgAOAAkAFAAKAAMAGwAAAAIAHAAdAAAABAAIAB4AIQAAABIAAgAXAAAAAAAAAA0AAAAAAAA="
const anonimaCapa11Javac = "yv66vgAAADQAKwkAAgADBwAEDAAFAAYBAAhDYXBhJDEkMQEABnRoaXMkMAEACExDYXBhJDE7CQACAAgMAAkACgEABXZhbCRtAQABSQoADAANBwAODAAPABABABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQASABMHABQMABUAFgEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKABgAGQcAGgwAGwAcAQATamF2YS9pby9QcmludFN0cmVhbQEAB3ByaW50bG4BAAQoSSlWBwAeAQASamF2YS9sYW5nL1J1bm5hYmxlAQAMKExDYXBhJDE7SSlWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEACVNpZ25hdHVyZQEAA3J1bgEAClNvdXJjZUZpbGUBAAlDYXBhLmphdmEBAA9FbmNsb3NpbmdNZXRob2QHACgBAAZDYXBhJDEMACMAEAEADElubmVyQ2xhc3NlcwAgAAIADAABAB0AAhAQAAkACgAAEBAABQAGAAAAAgAAAA8AHwACACAAAAAnAAIAAwAAAA8qK7UAASoctQAHKrcAC7EAAAABACEAAAAGAAEAAAAGACIAAAACABAAAQAjABAAAQAgAAAAIwACAAEAAAALsgARKrQAB7YAF7EAAAABACEAAAAGAAEAAAAHAAMAJAAAAAIAJQAmAAAABAAnACkAKgAAABIAAgAnAAAAAAAAAAIAAAAAAAA="

// TestAnonimaDentroDeAnonima: las tres clases salen con el mismo dex que las de javac 17: Capa$1$1
// lleva this$0 y val$m, el constructor (LCapa$1;I)V con Signature ()V y sin MethodParameters, el
// EnclosingMethod apunta a Capa$1.run y el InnerClasses lista al padre antes que a la interior.
func TestAnonimaDentroDeAnonima(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Capa.java", contenido: anonimaAnidadaFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 3 {
		t.Fatalf("%d clases, se esperaban 3: %s", len(salida), nombresDe(salida))
	}
	nuestras := map[string]classFull{}
	for _, s := range salida {
		c, err := readClassFull(s.datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestras[c.Name] = c
	}
	for _, b64 := range []string{anonimaCapaJavac, anonimaCapa1Javac, anonimaCapa11Javac} {
		datos, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		suya, err := readClassFull(datos)
		if err != nil {
			t.Fatal(err)
		}
		nuestra, ok := nuestras[suya.Name]
		if !ok {
			t.Errorf("no se escribió %s", suya.Name)
			continue
		}
		deUno, err := dexDeClases([]classFull{nuestra})
		if err != nil {
			t.Fatal(err)
		}
		deOtro, err := dexDeClases([]classFull{suya})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(deUno, deOtro) {
			t.Errorf("%s: dex de %d bytes contra %d de javac; primer byte distinto %d",
				suya.Name, len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
		}
	}
}
