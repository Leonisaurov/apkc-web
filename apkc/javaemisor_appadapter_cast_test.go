package main

import (
	"strings"
	"testing"
)

// Este fichero cierra las dos fronteras que quedaban en `AppAdapter.java` de ApkShare y que el test
// del constructor dejó escritas: el `check-cast` detrás de la llamada borrada y el puente del retorno
// covariante. Las dos se miden contra javac 17 `--release 8 -g:lines,source` y contra su dex, que es
// el oráculo del repo; y la primera se falsifica además con el verificador de la JVM, que es el que
// destapa el defecto de verdad (`Type 'java/lang/Object' … is not assignable to 'share/apps/util/App'`).

// claseEmitidaPorNombre: la `.class` de ese nombre interno en lo que emitimos nosotros.
func claseEmitidaPorNombre(t *testing.T, clases []claseEmitida, interno string) []byte {
	t.Helper()
	for _, c := range clases {
		if c.interno == interno {
			return c.datos
		}
	}
	t.Fatalf("no se emitió %s", interno)
	return nil
}

// claseLeidaPorNombre: la `.class` de ese nombre en descriptor (`Lshare/apps/util/AppAdapter;`) en lo
// que emitió javac.
func claseLeidaPorNombre(t *testing.T, clases []classFull, nombre string) classFull {
	t.Helper()
	for _, c := range clases {
		if c.Name == nombre {
			return c
		}
	}
	t.Fatalf("javac no dejó %s", nombre)
	return classFull{}
}

// dexDeUnaClase: el dex de esa única clase. Sirve para comparar byte a byte una clase suelta sin
// arrastrar las demás del proyecto: sirvió cuando el dex de las seis clases de ApkShare todavía traía
// fronteras abiertas en otra clase. Desde `javaemisor_ambito_local_test.go` ya no hace falta para
// eso —el dex entero de las seis es el de javac— pero sigue siendo la forma barata de mirar una sola
// clase cuando lo que se mide es suya.
func dexDeUnaClase(t *testing.T, datos []byte) []byte {
	t.Helper()
	leida, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	return dexDeUnaClaseLeida(t, leida)
}

// dexDeUnaClaseLeida: lo mismo para una clase que ya viene leída de javac.
func dexDeUnaClaseLeida(t *testing.T, c classFull) []byte {
	t.Helper()
	dex, err := dexDeClases([]classFull{c})
	if err != nil {
		t.Fatalf("dexando la clase: %v", err)
	}
	return dex
}

// fuenteSondaCast: el caso mínimo del `check-cast`, con las dos direcciones en el mismo método. `get`
// borra a `Object`, así que el hueco tipado necesita el `check-cast` y el hueco `Object` no. Es la
// falsificación de la regla: si se emitiera siempre, `Object o = …` saldría con una instrucción de
// más, y si no se emitiera nunca, `App a = …` quedaría con `Object` donde la firma declara `App`.
const fuenteSondaCast = `package p;

import java.util.ArrayList;
import java.util.List;

class App {
    String nombre = "x";
}

class Sonda {
    private List<App> lista;

    Sonda(List<App> lista) {
        this.lista = lista;
    }

    App primero() {
        return lista.get(0);
    }

    Object crudo() {
        Object o = lista.get(0);
        return o;
    }

    String nombre() {
        App a = lista.get(1);
        return a.nombre;
    }

    void guarda(List<App> otras) {
        for (App cada : lista) {
            otras.add(cada);
        }
    }
}
`

// cuentaCheckcast: cuántos `checkcast` (0xc0) hay en el Code.
func cuentaCheckcast(t *testing.T, code []byte) int {
	t.Helper()
	ins, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	n := 0
	for _, i := range ins {
		if i.op == opCheckcast {
			n++
		}
	}
	return n
}

// fuenteSondaPuente: el retorno covariante sobre una clase del propio fuente. `D.m()` devuelve `String`
// y `B.m()` `Object`, así que javac sintetiza `m:()Ljava/lang/Object;` en `D`.
const fuenteSondaPuente = `package p;

class B {
    Object m() {
        return null;
    }
}

class D extends B {
    @Override
    public String m() {
        return "a";
    }
}
`

// TestFalsificacionDelPuente: apagada la síntesis del puente, `AppAdapter` vuelve a tener un solo
// `getItem` —el defecto del informe— y el dex de la clase deja de coincidir con el de javac.
func TestFalsificacionDelPuente(t *testing.T) {
	jar := jarDelSDK(t)
	fuentes := fuentesAppAdapterShare()

	anterior := sintetizaPuentesCovariantes
	sintetizaPuentesCovariantes = false
	nuestras, err := compilarProyectoJava(fuentes, jar)
	sintetizaPuentesCovariantes = anterior
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes con la ruta apagada: %v", err)
	}
	leida, err := readClassFull(claseEmitidaPorNombre(t, nuestras, "share/apps/util/AppAdapter"))
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	n := 0
	for _, m := range leida.Methods {
		if m.Name == "getItem" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("sin el puente `AppAdapter` sale con %d `getItem` y se esperaba 1 (el defecto original)", n)
	}
}

// primeraLineaDeVerifyError: la línea del `VerifyError` y su motivo, para el log del test.
func primeraLineaDeVerifyError(salida string) string {
	lineas := strings.Split(salida, "\n")
	for i, l := range lineas {
		if strings.Contains(l, "VerifyError") && i+1 < len(lineas) {
			return strings.TrimSpace(l) + " | " + strings.TrimSpace(lineas[i+1])
		}
	}
	return strings.TrimSpace(salida)
}
