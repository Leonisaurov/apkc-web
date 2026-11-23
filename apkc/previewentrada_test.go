package main

// La prueba de la Fase E5 del emulador: la entrada del usuario. Una app con un EditText, un CheckBox
// y un RadioGroup refleja en el modelo lo que se le teclea y se le marca, y los oyentes que el propio
// proyecto registra (`TextWatcher`, `OnCheckedChangeListener`) corren de verdad: su salida se ve en
// la consola y en las capturas.
//
// Las órdenes se entregan en el orden de la línea de comandos, así que el guion mezcla `--escribir`,
// `--marcar` y `--pulsar` sobre la misma ejecución: teclear primero cambia lo que el `onClick` lee.
// Lo que queda fuera, y se dice en el plan: teclado en pantalla, IME, selección, autocompletado,
// arrastrar texto y portapapeles.

import (
	"fmt"
	"strings"
	"testing"
)

// Los ids de la pantalla de E5, en el orden en que la tabla los numera.
const (
	idEntradaAcepto = 0x7f010000
	idEntradaEco    = 0x7f010001
	idEntradaEnviar = 0x7f010002
	idEntradaGrupo  = 0x7f010003
	idEntradaNombre = 0x7f010004
	idEntradaRojo   = 0x7f010005
	idEntradaVerde  = 0x7f010006
)

// tablaDeEntrada es la tabla de recursos de la pantalla: siete ids y el layout main, con la ruta en
// el pool global, que es lo que resuelve un id de `R.layout`.
func tablaDeEntrada() tablaEscribir {
	ids := []entradaEscribir{
		{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 3, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 4, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 5, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
		{clave: 6, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
	}
	specs := make([]uint32, len(ids))
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.contador",
		global:        []string{"res/layout/main.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout"},
		nombresUTF8:   false,
		claves:        []string{"acepto", "eco", "enviar", "grupo", "nombre", "rojo", "verde", "main"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: specs, bloques: []bloqueEscribir{{entradas: ids}}},
			{id: 2, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 7, valor: valorEscribir{tipo: 0x03, datos: 0}},
			}}}},
		},
	}
}

// atributoDeEntrada arma un atributo de layout con el id del framework que le corresponde. Los ids
// son los de `android.R.attr` de la platform 34; el lector del modelo busca por nombre, pero un
// atributo sin id no es lo que escribe aapt2 y el layout de prueba dejaría de parecerse al real.
func atributoDeEntrada(nombre string, id uint32, tipo byte, datos uint32, texto string) atributoEscribir {
	return atributoEscribir{ns: "android", nombre: nombre, id: id, tipo: tipo, datos: datos, texto: texto}
}

