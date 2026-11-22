package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// tryResourcesFuente: la sonda de un recurso. El oráculo es javac 17 (`javac -g:lines,source`
// de /home/leonisauro/opt/jdk17), no la equivalencia con un try/finally escrito a mano: javac
// guarda la excepcion del cuerpo en un local, cierra el recurso dentro de un segundo rango
// protegido y, si `close()` tambien falla, le añade la segunda con `Throwable.addSuppressed`.
const tryResourcesFuente = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable c) throws Exception {
    try (Closeable r = c) { use(r); }
  }
}
`

// tryResourcesJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesJavac17 = "yv66vgAAAD0AIQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYLAA4ADwcAEAwAEQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUKABIAFQwAFgAXAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQANU3RhY2tNYXBUYWJsZQEACkV4Y2VwdGlvbnMHAB4BABNqYXZhL2xhbmcvRXhjZXB0aW9uAQAKU291cmNlRmlsZQEACFJlcy5qYXZhACEACAACAAAAAAADAAEABQAGAAEAGAAAAB0AAQABAAAABSq3AAGxAAAAAQAZAAAABgABAAAAAgAIAAsADAABABgAAAAZAAAAAQAAAAGxAAAAAQAZAAAABgABAAAAAwAIABoADAACABgAAACFAAIABAAAACoqTCu4AAcrxgAiK7kADQEApwAZTSvGABIruQANAQCnAAlOLC22ABQsv7EAAgACAAYAEwASABgAHgAhABIAAgAZAAAACgACAAAABQApAAYAGwAAACkABP8AEwACBwAOBwAOAAEHABL/AA0AAwcADgcADgcAEgABBwASBfkAAQAcAAAABAABAB0AAQAfAAAAAgAg"

// dexDeFuenteUnica compila el fuente con el emisor, dex el `.class` que sale y dex tambien el
// `.class` que javac dejó como oráculo, para compararlos byte a byte.
func dexDeFuenteUnica(t *testing.T, fuente, javac string) (nuestro, suyo []byte) {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("salieron %d clases: %s", len(salida), nombresDe(salida))
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := base64.StdEncoding.DecodeString(javac)
	if err != nil {
		t.Fatal(err)
	}
	suya, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	nuestro, err = dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	suyo, err = dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	return nuestro, suyo
}

// TestTryWithResourcesAddSuppressed: el dex del emisor es byte a byte el de javac 17, con dos
// try_item (el cuerpo y el cierre del camino excepcional) y una llamada a Throwable.addSuppressed.
func TestTryWithResourcesAddSuppressed(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesFuente, tryResourcesJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	td := newDexTables()
	cf, err := readClassFull(func() []byte {
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: tryResourcesFuente}}, jarDelSDK(t))
		if err != nil {
			t.Fatal(err)
		}
		return salida[0].datos
	}())
	if err != nil {
		t.Fatal(err)
	}
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatal(err)
	}
	td.orderTables()
	encontro := false
	for _, m := range td.methods {
		if m.owner == "Ljava/lang/Throwable;" && m.name == "addSuppressed" && m.desc == "(Ljava/lang/Throwable;)V" {
			encontro = true
		}
	}
	if !encontro {
		t.Error("no se internó Throwable.addSuppressed")
	}
}

// dos recursos: cada uno abre un nivel anidado, se cierran en orden inverso y cada manejador añade
// la excepción del cierre con `addSuppressed`. El oráculo es javac 17, igual que en el caso de uno
// solo: el dex del emisor tiene que salir byte a byte el suyo.
const tryResourcesDos = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a, Closeable b) throws Exception {
    try (Closeable x = a; Closeable y = b) { use(x); use(y); }
  }
}
`

// tryResourcesDosJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
// Medido con `javap -c -p -v`: las declaraciones en 0-3, el cuerpo protegido [4,12) -> 25, el cierre
// de `y` en [31,37) -> 40, el rango exterior [2,52) -> 65 y su cierre en [70,76) -> 79; el manejador
// exterior hace `astore_3`, el hueco que dejó `y`, y `locals=6`. La tabla de excepciones va de dentro
// hacia fuera.
const tryResourcesDosJavac17 = "yv66vgAAAD0AIgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYLAA4ADwcAEAwAEQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUKABIAFQwAFgAXAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQApKExqYXZhL2lvL0Nsb3NlYWJsZTtMamF2YS9pby9DbG9zZWFibGU7KVYBAA1TdGFja01hcFRhYmxlAQAKRXhjZXB0aW9ucwcAHwEAE2phdmEvbGFuZy9FeGNlcHRpb24BAApTb3VyY2VGaWxlAQAIUmVzLmphdmEAIQAIAAIAAAAAAAMAAQAFAAYAAQAYAAAAHQABAAEAAAAFKrcAAbEAAAABABkAAAAGAAEAAAACAAgACwAMAAEAGAAAABkAAAABAAAAAbEAAAABABkAAAAGAAEAAAADAAgAGgAbAAIAGAAAAO8AAgAGAAAAWipNK04suAAHLbgABy3GACctuQANAQCnAB46BC3GABUtuQANAQCnAAw6BRkEGQW2ABQZBL8sxgAkLLkADQEApwAbTizGABQsuQANAQCnAAs6BC0ZBLYAFC2/sQAEAAQADAAZABIAHwAlACgAEgACADQAQQASAEYATABPABIAAgAZAAAACgACAAAABQBZAAYAHAAAAFMACP8AGQAEBwAOBwAOBwAOBwAOAAEHABL/AA4ABQcADgcADgcADgcADgcAEgABBwASCPkAAkwHABL/AA0ABAcADgcADgcADgcAEgABBwASB/kAAQAdAAAABAABAB4AAQAgAAAAAgAh"

// cuerpo con un local propio: el manejador de la excepción reutiliza el hueco que dejó ese local.
// `javap -c -p -v` de javac 17 da decl `0:aload_0; 1:astore_1`, cuerpo `2:iconst_3; 3:istore_2; …`,
// cierre normal `15:aload_1; 16:ifnull 50; …; 25:goto 50`, manejador `28:astore_2` (el hueco de
// `local`), `42:astore_3` y `locals=4`. El marco del manejador solo lista los dos locales vivos al
// empezar el rango (`full_frame` con `[Closeable, Closeable]`), no el `int` que el cuerpo dejó.
const tryResourcesLocalPropio = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a) throws Exception {
    try (Closeable x = a) { int local = 3; use(x); System.out.println(local); }
  }
}
`

// tryResourcesLocalPropioJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesLocalPropioJavac17 = "yv66vgAAAD0ALQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYJAA4ADwcAEAwAEQASAQAQamF2YS9sYW5nL1N5c3RlbQEAA291dAEAFUxqYXZhL2lvL1ByaW50U3RyZWFtOwoAFAAVBwAWDAAXABgBABNqYXZhL2lvL1ByaW50U3RyZWFtAQAHcHJpbnRsbgEABChJKVYLABoAGwcAHAwAHQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAHwEAE2phdmEvbGFuZy9UaHJvd2FibGUKAB4AIQwAIgAjAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQANU3RhY2tNYXBUYWJsZQEACkV4Y2VwdGlvbnMHACoBABNqYXZhL2xhbmcvRXhjZXB0aW9uAQAKU291cmNlRmlsZQEACFJlcy5qYXZhACEACAACAAAAAAADAAEABQAGAAEAJAAAAB0AAQABAAAABSq3AAGxAAAAAQAlAAAABgABAAAAAgAIAAsADAABACQAAAAZAAAAAQAAAAGxAAAAAQAlAAAABgABAAAAAwAIACYADAACACQAAACOAAIABAAAADMqTAY9K7gAB7IADRy2ABMrxgAiK7kAGQEApwAZTSvGABIruQAZAQCnAAlOLC22ACAsv7EAAgACAA8AHAAeACEAJwAqAB4AAgAlAAAACgACAAAABQAyAAYAJwAAACkABP8AHAACBwAaBwAaAAEHAB7/AA0AAwcAGgcAGgcAHgABBwAeBfkAAQAoAAAABAABACkAAQArAAAAAgAs"

// cuerpo vacío: javac 17 no escribe tabla de excepciones ni `addSuppressed`; solo la declaración, la
// guarda `if (x != null) x.close();` y el `return` (`0:aload_0; 1:astore_1; 2:aload_1; 3:ifnull 12;
// 6:aload_1; 7:invokeinterface close; 12:return`, `locals=2`, filas en 0 y 12).
const tryResourcesCuerpoVacio = `import java.io.Closeable;
public class Res {
  static void t(Closeable a) throws Exception {
    try (Closeable x = a) { }
  }
}
`

// tryResourcesCuerpoVacioJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesCuerpoVacioJavac17 = "yv66vgAAAD0AGAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCwAIAAkHAAoMAAsABgEAEWphdmEvaW8vQ2xvc2VhYmxlAQAFY2xvc2UHAA0BAANSZXMBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABdAEAFihMamF2YS9pby9DbG9zZWFibGU7KVYBAA1TdGFja01hcFRhYmxlAQAKRXhjZXB0aW9ucwcAFQEAE2phdmEvbGFuZy9FeGNlcHRpb24BAApTb3VyY2VGaWxlAQAIUmVzLmphdmEAIQAMAAIAAAAAAAIAAQAFAAYAAQAOAAAAHQABAAEAAAAFKrcAAbEAAAABAA8AAAAGAAEAAAACAAgAEAARAAIADgAAADIAAQACAAAADSpMK8YACSu5AAcBALEAAAACAA8AAAAKAAIAAAAEAAwABQASAAAAAwABDAATAAAABAABABQAAQAWAAAAAgAX"

// dos recursos con cuerpo vacío: el cierre de `y` va sin rango ni manejador, y el de `x` conserva el
// suyo (`javap -c -p -v`: `5:ifnull 20` para `y`, rango [2,14) → 27 y cierre protegido [32,38) → 41
// para `x`), con el hueco de `y` reutilizado por la excepción primaria (`astore_3`).
const tryResourcesDosVacio = `import java.io.Closeable;
public class Res {
  static void t(Closeable a, Closeable b) throws Exception {
    try (Closeable x = a; Closeable y = b) { }
  }
}
`

// tryResourcesDosVacioJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesDosVacioJavac17 = "yv66vgAAAD0AHgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCwAIAAkHAAoMAAsABgEAEWphdmEvaW8vQ2xvc2VhYmxlAQAFY2xvc2UHAA0BABNqYXZhL2xhbmcvVGhyb3dhYmxlCgAMAA8MABAAEQEADWFkZFN1cHByZXNzZWQBABgoTGphdmEvbGFuZy9UaHJvd2FibGU7KVYHABMBAANSZXMBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABdAEAKShMamF2YS9pby9DbG9zZWFibGU7TGphdmEvaW8vQ2xvc2VhYmxlOylWAQANU3RhY2tNYXBUYWJsZQEACkV4Y2VwdGlvbnMHABsBABNqYXZhL2xhbmcvRXhjZXB0aW9uAQAKU291cmNlRmlsZQEACFJlcy5qYXZhACEAEgACAAAAAAACAAEABQAGAAEAFAAAAB0AAQABAAAABSq3AAGxAAAAAQAVAAAABgABAAAAAgAIABYAFwACABQAAACMAAIABQAAADQqTStOLcYACS25AAcBACzGACQsuQAHAQCnABtOLMYAFCy5AAcBAKcACzoELRkEtgAOLb+xAAIAAgAOABsADAAgACYAKQAMAAIAFQAAAAoAAgAAAAQAMwAFABgAAAAmAAX8AA4HAAhMBwAM/wANAAQHAAgHAAgHAAgHAAwAAQcADAf5AAEAGQAAAAQAAQAaAAEAHAAAAAIAHQ=="

// internadoDeFuente: si el dex del fuente internó el método `dueño.nombre` con la firma dada. Sirve
// para afirmar que un cuerpo vacío no arrastra `addSuppressed` aunque el dex coincida byte a byte.
func internadoDeFuente(t *testing.T, fuente, dueno, nombre string) bool {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatal(err)
	}
	td.orderTables()
	for _, m := range td.methods {
		if m.owner == dueno && m.name == nombre {
			return true
		}
	}
	return false
}

// tres recursos: los dos niveles internos sueltan sus huecos y cada manejador los reutiliza de dentro
// hacia fuera. El oráculo es javac 17, como en los demás casos.
const tryResourcesTres = `import java.io.Closeable;
public class Res {
  static void use(Closeable c) {}
  static void t(Closeable a, Closeable b, Closeable c) throws Exception {
    try (Closeable x = a; Closeable y = b; Closeable z = c) { use(x); }
  }
}
`

