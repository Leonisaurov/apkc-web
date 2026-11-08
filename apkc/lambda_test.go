package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Sondas de 5.22 compiladas con `javac --release 8`: la lambda estática sin capturas, la que captura
// un local, la que captura `this`, la referencia a un método con receptor (`this::suma`) y la
// referencia sin receptor (`Tipo::suma`), que se rechaza. Medidas en los §110 y §111.
const (
	lamJavac      = "yv66vgAAADQAOwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBAAkoKUxMYW0kRjsJAAwADQcADgwADwAQAQAQamF2YS9sYW5nL1N5c3RlbQEAA291dAEAFUxqYXZhL2lvL1ByaW50U3RyZWFtOwoAEgATBwAUDAAVAAoBAANMYW0BAAN1bm8LABcAGAcAGQwACQAaAQAFTGFtJEYBAAQoSSlJCgAcAB0HAB4MAB8AIAEAE2phdmEvaW8vUHJpbnRTdHJlYW0BAAdwcmludGxuAQAEKEkpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAARtYWluAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEADGxhbWJkYSR1bm8kMAEAClNvdXJjZUZpbGUBAAhMYW0uamF2YQEAEEJvb3RzdHJhcE1ldGhvZHMQABoPBgArCgASACwMACUAGg8GAC4KAC8AMAcAMQwAMgAzAQAiamF2YS9sYW5nL2ludm9rZS9MYW1iZGFNZXRhZmFjdG9yeQEAC21ldGFmYWN0b3J5AQDMKExqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMkTG9va3VwO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzAQABRgcANwEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHADkBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQASAAIAAAAAAAQAAQAFAAYAAQAhAAAAHQABAAEAAAAFKrcAAbEAAAABACIAAAAGAAEAAAABAAgAFQAKAAEAIQAAAB4AAQAAAAAABroABwAAsAAAAAEAIgAAAAYAAQAAAAMACQAjACQAAQAhAAAALQADAAEAAAARsgALuAARECm5ABYCALYAG7EAAAABACIAAAAKAAIAAAAFABAABhAKACUAGgABACEAAAAcAAIAAQAAAAQaBGCsAAAAAQAiAAAABgABAAAAAwADACYAAAACACcAKAAAAAwAAQAtAAMAKQAqACkANAAAABIAAgAXABIANQYIADYAOAA6ABk="
	lamFJavac     = "yv66vgAAADQADQcAAgEABUxhbSRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAQoSSlJAQAKU291cmNlRmlsZQEACExhbS5qYXZhAQAMSW5uZXJDbGFzc2VzBwALAQADTGFtAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
	lamLocJavac   = "yv66vgAAADQAPAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBAA0oSSlMTGFtTG9jJEY7CQAMAA0HAA4MAA8AEAEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKABIAEwcAFAwAFQAKAQAGTGFtTG9jAQADY29uCwAXABgHABkMAAkAGgEACExhbUxvYyRGAQAEKEkpSQoAHAAdBwAeDAAfACABABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEbWFpbgEAFihbTGphdmEvbGFuZy9TdHJpbmc7KVYBAAxsYW1iZGEkY29uJDABAAUoSUkpSQEAClNvdXJjZUZpbGUBAAtMYW1Mb2MuamF2YQEAEEJvb3RzdHJhcE1ldGhvZHMQABoPBgAsCgASAC0MACUAJg8GAC8KADAAMQcAMgwAMwA0AQAiamF2YS9sYW5nL2ludm9rZS9MYW1iZGFNZXRhZmFjdG9yeQEAC21ldGFmYWN0b3J5AQDMKExqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMkTG9va3VwO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzAQABRgcAOAEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHADoBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQASAAIAAAAAAAQAAQAFAAYAAQAhAAAAHQABAAEAAAAFKrcAAbEAAAABACIAAAAGAAEAAAABAAgAFQAKAAEAIQAAAB8AAQABAAAABxq6AAcAALAAAAABACIAAAAGAAEAAAADAAkAIwAkAAEAIQAAACoAAwABAAAAErIACwi4ABEQJbkAFgIAtgAbsQAAAAEAIgAAAAYAAQAAAAQQCgAlACYAAQAhAAAAHAACAAIAAAAEGxpgrAAAAAEAIgAAAAYAAQAAAAMAAwAnAAAAAgAoACkAAAAMAAEALgADACoAKwAqADUAAAASAAIAFwASADYGCAA3ADkAOwAZ"
	lamLocFJavac  = "yv66vgAAADQADQcAAgEACExhbUxvYyRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAQoSSlJAQAKU291cmNlRmlsZQEAC0xhbUxvYy5qYXZhAQAMSW5uZXJDbGFzc2VzBwALAQAGTGFtTG9jAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
	lamThisJavac  = "yv66vgAAADQAQQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBABYoTExhbVRoaXM7KUxMYW1UaGlzJEY7BwAMAQAHTGFtVGhpcwoACwADCQALAA8MABAAEQEAAXkBAAFJCQATABQHABUMABYAFwEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKAAsAGQwAGgAbAQAHY2FwdHVyYQEADSgpTExhbVRoaXMkRjsLAB0AHgcAHwwACQAgAQAJTGFtVGhpcyRGAQAEKEkpSQoAIgAjBwAkDAAlACYBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEbWFpbgEAFihbTGphdmEvbGFuZy9TdHJpbmc7KVYBABBsYW1iZGEkY2FwdHVyYSQwAQAKU291cmNlRmlsZQEADExhbVRoaXMuamF2YQEAEEJvb3RzdHJhcE1ldGhvZHMQACAPBwAxCgALADIMACsAIA8GADQKADUANgcANwwAOAA5AQAiamF2YS9sYW5nL2ludm9rZS9MYW1iZGFNZXRhZmFjdG9yeQEAC21ldGFmYWN0b3J5AQDMKExqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMkTG9va3VwO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzAQABRgcAPQEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHAD8BAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQALAAIAAAABAAAAEAARAAAABAABAAUABgABACcAAAAdAAEAAQAAAAUqtwABsQAAAAEAKAAAAAYAAQAAAAEAAAAaABsAAQAnAAAAHwABAAEAAAAHKroABwAAsAAAAAEAKAAAAAYAAQAAAAQACQApACoAAQAnAAAANwADAAIAAAAfuwALWbcADUwrCLUADrIAEiu2ABgQJbkAHAIAtgAhsQAAAAEAKAAAAAYAAQAAAAUQAgArACAAAQAnAAAAHwACAAIAAAAHGyq0AA5grAAAAAEAKAAAAAYAAQAAAAQAAwAsAAAAAgAtAC4AAAAMAAEAMwADAC8AMAAvADoAAAASAAIAHQALADsGCAA8AD4AQAAZ"
	lamThisFJavac = "yv66vgAAADQADQcAAgEACUxhbVRoaXMkRgcABAEAEGphdmEvbGFuZy9PYmplY3QBAAFmAQAEKEkpSQEAClNvdXJjZUZpbGUBAAxMYW1UaGlzLmphdmEBAAxJbm5lckNsYXNzZXMHAAsBAAdMYW1UaGlzAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
	lamRefJavac   = "yv66vgAAADQAQQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBABQoTExhbVJlZjspTExhbVJlZiRGOwkADAANBwAODAAPABABAAZMYW1SZWYBAAF5AQABSQoADAADCQATABQHABUMABYAFwEAEGphdmEvbGFuZy9TeXN0ZW0BAANvdXQBABVMamF2YS9pby9QcmludFN0cmVhbTsKAAwAGQwAGgAbAQADcmVmAQAMKClMTGFtUmVmJEY7CwAdAB4HAB8MAAkAIAEACExhbVJlZiRGAQAEKEkpSQoAIgAjBwAkDAAlACYBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAEc3VtYQEABG1haW4BABYoW0xqYXZhL2xhbmcvU3RyaW5nOylWAQAKU291cmNlRmlsZQEAC0xhbVJlZi5qYXZhAQAQQm9vdHN0cmFwTWV0aG9kcxAAIA8FADEKAAwAMgwAKQAgDwYANAoANQA2BwA3DAA4ADkBACJqYXZhL2xhbmcvaW52b2tlL0xhbWJkYU1ldGFmYWN0b3J5AQALbWV0YWZhY3RvcnkBAMwoTGphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXA7TGphdmEvbGFuZy9TdHJpbmc7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlOylMamF2YS9sYW5nL2ludm9rZS9DYWxsU2l0ZTsBAAxJbm5lckNsYXNzZXMBAAFGBwA9AQAlamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cAcAPwEAHmphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcwEABkxvb2t1cAAhAAwAAgAAAAEAAAAPABAAAAAEAAEABQAGAAEAJwAAAB0AAQABAAAABSq3AAGxAAAAAQAoAAAABgABAAAAAQAAABoAGwABACcAAAAfAAEAAQAAAAcqugAHAACwAAAAAQAoAAAABgABAAAABAAAACkAIAABACcAAAAfAAIAAgAAAAcbKrQAC2CsAAAAAQAoAAAABgABAAAABQAJACoAKwABACcAAAA3AAMAAgAAAB+7AAxZtwARTCsItQALsgASK7YAGBAluQAcAgC2ACGxAAAAAQAoAAAABgABAAAABgADACwAAAACAC0ALgAAAAwAAQAzAAMALwAwAC8AOgAAABIAAgAdAAwAOwYIADwAPgBAABk="
	lamRefFJavac  = "yv66vgAAADQADQcAAgEACExhbVJlZiRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAQoSSlJAQAKU291cmNlRmlsZQEAC0xhbVJlZi5qYXZhAQAMSW5uZXJDbGFzc2VzBwALAQAGTGFtUmVmAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
	lamUnbJavac   = "yv66vgAAADQAKgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBAAwoKUxMYW1VbmIkRjsHAAwBAAZMYW1VbmIBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQADcmVmAQAEc3VtYQEABChJKUkBAApTb3VyY2VGaWxlAQALTGFtVW5iLmphdmEBABBCb290c3RyYXBNZXRob2RzEAAWAQAMKExMYW1VbmI7SSlJDwUAGAoACwAZDAAQABEPBgAbCgAcAB0HAB4MAB8AIAEAImphdmEvbGFuZy9pbnZva2UvTGFtYmRhTWV0YWZhY3RvcnkBAAttZXRhZmFjdG9yeQEAzChMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cDtMamF2YS9sYW5nL1N0cmluZztMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7KUxqYXZhL2xhbmcvaW52b2tlL0NhbGxTaXRlOwEADElubmVyQ2xhc3NlcwcAIwEACExhbVVuYiRGAQABRgcAJgEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHACgBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQALAAIAAAAAAAMAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAAADwAKAAEADQAAAB4AAQABAAAABroABwAAsAAAAAEADgAAAAYAAQAAAAMAAAAQABEAAQANAAAAGgABAAIAAAACG6wAAAABAA4AAAAGAAEAAAAEAAMAEgAAAAIAEwAUAAAADAABABoAAwAVABcAFQAhAAAAEgACACIACwAkBggAJQAnACkAGQ=="
	lamUnbFJavac  = "yv66vgAAADQADQcAAgEACExhbVVuYiRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAwoTExhbVVuYjtJKUkBAApTb3VyY2VGaWxlAQALTGFtVW5iLmphdmEBAAxJbm5lckNsYXNzZXMHAAsBAAZMYW1VbmIBAAFGBgAAAQADAAAAAAABBAEABQAGAAAAAgAHAAAAAgAIAAkAAAAKAAEAAQAKAAwGCA=="
)

