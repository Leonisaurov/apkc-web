package main

// Oráculo de javac 17 para el emisor: los mismos seis `.class` del proyecto del studio que fija
// `classreader_test.go`, compilados con javac 17.0.21 `--release 8 -g:lines,source`. Los fixtures de
// aquel fichero se hicieron con javac 21 y se quedan como están: son el material con el que se miden las
// tablas del dexer contra `d8`, cosa que no depende del JDK del emisor. La comparación del emisor, en
// cambio, va contra javac 17 —el oráculo del repo y el que fija la CI—, y con los fixtures de 21 el
// constructor de la anónima llevaba `MethodParameters` y el dex dejaba de cuadrar.

import (
	"encoding/base64"
	"testing"
)

// com/hola/apkcstudio/MainActivity$1.class -- 851 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_1 = "yv66vgAAADQAKwkAAgADBwAEDAAFAAYBACJjb20vaG9sYS9hcGtjc3R1ZGlvL01haW5BY3" +
	"Rpdml0eSQxAQAGdGhpcyQwAQAiTGNvbS9ob2xhL2Fwa2NzdHVkaW8vTWFpbkFjdGl2aXR5" +
	"OwoACAAJBwAKDAALAAwBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCAAOAQ" +
	"AIQ2xpY2tlZCEHABABABRhbmRyb2lkL3dpZGdldC9Ub2FzdAoADwASDAATABQBAAhtYWtl" +
	"VGV4dAEASihMYW5kcm9pZC9jb250ZW50L0NvbnRleHQ7TGphdmEvbGFuZy9DaGFyU2VxdW" +
	"VuY2U7SSlMYW5kcm9pZC93aWRnZXQvVG9hc3Q7CgAPABYMABcADAEABHNob3cHABkBAC1h" +
	"bmRyb2lkL3ZpZXcvTWVudUl0ZW0kT25NZW51SXRlbUNsaWNrTGlzdGVuZXIBACUoTGNvbS" +
	"9ob2xhL2Fwa2NzdHVkaW8vTWFpbkFjdGl2aXR5OylWAQAEQ29kZQEAD0xpbmVOdW1iZXJU" +
	"YWJsZQEAD29uTWVudUl0ZW1DbGljawEAGihMYW5kcm9pZC92aWV3L01lbnVJdGVtOylaAQ" +
	"AKU291cmNlRmlsZQEAEU1haW5BY3Rpdml0eS5qYXZhAQAPRW5jbG9zaW5nTWV0aG9kBwAj" +
	"AQAgY29tL2hvbGEvYXBrY3N0dWRpby9NYWluQWN0aXZpdHkMACUAJgEAE29uQ3JlYXRlT3" +
	"B0aW9uc01lbnUBABYoTGFuZHJvaWQvdmlldy9NZW51OylaAQAMSW5uZXJDbGFzc2VzBwAp" +
	"AQAVYW5kcm9pZC92aWV3L01lbnVJdGVtAQAXT25NZW51SXRlbUNsaWNrTGlzdGVuZXIAIA" +
	"ACAAgAAQAYAAEQEAAFAAYAAAACAAAACwAaAAEAGwAAACIAAgACAAAACiortQABKrcAB7EA" +
	"AAABABwAAAAGAAEAAAAUAAEAHQAeAAEAGwAAACsAAwACAAAADyq0AAESDQO4ABG2ABUErA" +
	"AAAAEAHAAAAAoAAgAAABYADQAXAAMAHwAAAAIAIAAhAAAABAAiACQAJwAAABIAAgACAAAA" +
	"AAAAABgAKAAqBgk=" +
	""

