package main

import (
	"encoding/base64"
	"encoding/binary"
	"reflect"
	"testing"
)

// La primera pieza del paso 3-G: del `res/values/styles.xml` en texto a la tabla de recursos. La
// comprobacion no es un muestreo: lo que sale del texto tiene que dar EL MISMO modelo que escribio a
// mano la prueba del parrafo 29, que ya produce los bytes del fixture.

// stylesDelStudio es el res/values/styles.xml del proyecto, fijado en base64 (423 bytes, sha256
// a490b9438728f1af…), el mismo con el que aapt2 dejo los ficheros del parrafo 24.
const stylesDelStudio = "PHJlc291cmNlcz4KICAgIDxzdHlsZSBuYW1lPSJBcHBUaGVtZSIgcGFyZW50PSJAYW5kcm9pZDpzdHlsZS9UaGVtZS5EZXZpY2VEZWZhdWx0LkxpZ2h0Ij4KICAgICAgICA8aXRlbSBuYW1lPSJhbmRyb2lkOndpbmRvd0FjdGlvbkJhciI+ZmFsc2U8L2l0ZW0+CiAgICAgICAgPGl0ZW0gbmFtZT0iYW5kcm9pZDp3aW5kb3dOb1RpdGxlIj50cnVlPC9pdGVtPgogICAgICAgIDxpdGVtIG5hbWU9ImFuZHJvaWQ6c3RhdHVzQmFyQ29sb3IiPiNGRkZGRkY8L2l0ZW0+CiAgICAgICAgPGl0ZW0gbmFtZT0iYW5kcm9pZDp3aW5kb3dMaWdodFN0YXR1c0JhciI+dHJ1ZTwvaXRlbT4KICAgICAgICA8aXRlbSBuYW1lPSJhbmRyb2lkOndpbmRvd0JhY2tncm91bmQiPkBhbmRyb2lkOmNvbG9yL3doaXRlPC9pdGVtPgogICAgPC9zdHlsZT4KPC9yZXNvdXJjZXM+"

