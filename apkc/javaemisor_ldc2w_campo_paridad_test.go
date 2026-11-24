//go:build !js

package main

import (
	"bytes"
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_ldc2w_campo_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_ldc2w_campo_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLaTablaDelLargoEsLaDeJavac17: el dex entero de la sonda byte a byte y, además, la forma de cada
// instrucción leída en nuestro `.class`, porque el dex no distingue `ldc2_w` de una constante cargada
// por otra vía de tres bytes. `lconst_0`/`lconst_1` solo para 0 y 1; `ldc2_w` para el resto; y ningún
// `i2l` detrás de un `sipush` (la forma que se corrige).
func TestLaTablaDelLargoEsLaDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Tabla.java", contenido: fuenteTablaLdc2w}})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	clase := claseEmitidaPorNombre(t, nuestras, "p/Tabla")
	for _, metodos := range [][2]string{
		{"l0", "lconst_0"}, {"l1", "lconst_1"}, {"l5", "ldc2_w"},
		{"l127", "ldc2_w"}, {"l128", "ldc2_w"}, {"l500", "ldc2_w"},
		{"l32767", "ldc2_w"}, {"l32768", "ldc2_w"}, {"lm1", "ldc2_w"},
		{"lm32768", "ldc2_w"}, {"lm32769", "ldc2_w"}, {"lmax", "ldc2_w"},
		{"lgrande", "ldc2_w"},
		{"i0", "lconst_0"}, {"i1", "lconst_1"}, {"i5", "ldc2_w"},
		{"i127", "ldc2_w"}, {"i128", "ldc2_w"}, {"i500", "ldc2_w"},
		{"i32767", "ldc2_w"}, {"i32768", "ldc2_w"}, {"im1", "ldc2_w"},
		{"im32768", "ldc2_w"}, {"im32769", "ldc2_w"}, {"imax", "ldc2_w"},
		{"c0", "lconst_0"}, {"c1", "lconst_1"}, {"c500", "ldc2_w"},
		{"c32768", "ldc2_w"}, {"cneg", "ldc2_w"}, {"csuma", "ldc2_w"},
	} {
		codigo := codigoDeMetodoEmitido(t, clase, metodos[0], "()J")
		instrucciones, err := walkJvmCode(codigo.Code)
		if err != nil {
			t.Fatalf("%s no se desarma: %v", metodos[0], err)
		}
		carga, siguiente := instrucciones[0], instrucciones[1]
		if carga.name != metodos[1] {
			t.Errorf("%s carga con %s y javac con %s: %v", metodos[0], carga.name, metodos[1], instrucciones)
		}
		if siguiente.name != "lreturn" {
			t.Errorf("%s: detrás de la carga va %s y javac escribe lreturn: %v", metodos[0], siguiente.name, instrucciones)
		}
		for _, in := range instrucciones {
			if in.name == "i2l" {
				t.Errorf("%s escribe i2l y javac dobla la constante en el hueco ancho: %v", metodos[0], instrucciones)
			}
		}
	}
}

// TestElRetardoConstanteLlevaLdc2w: la clase entera dexa como la de javac 17 y la llamada del campo
// lleva su `ldc2_w 500l` delante del `invokevirtual`, sin `sipush; i2l` y sin dejar el `pop` del
// `boolean` de vuelta. Es el `MainActivity$3` de ApkShare reducido a lo que se mide.
func TestElRetardoConstanteLlevaLdc2w(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	clase := claseEmitidaPorNombre(t, nuestras, "p/Retardo")
	for _, nombre := range []string{"porCampo", "porLiteral"} {
		instrucciones, err := walkJvmCode(codigoDeMetodoEmitido(t, clase, nombre, "(Landroid/os/Handler;Ljava/lang/Runnable;)V").Code)
		if err != nil {
			t.Fatalf("%s no se desarma: %v", nombre, err)
		}
		visto := false
		for i, in := range instrucciones {
			if in.name == "i2l" {
				t.Fatalf("%s escribe i2l donde javac dobla la constante: %v", nombre, instrucciones)
			}
			if in.name != "invokevirtual" {
				continue
			}
			if i == 0 || instrucciones[i-1].name != "ldc2_w" {
				t.Fatalf("%s: el postDelayed de la pc %d no lleva ldc2_w delante: %v", nombre, in.pc, instrucciones)
			}
			visto = true
		}
		if !visto {
			t.Fatalf("%s no llama a postDelayed: %v", nombre, instrucciones)
		}
	}
}

// TestFalsificacionDelPlegadoDelCampoSuelto: con el interruptor apagado vuelve `sipush 500; i2l` y el
// dex deja de ser el de javac; con él encendido, la divergencia desaparece. Sin esta comprobación el
// arreglo podría estar tapado por otra ruta que también doblara el campo.
func TestFalsificacionDelPlegadoDelCampoSuelto(t *testing.T) {
	_, conPlegado, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}})
	if !bytes.Equal(conPlegado, suyo) {
		t.Fatalf("con el plegado encendido el dex ya no es el de javac: %d frente a %d bytes", len(conPlegado), len(suyo))
	}

	anterior := pliegaCampoSuelto
	pliegaCampoSuelto = false
	t.Cleanup(func() { pliegaCampoSuelto = anterior })

	sinPlegado, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Retardo.java", contenido: fuenteRetardoConstante}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("con el plegado apagado el emisor rechazó la sonda: %v", err)
	}
	nuestraSin, err := dexDeClases([]classFull{leidaDeEmitida(t, claseEmitidaPorNombre(t, sinPlegado, "p/Retardo"))})
	if err != nil {
		t.Fatalf("dexando la sonda sin plegado: %v", err)
	}
	if bytes.Equal(nuestraSin, conPlegado) {
		t.Fatalf("con el plegado apagado el dex no cambia: la falsificación no falsifica")
	}
	instrucciones, err := walkJvmCode(codigoDeMetodoEmitido(t, claseEmitidaPorNombre(t, sinPlegado, "p/Retardo"), "porCampo", "(Landroid/os/Handler;Ljava/lang/Runnable;)V").Code)
	if err != nil {
		t.Fatalf("la sonda sin plegado no se desarma: %v", err)
	}
	var nombres []string
	for _, in := range instrucciones {
		nombres = append(nombres, in.name)
	}
	if !strings.Contains(strings.Join(nombres, " "), "sipush i2l") {
		t.Fatalf("con el plegado apagado no aparece `sipush i2l`: %v", instrucciones)
	}
	// y el control de arriba: el dex de las dos versiones medidas difiere, que es la divergencia que
	// el arreglo elimina
	if len(nuestraSin) == len(suyo) && bytes.Equal(nuestraSin, suyo) {
		t.Fatalf("la sonda sin plegado dexa igual que javac: no hay divergencia que medir")
	}
}
