package main

// El reparto de huecos de un `try`/`catch` y el rango protegido, contra javac 17.0.21
// `--release 8 -g:lines,source` (la versión que fija la CI, §152) y el dex byte a byte.
//
// La frontera la abrió `MainActivity$6` de ApkShare: nosotros emitíamos `astore 4` donde javac
// reutiliza `astore_3`, el hueco del `InputStream` del `try`, para el `Uri` que se declara justo
// después del `catch`. La causa es que `intenta` no abría ámbito: lo que declaraba el cuerpo y la
// excepción de cada `catch` seguían ocupando su hueco al cerrarse el constructo. Y el rango
// protegido salía partido en la vuelta del bucle del propio cuerpo (`59 107 122` + `110 119 122`
// frente a `59 119 122`).
//
// La regla medida, sonda de este fichero:
//
//   - al cerrarse el `try`, sus huecos vuelven: `File otra` de después del `catch` cae en el hueco
//     del `InputStream` (`astore_2`) y `max_locals` no sube;
//   - la vuelta de un bucle del cuerpo NO parte el rango protegido (`goto` a un destino dentro del
//     rango); una salida del `try` sí (`return`, `break`/`continue` que saltan fuera). Medido con
//     javac 17 en este frente sobre cuatro formas: la vuelta de un bucle que es la primera sentencia
//     del `try`, un `break` de un bucle interior (sin cortar), un `break` que sale del `try` y un
//     `continue` de un bucle que lo envuelve (cortando en los dos últimos).
//   - dos locales vivos a la vez no comparten hueco: el `total` de `vivos` sigue en el suyo dentro
//     del `try`, y el `despues` de después reutiliza el que dejó el `catch` (`locals=3` en javac).

import (
	"bytes"
	"testing"
)

// fuenteSondaAstoreTry es el mínimo equivalente a `MainActivity$6`: el mismo `try` con
// `InputStream`/`OutputStream`, el búfer de 1024, el bucle de lectura, los dos `close()` dentro del
// cuerpo y el `printStackTrace` del `catch`; después, una local nueva (`File otra`) que es la que
// mide la reutilización. `vivos` es el control de los dos locales simultáneos y del `max_locals`.
const fuenteSondaAstoreTry = `package p;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;

public class SondaTry {
    static void copia(File src, File dst) {
        try {
            InputStream in = new FileInputStream(src);
            OutputStream out = new FileOutputStream(dst);
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
        File otra = new File("x");
        System.out.println(otra.getName());
    }

    static void vivos(int n) throws IOException {
        int total = n;
        try {
            int dentro = n + 1;
            total = dentro;
            throw new IOException("x");
        } catch (IOException e) {
            total = 0;
        }
        int despues = total + 1;
        System.out.println(despues);
    }
}
`

// nombresDeCodigo: el nombre de cada instrucción de un `Code`, que es la forma de comparar el
// reparto de huecos y las marcas de línea sin depender de los índices del pool (entre nuestro
// `.class` y el de javac el pool está ordenado distinto).
func nombresDeCodigo(t *testing.T, code []byte) []string {
	t.Helper()
	instrs, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el cuerpo no se desarma: %v", err)
	}
	nombres := make([]string, 0, len(instrs))
	for _, in := range instrs {
		nombres = append(nombres, in.name)
	}
	return nombres
}

// cuentaNombre: cuántas veces aparece una instrucción en el cuerpo.
func cuentaNombre(nombres []string, buscado string) int {
	n := 0
	for _, nombre := range nombres {
		if nombre == buscado {
			n++
		}
	}
	return n
}