// layoutDeEntrada es la pantalla de E5: un EditText con pista, el TextView que su TextWatcher
// reescribe, un CheckBox, un RadioGroup con dos opciones —la roja marcada por el layout— y el botón
// que lee lo tecleado.
func layoutDeEntrada() documentoAXML {
	ancho := func() atributoEscribir {
		return atributoDeEntrada("layout_width", 0x010100f4, tipoEnteroDec, 0xfffffffe, "")
	}
	alto := func() atributoEscribir {
		return atributoDeEntrada("layout_height", 0x010100f5, tipoEnteroDec, 0xfffffffe, "")
	}
	id := func(valor uint32) atributoEscribir {
		return atributoEscribir{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: valor}
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				atributoDeEntrada("orientation", 0x010100c4, tipoEnteroDec, 1, ""),
				atributoDeEntrada("layout_width", 0x010100f4, tipoEnteroDec, 0xffffffff, ""),
				atributoDeEntrada("layout_height", 0x010100f5, tipoEnteroDec, 0xffffffff, ""),
			},
			hijos: []*elementoEscribir{
				{
					nombre: "EditText",
					linea:  4,
					attrs: []atributoEscribir{
						id(idEntradaNombre), ancho(), alto(),
						atributoDeEntrada("hint", 0x01010150, tipoCadena, 0, "Escribe aquí"),
					},
				},
				{
					nombre: "TextView",
					linea:  8,
					attrs:  []atributoEscribir{id(idEntradaEco), ancho(), alto()},
				},
				{
					nombre: "CheckBox",
					linea:  12,
					attrs: []atributoEscribir{
						id(idEntradaAcepto), ancho(), alto(),
						atributoDeEntrada("text", 0x0101014f, tipoCadena, 0, "Acepto"),
						// la casilla la deja marcada el layout al inflar: es el otro estado que la
						// siembra tiene que respetar
						atributoDeEntrada("checked", 0x01010106, tipoEnteroDec, 1, ""),
					},
				},
				{
					nombre: "RadioGroup",
					linea:  16,
					attrs: []atributoEscribir{
						id(idEntradaGrupo), ancho(), alto(),
						// la opción que el layout deja marcada al inflar
						atributoDeEntrada("checkedButton", 0x01010215, tipoReferencia, idEntradaRojo, ""),
					},
					hijos: []*elementoEscribir{
						{
							nombre: "RadioButton",
							linea:  18,
							attrs: []atributoEscribir{
								id(idEntradaRojo), ancho(), alto(),
								atributoDeEntrada("text", 0x0101014f, tipoCadena, 0, "Rojo"),
							},
						},
						{
							nombre: "RadioButton",
							linea:  21,
							attrs: []atributoEscribir{
								id(idEntradaVerde), ancho(), alto(),
								atributoDeEntrada("text", 0x0101014f, tipoCadena, 0, "Verde"),
							},
						},
					},
				},
				{
					nombre: "Button",
					linea:  26,
					attrs: []atributoEscribir{
						id(idEntradaEnviar), ancho(), alto(),
						atributoDeEntrada("text", 0x0101014f, tipoCadena, 0, "Enviar"),
					},
				},
			},
		},
	}
}

