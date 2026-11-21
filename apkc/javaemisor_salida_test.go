package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// salidaCatchFuente: la sonda del párrafo 167. Un `try` con `catch` y `finally` cuyo cuerpo sale
// por más de un trozo (un `return` en una rama y la caída en la otra).
const salidaCatchFuente = `public class SalidaCatch {
  int campo;
  static int f(int n) {
    int r = 0;
    try {
      if (n > 0) { return 1; }
      r = n + 1;
    } catch (RuntimeException e) {
      r = 2;
    } finally {
      r = r + 10;
    }
    return r;
  }
  int g(int n) {
    try {
      if (n == 0) { return campo; }
      campo = n + 1;
    } catch (RuntimeException e) {
      campo = 7;
    } finally {
      campo = campo + 10;
    }
    return campo;
  }
  static int h(int n) {
    try {
      if (n < 0) { return 0 - n; }
      return n + 1;
    } catch (RuntimeException e) {
      return 0;
    }
  }
}
`

// salidaCatchJavac: `javac -g SalidaCatch.java && base64 -w0 SalidaCatch.class` con javac 17.
const salidaCatchJavac = "yv66vgAAAD0AIQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAaamF2YS9sYW5nL1J1bnRpbWVFeGNlcHRpb24JAAoACwcADAwADQAOAQALU2FsaWRhQ2F0Y2gBAAVjYW1wbwEAAUkBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQASTG9jYWxWYXJpYWJsZVRhYmxlAQAEdGhpcwEADUxTYWxpZGFDYXRjaDsBAAFmAQAEKEkpSQEAAWUBABxMamF2YS9sYW5nL1J1bnRpbWVFeGNlcHRpb247AQABbgEAAXIBAA1TdGFja01hcFRhYmxlBwAcAQATamF2YS9sYW5nL1Rocm93YWJsZQEAAWcBAAFoAQAKU291cmNlRmlsZQEAEFNhbGlkYUNhdGNoLmphdmEAIQAKAAIAAAABAAAADQAOAAAABAABAAUABgABAA8AAAAvAAEAAQAAAAUqtwABsQAAAAIAEAAAAAYAAQAAAAEAEQAAAAwAAQAAAAUAEgATAAAACAAUABUAAQAPAAAA3wACAAQAAAAwAzwangAMBD0bEApgPBysGgRgPBsQCmA8pwAWTQU8GxAKYDynAAtOGxAKYDwtvxusAAUAAgAIABsABwAPABMAGwAHAAIACAAmAAAADwATACYAAAAbAB4AJgAAAAMAEAAAADoADgAAAAQAAgAGAAgACwANAAYADwAHABMACwAYAAwAGwAIABwACQAeAAsAIwAMACYACwAsAAwALgANABEAAAAgAAMAHAACABYAFwACAAAAMAAYAA4AAAACAC4AGQAOAAEAGgAAAA8ABPwADwFLBwAHSgcAGwcAAAAdABUAAQAPAAAA+wADAAQAAABTG5oAFSq0AAk9Kiq0AAkQCmC1AAkcrCobBGC1AAkqKrQACRAKYLUACacAJk0qEAe1AAkqKrQACRAKYLUACacAEU4qKrQACRAKYLUACS2/KrQACawABQAAAAkAKwAHABYAHQArAAcAAAAJAEAAAAAWAB0AQAAAACsAMgBAAAAAAwAQAAAANgANAAAAEQAJABYAFAARABYAEgAdABYAKAAXACsAEwAsABQAMgAWAD0AFwBAABYATAAXAE4AGAARAAAAIAADACwABgAWABcAAgAAAFMAEgATAAAAAABTABgADgABABoAAAAMAAQWVAcAB1QHABsNAAgAHgAVAAEADwAAAGwAAgACAAAADxqcAAcDGmSsGgRgrEwDrAACAAAABwAMAAcACAALAAwABwADABAAAAASAAQAAAAcAAgAHQAMAB4ADQAfABEAAAAWAAIADQACABYAFwABAAAADwAYAA4AAAAaAAAABwACCEMHAAcAAQAfAAAAAgAg"

func TestElTryConCatchQueCruzaSalidaSeDexaIgualQueJavac(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/SalidaCatch.java", contenido: salidaCatchFuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	datos, err := base64.StdEncoding.DecodeString(salidaCatchJavac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
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
		if len(encontrada.Code) != len(esperado.Code) {
			t.Errorf("%s%s: Code de %d bytes contra %d de javac",
				esperado.Name, esperado.Desc, len(encontrada.Code), len(esperado.Code))
		}
		if len(encontrada.Tries) != len(esperado.Tries) {
			t.Errorf("%s%s: %d filas de excepción contra %d de javac",
				esperado.Name, esperado.Desc, len(encontrada.Tries), len(esperado.Tries))
			continue
		}
		for i := range esperado.Tries {
			if encontrada.Tries[i] != esperado.Tries[i] {
				t.Errorf("%s%s fila %d: %+v contra %+v de javac",
					esperado.Name, esperado.Desc, i, encontrada.Tries[i], esperado.Tries[i])
			}
		}
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
		t.Errorf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(deUno), len(deOtro), primerByteDistinto(deUno, deOtro))
	}
}
