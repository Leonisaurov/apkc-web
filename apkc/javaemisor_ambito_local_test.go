package main

import (
	"bytes"
	"testing"
)

// Este fichero cierra el reparto de huecos de las locales, que era la última frontera conocida del dex
// de las seis clases de ApkShare. La causa medida: `reserva` era monótona y solo se rebobinaba en
// `try`/`catch`, así que una local declarada dentro de un bloque seguía ocupando su hueco después de
// que el bloque cerrara. javac hace lo contrario con `endScopes`: el registro vuelve al valor con el
// que se abrió el bloque, y el siguiente local del método vuelve a caer en ese hueco. Se ve en
// `Apk.<init>` del zip: `ApplicationInfo appInfo` (`Apk.java:19`) se declara dentro del `if` de la
// línea 18 y `String name` (`Apk.java:27`) ocupa su hueco al cerrar ese bloque, así que javac escribe
// `astore 4` y `locals=5` donde nosotros escribíamos `astore 5` y `locals=6`.

// fuenteHuecosHermanos: el control del reparto. Cuatro cosas en tres métodos, que es lo que puede
// romperse al devolver huecos: dos locales con el MISMO nombre en ámbitos hermanos (`valor` en el `if`
// y en el `else` de `ramas`, y en los dos bloques seguidos de `hermanos`), un local exterior vivo
// dentro de un bloque interno (`total` dentro de `{ }`, que tiene que conservar su registro) y un
// `long` de dos huecos que, al liberarse, deja su primer hueco a un `int` (`istore 4` sobre la mitad
// de `grande`). Con javac 17 `--release 8 -g:lines,source`: `hermanos` y `ramas` salen con `locals=4`
// y `ancho` con `locals=6` —el `long` de dentro sigue contando los dos huecos en el alto del método
// aunque el `int` de después reutilice el primero—.
//
// Los dos `if` acaban el cuerpo de la rama en `return` a propósito: un `if` cuya rama cae deja en
// javac la fila de la llave en el `goto` de salida, y eso es otra frontera (la de `marcaLinea`) que
// aquí solo taparía la medida del reparto.
const fuenteHuecosHermanos = `package p;

class Huecos {
    int hermanos(int n) {
        int total = n;
        if (n > 0) {
            int valor = n + 1;
            total = valor;
            return total;
        }
        {
            int valor = n - 1;
            total = total + valor;
        }
        {
            int dentro = total + 1;
            total = total + dentro;
        }
        int valor = total * 2;
        return valor;
    }

    int ramas(int n) {
        int total = n;
        if (n > 0) {
            int valor = n + 1;
            return valor;
        } else {
            int valor = n - 1;
            return valor - total;
        }
    }

    long ancho(int n) {
        long suma = n;
        if (n > 0) {
            long grande = suma + 1L;
            suma = grande;
            return suma;
        }
        int pequeno = n * 2;
        return suma + pequeno;
    }
}
`

// compilaYDejaElDexDeJavac devuelve, para los mismos fuentes, nuestro dex y el de javac 17 con
// `--release 8 -g:lines,source`. Es el oráculo de todas las pruebas de este fichero.
func compilaYDejaElDexDeJavac(t *testing.T, fuentes []fuenteJava) (nuestras []claseEmitida, nuestro, suyo []byte) {
	t.Helper()
	jar := jarDelSDK(t)
	javac := javac17(t)
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	return nuestras, dexDeNuestrasClases(t, nuestras), dexDeLasDeJavac(t, deJavac)
}

// TestElDexDeLasSeisClasesDeApkShareEsElDeJavac17: el cierre de la frontera. Con el hueco de `appInfo`
// devuelto al cerrar el `if`, el dex de las seis clases del proyecto real (los cuatro fuentes de
// `fuentesAppAdapterShare`) es byte a byte el de javac 17 `--release 8 -g:lines,source`: mismo
// `locals`, mismos `registers` y las mismas instrucciones, ya sin la diferencia del byte 9 que abrió
// este frente (5240 bytes nuestros frente a 5244 de javac, `registers 13` frente a 9).
func TestElDexDeLasSeisClasesDeApkShareEsElDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, fuentesAppAdapterShare())
	if len(nuestras) != 6 {
		t.Fatalf("el proyecto de ApkShare debería dejar seis clases y dejó %d", len(nuestras))
	}
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))

	// el reparto que abrió el frente, leído en el `.class` y no solo en el dex: javac declara cinco
	// huecos en `Apk.<init>` porque `name` reutiliza el de `appInfo`
	const desc = "(Landroid/content/pm/PackageManager;Ljava/lang/String;)V"
	nuestroInit := metodoDeClaseEmitida(t, nuestras, "share/apps/util/Apk", "<init>", desc)
	if nuestroInit.LocalCount != 5 {
		t.Fatalf("`Apk.<init>` declara %d huecos y javac declara 5\n%s", nuestroInit.LocalCount, volcadoDeClases(nuestras))
	}
	t.Logf("dex de las seis clases byte a byte el de javac: %d bytes, `Apk.<init>` con locals=%d", len(nuestro), nuestroInit.LocalCount)
}

