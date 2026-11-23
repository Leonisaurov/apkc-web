package main

// Frontera de `invokedynamic`/`condy` medida con nombre y sitio, no supuesta. Los dos casos que
// quedan fuera del subconjunto se miden aquí con la forma literal del rechazo.
//
// Un `invokedynamic` con un bootstrap que no es de los medidos lo rechaza `sitioDeLambda`
// (`lambda.go:164`): `LambdaMetafactory.metafactory` y `altMetafactory` son los únicos que este fork
// desugra. La sonda es un `record` de Java 17, donde `javac` escribe tres `invokedynamic` con
// `java/lang/runtime/ObjectMethods.bootstrap` (toString, hashCode y equals). El equivalente medido
// en el oráculo es el de `d8`: con `--min-api 26` desugra el sitio a llamadas directas y sintetiza
// `Punto$$ExternalSyntheticRecord0` con `$record$getFieldsAsObjects` y un `m(...)` que arma el
// texto; no emite `invoke-custom`. Reproducirlo pide implementar el desugrado de records, no una
// rama más del dexer.
//
// Una constante dinámica (tag 17) la rechaza `refuseOutsideSubset` (`classreader.go:451`). La sonda
// es un `.class` mínimo escrito a mano con `ldc` de un `ConstantDynamic` cuyo bootstrap es
// `ConstantBootstraps.nullConstant`; `javap -v -p` lo lee sin queja y lo imprime como
// `Dynamic #0:X:Ljava/lang/Object;`. El oráculo `d8` lo rechaza también: «Unsupported dynamic
// constant (runtime provided bootstrap method)» y «const-dynamic are not supported at any API level
// known by the compiler». DEX no tiene constante dinámica, así que la frontera es la misma que la
// de la herramienta de referencia y no una carencia de este fork.
//
// Comandos que dejaron las dos sondas:
//
//	javac --release 17 -d out Punto.java && base64 -w0 out/Punto.class
//	d8 --min-api 26 --output d8out out/Punto.class && dexdump -d d8out/classes.dex
//	javap -v -p Condy.class && d8 --min-api 26 --output d8out Condy.class

import (
	"encoding/base64"
	"strings"
	"testing"
)

// puntoRecordJavac es `javac --release 17` sobre
//
//	public record Punto(int x, int y) {
//	    public int suma() { return x + y; }
//	}
//
// y `base64 -w0` del `.class`. Los tres `invokedynamic` de `ObjectMethods.bootstrap` están en
// `toString`, `hashCode` y `equals`.
const puntoRecordJavac = "yv66vgAAAD0AOQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvUmVjb3JkAQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEABVB1bnRvAQABeAEAAUkJAAgADgwADwAMAQABeRIAAAARDAASABMBAAh0b1N0cmluZwEAGyhMUHVudG87KUxqYXZhL2xhbmcvU3RyaW5nOxIAAAAVDAAWABcBAAhoYXNoQ29kZQEACihMUHVudG87KUkSAAAAGQwAGgAbAQAGZXF1YWxzAQAcKExQdW50bztMamF2YS9sYW5nL09iamVjdDspWgEABShJSSlWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEE1ldGhvZFBhcmFtZXRlcnMBAARzdW1hAQADKClJAQAUKClMamF2YS9sYW5nL1N0cmluZzsBABUoTGphdmEvbGFuZy9PYmplY3Q7KVoBAApTb3VyY2VGaWxlAQAKUHVudG8uamF2YQEABlJlY29yZAEAEEJvb3RzdHJhcE1ldGhvZHMPBgApCgAqACsHACwMAC0ALgEAH2phdmEvbGFuZy9ydW50aW1lL09iamVjdE1ldGhvZHMBAAlib290c3RyYXABALEoTGphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXA7TGphdmEvbGFuZy9TdHJpbmc7TGphdmEvbGFuZy9pbnZva2UvVHlwZURlc2NyaXB0b3I7TGphdmEvbGFuZy9DbGFzcztMamF2YS9sYW5nL1N0cmluZztbTGphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlOylMamF2YS9sYW5nL09iamVjdDsIADABAAN4O3kPAQAHDwEADQEADElubmVyQ2xhc3NlcwcANQEAJWphdmEvbGFuZy9pbnZva2UvTWV0aG9kSGFuZGxlcyRMb29rdXAHADcBAB5qYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMBAAZMb29rdXAAMQAIAAIAAAACABIACwAMAAAAEgAPAAwAAAAHAAEABQAcAAIAHQAAACcAAgADAAAADyq3AAEqG7UAByoctQANsQAAAAEAHgAAAAYAAQAAAAEAHwAAAAkCAAsAAAAPAAAAAQAgACEAAQAdAAAAIgACAAEAAAAKKrQAByq0AA1grAAAAAEAHgAAAAYAAQAAAAIAEQASACIAAQAdAAAAHwABAAEAAAAHKroAEAAAsAAAAAEAHgAAAAYAAQAAAAEAEQAWACEAAQAdAAAAHwABAAEAAAAHKroAFAAArAAAAAEAHgAAAAYAAQAAAAEAEQAaACMAAQAdAAAAIAACAAIAAAAIKiu6ABgAAKwAAAABAB4AAAAGAAEAAAABAAEACwAhAAEAHQAAAB0AAQABAAAABSq0AAesAAAAAQAeAAAABgABAAAAAQABAA8AIQABAB0AAAAdAAEAAQAAAAUqtAANrAAAAAEAHgAAAAYAAQAAAAEABAAkAAAAAgAlACYAAAAOAAIACwAMAAAADwAMAAAAJwAAAA4AAQAoAAQACAAvADEAMgAzAAAACgABADQANgA4ABk="

