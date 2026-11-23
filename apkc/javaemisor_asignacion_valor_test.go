package main

// La asignación como expresión. El que abrió este frente fue la línea 187 de `MainActivity.java` del
// zip de ApkShare, con el error literal:
//
//	src/share/apps/util/MainActivity.java: linea 187: se esperaba ")" y hay "="
//
// La línea es `while ((length = in.read(buffer)) > 0) {`: el `=` dentro de los paréntesis es una
// asignación como expresión (JLS 15.26.1), el operador de menor precedencia y asociativo a la
// derecha, y el árbol que lee javac es `(length = in.read(buffer)) > 0`, con el valor asignado como
// operando izquierdo de la comparación. El emisor tiene que dejar ese valor: `dup` antes del
// `istore` en un local de una palabra (`dup2` en un ancho).
//
// El oráculo es javac 17 `--release 8 -g:lines,source` con el bloque verbatim del zip: métodos,
// `max_stack`/`locals`, la LineNumberTable y el dex entero byte a byte. El dex de la copia con el
// `try`/`catch` no sale byte a byte por una diferencia que ya estaba —12 bytes en el primer ítem de
// anotaciones de la sección de datos—, y por eso hay dos casos: el mínimo (dex idéntico) y el bloque
// entero (instrucciones comparadas opcode a opcode).

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteCopiaMinima: el bucle de la línea 187 del zip palabra por palabra, sin el `try`/`catch` que
// lo rodea, que no toca el bucle y que en el zip solo cierra los flujos. Sirve para el oráculo
// estricto: el dex entero byte a byte.
const fuenteCopiaMinima = `package share.apps.util;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;

public class Copia {
    public int copia(InputStream in, OutputStream out, byte[] buffer) throws IOException {
        int length;
        while ((length = in.read(buffer)) > 0) {
            out.write(buffer, 0, length);
        }
        return length;
    }
}
`

// fuenteCopiaBloque: las líneas 181-194 de `MainActivity.java` del zip verbatim —el `try`, los dos
// flujos, el bucle y el `catch`— dentro de un método con dos `File` de parámetro, porque el zip los
// saca de `apk.filePath` y de `getApplicationContext()`, que no hacen falta para medir el bucle. La
// línea del `while` es la del zip, carácter por carácter.
const fuenteCopiaBloque = `package share.apps.util;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;

public class Copia {
    public void copia(File apkFile, File toApkFile) {
        try {
            InputStream in = new FileInputStream(apkFile);
            OutputStream out = new FileOutputStream(toApkFile);

            byte[] buffer = new byte[1024];
            int length;
            while ((length = in.read(buffer)) > 0) {
                out.write(buffer, 0, length);
            }
            in.close();
            out.close();
        } catch (IOException e) {
            e.printStackTrace();
        }
    }
}
`

var fuentesCopiaMinima = []fuenteJava{
	{nombre: "src/share/apps/util/Copia.java", contenido: fuenteCopiaMinima},
}

var fuentesCopiaBloque = []fuenteJava{
	{nombre: "src/share/apps/util/Copia.java", contenido: fuenteCopiaBloque},
}

// TestAsignacionComoValorContraJavac17: el bucle de la línea 187, con el `dup` del valor asignado y
// la comparación encima. Si el `dup` falta, el `istore` se lleva el único valor y el `if_icmple` se
// queda sin operando: el verificador lo diría, pero el dex de javac lo dice antes.
func TestAsignacionComoValorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	for _, f := range fuentesCopiaMinima {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentesCopiaMinima, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la asignación como valor de la línea 187: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesCopiaMinima)
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/Copia;", deJavac)

	comparaMetodos(t, nuestra, suya)

	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestCopiaDelBloqueDeLaLinea187: el bloque entero del zip, con el `try`/`catch`. Aquí el dex no sale
// byte a byte por una diferencia anterior a este frente —la sección de datos mide 12 bytes más en
// nuestro lado, en el primer ítem de anotaciones—, así que se compara lo que sí es de la línea: las
// instrucciones, una a una, aceptando solo que los índices del pool no coincidan (son índices, no
// opcodes). Un control sin asignación en la condición mide los mismos 12 bytes de diferencia.
func TestCopiaDelBloqueDeLaLinea187(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	for _, f := range fuentesCopiaBloque {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}

	nuestras, err := compilarProyectoJava(fuentesCopiaBloque, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el bloque de la línea 187: %v", err)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentesCopiaBloque)
	nuestra, suya := buscaLaClase(t, nuestras, "Lshare/apps/util/Copia;", deJavac)

	comparaMetodos(t, nuestra, suya)

	nuestroMetodo, ok := buscaMetodoDesc(nuestra, "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if !ok {
		t.Fatalf("no salió copia(File,File)V: %s", listaDeMetodos(nuestra))
	}
	suMetodo, ok := buscaMetodoDesc(suya, "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if !ok {
		t.Fatal("javac no dejó copia(File,File)V")
	}
	if err := comparaInstrucciones(nuestroMetodo.Code, suMetodo.Code); err != nil {
		t.Fatal(err)
	}
}

// compilaConJavac corre javac 17 con el `-g` que se usa de oráculo y devuelve las clases leídas.
func compilaConJavac(t *testing.T, javac, jar, raiz string, fuentes []fuenteJava) []classFull {
	t.Helper()
	salida := filepath.Join(raiz, "javac-out")
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida}
	for _, f := range fuentes {
		args = append(args, filepath.Join(raiz, f.nombre[len("src/"):]))
	}
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	return deJavac
}

