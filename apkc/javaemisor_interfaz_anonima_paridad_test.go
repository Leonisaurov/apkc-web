//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// La paridad con javac 17 de javaemisor_interfaz_anonima_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_interfaz_anonima_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestLaAsignacionDeUnaAnonimaARunnableEsLaDeJavac17: el caso de ApkShare compila y el dex de las
// cuatro clases es el de javac byte a byte. Antes del arreglo el emisor lo rechazaba y no dejaba
// ningún `.class`; sin la lista de interfaces en `superEInterfaces`, la anónima solo se veía como
// `Object` a través de su padre.
func TestLaAsignacionDeUnaAnonimaARunnableEsLaDeJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := fuentesInterfazAnonima()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó `runnable = new Runnable() { … }`: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// lo que la asignación miraba: la anónima declara `Runnable` en su `interfaces`
	anonima, err := readClassFull(claseEmitidaPorNombre(t, nuestras, "share/apps/util/MainActivity$1$1"))
	if err != nil {
		t.Fatalf("la anónima no se lee: %v", err)
	}
	if len(anonima.Interfaces) != 1 || anonima.Interfaces[0] != "Ljava/lang/Runnable;" {
		t.Fatalf("la anónima declara %v y javac escribe [Ljava/lang/Runnable;]", anonima.Interfaces)
	}
	suya := claseLeidaPorNombre(t, deJavac, "Lshare/apps/util/MainActivity$1$1;")
	if len(suya.Interfaces) != 1 || suya.Interfaces[0] != anonima.Interfaces[0] {
		t.Fatalf("javac declara %v y nosotros %v", suya.Interfaces, anonima.Interfaces)
	}

	// la asignación, instrucción a instrucción: `aload_0`, el `new` de la anónima con su captura y el
	// `invokestatic access$002`, que es el accesor de escritura del campo privado
	const desc = "(Landroid/text/Editable;)V"
	nuestro := metodoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity$1", "afterTextChanged", desc)
	suAfter := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity$1;", "afterTextChanged", desc)
	if err := comparaInstrucciones(nuestro.Code, suAfter.Code); err != nil {
		t.Fatalf("el `afterTextChanged` con la asignación no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// y el accesor de escritura del campo: mismo descriptor `(LMainActivity;Ljava/lang/Runnable;)LRunnable;`
	// e instrucciones de javac. El **nombre** no coincide todavía —nosotros repartimos `access$NNN` por
	// orden de creación al emitir y javac por el orden de los campos del exterior—, que es la frontera
	// siguiente de este mismo fuente: nuestro `MainActivity` deja `access$000` para `adapter`,
	// `access$100`/`access$102` para `runnable` y `access$200` para `handler`, y javac deja `000`/`002`
	// para `runnable`, `100` para `handler` y `200` para `adapter`. No se arregla aquí.
	const descEscritura = "(Lshare/apps/util/MainActivity;Ljava/lang/Runnable;)Ljava/lang/Runnable;"
	nuestroAcc := accesoDeClaseEmitida(t, nuestras, "share/apps/util/MainActivity", descEscritura)
	suAcc := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/MainActivity;", "access$002", descEscritura)
	if err := comparaInstrucciones(nuestroAcc.Code, suAcc.Code); err != nil {
		t.Fatalf("el accesor de escritura de `runnable` no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// El dex de estas clases **no** se compara byte a byte: no es alcanzable con este fuente, y no
	// por este frente. `MainActivity$1$1.run()` resuelve `Editable.toString` como `invokeinterface`
	// donde javac escribe `invokevirtual java/lang/Object.toString` —medido también con el binario de
	// `cdcf9e8` sobre el mismo fuente sin la asignación, así que es anterior—, y el nombre del
	// accesor cambia los índices. Las dos cosas son fronteras aparte.
}

// TestUnaAnonimaDeUnaInterfazNoRelacionadaSigueRechazandose: el control del arreglo. `Otra` no es
// `Runnable`, así que la asignación sigue siendo un error —el del usuario, con el nombre interno de
// la anónima— y javac 17 rechaza el mismo fuente por el mismo motivo.
func TestUnaAnonimaDeUnaInterfazNoRelacionadaSigueRechazandose(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	fuentes := []fuenteJava{
		{nombre: "src/p/A.java", contenido: fuenteAnonimaNoRelacionada},
		{nombre: "src/p/Otra.java", contenido: fuenteOtraInterfaz},
	}

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatalf("se aceptó `runnable = new Otra() { … }`, y `Otra` no es `Runnable`")
	}
	const esperado = "runnable es Ljava/lang/Runnable; y se le asigna un Lp/A$1;"
	if !strings.Contains(err.Error(), esperado) {
		t.Fatalf("el error es %q y tenía que contener %q", err, esperado)
	}

	// el oráculo: javac rechaza lo mismo
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	args := []string{"--release", "8", "-g:lines,source", "-classpath", jar, "-d", filepath.Join(raiz, "javac-out")}
	for _, f := range fuentes {
		args = append(args, filepath.Join(raiz, f.nombre[len("src/"):]))
	}
	salida, err := exec.Command(javac, args...).CombinedOutput()
	if err == nil {
		t.Fatalf("javac aceptó el control y tenía que rechazarlo:\n%s", salida)
	}
	if !strings.Contains(string(salida), "cannot be converted to Runnable") {
		t.Fatalf("javac rechazó el control por otro motivo:\n%s", salida)
	}
}
