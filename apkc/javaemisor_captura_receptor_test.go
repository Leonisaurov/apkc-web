package main

// La captura de una local en posición de receptor: `apps.getAdapter()` dentro de una anónima. El
// nombre seguido de punto se tomaba por una cadena de tipo cualificado (`R.layout.main`) y nunca se
// registraba como captura, así que el emisor buscaba un tipo `apps` que no existe. Es el bloqueo de
// ApkShare con `--compiler go`:
//
//	MainActivity.java: linea 52: linea 53: linea 55, columna 44: al leer apps, el tipo apps no se
//	resuelve: ni en el fuente, ni en su paquete, ni en los imports, ni en java.lang, ni en el
//	classpath (android.jar ni jar/)
//
// La línea 55 es `openAppDetailsDialog((App) apps.getAdapter().getItem(positon));` dentro del
// `OnItemClickListener` de las líneas 52-55. El oráculo es el de siempre: nuestro `.class` y el de
// javac 17 `--release 8 -g:lines,source` por el dexer, byte a byte; `abreDetalle` está sobrecargado a
// propósito para que el molde `(App)` siga eligiendo la sobrecarga de `App` y no la de `Object`.
//
// Lo que separa una captura de un nombre de tipo cualificado no es el punto: es que el nombre esté en
// los locales del método de encierro (`registraCaptura` ya devuelve falso para `R`, `android` o el
// nombre de un campo), así que `R.layout.main` y `android.os.Bundle` siguen su camino de siempre.

import (
	"strings"
	"testing"
)

// fuenteSondaCapturaReceptor: el bloque de `MainActivity.java:52-55` verbatim, con el contexto mínimo
// para que `apps` y `abreDetalle` existan. El tipo estático de `apps` es `AdapterView` y no `ListView`
// a propósito: `ListView.getAdapter` hereda dos declaraciones con la misma lista de parámetros
// (`AbsListView.getAdapter:()Landroid/widget/ListAdapter;` y el genérico de
// `AdapterView.getAdapter:()Landroid/widget/Adapter;`), y elegir entre ellas es el desempate por tipo
// de vuelta covariante, que este subconjunto no hace: es el bloqueo siguiente de ApkShare, no el de
// esta sonda. Con `AdapterView` queda una sola `getAdapter` y el dex se puede comparar.
const fuenteSondaCapturaReceptor = `package p;

import android.app.Activity;
import android.view.View;
import android.widget.AdapterView;
import android.widget.ListView;

class App {
    public int id;
}

class Sonda extends Activity {
    public void abreDetalle(App app) {
        app.id = 1;
    }

    public void abreDetalle(Object objeto) {
        objeto.hashCode();
    }

    public void crea() {
        AdapterView apps = new ListView(this);
        apps.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            @Override
            public void onItemClick(AdapterView<?> adapter, View view, int positon, long id) {
                abreDetalle((App) apps.getAdapter().getItem(positon));
            }
        });
    }
}
`

// fuenteSondaCapturaReceptorSinAnonima es la misma llamada fuera de la anónima: ahí `apps` es una local
// del propio método y no hay captura, así que el emisor nunca pasó por la ruta que se toca aquí.
const fuenteSondaCapturaReceptorSinAnonima = `package p;

import android.widget.AdapterView;
import android.widget.ListView;

class App {
    public int id;
}

class Sonda {
    public void abreDetalle(App app) {
        app.id = 1;
    }

    public void abreDetalle(Object objeto) {
        objeto.hashCode();
    }

    public void crea() {
        AdapterView apps = new ListView(null);
        abreDetalle((App) apps.getAdapter().getItem(0));
    }
}
`

func fuentesSondaCapturaReceptor(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuente}}
}

// comparaDexSalvoConstructorDeAnonima: el dex de los dos lados, byte a byte, con el atributo del
// constructor de la anónima fuera de los dos. Es la única diferencia que queda y no es de esta ruta:
// este emisor escribe `MethodParameters` en ese constructor (apkc/javaemisor.go:7743, medido contra el
// fixture del párrafo 100) y solo pone `Signature: ()V` en la anónima que se crea dentro de otra
// (apkc/javaemisor.go:7746); javac 17 hace lo contrario, deja `Signature: ()V` y ningún
// `MethodParameters` en cualquier anónima. Lo fija aparte el test del constructor, para que arreglarlo
// obligue a pasar por aquí.
func comparaDexSalvoConstructorDeAnonima(t *testing.T, nuestras []claseEmitida, deJavac []classFull) error {
	t.Helper()
	normaliza := func(cs []classFull) []classFull {
		out := make([]classFull, len(cs))
		for i, c := range cs {
			ms := make([]classMethod, len(c.Methods))
			copy(ms, c.Methods)
			for j := range ms {
				if ms[j].Name != "<init>" {
					continue
				}
				ms[j].Parameters = false
				ms[j].Parametros = nil
				ms[j].Signature = ""
			}
			c.Methods = ms
			out[i] = c
		}
		return out
	}
	clases := make([]classFull, 0, len(nuestras))
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	dexNuestro, err := dexDeClases(normaliza(clases))
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}
	dexJavac, err := dexDeClases(normaliza(deJavac))
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		return &instruccionDistinta{detalle: "nuestro dex mide " + itoa(len(dexNuestro)) + " bytes y el de javac " + itoa(len(dexJavac))}
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			return &instruccionDistinta{detalle: "dex distinto en el byte " + itoa(i)}
		}
	}
	return nil
}

