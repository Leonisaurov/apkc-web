package main

// La herencia del locale en la sesión viva (§326). El idioma NO se reenvía en cada `--pulsar`:
// el hospedador lo pone en APKC_SESION_LOCALE al crear la instancia y el arranque wasm lo lee una
// sola vez. Este test nativo exercise esa misma puerta —nuevaSesionPreviewDelEntorno— para que una
// reversión del adaptador wasm lo rompa: sin el entorno el modelo vuelve a decir Hello!.

import (
	"encoding/json"
	"strings"
	"testing"
)

// tablaConSaludoLocalizado deriva la tabla del Contador y convierte `string/saludo` en un valor con
// dos configuraciones: la base dice Hello! y `values-es` dice Hola!. El resto de la tabla (ids,
// layout y actividad) queda intacto para que el dex del Contador siga ejecutándose.
func tablaConSaludoLocalizado(t *testing.T) tablaEscribir {
	t.Helper()
	tb := tablaDelContador()
	if len(tb.tipos) < 3 || len(tb.tipos[2].bloques) == 0 || len(tb.tipos[2].bloques[0].entradas) == 0 {
		t.Fatal("la tabla del contador no trae la entrada de string/saludo que se va a localizar")
	}
	tb.global = []string{"res/layout/main.xml", "Hello!", "Hola!"}
	tb.tipos[2].bloques[0].entradas[0].valor = valorEscribir{tipo: 0x03, datos: 1}
	tb.tipos[2].bloques = append(tb.tipos[2].bloques, bloqueEscribir{
		config: "es",
		entradas: []entradaEscribir{
			{clave: 3, valor: valorEscribir{tipo: 0x03, datos: 2}},
		},
	})
	return tb
}

// layoutConSaludoLocalizado apunta el TextView del Contador a `@string/saludo` en vez de al literal
// "Pulsos: 0". El clic sigue cayendo en el listener de id/pulsar, pero el texto estático del nodo
// se resuelve contra la tabla, que es donde manda el locale.
func layoutConSaludoLocalizado(t *testing.T) documentoAXML {
	t.Helper()
	d := layoutDelContador()
	attrs := d.raiz.hijos[0].attrs
	encontrado := false
	for i := range attrs {
		if attrs[i].nombre == "text" {
			attrs[i] = atributoEscribir{ns: "android", nombre: "text", id: 0x0101014f,
				tipo: tipoReferencia, datos: 0x7f030000}
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatal("el layout del contador no trae el atributo android:text que se va a localizar")
	}
	return d
}

// apkDelSaludoLocalizado arma la apk de la sonda: manifiesto y dex del Contador, tabla con values-es
// y un layout cuyo TextView lee el recurso traducido.
func apkDelSaludoLocalizado(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelContador())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutConSaludoLocalizado(t))
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaConSaludoLocalizado(t))
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
}

// saludoDelModelo lee el texto ESTÁTICO del primer TextView del modelo publicado. El dinámico lo
// sobrescribe el setText del Contador al pulsar; el estático es el que sale de values-es.
func saludoDelModelo(t *testing.T, linea string) string {
	t.Helper()
	var modelo struct {
		Raiz struct {
			Hijos []struct {
				Texto string `json:"texto"`
			} `json:"hijos"`
		} `json:"raiz"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo de la orden no se lee: %v", err)
	}
	if len(modelo.Raiz.Hijos) == 0 {
		t.Fatalf("el modelo no trae el TextView del saludo: %s", linea)
	}
	return modelo.Raiz.Hijos[0].Texto
}

// TestLaSesionHeredaElLocaleDelEntorno es la falsación de la herencia: con APKC_SESION_LOCALE=es al
// crear la sesión, una orden `--pulsar id/pulsar` SIN `--locale` debe seguir devolviendo Hola!. Sin
// el entorno debe volver a Hello!, que es el fallo que refutó el auditor.
func TestLaSesionHeredaElLocaleDelEntorno(t *testing.T) {
	apk := apkDelSaludoLocalizado(t)

	t.Run("con locale en el entorno", func(t *testing.T) {
		t.Setenv("APKC_SESION_LOCALE", "es")
		s, err := nuevaSesionPreviewDelEntorno(apk, "")
		if err != nil {
			t.Fatalf("la sesión no arrancó: %v", err)
		}
		if s.locale != "es" {
			t.Fatalf("la sesión guardó locale %q y el entorno decía es", s.locale)
		}
		linea, codigo := entrega(t, s, "clic-1", []string{"--pulsar", "id/pulsar"})
		if codigo != 0 {
			t.Fatalf("la orden devolvió código %d: %s", codigo, linea)
		}
		if texto := saludoDelModelo(t, linea); texto != "Hola!" {
			t.Fatalf("tras el clic el saludo es %q y con APKC_SESION_LOCALE=es debe ser Hola!", texto)
		}
		if strings.Contains(linea, "Hello!") {
			t.Error("el modelo volvió a la base aunque la sesión nació con el locale es")
		}
	})

	t.Run("sin locale en el entorno", func(t *testing.T) {
		t.Setenv("APKC_SESION_LOCALE", "")
		s, err := nuevaSesionPreviewDelEntorno(apk, "")
		if err != nil {
			t.Fatalf("la sesión sin locale no arrancó: %v", err)
		}
		if s.locale != "" {
			t.Fatalf("la sesión guardó locale %q y el entorno estaba vacío", s.locale)
		}
		linea, _ := entrega(t, s, "clic-1", []string{"--pulsar", "id/pulsar"})
		if texto := saludoDelModelo(t, linea); texto != "Hello!" {
			t.Fatalf("sin locale el saludo es %q y la base guarda Hello!", texto)
		}
	})
}
