package main

// La captura de una local a traves de una anonima anidada: `adapter.searchQuery("")` dentro del
// `Runnable` que `runOnUiThread` crea dentro del `Runnable` que crea `onCreate`. La local
// `AppAdapter adapter` es de `MainActivity.onCreate` (linea 59) y la unica anonima que la nombra es la
// de dentro (`MainActivity.java:74`, dentro del `Runnable` de la linea 71), asi que la cadena de
// captura tiene que atravesar dos niveles. Es el bloqueo de ApkShare con `--compiler go`:
//
//	MainActivity.java: linea 71: linea 72: linea 74, columna 25: al leer adapter, el tipo adapter no
//	se resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el
//	classpath (android.jar ni jar/)
//
// Lo medido con javac 17 `--release 8 -g:lines,source` sobre el fuente real:
//
//   - `MainActivity$2` guarda la Activity en `this$0` y la local en `val$adapter` —la captura la crea
//     javac aunque la referencia viva en la anonima interior—, y su constructor la recibe.
//   - `MainActivity$2$1` NO tiene `val$adapter`: su unico campo sintetico es `this$1`, del tipo de la
//     anonima exterior (`this$0` seria el nombre si esa exterior no tuviera a su vez exterior, como
//     `Capa$1$1` de `javaemisor_anonima_anidada_test.go`).
//   - El cuerpo de su `run()` lee la local por la cadena `aload_0; getfield this$1:LMainActivity$2;
//     getfield val$adapter:LAppAdapter;`, y un metodo de la Activity por `this$1` y luego `this$0`.
//
// El oraculo es el de siempre: el dex de los dos lados byte a byte, con el atributo del constructor de
// la anonima fuera —las dos divergencias ya fijadas en `javaemisor_captura_receptor_test.go`—, y las
// instrucciones del `run()` de la anonima interior, una a una. El descriptor del `val$adapter` tiene
// que ser el de la local real: si el emisor inventara un tipo, aqui se ve.

import (
	"strings"
	"testing"
)

// fuenteSondaCapturaAnidada: el bloque de `MainActivity.java:59-80` con el contexto minimo para que la
// local, las dos anonimas y las llamadas al exterior existan. `hilo` es el `package_fetch.execute(...)`
// de ApkShare y `runOnUiThread` el de verdad, que es un metodo de `Activity`: asi el metodo de la
// anonima exterior se llama con un salto y el de la Activity —`esconde`— con dos.
const fuenteSondaCapturaAnidada = `package p;

import android.app.Activity;

class Adaptador {
    public void searchQuery(String q) { }
    public void notifyDataSetChanged() { }
}

class Sonda extends Activity {
    private boolean cargando;

    public void cargar() {
        Adaptador adapter = new Adaptador();
        hilo(new Runnable() {
            @Override
            public void run() {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        adapter.searchQuery("");
                        adapter.notifyDataSetChanged();
                        esconde();
                    }
                });
            }
        });
    }

    void hilo(Runnable r) { }

    void esconde() { cargando = false; }
}
`

func fuentesSondaCapturaAnidada(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuente}}
}

// campoDe: el campo con ese nombre en una clase ya leida, o vacio.
func campoDe(cf classFull, nombre string) classField {
	for _, f := range cf.Fields {
		if f.Name == nombre {
			return f
		}
	}
	return classField{}
}

// TestLaCapturaDeLocalAtraviesaLaAnonimaAnidada: `adapter` dentro de la anonima interior se lee por
// `this$1` y el `val$adapter` de la exterior, con el tipo real de la local; `esconde()` sube los dos
// saltos hasta la Activity; y todo el dex coincide con el de javac 17 salvo el constructor de la
// anonima, que ya tenia sus dos divergencias fijadas.
func TestLaCapturaDeLocalAtraviesaLaAnonimaAnidada(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesSondaCapturaAnidada(fuenteSondaCapturaAnidada)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo la captura a traves de la anonima anidada: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaAnidada)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// el nombre de la anonima interior es el de javac: Sonda$1$1
	if !strings.Contains(nombresDe(nuestras), "p/Sonda$1$1") {
		t.Fatalf("no se emitio p/Sonda$1$1: %s", nombresDe(nuestras))
	}

	nuestraInterior, suyaInterior := buscaLaClase(t, nuestras, "Lp/Sonda$1$1;", deJavac)
	nuestraExterior, suyaExterior := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)

	// el campo sintetico de la interior es `this$1` —no `this$0`— y del tipo de la exterior; y no
	// lleva un `val$adapter` propio, que es lo que distingue la cadena de javac de una captura directa
	if f := campoDe(nuestraInterior, "this$1"); f.Desc != "Lp/Sonda$1;" {
		t.Errorf("la anonima interior no guarda su exterior en this$1:Lp/Sonda$1;: %+v (campos %v)",
			f, nuestraInterior.Fields)
	}
	if f := campoDe(nuestraInterior, "this$0"); f.Name != "" {
		t.Errorf("la anonima interior se quedo con this$0: %+v", f)
	}
	if f := campoDe(nuestraInterior, "val$adapter"); f.Name != "" {
		t.Errorf("la anonima interior capturo la local por su cuenta: %+v", f)
	}

	// y la exterior si la tiene, con el tipo real de la local
	if f := campoDe(nuestraExterior, "val$adapter"); f.Desc != "Lp/Adaptador;" {
		t.Errorf("la anonima exterior no tiene val$adapter:Lp/Adaptador;: %+v (campos %v)", f, nuestraExterior.Fields)
	}

	// el cuerpo que usa la local, instruccion a instruccion contra javac
	nuestroRun := metodoDeClaseEmitida(t, nuestras, "p/Sonda$1$1", "run", "()V")
	suRun := metodoDeClaseLeida(t, deJavac, "Lp/Sonda$1$1;", "run", "()V")
	if err := comparaInstrucciones(nuestroRun.Code, suRun.Code); err != nil {
		t.Fatalf("el run() de la anonima interior no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// los metodos y las LineNumberTable de las dos anonimas y de la clase
	comparaMetodos(t, nuestraInterior, suyaInterior)
	comparaMetodos(t, nuestraExterior, suyaExterior)

	// el dex entero, con el atributo del constructor de la anonima fuera
	if err := comparaDexComoJavac17(t, nuestras, deJavac); err != nil {
		t.Fatalf("%v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestLaCapturaAnidadaFalsificacion: con la ruta apagada la sonda vuelve a fallar con el mensaje que el
// usuario leyo, con el fuente y la posicion de la llamada (`Sonda.java`, `linea 18: linea 19: linea 21,
// columna 25`, que es `linea 71: linea 72: linea 74, columna 25` de MainActivity.java en el fuente
// real).
func TestLaCapturaAnidadaFalsificacion(t *testing.T) {
	const literal = "al leer adapter, el tipo adapter no se resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el classpath (android.jar ni jar/)"
	jar := jarDelSDK(t)
	fuentes := fuentesSondaCapturaAnidada(fuenteSondaCapturaAnidada)

	// encendida, la sonda se acepta
	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta de la captura anidada encendida la sonda no se acepto: %v", err)
	}

	anterior := capturaDeLocalEnAnonimaAnidada
	capturaDeLocalEnAnonimaAnidada = false
	t.Cleanup(func() { capturaDeLocalEnAnonimaAnidada = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta apagada la sonda fue aceptada: la falsificacion no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y no lleva el mensaje del usuario", err)
	}
	if !strings.HasPrefix(err.Error(), "Sonda.java: ") || !strings.Contains(err.Error(), "columna 25") {
		t.Fatalf("el rechazo no nombra el fuente y la posicion de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}