// TestLaCapturaDeLocalEnReceptorSeEscribeComoJavac: `apps.getAdapter()` dentro de la anónima se lee
// como `this.val$apps.getAdapter()`, con el tipo de la local en el campo `val$apps` y en el constructor
// de la anónima, y el cuerpo del `onItemClick` sale instrucción a instrucción igual al de javac.
func TestLaCapturaDeLocalEnReceptorSeEscribeComoJavac(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptor)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la captura de una local en posición de receptor: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaReceptor)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// el campo `val$apps`, del tipo de la local capturada, y las instrucciones que la usan
	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)
	comparaMetodos(t, nuestraAnonima, suyaAnonima)
	nuestroOnItem := metodoDeClaseEmitida(t, nuestras, "p/Sonda$1", "onItemClick", "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V")
	suOnItem := metodoDeClaseLeida(t, deJavac, "Lp/Sonda$1;", "onItemClick", "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V")
	if err := comparaInstrucciones(nuestroOnItem.Code, suOnItem.Code); err != nil {
		t.Fatalf("onItemClick no es el de javac: %v\n%s", err, volcadoDeClases(nuestras))
	}

	// y fuera de la anónima la misma llamada sigue sin captura: es el caso que no toca esta ruta
	if _, err := compilarProyectoJava(fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptorSinAnonima), jar); err != nil {
		t.Fatalf("la misma llamada sin anónima dejó de compilar: %v", err)
	}

	if err := comparaDexSalvoConstructorDeAnonima(t, nuestras, deJavac); err != nil {
		t.Fatalf("%v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestElConstructorDeLaAnonimaSigueComoEstaba fija la diferencia que `comparaDexSalvoConstructorDeAnonima`
// deja fuera: en el constructor de la anónima este emisor escribe `MethodParameters` y javac 17 escribe
// `Signature: ()V` (apkc/javaemisor.go:7733-7750). Es anterior a la captura en posición de receptor —
// sale igual en la sonda del párrafo 100, sin ningún punto detrás de la captura— y arreglarlo es otro
// cambio; aquí solo se fija para que no se cuele en el oráculo.
func TestElConstructorDeLaAnonimaSigueComoEstaba(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptor)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaCapturaReceptor)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	nuestraAnonima, suyaAnonima := buscaLaClase(t, nuestras, "Lp/Sonda$1;", deJavac)
	nuestro, okNuestro := buscaMetodo(nuestraAnonima, "<init>")
	suyo, okSuyo := buscaMetodo(suyaAnonima, "<init>")
	if !okNuestro || !okSuyo {
		t.Fatalf("sin constructor en la anónima: nuestro %v, javac %v", okNuestro, okSuyo)
	}
	if !nuestro.Parameters || nuestro.Signature != "" {
		t.Errorf("nuestro constructor ya no es el de este emisor (MethodParameters sin Signature): Parameters=%v Signature=%q; "+
			"si se ha arreglado, quítese la normalización de comparaDexSalvoConstructorDeAnonima", nuestro.Parameters, nuestro.Signature)
	}
	if suyo.Parameters || suyo.Signature != "()V" {
		t.Errorf("javac 17 ya no deja Signature: ()V en el constructor de la anónima: Parameters=%v Signature=%q", suyo.Parameters, suyo.Signature)
	}
}

// TestLaCapturaDeLocalEnReceptorFalsificacion: con la ruta apagada la sonda vuelve a fallar con el
// mensaje que el usuario leyó, con el fuente y la línea de la llamada (`Sonda.java` y `linea 26`, que
// es la 55 de MainActivity.java en el fuente real).
func TestLaCapturaDeLocalEnReceptorFalsificacion(t *testing.T) {
	const literal = "al leer apps, el tipo apps no se resuelve"
	jar := jarDelSDK(t)
	fuentes := fuentesSondaCapturaReceptor(fuenteSondaCapturaReceptor)

	// encendida, la sonda se acepta y llega hasta el final
	if _, err := compilarProyectoJava(fuentes, jar); err != nil {
		t.Fatalf("con la ruta de la captura encendida la sonda no se aceptó: %v", err)
	}

	anterior := capturaDeLocalComoReceptor
	capturaDeLocalComoReceptor = false
	t.Cleanup(func() { capturaDeLocalComoReceptor = anterior })

	_, err := compilarProyectoJava(fuentes, jar)
	if err == nil {
		t.Fatal("con la ruta de la captura apagada la sonda fue aceptada: la falsificación no falsifica")
	}
	if !strings.Contains(err.Error(), literal) {
		t.Fatalf("con la ruta apagada el rechazo es %q, y no lleva el mensaje del usuario %q", err, literal)
	}
	if !strings.HasPrefix(err.Error(), "Sonda.java: ") || !strings.Contains(err.Error(), "linea 26") {
		t.Fatalf("el rechazo no nombra el fuente y la línea de la llamada: %q", err)
	}
	t.Logf("sin la ruta: %v", err)
}
