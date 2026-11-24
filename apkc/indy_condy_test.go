package main

// Frontera de `invokedynamic`/`condy` medida con nombre y sitio, no supuesta.
//
// Los tres sitios de un record (`java/lang/runtime/ObjectMethods.bootstrap`) se desugran en
// record.go desde que la variante directa se midió en ART; lo que sigue fuera es el mismo bootstrap
// sobre una clase que no extiende `java/lang/Record`, y un `invokedynamic` de lambda cuyo bootstrap
// no es `LambdaMetafactory` (`sitioDeLambda`, `lambda.go:164`). La sonda es el `.class` de un
// record de Java 17 con el `super` cambiado a `java/lang/Object` en su pool.
//
// Una constante dinámica (tag 17) la rechaza `refuseOutsideSubset` (`classreader.go:590`; la cita
// antigua decía `:451`). La sonda
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

// TestIndyDeRecordFueraDeSuClaseSeRechazaConNombre: el bootstrap de `ObjectMethods` solo se
// desugra sobre la clase que extiende `java/lang/Record` y nombra a sí misma. La sonda es el mismo
// `.class` del record con el `super` cambiado a `java/lang/Object`: el rechazo dice el sitio y la
// condición que no se cumple. Los tres sitios que sí se desugran están en record_test.go.
func TestIndyDeRecordFueraDeSuClaseSeRechazaConNombre(t *testing.T) {
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
	// el `super` del `.class` es java/lang/Record; se cambia en el pool y en la clase para que el
	// sitio quede en una clase que no es un record
	cambiado := false
	for _, u := range cf.p.class {
		if cf.p.utf[u] == "java/lang/Record" {
			cf.p.utf[u] = "java/lang/Object"
			cambiado = true
		}
	}
	if !cambiado {
		t.Fatal("el pool del fixture no nombra java/lang/Record")
	}
	cf.Super = "Ljava/lang/Object;"
	_, err = expandirLambdas([]classFull{cf})
	if err == nil {
		t.Fatal("el sitio de un record pasó la expansión en una clase que no extiende Record")
	}
	for _, quiere := range []string{"toString", "no extiende java/lang/Record"} {
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