func lambdaFixture(t *testing.T, b64 string) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	return cf
}

// expandirPar devuelve las clases expandidas de la clase dueña y su interfaz SAM.
func expandirPar(t *testing.T, clase, interfaz string) []classFull {
	t.Helper()
	clases, err := expandirLambdas([]classFull{lambdaFixture(t, clase), lambdaFixture(t, interfaz)})
	if err != nil {
		t.Fatalf("la expansion se cayo: %v", err)
	}
	return clases
}

func sinteticaDe(t *testing.T, clases []classFull, nombre string) classFull {
	t.Helper()
	for _, c := range clases {
		if c.Name == nombre {
			return c
		}
	}
	t.Fatalf("no se sintetizo %s: %v", nombre, nombresDeLambda(clases))
	return classFull{}
}

// TestLaLambdaEstaticaSinCapturasSeDesugra: el caso base, sin campos ni capturas.
func TestLaLambdaEstaticaSinCapturasSeDesugra(t *testing.T) {
	clases := expandirPar(t, lamJavac, lamFJavac)
	if len(clases) != 3 {
		t.Fatalf("salieron %d clases, se esperaban 3", len(clases))
	}
	if len(clases[0].Lambdas) != 1 {
		t.Fatalf("Lam tiene %d sitios de lambda, se esperaba 1", len(clases[0].Lambdas))
	}
	s := sinteticaDe(t, clases, "LLam$$Lambda$0;")
	if len(s.Fields) != 0 || len(s.Interfaces) != 1 || s.Interfaces[0] != "LLam$F;" {
		t.Errorf("la sintetica sin capturas tiene campos %d e interfaces %v", len(s.Fields), s.Interfaces)
	}
	if m, ok := buscaMetodo(s, "<init>"); !ok || m.Desc != "()V" {
		t.Errorf("constructor %v", m.Desc)
	}
	dex, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("nuestro dex no se relee: %v", err)
	}
	vistas := map[string]bool{}
	for _, c := range d.classes {
		vistas[c.name] = true
	}
	for _, quiero := range []string{"LLam;", "LLam$F;", "LLam$$Lambda$0;"} {
		if !vistas[quiero] {
			t.Errorf("el dex no lleva la clase %s: %v", quiero, vistas)
		}
	}
}

