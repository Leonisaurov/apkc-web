package main

import (
	"strings"
	"testing"
)

// Sondas de `StringConcatFactory.makeConcatWithConstants` (5.22, §116), compiladas con
// `javac --release 11`: `ConcatB` es el caso en que el literal contiene los dos marcadores y `javac`
// los saca a una constante del bootstrap (`\u0002`), y `ConcatW` conserva un argumento dinámico de
// cada tipo (`int`, `long`, `float`, `double`, `char`, `boolean` y `String`).
const (
	concatBJavac = "yv66vgAAADcAPwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAF21ha2VDb25jYXRXaXRoQ29uc3RhbnRzAQAVKEkpTGphdmEvbGFuZy9TdHJpbmc7CQAMAA0HAA4MAA8AEAEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKABIAEwcAFAwAFQAWAQAQamF2YS9sYW5nL1N0cmluZwEABmxlbmd0aAEAAygpSQoAEgAYDAAZABoBAAZlcXVhbHMBABUoTGphdmEvbGFuZy9PYmplY3Q7KVoSAAEAHAwACQAdAQAWKElaKUxqYXZhL2xhbmcvU3RyaW5nOwoAHwAgBwAhDAAiACMBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEAFShMamF2YS9sYW5nL1N0cmluZzspVgcAJQEAB0NvbmNhdEIBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEbWFpbgEAFihbTGphdmEvbGFuZy9TdHJpbmc7KVYBAApTb3VyY2VGaWxlAQAMQ29uY2F0Qi5qYXZhAQAQQm9vdHN0cmFwTWV0aG9kcwgALgEAAgIBCAAwAQAFYQFiAmMIADIBAAMBOgEPBgA0CgA1ADYHADcMAAkAOAEAJGphdmEvbGFuZy9pbnZva2UvU3RyaW5nQ29uY2F0RmFjdG9yeQEAmChMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cDtMamF2YS9sYW5nL1N0cmluZztMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlO0xqYXZhL2xhbmcvU3RyaW5nO1tMamF2YS9sYW5nL09iamVjdDspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzBwA7AQAlamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cAcAPQEAHmphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcwEABkxvb2t1cAAhACQAAgAAAAAAAgABAAUABgABACYAAAAdAAEAAQAAAAUqtwABsQAAAAEAJwAAAAYAAQAAAAEACQAoACkAAQAmAAAASAAEAAMAAAAkKr48G7oABwAATbIACyy2ABEsG7oABwAAtgAXugAbAAC2AB6xAAAAAQAnAAAAEgAEAAAAAwADAAQACgAFACMABgADACoAAAACACsALAAAABAAAgAzAAIALQAvADMAAQAxADkAAAAKAAEAOgA8AD4AGQ=="
	concatWJavac = "yv66vgAAADcAVgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAF21ha2VDb25jYXRXaXRoQ29uc3RhbnRzAQAWKFpDKUxqYXZhL2xhbmcvU3RyaW5nOxIAAQAMDAAJAA0BABcoSkZEKUxqYXZhL2xhbmcvU3RyaW5nOwoADwAQBwARDAASABMBABBqYXZhL2xhbmcvU3RyaW5nAQAHdmFsdWVPZgEAJihMamF2YS9sYW5nL09iamVjdDspTGphdmEvbGFuZy9TdHJpbmc7EgACABUMAAkAFgEAOShJTGphdmEvbGFuZy9TdHJpbmc7TGphdmEvbGFuZy9TdHJpbmc7KUxqYXZhL2xhbmcvU3RyaW5nOwkAGAAZBwAaDAAbABwBABBqYXZhL2xhbmcvU3lzdGVtAQADb3V0AQAVTGphdmEvaW8vUHJpbnRTdHJlYW07CgAeAB8HACAMACEACgEAB0NvbmNhdFcBAAN1bm8FAAAAAAAAAAIEP8AAAAZABAAAAAAAAAoAHgAoDAApAA0BAANkb3MIACsBAAFzCAAtAQADb2JqCgAeAC8MADAAMQEABHRyZXMBADkoSUxqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvT2JqZWN0OylMamF2YS9sYW5nL1N0cmluZzsSAAMAMwwACQA0AQBKKExqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvU3RyaW5nOylMamF2YS9sYW5nL1N0cmluZzsKADYANwcAOAwAOQA6AQATamF2YS9pby9QcmludFN0cmVhbQEAB3ByaW50bG4BABUoTGphdmEvbGFuZy9TdHJpbmc7KVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEbWFpbgEAFihbTGphdmEvbGFuZy9TdHJpbmc7KVYBAApTb3VyY2VGaWxlAQAMQ29uY2F0Vy5qYXZhAQAQQm9vdHN0cmFwTWV0aG9kcwgAQwEABHoBYwEIAEUBAAZqAWYBZAEIAEcBAAZpAXMBbwEIAEkBAAMBAQEPBgBLCgBMAE0HAE4MAAkATwEAJGphdmEvbGFuZy9pbnZva2UvU3RyaW5nQ29uY2F0RmFjdG9yeQEAmChMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cDtMamF2YS9sYW5nL1N0cmluZztMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlO0xqYXZhL2xhbmcvU3RyaW5nO1tMamF2YS9sYW5nL09iamVjdDspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzBwBSAQAlamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cAcAVAEAHmphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcwEABkxvb2t1cAAhAB4AAgAAAAAABQABAAUABgABADsAAAAdAAEAAQAAAAUqtwABsQAAAAEAPAAAAAYAAQAAAAEACAAhAAoAAQA7AAAAIAACAAIAAAAIGhu6AAcAALAAAAABADwAAAAGAAEAAAACAAgAKQANAAEAOwAAACEABQAFAAAACR4kKboACwAAsAAAAAEAPAAAAAYAAQAAAAMACAAwADEAAQA7AAAAJAADAAMAAAAMGissuAAOugAUAACwAAAAAQA8AAAABgABAAAABAAJAD0APgABADsAAABCAAcAAQAAACayABcEEHi4AB0UACISJBQAJbgAJyq+EioSLLgALroAMgAAtgA1sQAAAAEAPAAAAAoAAgAAAAYAJQAHAAMAPwAAAAIAQABBAAAAGgAEAEoAAQBCAEoAAQBEAEoAAQBGAEoAAQBIAFAAAAAKAAEAUQBTAFUAGQ=="
)