// opcodesConPool: los que llevan un índice del pool en su operando. Entre nuestro `.class` y el de
// javac el pool está ordenado distinto, así que ese índice no se compara; el opcode sí.
var opcodesConPool = map[byte]bool{
	0x12: true, 0x13: true, 0x14: true, // ldc, ldc_w, ldc2_w
	0xb2: true, 0xb3: true, 0xb4: true, 0xb5: true, // getstatic, putstatic, getfield, putfield
	0xb6: true, 0xb7: true, 0xb8: true, 0xb9: true, 0xba: true, // invokevirtual, invokespecial, invokestatic, invokeinterface, invokedynamic
	0xbb: true, 0xbd: true, 0xc0: true, 0xc1: true, 0xc5: true, // new, anewarray, checkcast, instanceof, multianewarray
}

// comparaInstrucciones: las dos `Code` desarmadas con `walkJvmCode`, el mismo número de
// instrucciones, los mismos opcodes y los mismos operandos salvo los índices del pool. Es lo que
// queda de la línea 187 cuando el dex entero no se puede comparar.
func comparaInstrucciones(nuestro, suyo []byte) error {
	nuestras, err := walkJvmCode(nuestro)
	if err != nil {
		return err
	}
	suyas, err := walkJvmCode(suyo)
	if err != nil {
		return err
	}
	if len(nuestras) != len(suyas) {
		return &instruccionDistinta{detalle: "nuestro método tiene " + itoa(len(nuestras)) + " instrucciones y javac " + itoa(len(suyas))}
	}
	for k := range suyas {
		n, s := nuestras[k], suyas[k]
		if n.op != s.op {
			return &instruccionDistinta{detalle: "en " + itoa(k) + " (pc " + itoa(n.pc) + "): nuestro opcode es " + itoa(int(n.op)) + " y javac " + itoa(int(s.op))}
		}
		if opcodesConPool[n.op] {
			continue
		}
		if len(n.ops) != len(s.ops) {
			return &instruccionDistinta{detalle: "en " + itoa(k) + " (pc " + itoa(n.pc) + "): operandos de distinto largo"}
		}
		for j := range s.ops {
			if n.ops[j] != s.ops[j] {
				return &instruccionDistinta{detalle: "en " + itoa(k) + " (pc " + itoa(n.pc) + ", opcode " + itoa(int(n.op)) + "): el operando " + itoa(j) + " es " + itoa(int(n.ops[j])) + " y javac escribe " + itoa(int(s.ops[j]))}
			}
		}
	}
	return nil
}

type instruccionDistinta struct{ detalle string }

func (e *instruccionDistinta) Error() string { return "las instrucciones no coinciden: " + e.detalle }

// TestAsignacionComoValorSoloLocal: en la pila el destino de una asignación como valor tiene que ser
// una variable local; el campo y el elemento de arreglo piden `dup_x1`/`dup_x2` con el receptor
// debajo y no están medidos. Sin esta guarda, el destino se emitiría como si fuera una variable local
// y el `istore` iría al hueco de otro.
func TestAsignacionComoValorSoloLocal(t *testing.T) {
	fuente := `package p;

class A {
    int campo;

    int f() {
        return 1;
    }

    void g() {
        while ((campo = f()) > 0) {
            campo = campo - 1;
        }
    }
}
`
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "A.java", contenido: fuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("el emisor aceptó una asignación como valor sobre un campo")
	}
	if !strings.Contains(err.Error(), "variable local") {
		t.Fatalf("el rechazo no dice que solo está medida la variable local: %v", err)
	}
}

// nombresDeMetodos: la lista de métodos de una clase, para los fallos.
func listaDeMetodos(cf classFull) string {
	out := make([]string, 0, len(cf.Methods))
	for _, m := range cf.Methods {
		out = append(out, m.Name+m.Desc)
	}
	return strings.Join(out, " ")
}