// TestElRepartoDeHuecosDeLosBloquesEsElDeJavac17: el control del reparto, con el dex byte a byte. Si
// la devolución del hueco se comiera un registro vivo (el `total` exterior dentro del bloque interno)
// o no reutilizara el de los hermanos (`valor`), el dex dejaría de ser el de javac; si un `long` de
// dos huecos devolviera solo uno, `ancho` saldría con un local de más.
func TestElRepartoDeHuecosDeLosBloquesEsElDeJavac17(t *testing.T) {
	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Huecos.java", contenido: fuenteHuecosHermanos}})

	hermanos := metodoDeClaseEmitida(t, nuestras, "p/Huecos", "hermanos", "")
	ramas := metodoDeClaseEmitida(t, nuestras, "p/Huecos", "ramas", "")
	ancho := metodoDeClaseEmitida(t, nuestras, "p/Huecos", "ancho", "")
	if hermanos.LocalCount != 4 {
		t.Fatalf("`hermanos` declara %d huecos y javac declara 4 (this, n, total y el `valor`/`dentro` reutilizado)", hermanos.LocalCount)
	}
	if ramas.LocalCount != 4 {
		t.Fatalf("`ramas` declara %d huecos y javac declara 4 (el `valor` de los dos brazos en el mismo hueco)", ramas.LocalCount)
	}
	if ancho.LocalCount != 6 {
		t.Fatalf("`ancho` declara %d huecos y javac declara 6 (this, n, el `long` de dos huecos y su primer hueco reutilizado por el `int`)", ancho.LocalCount)
	}
	comparaDexByteAByte(t, nuestro, suyo, volcadoDeClases(nuestras))
}

// TestFalsificacionDelHuecoReutilizado: con el interruptor apagado, `reserva` vuelve a ser monótona y
// las dos caras de la frontera se ven: el dex de las seis clases deja de ser el de javac y `Apk.<init>`
// vuelve a declarar seis huecos, que es exactamente lo que midió el frente anterior.
func TestFalsificacionDelHuecoReutilizado(t *testing.T) {
	anterior := reutilizaHuecos
	reutilizaHuecos = false
	t.Cleanup(func() { reutilizaHuecos = anterior })

	nuestras, nuestro, suyo := compilaYDejaElDexDeJavac(t, fuentesAppAdapterShare())
	if bytes.Equal(nuestro, suyo) {
		t.Fatalf("con los huecos sin devolver el dex de las seis clases sigue siendo el de javac")
	}
	primero := -1
	for i := 0; i < len(nuestro) && i < len(suyo); i++ {
		if nuestro[i] != suyo[i] {
			primero = i
			break
		}
	}
	const desc = "(Landroid/content/pm/PackageManager;Ljava/lang/String;)V"
	nuestroInit := metodoDeClaseEmitida(t, nuestras, "share/apps/util/Apk", "<init>", desc)
	if nuestroInit.LocalCount != 6 {
		t.Fatalf("con los huecos sin devolver `Apk.<init>` declara %d huecos: el interruptor no apaga la ruta", nuestroInit.LocalCount)
	}
	t.Logf("sin devolver los huecos: nuestro dex %d bytes, javac %d, primer byte distinto en %d, y `Apk.<init>` con locals=%d frente a 5",
		len(nuestro), len(suyo), primero, nuestroInit.LocalCount)

	// y el control de los hermanos también pierde el reparto: los dos `valor` y el `dentro` dejan de
	// compartir hueco y `hermanos` sube de cuatro a siete
	otras, _, _ := compilaYDejaElDexDeJavac(t, []fuenteJava{{nombre: "src/p/Huecos.java", contenido: fuenteHuecosHermanos}})
	hermanos := metodoDeClaseEmitida(t, otras, "p/Huecos", "hermanos", "")
	if hermanos.LocalCount != 7 {
		t.Fatalf("con los huecos sin devolver `hermanos` declara %d huecos y lo medido es 7", hermanos.LocalCount)
	}
	t.Logf("sin devolver los huecos: `Huecos.hermanos` con locals=%d frente a 4", hermanos.LocalCount)
}
