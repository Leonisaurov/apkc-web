package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Sonda de 5.22: `x -> x + 1` en un método `static`, compilada por `javac --release 8`. El
// `invokedynamic` del sitio se desugra a `LLam$$Lambda$0;`, un constructor `()V` y un método `f`
// que llama al `lambda$uno$0` que javac ya dejó en la clase. Medido en el §110: el dex corre en ART.
const (
	lamJavac  = "yv66vgAAADQAOwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBAAkoKUxMYW0kRjsJAAwADQcADgwADwAQAQAQamF2YS9sYW5nL1N5c3RlbQEAA291dAEAFUxqYXZhL2lvL1ByaW50U3RyZWFtOwoAEgATBwAUDAAVAAoBAANMYW0BAAN1bm8LABcAGAcAGQwACQAaAQAFTGFtJEYBAAQoSSlJCgAcAB0HAB4MAB8AIAEAE2phdmEvaW8vUHJpbnRTdHJlYW0BAAdwcmludGxuAQAEKEkpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAARtYWluAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEADGxhbWJkYSR1bm8kMAEAClNvdXJjZUZpbGUBAAhMYW0uamF2YQEAEEJvb3RzdHJhcE1ldGhvZHMQABoPBgArCgASACwMACUAGg8GAC4KAC8AMAcAMQwAMgAzAQAiamF2YS9sYW5nL2ludm9rZS9MYW1iZGFNZXRhZmFjdG9yeQEAC21ldGFmYWN0b3J5AQDMKExqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMkTG9va3VwO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTtMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kVHlwZTspTGphdmEvbGFuZy9pbnZva2UvQ2FsbFNpdGU7AQAMSW5uZXJDbGFzc2VzAQABRgcANwEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHADkBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQASAAIAAAAAAAQAAQAFAAYAAQAhAAAAHQABAAEAAAAFKrcAAbEAAAABACIAAAAGAAEAAAABAAgAFQAKAAEAIQAAAB4AAQAAAAAABroABwAAsAAAAAEAIgAAAAYAAQAAAAMACQAjACQAAQAhAAAALQADAAEAAAARsgALuAARECm5ABYCALYAG7EAAAABACIAAAAKAAIAAAAFABAABhAKACUAGgABACEAAAAcAAIAAQAAAAQaBGCsAAAAAQAiAAAABgABAAAAAwADACYAAAACACcAKAAAAAwAAQAtAAMAKQAqACkANAAAABIAAgAXABIANQYIADYAOAA6ABk="
	lamFJavac = "yv66vgAAADQADQcAAgEABUxhbSRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAQoSSlJAQAKU291cmNlRmlsZQEACExhbS5qYXZhAQAMSW5uZXJDbGFzc2VzBwALAQADTGFtAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
	// la misma sonda capturando `this` (LamCap$1 del §109): el sitio lleva el receptor y se rechaza.
	lamCapJavac  = "yv66vgAAADQALgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWEgAAAAgMAAkACgEAAWYBABQoTExhbUNhcDspTExhbUNhcCRGOwkADAANBwAODAAPABABAAZMYW1DYXABAAF5AQABSQEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAdjYXB0dXJhAQAMKClMTGFtQ2FwJEY7AQAQbGFtYmRhJGNhcHR1cmEkMAEABChJKUkBAApTb3VyY2VGaWxlAQALTGFtQ2FwLmphdmEBABBCb290c3RyYXBNZXRob2RzEAAWDwcAHAoADAAdDAAVABYPBgAfCgAgACEHACIMACMAJAEAImphdmEvbGFuZy9pbnZva2UvTGFtYmRhTWV0YWZhY3RvcnkBAAttZXRhZmFjdG9yeQEAzChMamF2YS9sYW5nL2ludm9rZS9NZXRob2RIYW5kbGVzJExvb2t1cDtMamF2YS9sYW5nL1N0cmluZztMamF2YS9sYW5nL2ludm9rZS9NZXRob2RUeXBlO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7TGphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlO0xqYXZhL2xhbmcvaW52b2tlL01ldGhvZFR5cGU7KUxqYXZhL2xhbmcvaW52b2tlL0NhbGxTaXRlOwEADElubmVyQ2xhc3NlcwcAJwEACExhbUNhcCRGAQABRgcAKgEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHACwBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAIQAMAAIAAAABAAAADwAQAAAAAwABAAUABgABABEAAAAdAAEAAQAAAAUqtwABsQAAAAEAEgAAAAYAAQAAAAEAAAATABQAAQARAAAAHwABAAEAAAAHKroABwAAsAAAAAEAEgAAAAYAAQAAAAQQAgAVABYAAQARAAAAHwACAAIAAAAHGyq0AAtgrAAAAAEAEgAAAAYAAQAAAAQAAwAXAAAAAgAYABkAAAAMAAEAHgADABoAGwAaACUAAAASAAIAJgAMACgGCAApACsALQAZ"
	lamCapFJavac = "yv66vgAAADQADQcAAgEACExhbUNhcCRGBwAEAQAQamF2YS9sYW5nL09iamVjdAEAAWYBAAQoSSlJAQAKU291cmNlRmlsZQEAC0xhbUNhcC5qYXZhAQAMSW5uZXJDbGFzc2VzBwALAQAGTGFtQ2FwAQABRgYAAAEAAwAAAAAAAQQBAAUABgAAAAIABwAAAAIACAAJAAAACgABAAEACgAMBgg="
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

func TestLaLambdaEstaticaSinCapturasSeDesugra(t *testing.T) {
	lam := lambdaFixture(t, lamJavac)
	interfaz := lambdaFixture(t, lamFJavac)
	clases, err := expandirLambdas([]classFull{lam, interfaz})
	if err != nil {
		t.Fatalf("la expansion se cayo: %v", err)
	}
	if len(clases) != 3 {
		t.Fatalf("salieron %d clases, se esperaban 3", len(clases))
	}
	if len(clases[0].Lambdas) != 1 {
		t.Fatalf("Lam tiene %d sitios de lambda, se esperaba 1", len(clases[0].Lambdas))
	}
	var sintetica *classFull
	for i := range clases {
		if clases[i].Name == "LLam$$Lambda$0;" {
			sintetica = &clases[i]
		}
	}
	if sintetica == nil {
		t.Fatalf("no se sintetizo LLam$$Lambda$0;: %v", nombresDeLambda(clases))
	}
	if sintetica.Access&accSynthetic == 0 || sintetica.Access&accFinal == 0 {
		t.Errorf("la clase sintetica tiene flags %#x", sintetica.Access)
	}
	if len(sintetica.Interfaces) != 1 || sintetica.Interfaces[0] != "LLam$F;" {
		t.Errorf("interfaces %v", sintetica.Interfaces)
	}
	nombres := []string{}
	for _, m := range sintetica.Methods {
		nombres = append(nombres, m.Name+m.Desc)
	}
	if strings.Join(nombres, ",") != "<init>()V,f(I)I" {
		t.Errorf("metodos de la sintetica: %v", nombres)
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
	metodo := false
	for _, m := range d.methods {
		if m.owner == "LLam$$Lambda$0;" && m.name == "f" && m.proto.returnType == "I" && len(m.proto.params) == 1 {
			metodo = true
		}
	}
	if !metodo {
		t.Error("el dex no tiene LLam$$Lambda$0;.f(I)I")
	}
}

func TestLaLambdaConCapturaSeRechazaPorNombre(t *testing.T) {
	lam := lambdaFixture(t, lamCapJavac)
	interfaz := lambdaFixture(t, lamCapFJavac)
	_, err := expandirLambdas([]classFull{lam, interfaz})
	if err == nil || !strings.Contains(err.Error(), "captura 1 valores") {
		t.Fatalf("se esperaba el rechazo por captura y salio: %v", err)
	}
}

func nombresDeLambda(clases []classFull) []string {
	out := make([]string, 0, len(clases))
	for _, c := range clases {
		out = append(out, c.Name)
	}
	return out
}