// TestLaConcatenacionConConstantesDelBootstrap: la receta `\u0002\u0001` de `ConcatB` saca el
// literal a una constante del bootstrap y deja un solo argumento dinámico. El sitio no crea clase
// sintética y el dex sale con el constructor, los dos `append` y `toString`.
func TestLaConcatenacionConConstantesDelBootstrap(t *testing.T) {
	clases, err := expandirLambdas([]classFull{lambdaFixture(t, concatBJavac)})
	if err != nil {
		t.Fatalf("la expansion se cayo: %v", err)
	}
	if len(clases) != 1 {
		t.Fatalf("la concatenacion no debe crear clases: %v", nombresDeLambda(clases))
	}
	sitios := clases[0].Concats
	if len(sitios) != 3 {
		t.Fatalf("ConcatB tiene %d sitios de concatenacion, se esperaban 3", len(sitios))
	}
	var visto bool
	for _, s := range sitios {
		if len(s.params) != 1 || s.params[0] != "I" {
			continue
		}
		if len(s.piezas) != 2 || s.piezas[0].cons != 0 || s.piezas[1].arg != 0 {
			continue
		}
		if len(s.constantes) != 1 || s.constantes[0].tag != cpString || s.constantes[0].texto != "a\u0001b\u0002c" {
			t.Fatalf("constante del sitio: %+v", s.constantes)
		}
		visto = true
	}
	if !visto {
		t.Fatalf("no aparecio el sitio de la receta \\u0002\\u0001: %+v", sitios)
	}
	datos, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
	d, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	if !tieneMetodo(d, "Ljava/lang/StringBuilder;", "append", "(Ljava/lang/String;)Ljava/lang/StringBuilder;") {
		t.Error("falta StringBuilder.append(String)")
	}
	if !tieneMetodo(d, "Ljava/lang/StringBuilder;", "append", "(I)Ljava/lang/StringBuilder;") {
		t.Error("falta StringBuilder.append(I)")
	}
	if !tieneMetodo(d, "Ljava/lang/StringBuilder;", "toString", "()Ljava/lang/String;") {
		t.Error("falta StringBuilder.toString()")
	}
}

// TestLaConcatenacionEligeElAppendDeCadaTipo: `ConcatW` conserva un argumento de cada tipo; el
// dex tiene que nombrar el `append` de los seis y el de `String`, sin `Object` para los primitivos.
func TestLaConcatenacionEligeElAppendDeCadaTipo(t *testing.T) {
	clases, err := expandirLambdas([]classFull{lambdaFixture(t, concatWJavac)})
	if err != nil {
		t.Fatalf("la expansion se cayo: %v", err)
	}
	vistos := map[string]bool{}
	for _, s := range clases[0].Concats {
		for _, p := range s.piezas {
			if p.arg >= 0 {
				vistos[appendDeTipo(s.params[p.arg])] = true
			}
		}
	}
	for _, tipo := range []string{"I", "J", "F", "D", "C", "Z"} {
		desc := appendDeTipo(tipo)
		if !vistos[desc] {
			t.Errorf("ningun sitio pide el append de %q (%s): %v", tipo, desc, vistos)
		}
	}
	if !vistos["(Ljava/lang/String;)Ljava/lang/StringBuilder;"] {
		t.Errorf("ningun sitio pide el append de String: %v", vistos)
	}
	if vistos["(Ljava/lang/Object;)Ljava/lang/StringBuilder;"] {
		t.Errorf("un primitivo cayo en el append de Object: %v", vistos)
	}
	datos, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
	d, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	for _, tipo := range []string{"I", "J", "F", "D", "C", "Z"} {
		desc := appendDeTipo(tipo)
		if !tieneMetodo(d, "Ljava/lang/StringBuilder;", "append", desc) {
			t.Errorf("falta StringBuilder.append%s", desc)
		}
	}
}

// tieneMetodo: ¿está el method_id en el dex releído?
func tieneMetodo(d *dexFile, owner, nombre, desc string) bool {
	_, ret, params := splitDescriptor(desc)
	for _, m := range d.methods {
		if m.owner != owner || m.name != nombre || m.proto.returnType != ret {
			continue
		}
		if strings.Join(m.proto.params, ",") == strings.Join(params, ",") {
			return true
		}
	}
	return false
}