// fuenteEntrada es la actividad del caso: registra un TextWatcher que copia a `eco` lo tecleado, un
// OnCheckedChangeListener en la casilla y otro en cada radio —cada uno lleva su cuenta, para que la
// consola diga cuántas veces corrió— y lee `getEditableText` en el onClick del botón. El emisor del
// fork rechaza el booleano suelto en un `if` y la concatenación de un `boolean`, así que la fuente se
// queda en lo que el propio proyecto sabe compilar: el estado se comprueba en el modelo y en la
// consola, no dentro del fuente.
const fuenteEntrada = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.text.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    EditText nombre;
    TextView eco;
    CheckBox acepto;
    int avisosAcepto = 0;
    int avisosRojo = 0;
    int avisosVerde = 0;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        nombre = (EditText) findViewById(R.id.nombre);
        eco = (TextView) findViewById(R.id.eco);
        nombre.setHint("Escribe aquí");
        nombre.addTextChangedListener(new TextWatcher() {
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }
            public void onTextChanged(CharSequence s, int start, int before, int count) {
                eco.setText("Eco: " + s.toString());
                System.out.println("tecleo=" + s.toString());
            }
            public void afterTextChanged(Editable s) {
                System.out.println("despues=" + s.toString());
            }
        });
        acepto = (CheckBox) findViewById(R.id.acepto);
        acepto.setOnCheckedChangeListener(new CompoundButton.OnCheckedChangeListener() {
            public void onCheckedChanged(CompoundButton boton, boolean marcado) {
                avisosAcepto = avisosAcepto + 1;
                System.out.println("acepto=" + avisosAcepto);
            }
        });
        RadioButton rojo = (RadioButton) findViewById(R.id.rojo);
        rojo.setOnCheckedChangeListener(new CompoundButton.OnCheckedChangeListener() {
            public void onCheckedChanged(CompoundButton boton, boolean marcado) {
                avisosRojo = avisosRojo + 1;
                System.out.println("rojo=" + avisosRojo);
            }
        });
        RadioButton verde = (RadioButton) findViewById(R.id.verde);
        verde.setOnCheckedChangeListener(new CompoundButton.OnCheckedChangeListener() {
            public void onCheckedChanged(CompoundButton boton, boolean marcado) {
                avisosVerde = avisosVerde + 1;
                System.out.println("verde=" + avisosVerde);
            }
        });
        findViewById(R.id.enviar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                eco.setText("Enviado: " + nombre.getEditableText().toString());
            }
        });
    }
}
`

// apkDeEntrada arma el apk de la pantalla: el manifiesto del contador, su tabla, el layout de E5 y el
// dex que sale del emisor del fork.
func apkDeEntrada(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoConActividad(".MainActivity"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDeEntrada())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDeEntrada())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDeEntrada())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteEntrada}),
	})
}

// nodoDeEntrada busca un nodo por el id de recurso de la vista; el modelo publica ese id como
// `idRef` en hexadecimal.
func nodoDeEntrada(n *nodoVista, id uint32) *nodoVista {
	if n == nil {
		return nil
	}
	if n.IDRef == refDeEntrada(id) {
		return n
	}
	for _, h := range n.Hijos {
		if encontrado := nodoDeEntrada(h, id); encontrado != nil {
			return encontrado
		}
	}
	return nil
}

// refDeEntrada es el `idRef` que el modelo publica para un id: el mismo formato en hexadecimal.
func refDeEntrada(id uint32) string { return fmt.Sprintf("%#08x", id) }

// consolaDeEntrada junta la consola en una sola cadena, para buscar líneas concretas.
func consolaDeEntrada(e *resultadoEjecucion) string {
	lineas := make([]string, 0, len(e.Consola))
	for _, l := range e.Consola {
		lineas = append(lineas, l.Texto)
	}
	return strings.Join(lineas, "\n")
}

// TestElPreviewEscribeYMarcaLoQueSeLePide es el criterio de E5: una app con EditText, CheckBox y
// RadioGroup enseña en el modelo lo que se le teclea y se le marca, los oyentes del proyecto corren
// (su salida llega a la consola) y las órdenes se pueden mezclar con `--pulsar` en el mismo guion.
func TestElPreviewEscribeYMarcaLoQueSeLePide(t *testing.T) {
	apk := apkDeEntrada(t)

	// en reposo: el layout manda —la pista del EditText, la casilla marcada por `android:checked` y el
	// radio rojo marcado por `android:checkedButton`— y no hay ni capturas ni consola.
	reposo, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk en reposo no se lee: %v", err)
	}
	if reposo.Ejecucion == nil || len(reposo.Ejecucion.Avisos) != 0 {
		t.Fatalf("el reposo dejó avisos: %+v", reposo.Ejecucion)
	}
	if len(reposo.Ejecucion.Capturas) != 0 || len(reposo.Ejecucion.Consola) != 0 {
		t.Errorf("en reposo no hay capturas ni consola: %+v", reposo.Ejecucion)
	}
	if n := nodoDeEntrada(reposo.Raiz, idEntradaNombre); n == nil || n.Pista != "Escribe aquí" {
		t.Errorf("el EditText no publica su pista: %+v", n)
	}
	if n := nodoDeEntrada(reposo.Raiz, idEntradaAcepto); n == nil || n.Marcado == nil || !*n.Marcado {
		t.Errorf("la casilla no arranca marcada por `android:checked`: %+v", n)
	}
	if n := nodoDeEntrada(reposo.Raiz, idEntradaRojo); n == nil || n.Marcado == nil || !*n.Marcado {
		t.Errorf("el radio que marca `checkedButton` no sale marcado: %+v", n)
	}
	if n := nodoDeEntrada(reposo.Raiz, idEntradaVerde); n == nil || n.Marcado == nil || *n.Marcado {
		t.Errorf("el radio verde arranca marcado: %+v", n)
	}

	// el guion: teclear, desmarcar y volver a marcar la casilla, cambiar el radio y pulsar el botón
	// que lee lo tecleado. El orden importa: el onClick ve el texto y la marca que dejaron las
	// órdenes anteriores.
	guion := []eventoUsuario{
		{Tipo: "escribir", Dato: "id/nombre/hola mundo"},
		{Tipo: "desmarcar", Dato: "id/acepto"},
		{Tipo: "marcar", Dato: "id/acepto"},
		{Tipo: "marcar", Dato: "id/verde"},
		{Tipo: "pulsar", Dato: "id/enviar"},
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("el guion devolvió error: %v", err)
	}
	e := m.Ejecucion
	if len(e.Avisos) != 0 {
		t.Errorf("el guion dejó avisos: %v", e.Avisos)
	}
	if len(e.Escrituras) != 1 || e.Escrituras[0] != "id/nombre/hola mundo" {
		t.Errorf("las escrituras entregadas son %v", e.Escrituras)
	}
	if e.Interacciones != len(guion) {
		t.Errorf("las interacciones son %d y el guion entregó %d órdenes", e.Interacciones, len(guion))
	}

	// el modelo: el texto tecleado queda en el EditText, el TextWatcher copió a `eco` y el onClick
	// reescribió `eco` con `getEditableText().toString()` y `isChecked()`.
	if e := nodoDeEntrada(m.Raiz, idEntradaNombre); e == nil || e.TextoDinamico != "hola mundo" {
		t.Errorf("el EditText no refleja lo tecleado: %+v", e)
	}
	if eco := nodoDeEntrada(m.Raiz, idEntradaEco); eco == nil ||
		eco.TextoDinamico != "Enviado: hola mundo" {
		t.Errorf("el TextView del eco es %+v y se esperaba el texto del onClick", eco)
	}
	// las marcas: la casilla marcada, el verde ganó el grupo y el rojo quedó desmarcado
	if n := nodoDeEntrada(m.Raiz, idEntradaAcepto); n == nil || n.Marcado == nil || !*n.Marcado {
		t.Errorf("la casilla no quedó marcada: %+v", n)
	}
	if n := nodoDeEntrada(m.Raiz, idEntradaVerde); n == nil || n.Marcado == nil || !*n.Marcado {
		t.Errorf("el radio verde no quedó marcado: %+v", n)
	}
	if n := nodoDeEntrada(m.Raiz, idEntradaRojo); n == nil || n.Marcado == nil || *n.Marcado {
		t.Errorf("el RadioGroup dejó dos opciones marcadas: el rojo sigue en %+v", n)
	}

	// las capturas y la consola: lo tecleado es una escritura del usuario y los tres callbacks del
	// proyecto corrieron.
	if len(e.Capturas) < 1 || e.Capturas[0].Origen != "usuario.escribir" ||
		e.Capturas[0].ID != "id/nombre" || e.Capturas[0].Texto != "hola mundo" {
		t.Errorf("la captura del tecleo es %+v", e.Capturas)
	}
	consola := consolaDeEntrada(e)
	for _, esperado := range []string{"tecleo=hola mundo", "despues=hola mundo",
		"acepto=1", "acepto=2", "rojo=1", "verde=1"} {
		if !strings.Contains(consola, esperado) {
			t.Errorf("la consola no trae %q: %q", esperado, consola)
		}
	}
	if len(e.Marcas) != 3 {
		t.Errorf("las marcas publicadas son %+v y se esperaban la casilla y los dos radios", e.Marcas)
	}

	// desmarcar la casilla vuelve a avisar al oyente con false, y el modelo lo refleja
	soloBaja := []eventoUsuario{{Tipo: "desmarcar", Dato: "id/acepto"}}
	baja, err := modeloVistaDeAPKConEventos(apk, "", true, soloBaja)
	if err != nil {
		t.Fatalf("el desmarcado falló: %v", err)
	}
	if n := nodoDeEntrada(baja.Raiz, idEntradaAcepto); n == nil || n.Marcado == nil || *n.Marcado {
		t.Errorf("desmarcar no limpió la casilla: %+v", n)
	}
	if consola := consolaDeEntrada(baja.Ejecucion); !strings.Contains(consola, "acepto=1") {
		t.Errorf("desmarcar no avisó al oyente: %q", consola)
	}

	// marcar la casilla que ya está marcada por el layout no dispara nada: el framework tampoco avisa
	// cuando el estado no cambia
	sinCambio, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: "marcar", Dato: "id/acepto"}})
	if err != nil {
		t.Fatalf("marcar la casilla ya marcada falló: %v", err)
	}
	if consola := consolaDeEntrada(sinCambio.Ejecucion); strings.Contains(consola, "acepto=") {
		t.Errorf("marcar la casilla ya marcada avisó al oyente: %q", consola)
	}

	// y desmarcar un radio que ya está sin marcar tampoco avisa
	sigueLibre, err := modeloVistaDeAPKConEventos(apk, "", true,
		[]eventoUsuario{{Tipo: "desmarcar", Dato: "id/verde"}})
	if err != nil {
		t.Fatalf("desmarcar el radio libre falló: %v", err)
	}
	if consola := consolaDeEntrada(sigueLibre.Ejecucion); strings.Contains(consola, "verde=") {
		t.Errorf("desmarcar un radio ya libre avisó al oyente: %q", consola)
	}
}

// TestLosStubsDeLaCasillaDevuelvenYCambianElEstado cierra los stubs de la casilla que la fuente de la
// prueba no puede ejercitar: el emisor del fork todavía rechaza un booleano suelto en un `if`, así que
// `isChecked()`, `setChecked()` y `toggle()` se comprueban sobre la máquina, que es donde el
// intérprete recibe la llamada del dex de cualquier app.
func TestLosStubsDeLaCasillaDevuelvenYCambianElEstado(t *testing.T) {
	m := maquinaDelContador(t)
	casilla := m.vista(0x7f010001)
	casilla.clase = "Landroid/widget/CheckBox;"
	llama := func(nombre, proto string, vals []any) any {
		t.Helper()
		v, ok, err := m.intentaStub([]string{"Landroid/widget/CheckBox;", "Landroid/widget/CompoundButton;"},
			nombre, proto, vals, casilla)
		if err != nil || !ok {
			t.Fatalf("%s no se atendió: ok=%v err=%v", nombre, ok, err)
		}
		return v
	}
	if v := llama("isChecked", "()Z", nil); v != int32(0) {
		t.Errorf("una casilla sin marcar devuelve %v en isChecked", v)
	}
	llama("setChecked", "(Z)V", []any{int32(1)})
	if v := llama("isChecked", "()Z", nil); v != int32(1) {
		t.Errorf("tras setChecked(true) isChecked devuelve %v", v)
	}
	llama("toggle", "()V", nil)
	if v := llama("isChecked", "()Z", nil); v != int32(0) {
		t.Errorf("tras toggle isChecked devuelve %v", v)
	}
	if len(m.avisos) != 0 {
		t.Errorf("los stubs de la casilla dejaron avisos: %v", m.avisos)
	}
}

// TestElPreviewRechazaPorNombreLasOrdenesQueNoVan fija las fronteras de E5: teclear en algo que no es
// un EditText, marcar algo que no se marca y tocar un id que la tabla no tiene devuelven un error que
// nombra la vista y el motivo, en vez de un modelo a medias.
func TestElPreviewRechazaPorNombreLasOrdenesQueNoVan(t *testing.T) {
	apk := apkDeEntrada(t)

	casos := []struct {
		nombre string
		guion  []eventoUsuario
		espera string
	}{
		{"teclear en un TextView", []eventoUsuario{{Tipo: "escribir", Dato: "id/eco/hola"}},
			"solo se teclea"},
		{"marcar un TextView", []eventoUsuario{{Tipo: "marcar", Dato: "id/eco"}},
			"solo se marcan"},
		{"un id que no está", []eventoUsuario{{Tipo: "marcar", Dato: "id/no-existe"}},
			"la tabla no tiene"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := modeloVistaDeAPKConEventos(apk, "", true, c.guion)
			if err == nil {
				t.Fatalf("%s se aceptó", c.nombre)
			}
			if !strings.Contains(err.Error(), c.espera) {
				t.Errorf("el error de %s es %q y tenía que nombrar %q", c.nombre, err, c.espera)
			}
		})
	}

	// el flag exige la forma id/texto: sin la barra no hay id que resolver y el error lo dice antes de
	// ejecutar nada. `id/nombre` sí es válido: es el id sin texto, que vacía el campo.
	eventos := []eventoUsuario{}
	if err := (guionDeEntrada{tipo: "escribir", eventos: &eventos}).Set("idnombre"); err == nil {
		t.Error("--escribir aceptó un valor sin texto")
	}
	if err := (guionDeEntrada{tipo: "marcar", eventos: &eventos}).Set("  "); err == nil {
		t.Error("--marcar aceptó un id vacío")
	}
	if err := (guionDeEntrada{tipo: "escribir", eventos: &eventos}).Set("id/nombre"); err != nil {
		t.Errorf("--escribir rechazó el id sin texto, que es vaciar el campo: %v", err)
	}
	if len(eventos) != 1 {
		t.Errorf("los flags aceptados dejan %d eventos y se esperaba uno: %+v", len(eventos), eventos)
	}
}

// TestElGuionDeEntradaNoTieneTope mezcla tecleos, marcas y clics cuarenta veces: el emulador no
// recorta el guion ni avisa de un tope, porque las órdenes las pone el usuario y todas se entregan.
// Es la mitad de E5 de la prueba de los 100 clics: aquí el texto y la marca también pasan de largo
// de cualquier cifra redonda, y el estado que lee el último `onClick` es el que dejó la orden 160.
func TestElGuionDeEntradaNoTieneTope(t *testing.T) {
	apk := apkDeEntrada(t)
	guion := make([]eventoUsuario, 0, 160)
	for i := 1; i <= 40; i++ {
		guion = append(guion,
			eventoUsuario{Tipo: "escribir", Dato: fmt.Sprintf("id/nombre/hola %d", i)},
			eventoUsuario{Tipo: "marcar", Dato: "id/acepto"},
			eventoUsuario{Tipo: "desmarcar", Dato: "id/acepto"},
			eventoUsuario{Tipo: "pulsar", Dato: "id/enviar"},
		)
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("las %d órdenes devolvieron error: %v", len(guion), err)
	}
	if len(m.Ejecucion.Escrituras) != 40 {
		t.Errorf("se entregaron %d tecleos de 40", len(m.Ejecucion.Escrituras))
	}
	if len(m.Ejecucion.Pulsaciones) != 40 {
		t.Errorf("se entregaron %d clics de 40", len(m.Ejecucion.Pulsaciones))
	}
	if m.Ejecucion.Interacciones < len(guion) {
		t.Errorf("las interacciones son %d y no llegan a las %d órdenes", m.Ejecucion.Interacciones,
			len(guion))
	}
	if avisos := strings.Join(m.Ejecucion.Avisos, "\n"); strings.Contains(avisos, "tope") {
		t.Errorf("no debería haber aviso de tope: %v", m.Ejecucion.Avisos)
	}
	if n := nodoDeEntrada(m.Raiz, idEntradaEco); n == nil || n.TextoDinamico != "Enviado: hola 40" {
		t.Errorf("tras el guion el eco es %+v", n)
	}
	if n := nodoDeEntrada(m.Raiz, idEntradaAcepto); n == nil || n.Marcado == nil || *n.Marcado {
		t.Errorf("la casilla tenía que quedar desmarcada por la última orden: %+v", n)
	}
}
