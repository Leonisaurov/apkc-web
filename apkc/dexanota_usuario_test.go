package main

// Las anotaciones del USUARIO, que este fork no escribía: `@Retention`, `@Target` y los
// `AnnotationDefault` de los elementos viajan en el bloque de anotaciones del dex, y ART los lee al
// pedirlos por reflexión. El fichero es la clase del fixture `banco/art-verificador/fixtures/anota`
// tal como la dejó javac 21 (632 bytes), para que el lector del `.class` y el emisor del dex se
// midan sobre bytes reales y no sobre uno construido a mano.

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// etiquetaJavac: `com/ejemplo/artver/anota/Etiqueta.class`, la interfaz de anotación del fixture.
const etiquetaJavac = "yv66vgAAAEEAHgcAAgEAIWNvbS9lamVtcGxvL2FydHZlci9hbm90YS9FdGlxdWV0YQcABAEAEGph" +
	"dmEvbGFuZy9PYmplY3QHAAYBAB9qYXZhL2xhbmcvYW5ub3RhdGlvbi9Bbm5vdGF0aW9uAQAFY29s" +
	"b3IBACIoKUxjb20vZWplbXBsby9hcnR2ZXIvYW5vdGEvQ29sb3I7AQARQW5ub3RhdGlvbkRlZmF1" +
	"bHQBACBMY29tL2VqZW1wbG8vYXJ0dmVyL2Fub3RhL0NvbG9yOwEABFJPSk8BAAVmbGFncwEAAygp" +
	"SQMAAAAAAQAFZmlybWEBABQoKUxqYXZhL2xhbmcvU3RyaW5nOwEAAAEAClNvdXJjZUZpbGUBAA1F" +
	"dGlxdWV0YS5qYXZhAQAZUnVudGltZVZpc2libGVBbm5vdGF0aW9ucwEAIExqYXZhL2xhbmcvYW5u" +
	"b3RhdGlvbi9SZXRlbnRpb247AQAFdmFsdWUBACZMamF2YS9sYW5nL2Fubm90YXRpb24vUmV0ZW50" +
	"aW9uUG9saWN5OwEAB1JVTlRJTUUBAB1MamF2YS9sYW5nL2Fubm90YXRpb24vVGFyZ2V0OwEAIkxq" +
	"YXZhL2xhbmcvYW5ub3RhdGlvbi9FbGVtZW50VHlwZTsBAARUWVBFAQAFRklFTEQBAAZNRVRIT0Qm" +
	"AQABAAMAAQAFAAAAAwQBAAcACAABAAkAAAAFZQAKAAsEAQAMAA0AAQAJAAAAA0kADgQBAA8AEAAB" +
	"AAkAAAADcwARAAIAEgAAAAIAEwAUAAAAJQACABUAAQAWZQAXABgAGQABABZbAANlABoAG2UAGgAc" +
	"ZQAaAB0="