// marcoDelStudio es la parte del framework que usa este proyecto. Los ids y los valores no estan
// inventados: son los que comprobaron TestMarcoDelAndroidJar y TestMarcoValoresDeAtributo contra
// `aapt2 dump resources android.jar`, y aqui se repiten a mano para que la prueba corra tambien en el
// hospedador de navegador, que no tiene el jar.
func marcoDelStudio() *marcoRecursos {
	return &marcoRecursos{
		nombre: "android",
		ids: map[string]uint32{
			"attr/windowActionBar":            0x010102cd,
			"attr/windowNoTitle":              0x01010056,
			"attr/statusBarColor":             0x01010451,
			"attr/windowLightStatusBar":       0x010104e0,
			"attr/windowBackground":           0x01010054,
			"attr/layout_width":               0x010100f4,
			"attr/layout_height":              0x010100f5,
			"attr/text":                       0x0101014f,
			"attr/id":                         0x010100d0,
			"attr/gravity":                    0x010100af,
			"attr/orientation":                0x010100c4,
			"style/Theme.DeviceDefault.Light": 0x0103012b,
			"style/TextAppearance.Large":      0x01030042,
			"color/white":                     0x0106000b,
		},
		porId: map[uint32]string{
			0x010102cd: "attr/windowActionBar",
			0x01010056: "attr/windowNoTitle",
			0x01010451: "attr/statusBarColor",
			0x010104e0: "attr/windowLightStatusBar",
			0x01010054: "attr/windowBackground",
			0x0103012b: "style/Theme.DeviceDefault.Light",
			0x0106000b: "color/white",
		},
		valores: map[string]map[string]valorEscribir{
			"layout_width": {
				"wrap_content": {tipo: tipoEnteroDec, datos: 0xfffffffe},
				"fill_parent":  {tipo: tipoEnteroDec, datos: 0xffffffff},
				"match_parent": {tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			"layout_height": {
				"wrap_content": {tipo: tipoEnteroDec, datos: 0xfffffffe},
				"fill_parent":  {tipo: tipoEnteroDec, datos: 0xffffffff},
				"match_parent": {tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			"gravity": {
				"center":            {tipo: tipoEnteroHex, datos: 0x11},
				"center_horizontal": {tipo: tipoEnteroHex, datos: 0x1},
				"center_vertical":   {tipo: tipoEnteroHex, datos: 0x10},
			},
			"orientation": {
				"vertical":   {tipo: tipoEnteroDec, datos: 1},
				"horizontal": {tipo: tipoEnteroDec, datos: 0},
			},
		},
	}
}

func TestValoresDelStudioDanLaMismaTabla(t *testing.T) {
	contenido, err := base64.StdEncoding.DecodeString(stylesDelStudio)
	if err != nil {
		t.Fatal(err)
	}
	if len(contenido) != 423 {
		t.Fatalf("el fuente fijado mide %d bytes, no 423", len(contenido))
	}
	m := marcoDelStudio()
	recursos, err := recursosDeValores(fuenteXML{nombre: "res/values/styles.xml", contenido: contenido}, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(recursos) != 1 {
		t.Fatalf("salieron %d recursos, se esperaba el estilo", len(recursos))
	}
	// el id que declara el layout y la ruta del layout son lo que descubre la segunda mitad del paso
	// (layouts.go); aqui se pasan como ya descubiertos, y lo que se comprueba es el orden y los valores
	recursos = append(recursos, recursoEntabla{tipo: "id", clave: "pulsar", valor: valorEscribir{tipo: tipoBooleano}})

	layouts := []fuenteXML{{nombre: "res/layout/main.xml"}}
	tabla, err := tablaDeRecursos(recursos, layouts, nil, "com.hola.apkcstudio")
	if err != nil {
		t.Fatal(err)
	}

	querida := tablaDelStudio()
	if !reflect.DeepEqual(tabla.nombresTipo, querida.nombresTipo) {
		t.Errorf("tipos %v, queridos %v", tabla.nombresTipo, querida.nombresTipo)
	}
	if !reflect.DeepEqual(tabla.claves, querida.claves) {
		t.Errorf("claves %v, queridas %v", tabla.claves, querida.claves)
	}
	if !reflect.DeepEqual(tabla.global, querida.global) {
		t.Errorf("pool global %v, querido %v", tabla.global, querida.global)
	}
	if len(tabla.tipos) != len(querida.tipos) {
		t.Fatalf("%d tipos escritos, %d queridos", len(tabla.tipos), len(querida.tipos))
	}
	for i := range querida.tipos {
		q := querida.tipos[i]
		o := tabla.tipos[i]
		nombreTipo := querida.nombresTipo[int(q.id)-1]
		if o.id != q.id || len(o.bloques) != len(q.bloques) || len(o.bloques[0].entradas) != len(q.bloques[0].entradas) {
			t.Errorf("tipo #%d: id %d con %d configuraciones/%d entradas, querido id %d con %d/%d", i, o.id, len(o.bloques), len(o.bloques[0].entradas), q.id, len(q.bloques), len(q.bloques[0].entradas))
			continue
		}
		for j := range q.bloques[0].entradas {
			qe, oe := q.bloques[0].entradas[j], o.bloques[0].entradas[j]
			if qe.clave != oe.clave || qe.flags != oe.flags || qe.compleja != oe.compleja {
				t.Errorf("%s entrada #%d: clave %d flags %#04x compleja %v, queridas %d %#04x %v",
					nombreTipo, j, oe.clave, oe.flags, oe.compleja, qe.clave, qe.flags, qe.compleja)
			}
			if qe.valor != oe.valor {
				t.Errorf("%s entrada #%d: valor %+v, querido %+v", nombreTipo, j, oe.valor, qe.valor)
			}
			if qe.padre != oe.padre {
				t.Errorf("%s: padre %#08x, querido %#08x", nombreTipo, oe.padre, qe.padre)
			}
			if !reflect.DeepEqual(qe.registros, oe.registros) {
				t.Errorf("%s: registros %+v, queridos %+v", nombreTipo, oe.registros, qe.registros)
			}
		}
	}

	// y lo que vale de verdad: escrito el modelo, los bytes tienen que seguir siendo los del fixture
	nuestra, err := escribeTablaRecursos(*tabla)
	if err != nil {
		t.Fatal(err)
	}
	fijado, _ := base64.StdEncoding.DecodeString(arscTabla)
	if len(nuestra) != 936 {
		t.Fatalf("la tabla producida desde el texto mide %d bytes, se esperaban 936", len(nuestra))
	}
	esperado := append([]byte{}, fijado...)
	binary.LittleEndian.PutUint32(esperado[4:], uint32(len(nuestra)))
	binary.LittleEndian.PutUint32(esperado[68+4:], uint32(len(nuestra)-68))
	for i := 0; i < 752; i++ {
		if nuestra[i] != esperado[i] {
			t.Fatalf("el byte #%d de la tabla producida desde el texto es %#02x y el de aapt2 es %#02x", i, nuestra[i], esperado[i])
		}
	}

	// la R, salida del mismo modelo, sigue siendo la de aapt2
	r, err := rDeTabla(*tabla)
	if err != nil {
		t.Fatal(err)
	}
	if r != rDeAapt2 {
		t.Errorf("la R desde el texto no casa con la de aapt2:\n%s", r)
	}
}

func TestValoresRechazanLoNoMedido(t *testing.T) {
	m := marcoDelStudio()

	salida := func(texto string) ([]recursoEntabla, error) {
		return recursosDeValores(fuenteXML{nombre: "res/values/x.xml", contenido: []byte(texto)}, m)
	}
	if _, err := salida("<resources><style>sin nombre</style></resources>"); err == nil {
		t.Error("un estilo sin nombre fue aceptado")
	}
	if _, err := salida(`<resources><item name="x" type="desconocido">1</item></resources>`); err == nil {
		t.Error("un tipo de item desconocido fue aceptado")
	}
	if _, err := salida("<resources><string name=\"a\">uno<string name=\"b\">dos</string></string></resources>"); err == nil {
		t.Error("un string con un hijo fue aceptado")
	}
	if _, err := salida("<resources><color name=\"c\">noesuncolor</color></resources>"); err == nil {
		t.Error("un color que no es color fue aceptado")
	}
	if _, err := salida(`<resources><string name="s">@style/Otro</string></resources>`); err == nil {
		t.Error("una referencia al propio paquete resuelta en la primera pasada")
	}
	if _, err := salida(`<resources><style name="S"><item name="tools:x">1</item></style></resources>`); err == nil {
		t.Error("un item que no es android: fue aceptado")
	}
	if _, err := salida(`<resources><style name="S" parent="@android:style/NoExiste"><item name="android:windowNoTitle">true</item></style></resources>`); err == nil {
		t.Error("un padre inexistente fue aceptado")
	}
	if _, err := salida("<resources><style name=\"S\"><item name=\"android:windowNoTitle\">true</item></style></resources>"); err != nil {
		t.Errorf("la forma medida fue rechazada: %v", err)
	}
}