// tryResourcesTresJavac17: `javac -g:lines,source Res.java` con javac 17.0.21 y `base64 -w0`.
const tryResourcesTresJavac17 = "yv66vgAAAD0AIgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAA1JlcwEAA3VzZQEAFihMamF2YS9pby9DbG9zZWFibGU7KVYLAA4ADwcAEAwAEQAGAQARamF2YS9pby9DbG9zZWFibGUBAAVjbG9zZQcAEwEAE2phdmEvbGFuZy9UaHJvd2FibGUKABIAFQwAFgAXAQANYWRkU3VwcHJlc3NlZAEAGChMamF2YS9sYW5nL1Rocm93YWJsZTspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQA8KExqYXZhL2lvL0Nsb3NlYWJsZTtMamF2YS9pby9DbG9zZWFibGU7TGphdmEvaW8vQ2xvc2VhYmxlOylWAQANU3RhY2tNYXBUYWJsZQEACkV4Y2VwdGlvbnMHAB8BABNqYXZhL2xhbmcvRXhjZXB0aW9uAQAKU291cmNlRmlsZQEACFJlcy5qYXZhACEACAACAAAAAAADAAEABQAGAAEAGAAAAB0AAQABAAAABSq3AAGxAAAAAQAZAAAABgABAAAAAgAIAAsADAABABgAAAAZAAAAAQAAAAGxAAAAAQAZAAAABgABAAAAAwAIABoAGwACABgAAAFlAAIACAAAAI0qTis6BCw6BS24AAcZBcYAKhkFuQANAQCnACA6BhkFxgAWGQW5AA0BAKcADDoHGQYZB7YAFBkGvxkExgAqGQS5AA0BAKcAIDoFGQTGABYZBLkADQEApwAMOgYZBRkGtgAUGQW/LcYAJy25AA0BAKcAHjoELcYAFS25AA0BAKcADDoFGQQZBbYAFBkEv7EABgAIAAwAGwASACIAKQAsABIABQA4AEcAEgBOAFUAWAASAAIAZABxABIAdwB9AIAAEgACABkAAAAKAAIAAAAFAIwABgAcAAAAhgAM/wAbAAYHAA4HAA4HAA4HAA4HAA4HAA4AAQcAEv8AEAAHBwAOBwAOBwAOBwAOBwAOBwAOBwASAAEHABII+QACTgcAEv8AEAAGBwAOBwAOBwAOBwAOBwAOBwASAAEHABII+QACTAcAEv8ADgAFBwAOBwAOBwAOBwAOBwASAAEHABII+QACAB0AAAAEAAEAHgABACAAAAACACE="

// TestTryWithResourcesTres: tres recursos anidados, el dex byte a byte el de javac 17.
func TestTryWithResourcesTres(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesTres, tryResourcesTresJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}

// TestTryWithResourcesHuecoDelCuerpo: con un local en el cuerpo, el manejador reutiliza su hueco
// (`astore_2`) en vez de pedir uno nuevo, que es lo que hace javac. El dex sale byte a byte.
func TestTryWithResourcesHuecoDelCuerpo(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesLocalPropio, tryResourcesLocalPropioJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}

// TestTryWithResourcesCuerpoVacio: sin sentencias en el cuerpo, javac no escribe ni tabla de
// excepciones ni `addSuppressed`: solo la declaración, la guarda de cierre y el `return`.
func TestTryWithResourcesCuerpoVacio(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesCuerpoVacio, tryResourcesCuerpoVacioJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	if item := codeDeFuenteUnica(t, tryResourcesCuerpoVacio, "t"); item.triesSize != 0 {
		t.Errorf("%d try_item, medido 0", item.triesSize)
	}
	if interno := internadoDeFuente(t, tryResourcesCuerpoVacio, "Ljava/lang/Throwable;", "addSuppressed"); interno {
		t.Error("se internó Throwable.addSuppressed en un cuerpo vacío")
	}
}

// TestTryWithResourcesDosCuerpoVacio: con dos recursos y el cuerpo vacío, el de dentro se cierra sin
// protección y el de fuera conserva su manejador con `addSuppressed`.
func TestTryWithResourcesDosCuerpoVacio(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesDosVacio, tryResourcesDosVacioJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
}

func codeDeFuenteUnica(t *testing.T, fuente, metodo string) codeItem {
	t.Helper()
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/Res.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("salieron %d clases: %s", len(salida), nombresDe(salida))
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatal(err)
	}
	d, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatal(err)
	}
	dex, err := readDex(d)
	if err != nil {
		t.Fatal(err)
	}
	item, err := codeItemDeMetodo(dex, metodo)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// TestTryWithResourcesDos: con dos recursos, el dex del emisor es byte a byte el de javac 17. El
// `.class` sale con cuatro filas de excepción —el cuerpo y el cierre de `y`, y el cuerpo y el cierre
// de `x`— de dentro hacia fuera.
func TestTryWithResourcesDos(t *testing.T) {
	nuestro, suyo := dexDeFuenteUnica(t, tryResourcesDos, tryResourcesDosJavac17)
	if !bytes.Equal(nuestro, suyo) {
		t.Fatalf("el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
			len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
	}
	if item := codeDeFuenteUnica(t, tryResourcesDos, "t"); item.triesSize != 6 {
		// el `.class` lleva 4 filas (dos por recurso); al traducir, el rango exterior solapa con los
		// interiores y el dexer lo parte: [2,4), [4,12), [12,31), [31,37), [37,52) y [70,76)
		t.Errorf("%d try_item, medidos 6 (4 filas del .class partidas por el solape)", item.triesSize)
	}
}
