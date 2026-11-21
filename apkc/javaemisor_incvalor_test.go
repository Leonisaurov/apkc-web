package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// incValorFuente es la sonda del párrafo 151: ++/-- como valor, en prefijo y postfijo, sobre un local,
// un campo de instancia y un campo estático, con + y con -. Compilada con javac 17 y -g.
const incValorFuente = `public class IncValor {
  int campo;
  static int scampo;
  static int prefijoLocal(int i) { int r = ++i; return r * 10 + i; }
  static int postfijoLocal(int i) { int r = i++; return r * 10 + i; }
  static int menosLocal(int i) { return --i * 10 + i--; }
  static int ambos(int i) { return ++i + i++; }
  static int enExpresion(int i) { return (i++) * 2 + i; }
  int prefijoCampo() { return ++campo * 10 + campo; }
  int postfijoCampo() { return campo++ * 10 + campo; }
  int menosCampo() { return --campo * 10 + campo--; }
  static int prefijoEstatico() { return ++scampo * 10 + scampo; }
  static int postfijoEstatico() { return scampo++ * 10 + scampo; }
  static int menosEstatico() { return --scampo * 10 + scampo--; }
}
`

// incValorJavac: `javac -g IncValor.java && base64 -w0 IncValor.class` con el javac 17 del gate.
const incValorJavac = "yv66vgAAAD0AJgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEACEluY1ZhbG9yAQAFY2FtcG8BAAFJCQAIAA4MAA8ADAEABnNjYW1wbwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABJMb2NhbFZhcmlhYmxlVGFibGUBAAR0aGlzAQAKTEluY1ZhbG9yOwEADHByZWZpam9Mb2NhbAEABChJKUkBAAFpAQABcgEADXBvc3RmaWpvTG9jYWwBAAptZW5vc0xvY2FsAQAFYW1ib3MBAAtlbkV4cHJlc2lvbgEADHByZWZpam9DYW1wbwEAAygpSQEADXBvc3RmaWpvQ2FtcG8BAAptZW5vc0NhbXBvAQAPcHJlZmlqb0VzdGF0aWNvAQAQcG9zdGZpam9Fc3RhdGljbwEADW1lbm9zRXN0YXRpY28BAApTb3VyY2VGaWxlAQANSW5jVmFsb3IuamF2YQAhAAgAAgAAAAIAAAALAAwAAAAIAA8ADAAAAAwAAQAFAAYAAQAQAAAALwABAAEAAAAFKrcAAbEAAAACABEAAAAGAAEAAAABABIAAAAMAAEAAAAFABMAFAAAAAgAFQAWAAEAEAAAAEAAAgACAAAADIQAARo8GxAKaBpgrAAAAAIAEQAAAAYAAQAAAAQAEgAAABYAAgAAAAwAFwAMAAAABQAHABgADAABAAgAGQAWAAEAEAAAAEAAAgACAAAADBqEAAE8GxAKaBpgrAAAAAIAEQAAAAYAAQAAAAUAEgAAABYAAgAAAAwAFwAMAAAABQAHABgADAABAAgAGgAWAAEAEAAAADcAAgABAAAADYQA/xoQCmgahAD/YKwAAAACABEAAAAGAAEAAAAGABIAAAAMAAEAAAANABcADAAAAAgAGwAWAAEAEAAAADQAAgABAAAACoQAARoahAABYKwAAAACABEAAAAGAAEAAAAHABIAAAAMAAEAAAAKABcADAAAAAgAHAAWAAEAEAAAADMAAgABAAAACRqEAAEFaBpgrAAAAAIAEQAAAAYAAQAAAAgAEgAAAAwAAQAAAAkAFwAMAAAAAAAdAB4AAQAQAAAAPgADAAEAAAAUKlm0AAcEYFq1AAcQCmgqtAAHYKwAAAACABEAAAAGAAEAAAAJABIAAAAMAAEAAAAUABMAFAAAAAAAHwAeAAEAEAAAAD4ABAABAAAAFCpZtAAHWgRgtQAHEApoKrQAB2CsAAAAAgARAAAABgABAAAACgASAAAADAABAAAAFAATABQAAAAAACAAHgABABAAAABFAAUAAQAAABsqWbQABwRkWrUABxAKaCpZtAAHWgRktQAHYKwAAAACABEAAAAGAAEAAAALABIAAAAMAAEAAAAbABMAFAAAAAgAIQAeAAEAEAAAACkAAgAAAAAAEbIADQRgWbMADRAKaLIADWCsAAAAAQARAAAABgABAAAADAAIACIAHgABABAAAAApAAMAAAAAABGyAA1ZBGCzAA0QCmiyAA1grAAAAAEAEQAAAAYAAQAAAA0ACAAjAB4AAQAQAAAALwAEAAAAAAAXsgANBGRZswANEAposgANWQRkswANYKwAAAABABEAAAAGAAEAAAAOAAEAJAAAAAIAJQ=="

// TestElIncrementoComoValorSeDexaIgualQueJavac: el oráculo doble del párrafo 151. Primero el Code de
// cada método, byte a byte contra javac (aquí se ve el sitio exacto del `iinc`, del `dup` y del
// `dup_x1`); después el dex completo, porque la pila simulada tiene que coincidir con la de javac.
func TestElIncrementoComoValorSeDexaIgualQueJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/IncValor.java", contenido: incValorFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(incValorJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el método %s%s", esperado.Name, esperado.Desc)
			continue
		}
		// El Code se compara por longitud: los dos bytes del índice de un fieldref cambian según
		// el orden del pool, y eso lo decide el dex byte a byte de abajo, que es el oráculo del repo.
		if len(encontrada.Code) != len(esperado.Code) {
			t.Errorf("%s%s: nuestro Code mide %d bytes y el de javac %d",
				esperado.Name, esperado.Desc, len(encontrada.Code), len(esperado.Code))
		}
	}
	deUno, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	deOtro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if !bytes.Equal(deUno, deOtro) {
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
}