// condyJavac es el `.class` mínimo con `ldc` de un `ConstantDynamic`: campo estático `X`, un
// `<clinit>` con `ldc #15; putstatic #20; return` y un `BootstrapMethods` que apunta a
// `ConstantBootstraps.nullConstant`.
const condyJavac = "yv66vgAAADQAFQEABUNvbmR5BwABAQAQamF2YS9sYW5nL09iamVjdAcAAwEAAVgBABJMamF2YS9sYW5nL09iamVjdDsMAAUABgEAI2phdmEvbGFuZy9pbnZva2UvQ29uc3RhbnRCb290c3RyYXBzBwAIAQAMbnVsbENvbnN0YW50AQBeKExqYXZhL2xhbmcvaW52b2tlL01ldGhvZEhhbmRsZXMkTG9va3VwO0xqYXZhL2xhbmcvU3RyaW5nO0xqYXZhL2xhbmcvQ2xhc3M7KUxqYXZhL2xhbmcvT2JqZWN0OwwACgALCgAJAAwPBgANEQAAAAcBAARDb2RlAQAIPGNsaW5pdD4BAAMoKVYBABBCb290c3RyYXBNZXRob2RzCQACAAcAIQACAAQAAAABAAgABQAGAAAAAQAIABEAEgABABAAAAASAAEAAAAAAAYSD7MAFLEAAAAAAAEAEwAAAAYAAQAOAAA="

// TestIndyDeRecordSeRechazaConNombre: el bootstrap de `ObjectMethods` no es de los medidos y el
// error dice el sitio (`toString`) y el bootstrap exacto. `readClassFull` lo acepta (el
// `invokedynamic` no se rechaza en el lector); es la expansión la que lo nombra.
func TestIndyDeRecordSeRechazaConNombre(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(puntoRecordJavac)
	if err != nil {
		t.Fatalf("el fixture del record no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el lector rechazó el record: %v", err)
	}
	if cf.Name != "LPunto;" {
		t.Fatalf("el fixture no es LPunto;: %s", cf.Name)
	}
	_, err = expandirLambdas([]classFull{cf})
	if err == nil {
		t.Fatal("el record pasó la expansión y su bootstrap no está medido")
	}
	for _, quiere := range []string{"toString", "ObjectMethods", "LambdaMetafactory.metafactory y altMetafactory"} {
		if !strings.Contains(err.Error(), quiere) {
			t.Errorf("el rechazo no nombra %q: %v", quiere, err)
		}
	}
}

// TestCondySeRechazaConNombre: una constante dinámica no se dexa, y el rechazo nombra la entrada
// del pool. También es el comportamiento de `d8` (`const-dynamic are not supported at any API level`).
func TestCondySeRechazaConNombre(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(condyJavac)
	if err != nil {
		t.Fatalf("el fixture de condy no es base64: %v", err)
	}
	_, err = readClassFull(datos)
	if err == nil {
		t.Fatal("la constante dinámica pasó el lector")
	}
	for _, quiere := range []string{"pool entry 15", "dynamic constant"} {
		if !strings.Contains(err.Error(), quiere) {
			t.Errorf("el rechazo no nombra %q: %v", quiere, err)
		}
	}
}