func TestElLectorGuardaLasAnotacionesDelUsuario(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(etiquetaJavac)
	if err != nil {
		t.Fatalf("la fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("readClassFull: %v", err)
	}
	if len(cf.Anotaciones) != 2 {
		t.Fatalf("%d anotaciones visibles, esperaba Retention y Target", len(cf.Anotaciones))
	}
	// Retention y Target son las dos de runtime; el orden del `.class` no se toca aquí
	tipos := map[string]bool{}
	for _, a := range cf.Anotaciones {
		tipos[a.tipo] = true
		if a.visibilidad != visibilidadRuntime {
			t.Errorf("%s con visibilidad %#02x, esperaba runtime", a.tipo, a.visibilidad)
		}
	}
	for _, quiero := range []string{"Ljava/lang/annotation/Retention;", "Ljava/lang/annotation/Target;"} {
		if !tipos[quiero] {
			t.Errorf("falta %s", quiero)
		}
	}
	// los valores por omisión: enum ROJO, cadena vacía e int 0
	defectos := map[string]valorAnotacion{}
	for _, m := range cf.Methods {
		if m.Default != nil {
			defectos[m.Name] = *m.Default
		}
	}
	if len(defectos) != 3 {
		t.Fatalf("%d valores por omisión, esperaba color, flags y firma", len(defectos))
	}
	if d := defectos["color"]; d.clase != valueEnum || d.nombre != "ROJO" ||
		d.duenio != "Lcom/ejemplo/artver/anota/Color;" {
		t.Errorf("color por omisión = %+v", d)
	}
	if d := defectos["flags"]; d.clase != valueInt || d.entero != 0 {
		t.Errorf("flags por omisión = %+v", d)
	}
	if d := defectos["firma"]; d.clase != valueString || d.cadena != "" {
		t.Errorf("firma por omisión = %+v", d)
	}
}

// TestElEnumDeUnaAnotacionEsUnIndiceDeFieldId: en el dex un `VALUE_ENUM` no lleva tipo y nombre,
// lleva UN índice de `field_ids` con su anchura en la cabecera, igual que `VALUE_METHOD`. Medido
// sobre el fichero de `d8` para `@Retention(RUNTIME)`: `01 08 01 16 1b 04`, o sea el campo 4 de
// `RetentionPolicy`. Con la pareja tipo/nombre en su lugar, ART corta con «Bad index for
// encoded_value field» y `dexdump` no lo ve.
func TestElEnumDeUnaAnotacionEsUnIndiceDeFieldId(t *testing.T) {
	td := &dexTables{
		typeIdx:   map[string]int{"Ljava/lang/annotation/Retention;": 0},
		stringIdx: map[string]int{"value": 1},
		fieldIdx: map[fieldKey]int{
			{owner: "Ljava/lang/annotation/RetentionPolicy;", typeOf: "Ljava/lang/annotation/RetentionPolicy;",
				name: "RUNTIME"}: 4,
		},
	}
	a := &anotacion{tipo: "Ljava/lang/annotation/Retention;", visibilidad: visibilidadRuntime,
		elementos: []par{{nombre: "value", valor: valorEnum(
			"Ljava/lang/annotation/RetentionPolicy;", "RUNTIME")}}}
	quiero := []byte{visibilidadRuntime, 0x00, 0x01, 0x01, valueEnum, 0x04}
	if got := a.bytes(td); !bytes.Equal(got, quiero) {
		t.Errorf("bytes % x, esperaba % x", got, quiero)
	}
}

// TestLaAnotacionDefaultEsUnaAnotacionAnidada: `d8` deja en la CLASE de la anotación un
// `Ldalvik/annotation/AnnotationDefault;` de sistema cuyo `value` es la anotación del propio tipo
// con los elementos que llevan valor por omisión. javac, en cambio, deja un `AnnotationDefault` en
// cada MÉTODO: `reglasDe` los junta.
func TestLaAnotacionDefaultEsUnaAnotacionAnidada(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(etiquetaJavac)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatal(err)
	}
	reglas := reglasDe(cf)
	var defecto *anotacion
	for i := range reglas.clase {
		if reglas.clase[i].tipo == anotacionAnnotationDefault {
			defecto = &reglas.clase[i]
		}
	}
	if defecto == nil {
		t.Fatal("no se armó el AnnotationDefault de la clase")
	}
	if defecto.visibilidad != visibilidadSistema || len(defecto.elementos) != 1 ||
		defecto.elementos[0].nombre != elementoValue {
		t.Fatalf("AnnotationDefault = %+v", *defecto)
	}
	anidada := defecto.elementos[0].valor
	if anidada.clase != valueAnnotation || anidada.interna == nil ||
		anidada.interna.tipo != cf.Name || len(anidada.interna.elementos) != 3 {
		t.Errorf("la anotación anidada = %+v", anidada)
	}
}