// com/hola/apkcstudio/MainActivity.class -- 1012 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_2 = "yv66vgAAADQANAoAAgADBwAEDAAFAAYBABRhbmRyb2lkL2FwcC9BY3Rpdml0eQEABjxpbm" +
	"l0PgEAAygpVgoAAgAIDAAJAAoBAAhvbkNyZWF0ZQEAFihMYW5kcm9pZC9vcy9CdW5kbGU7" +
	"KVYHAAwBABxjb20vaG9sYS9hcGtjc3R1ZGlvL1IkbGF5b3V0A38CAAAKAA8AEAcAEQwAEg" +
	"ATAQAgY29tL2hvbGEvYXBrY3N0dWRpby9NYWluQWN0aXZpdHkBAA5zZXRDb250ZW50Vmll" +
	"dwEABChJKVYIABUBAAxIZWxsbyBXb3JsZCELABcAGAcAGQwAGgAbAQARYW5kcm9pZC92aW" +
	"V3L01lbnUBAANhZGQBADEoTGphdmEvbGFuZy9DaGFyU2VxdWVuY2U7KUxhbmRyb2lkL3Zp" +
	"ZXcvTWVudUl0ZW07BwAdAQAiY29tL2hvbGEvYXBrY3N0dWRpby9NYWluQWN0aXZpdHkkMQ" +
	"oAHAAfDAAFACABACUoTGNvbS9ob2xhL2Fwa2NzdHVkaW8vTWFpbkFjdGl2aXR5OylWCwAi" +
	"ACMHACQMACUAJgEAFWFuZHJvaWQvdmlldy9NZW51SXRlbQEAGnNldE9uTWVudUl0ZW1DbG" +
	"lja0xpc3RlbmVyAQBIKExhbmRyb2lkL3ZpZXcvTWVudUl0ZW0kT25NZW51SXRlbUNsaWNr" +
	"TGlzdGVuZXI7KUxhbmRyb2lkL3ZpZXcvTWVudUl0ZW07AQAEQ29kZQEAD0xpbmVOdW1iZX" +
	"JUYWJsZQEAE29uQ3JlYXRlT3B0aW9uc01lbnUBABYoTGFuZHJvaWQvdmlldy9NZW51Oyla" +
	"AQAKU291cmNlRmlsZQEAEU1haW5BY3Rpdml0eS5qYXZhAQAMSW5uZXJDbGFzc2VzBwAvAQ" +
	"AVY29tL2hvbGEvYXBrY3N0dWRpby9SAQAGbGF5b3V0BwAyAQAtYW5kcm9pZC92aWV3L01l" +
	"bnVJdGVtJE9uTWVudUl0ZW1DbGlja0xpc3RlbmVyAQAXT25NZW51SXRlbUNsaWNrTGlzdG" +
	"VuZXIAIQAPAAIAAAAAAAMAAQAFAAYAAQAnAAAAHQABAAEAAAAFKrcAAbEAAAABACgAAAAG" +
	"AAEAAAALAAEACQAKAAEAJwAAACwAAgACAAAADCortwAHKhINtgAOsQAAAAEAKAAAAA4AAw" +
	"AAAA4ABQAPAAsAEAABACkAKgABACcAAAA0AAQAAgAAABgrEhS5ABYCALsAHFkqtwAeuQAh" +
	"AgBXBKwAAAABACgAAAAKAAIAAAAUABYAGgACACsAAAACACwALQAAABoAAwALAC4AMAAZAB" +
	"wAAAAAAAAAMQAiADMGCQ==" +
	""

// com/hola/apkcstudio/R$id.class -- 312 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_3 = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQ" +
	"ADKClWBwAIAQAYY29tL2hvbGEvYXBrY3N0dWRpby9SJGlkAQAGcHVsc2FyAQABSQEADUNv" +
	"bnN0YW50VmFsdWUDfwEAAAEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAApTb3VyY2VGaW" +
	"xlAQAGUi5qYXZhAQAMSW5uZXJDbGFzc2VzBwATAQAVY29tL2hvbGEvYXBrY3N0dWRpby9S" +
	"AQACaWQAMQAHAAIAAAABABkACQAKAAEACwAAAAIADAABAAEABQAGAAEADQAAAB0AAQABAA" +
	"AABSq3AAGxAAAAAQAOAAAABgABAAAACwACAA8AAAACABAAEQAAAAoAAQAHABIAFAAZ" +
	""

// com/hola/apkcstudio/R$layout.class -- 318 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_4 = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQ" +
	"ADKClWBwAIAQAcY29tL2hvbGEvYXBrY3N0dWRpby9SJGxheW91dAEABG1haW4BAAFJAQAN" +
	"Q29uc3RhbnRWYWx1ZQN/AgAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdXJjZU" +
	"ZpbGUBAAZSLmphdmEBAAxJbm5lckNsYXNzZXMHABMBABVjb20vaG9sYS9hcGtjc3R1ZGlv" +
	"L1IBAAZsYXlvdXQAMQAHAAIAAAABABkACQAKAAEACwAAAAIADAABAAEABQAGAAEADQAAAB" +
	"0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAADgACAA8AAAACABAAEQAAAAoAAQAHABIA" +
	"FAAZ" +
	""