// TestLaLambdaQueCapturaUnLocal: el sitio lleva `(I)`; la sintetica guarda `arg$1:I`, su constructor
// es `(I)V` y el SAM carga el campo antes del argumento.
func TestLaLambdaQueCapturaUnLocal(t *testing.T) {
	clases := expandirPar(t, lamLocJavac, lamLocFJavac)
	s := sinteticaDe(t, clases, "LLamLoc$$Lambda$0;")
	if len(s.Fields) != 1 || s.Fields[0].Desc != "I" {
		t.Fatalf("campos de la sintetica: %+v", s.Fields)
	}
	var sitio classLambda
	for _, s := range clases[0].Lambdas {
		sitio = s
	}
	if sitio.ctorDesc != "(I)V" || strings.Join(sitio.capturas, "") != "I" {
		t.Errorf("sitio desugrado: %+v", sitio)
	}
	if m, ok := buscaMetodo(s, "<init>"); !ok || m.Desc != "(I)V" || m.ArgCount != 2 {
		t.Errorf("constructor: %+v", m)
	}
	if _, err := dexDeClases(clases); err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
}

// TestLaLambdaQueCapturaThis: el sitio lleva `(LLamThis;)`; el campo es el receptor y el constructor
// `(LLamThis;)V`. El implementador es un método de instancia al que se le quitó `private`.
func TestLaLambdaQueCapturaThis(t *testing.T) {
	clases := expandirPar(t, lamThisJavac, lamThisFJavac)
	s := sinteticaDe(t, clases, "LLamThis$$Lambda$0;")
	if len(s.Fields) != 1 || s.Fields[0].Desc != "LLamThis;" {
		t.Fatalf("campos de la sintetica: %+v", s.Fields)
	}
	if m, ok := buscaMetodo(s, "<init>"); !ok || m.Desc != "(LLamThis;)V" || m.ArgCount != 2 {
		t.Errorf("constructor: %+v", m)
	}
	privado := false
	for _, m := range clases[0].Methods {
		if m.Name == "lambda$captura$0" {
			privado = m.Access&accPrivate != 0
		}
	}
	if privado {
		t.Error("el implementador de instancia conserva private y la clase sintetica no puede llamarlo")
	}
	if _, err := dexDeClases(clases); err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
}