// TestElTrySeDexaComoJavac17: el dex del proyecto de la sonda byte a byte, y además el reparto
// leído en nuestro `.class`: `copia` declara seis huecos y `vivos` tres, el `File otra` de después
// del `catch` se guarda con `astore_2` (el hueco del `InputStream`), y el rango protegido de
// `copia` es UNA fila que cubre los dos `close()`, la misma que la de javac.
func TestElTrySeDexaComoJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/SondaTry.java", contenido: fuenteSondaAstoreTry}})
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	copia := metodoDeClaseEmitida(t, nuestras, "p/SondaTry", "copia", "(Ljava/io/File;Ljava/io/File;)V")
	vivos := metodoDeClaseEmitida(t, nuestras, "p/SondaTry", "vivos", "(I)V")
	if copia.LocalCount != 6 {
		t.Fatalf("`copia` declara %d huecos y javac declara 6 (el `File otra` reutiliza el del `InputStream`)\n%s",
			copia.LocalCount, volcadoDeClases(nuestras))
	}
	if vivos.LocalCount != 3 {
		t.Fatalf("`vivos` declara %d huecos y javac declara 3 (`despues` reutiliza el del `catch`)\n%s",
			vivos.LocalCount, volcadoDeClases(nuestras))
	}

	// el reparto, instrucción a instrucción, contra el de javac: si el `File otra` se guardara en el
	// hueco del `OutputStream`, la secuencia dejaría de coincidir en el `astore`
	nombres := nombresDeCodigo(t, copia.Code)
	if n := cuentaNombre(nombres, "astore_2"); n != 3 {
		t.Fatalf("`copia` guarda en el hueco 2 %d veces y javac 3 (el `in`, la excepción y el `File otra`): %v", n, nombres)
	}
	if n := cuentaNombre(nombres, "astore_3"); n != 1 {
		t.Fatalf("`copia` guarda en el hueco 3 %d veces y javac 1 (el `out`): %v", n, nombres)
	}

	// el rango protegido: una sola fila, la de javac, sin la partición de la vuelta del bucle
	if len(copia.Tries) != 1 {
		t.Fatalf("`copia` lleva %d filas de excepción y javac 1: %+v", len(copia.Tries), copia.Tries)
	}
	deJavac := metodoDeClaseLeida(t, compilaConJavacSondaTry(t), "Lp/SondaTry;", "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if copia.Tries[0] != deJavac.Tries[0] {
		t.Fatalf("la fila del `try` es %+v y la de javac %+v", copia.Tries[0], deJavac.Tries[0])
	}
	// la fila cubre los dos `close()`: no acaba en la vuelta del bucle
	closes := 0
	for _, in := range mustWalk(t, copia.Code) {
		if in.pc < copia.Tries[0].start || in.pc >= copia.Tries[0].end {
			continue
		}
		if in.name == "invokevirtual" || in.name == "invokeinterface" {
			closes++
		}
	}
	if closes < 4 {
		t.Fatalf("el rango %+v protege %d llamadas y tiene que proteger también los dos `close()`", copia.Tries[0], closes)
	}
}

// TestFalsificacionDelHuecoDelTry: con `reutilizaHuecos` apagado vuelve la asignación vieja —
// el `File otra` en el hueco 3, `vivos` con un local de más y el dex distinto del de javac—, y con
// `cortaRangoEnTodoGoto` encendido vuelve el rango partido en la vuelta del bucle.
func TestFalsificacionDelHuecoDelTry(t *testing.T) {
	anterior := reutilizaHuecos
	reutilizaHuecos = false
	t.Cleanup(func() { reutilizaHuecos = anterior })

	nuestras, conAtraso, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/SondaTry.java", contenido: fuenteSondaAstoreTry}})
	if bytes.Equal(conAtraso, suyo) {
		t.Fatalf("con el reparto de antes el dex de la sonda sigue siendo el de javac")
	}
	copia := metodoDeClaseEmitida(t, nuestras, "p/SondaTry", "copia", "(Ljava/io/File;Ljava/io/File;)V")
	vivos := metodoDeClaseEmitida(t, nuestras, "p/SondaTry", "vivos", "(I)V")
	nombres := nombresDeCodigo(t, copia.Code)
	if cuentaNombre(nombres, "astore_3") != 2 {
		t.Fatalf("con el reparto de antes el `File otra` no se guarda en el hueco 3: %v", nombres)
	}
	if vivos.LocalCount != 4 {
		t.Fatalf("con el reparto de antes `vivos` declara %d huecos y lo medido es 4", vivos.LocalCount)
	}
	t.Logf("con el reparto de antes: `copia` con %d huecos y `vivos` con %d, y el dex mide %d frente a %d de javac",
		copia.LocalCount, vivos.LocalCount, len(conAtraso), len(suyo))

	// el rango: con el corte en cualquier `goto` vuelve la partición del bucle, que es la que se vio
	// en `MainActivity$6` (`59 107 122` + `110 119 122`)
	anteriorRango := cortaRangoEnTodoGoto
	cortaRangoEnTodoGoto = true
	t.Cleanup(func() { cortaRangoEnTodoGoto = anteriorRango })
	sinFusionar, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/SondaTry.java", contenido: fuenteSondaAstoreTry}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("con el corte en cada `goto` el emisor rechazó la sonda: %v", err)
	}
	partida := metodoDeClaseEmitida(t, sinFusionar, "p/SondaTry", "copia", "(Ljava/io/File;Ljava/io/File;)V")
	if len(partida.Tries) != 2 {
		t.Fatalf("con el corte en cada `goto` el rango no se parte en dos filas: %+v", partida.Tries)
	}
	t.Logf("con el corte en cada `goto`: dos filas %+v frente a la única de javac", partida.Tries)
}

// compilaConJavacSondaTry: las clases de javac 17 de la sonda, leídas, para comparar la tabla de
// excepciones con la nuestra.
func compilaConJavacSondaTry(t *testing.T) []classFull {
	t.Helper()
	raiz := t.TempDir()
	fuentes := []fuenteJava{{nombre: "src/p/SondaTry.java", contenido: fuenteSondaAstoreTry}}
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	return compilaConJavac(t, javac17(t), jarDelSDK(t), raiz, fuentes)
}

func mustWalk(t *testing.T, code []byte) []jvmInstr {
	t.Helper()
	instrs, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el cuerpo no se desarma: %v", err)
	}
	return instrs
}