// com/hola/apkcstudio/R$style.class -- 320 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_5 = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQ" +
	"ADKClWBwAIAQAbY29tL2hvbGEvYXBrY3N0dWRpby9SJHN0eWxlAQAIQXBwVGhlbWUBAAFJ" +
	"AQANQ29uc3RhbnRWYWx1ZQN/AwAAAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAClNvdX" +
	"JjZUZpbGUBAAZSLmphdmEBAAxJbm5lckNsYXNzZXMHABMBABVjb20vaG9sYS9hcGtjc3R1" +
	"ZGlvL1IBAAVzdHlsZQAxAAcAAgAAAAEAGQAJAAoAAQALAAAAAgAMAAEAAQAFAAYAAQANAA" +
	"AAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAARAAIADwAAAAIAEAARAAAACgABAAcA" +
	"EgAUABk=" +
	""

// com/hola/apkcstudio/R.class -- 362 bytes, javac 17.0.21 con `--release 8 -g:lines,source`
const claseJavac17_6 = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQ" +
	"ADKClWBwAIAQAVY29tL2hvbGEvYXBrY3N0dWRpby9SAQAEQ29kZQEAD0xpbmVOdW1iZXJU" +
	"YWJsZQEAClNvdXJjZUZpbGUBAAZSLmphdmEBAAxJbm5lckNsYXNzZXMHAA8BABtjb20vaG" +
	"9sYS9hcGtjc3R1ZGlvL1Ikc3R5bGUBAAVzdHlsZQcAEgEAHGNvbS9ob2xhL2Fwa2NzdHVk" +
	"aW8vUiRsYXlvdXQBAAZsYXlvdXQHABUBABhjb20vaG9sYS9hcGtjc3R1ZGlvL1IkaWQBAA" +
	"JpZAAxAAcAAgAAAAAAAQABAAUABgABAAkAAAAdAAEAAQAAAAUqtwABsQAAAAEACgAAAAYA" +
	"AQAAAAoAAgALAAAAAgAMAA0AAAAaAAMADgAHABAAGQARAAcAEwAZABQABwAWABk=" +
	""

// leeLasSeisDeJavac17: las seis clases del oráculo de 17, leídas para dexarlas con el mismo código que
// las nuestras.
func leeLasSeisDeJavac17(t *testing.T) []classFull {
	t.Helper()
	fixtures := []string{claseJavac17_1, claseJavac17_2, claseJavac17_3, claseJavac17_4, claseJavac17_5, claseJavac17_6}
	out := []classFull{}
	for i, f := range fixtures {
		b, err := base64.StdEncoding.DecodeString(f)
		if err != nil {
			t.Fatalf("fixture de javac 17 %d no es base64: %v", i+1, err)
		}
		cf, err := readClassFull(b)
		if err != nil {
			t.Fatalf("el fixture de javac 17 %d no se lee: %v", i+1, err)
		}
		out = append(out, cf)
	}
	return out
}

// TestLosFixturesDeJavac17SonLasSeisClases: el oráculo de 17 trae las mismas seis clases, en el mismo
// orden y con la versión 52 que deja `--release 8`. Si alguien las regenera con otro JDK, se ve aquí.
func TestLosFixturesDeJavac17SonLasSeisClases(t *testing.T) {
	clases := leeLasSeisDeJavac17(t)
	nombres := []string{
		"Lcom/hola/apkcstudio/MainActivity$1;",
		"Lcom/hola/apkcstudio/MainActivity;",
		"Lcom/hola/apkcstudio/R$id;",
		"Lcom/hola/apkcstudio/R$layout;",
		"Lcom/hola/apkcstudio/R$style;",
		"Lcom/hola/apkcstudio/R;",
	}
	if len(clases) != len(nombres) {
		t.Fatalf("%d clases en el oráculo de javac 17, medidas seis", len(clases))
	}
	for i, cf := range clases {
		if cf.Name != nombres[i] {
			t.Errorf("clase %d: %s, se esperaba %s", i+1, cf.Name, nombres[i])
		}
		if cf.Major != 52 {
			t.Errorf("%s: major version %d, javac 17 con `--release 8` deja 52", cf.Name, cf.Major)
		}
	}
	// y el constructor de la anónima no lleva `MethodParameters`, que es la razón de este oráculo
	anona := clases[0]
	for _, m := range anona.Methods {
		if m.Name == "<init>" && m.Parameters {
			t.Errorf("el constructor de MainActivity$1 lleva MethodParameters en el oráculo de javac 17")
		}
	}
}