// TestLaReferenciaAUnMetodoConReceptor: `this::suma` también captura el receptor; el implementador
// es el propio método público (REF_invokeVirtual), sin `lambda$`.
func TestLaReferenciaAUnMetodoConReceptor(t *testing.T) {
	clases := expandirPar(t, lamRefJavac, lamRefFJavac)
	s := sinteticaDe(t, clases, "LLamRef$$Lambda$0;")
	if len(s.Fields) != 1 || s.Fields[0].Desc != "LLamRef;" {
		t.Fatalf("campos de la sintetica: %+v", s.Fields)
	}
	if m, ok := buscaMetodo(s, "<init>"); !ok || m.Desc != "(LLamRef;)V" {
		t.Errorf("constructor: %+v", m)
	}
	if _, err := dexDeClases(clases); err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
}

// TestLaReferenciaSinReceptorSeRechazaPorNombre: `Tipo::suma` deja el receptor como primer argumento
// del SAM, no como captura; ese caso no está medido.
func TestLaReferenciaSinReceptorSeRechazaPorNombre(t *testing.T) {
	_, err := expandirLambdas([]classFull{lambdaFixture(t, lamUnbJavac), lambdaFixture(t, lamUnbFJavac)})
	if err == nil || !strings.Contains(err.Error(), "receptor") {
		t.Fatalf("se esperaba el rechazo por receptor y salio: %v", err)
	}
}

func nombresDeLambda(clases []classFull) []string {
	out := make([]string, 0, len(clases))
	for _, c := range clases {
		out = append(out, c.Name)
	}
	return out
}
