package main

// Las pruebas de la Fase 2 de la vista previa: el intérprete Dalvik.
//
// El dex de las pruebas no se versiona (regla 2): se genera en caliente con el emisor del fork
// (`compilarProyectoJava` + `dexDeClases`), que es el mismo camino que usa `apkc build --compiler go`.
// El R.java también se genera con `rDeTabla` desde la tabla que escribe `arscwriter.go`, para que los
// ids del fuente no discrepen de los del apk.
//
// La prueba que cierra el criterio de aceptación de la fase es
// TestElInterpreteEjecutaOnCreateYCapturaElSetText: la plantilla Contador del studio corre y su
// `total.setText("Pulsos: " + pulsos)` sale capturado.

import (
	"strings"
	"testing"
)

// dexDeFuentes compila un proyecto Java con el emisor del fork y devuelve su classes.dex.
func dexDeFuentes(t *testing.T, fuentes ...fuenteJava) []byte {
	t.Helper()
	emitidas, err := compilarProyectoJava(fuentes, jarDelSDK(t))
	if err != nil {
		t.Fatalf("las fuentes no compilan: %v", err)
	}
	completas := make([]classFull, 0, len(emitidas))
	for _, c := range emitidas {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", c.interno, err)
		}
		completas = append(completas, cf)
	}
	datos, err := dexDeClases(completas)
	if err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
	return datos
}

// tablaDelContador es la tabla de recursos de la plantilla Contador: los dos ids del layout y el
// layout, con la ruta en el pool global, que es lo que resuelve un id de `R.layout`.
func tablaDelContador() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.contador",
		global:        []string{"res/layout/main.xml", "Hola desde un recurso"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout", "string"},
		nombresUTF8:   false,
		claves:        []string{"pulsar", "total", "main", "saludo"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 2, valor: valorEscribir{tipo: 0x03, datos: 0}},
				}}},
			},
			{
				id:        3,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 3, valor: valorEscribir{tipo: 0x03, datos: 1}},
				}}},
			},
		},
	}
}

// layoutDelContador es el layout de la plantilla, con los dos ids que usa su actividad.
func layoutDelContador() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: 0x010100c4, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010001},
						{nombre: "style", tipo: tipoReferencia, datos: 0x01030042},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Pulsos: 0"},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Pulsar"},
					},
				},
			},
		},
	}
}

// manifiestoDelContador: la actividad lanzadora es `.MainActivity`.
func manifiestoDelContador() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.contador"},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "application",
					linea:  9,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Contador"},
					},
					hijos: []*elementoEscribir{
						{
							nombre: "activity",
							linea:  11,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: ".MainActivity"},
							},
							hijos: []*elementoEscribir{
								{
									nombre: "intent-filter",
									linea:  1,
									hijos: []*elementoEscribir{
										{
											nombre: "action",
											linea:  2,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
													texto: "android.intent.action.MAIN"},
											},
										},
										{
											nombre: "category",
											linea:  3,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
													texto: "android.intent.category.LAUNCHER"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// fuenteContadorStudio es la actividad de la plantilla Contador del studio, copiada de su texto
// generado: campo, listener anónimo y concatenación con un entero.
const fuenteContadorStudio = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText("Pulsos: " + pulsos);
            }
        });
    }
}
`

// fuentesDelContador arma el proyecto: el R.java sale de la tabla, para que los ids no discrepen.
func fuentesDelContador(t *testing.T) []fuenteJava {
	t.Helper()
	r, err := rDeTabla(tablaDelContador())
	if err != nil {
		t.Fatalf("el R.java no sale de la tabla: %v", err)
	}
	return []fuenteJava{
		{nombre: "R.java", contenido: r},
		{nombre: "MainActivity.java", contenido: fuenteContadorStudio},
	}
}

// TestElInterpreteEjecutaOnCreateYCapturaElSetText es el criterio de aceptación de la Fase 2 sobre el
// dex, sin apk: el onCreate del Contador corre, el layout sale de su setContentView y la pulsación
// simulada deja `Pulsos: 1` capturado sobre el id 0x7f010001.
func TestElInterpreteEjecutaOnCreateYCapturaElSetText(t *testing.T) {
	d, err := readDex(dexDeFuentes(t, fuentesDelContador(t)...))
	if err != nil {
		t.Fatalf("el dex del Contador no se relee: %v", err)
	}
	// la primera ejecución va sin tabla (el id se queda en su número) y la segunda con la del propio
	// fork, que sí lo nombra
	res, layoutID := ejecutaOnCreate(d, nil, "Lcom/hola/contador/MainActivity;")
	if res.Error != "" {
		t.Fatalf("el onCreate falló: %s", res.Error)
	}
	if res.Actividad != "com.hola.contador.MainActivity" {
		t.Errorf("se ejecutó %q y la lanzadora es com.hola.contador.MainActivity", res.Actividad)
	}
	if res.Metodo != "onCreate" {
		t.Errorf("se ejecutó %q y se esperaba onCreate", res.Metodo)
	}
	if layoutID != 0x7f020000 {
		t.Errorf("el setContentView dio el layout %#08x y R.layout.main es 0x7f020000", layoutID)
	}
	if res.Pasos == 0 {
		t.Error("el intérprete no contó ni un paso")
	}
	if res.Interacciones != 1 {
		t.Errorf("se pulsaron %d listeners y el Contador registra uno", res.Interacciones)
	}
	if len(res.Capturas) != 1 {
		t.Fatalf("%d capturas: %+v", len(res.Capturas), res.Capturas)
	}
	c := res.Capturas[0]
	if c.Origen != "TextView.setText" || c.Texto != "Pulsos: 1" {
		t.Errorf("la captura es %q con texto %q", c.Origen, c.Texto)
	}
	if c.IDRef != "0x7f010001" {
		t.Errorf("la captura apunta a %q y R.id.total es 0x7f010001", c.IDRef)
	}
	if len(res.Avisos) != 0 {
		t.Errorf("la ejecución dejó avisos que no debería: %v", res.Avisos)
	}
	// y con la tabla del propio fork el id se nombra
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla del Contador no se escribe: %v", err)
	}
	tablaFork, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla del Contador no se relee: %v", err)
	}
	res, _ = ejecutaOnCreate(d, tablaFork, "Lcom/hola/contador/MainActivity;")
	if len(res.Capturas) != 1 || res.Capturas[0].ID != "id/total" {
		t.Fatalf("con la tabla del fork la captura es %+v y se esperaba id/total", res.Capturas)
	}
}

// TestElPreviewLlevaElTextoDinamicoAlModelo cierra la fase de punta a punta sin sdk de por medio: un
// apk armado en memoria con los escritores del fork y el classes.dex del Contador. El nodo conserva
// el texto declarado y añade el que la app escribió.
func TestElPreviewLlevaElTextoDinamicoAlModelo(t *testing.T) {
	manifiesto, err := escribeAXML(manifiestoDelContador())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelContador())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk del Contador no se deja leer: %v", err)
	}
	if m.Ejecucion == nil {
		t.Fatal("el modelo no trae la ejecución")
	}
	if m.Ejecucion.Error != "" {
		t.Fatalf("la ejecución falló: %s", m.Ejecucion.Error)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("el layout se resolvió como %q", m.Layout)
	}
	if m.Raiz == nil || len(m.Raiz.Hijos) != 2 {
		t.Fatalf("el modelo no tiene los dos hijos del layout: %+v", m.Raiz)
	}
	texto, boton := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if texto.Texto != "Pulsos: 0" {
		t.Errorf("el texto declarado del TextView es %q", texto.Texto)
	}
	if texto.TextoDinamico != "Pulsos: 1" {
		t.Errorf("el texto dinámico del TextView es %q y la app escribió «Pulsos: 1»", texto.TextoDinamico)
	}
	if boton.Texto != "Pulsar" || boton.TextoDinamico != "" {
		t.Errorf("el Button se lee como %q / %q y nadie le escribió texto", boton.Texto, boton.TextoDinamico)
	}
	// el ancho estimado usa el texto que se ve: «Pulsos: 1» mide lo mismo que «Pulsos: 0»
	if texto.Rect.Ancho != len([]rune("Pulsos: 0"))*dpPorCaracter+dpRellenoTexto {
		t.Errorf("el ancho del TextView es %d", texto.Rect.Ancho)
	}
}

// TestElPreviewSinEjecutarNoInterpreta: la bandera de escape deja el modelo de la Fase 1, aunque el
// apk traiga un dex que sí se ejecuta.
func TestElPreviewSinEjecutarNoInterpreta(t *testing.T) {
	manifiesto, _ := escribeAXML(manifiestoDelContador())
	layout, _ := escribeAXML(layoutDelContador())
	arsc, _ := escribeTablaRecursos(tablaDelContador())
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
	m, err := modeloVistaDeAPKCon(apk, "", false)
	if err != nil {
		t.Fatalf("el apk no se lee: %v", err)
	}
	if m.Ejecucion != nil {
		t.Errorf("se ejecutó el dex con --sin-ejecutar: %+v", m.Ejecucion)
	}
	if m.Raiz.Hijos[0].TextoDinamico != "" {
		t.Errorf("sin ejecutar no puede haber texto dinámico: %q", m.Raiz.Hijos[0].TextoDinamico)
	}
}

// TestElInterpreteDecodificaCuentasRamasArreglosYAnchos cubre el decodificador con una sonda: bucles,
// `if`, `switch`, arreglos, `long`, cadenas y las dos salidas (`println` y `Log.d`).
func TestElInterpreteDecodificaCuentasRamasArreglosYAnchos(t *testing.T) {
	const sonda = `package com.myapp;

import android.util.Log;

public class Sonda {
    static int suma(int n) { int t = 0; for (int i = 0; i < n; i++) { t += i; } return t; }
    static long producto(long a, long b) { return a * b + 1; }
    static int arreglo() { int[] v = new int[3]; v[0] = 1; v[1] = 2; v[2] = 3; return v[0] + v[1] + v[2]; }
    static int ramas(int x) { if (x > 10) { return 1; } else if (x == 10) { return 2; } return 3; }
    static int casos(int x) { switch (x) { case 1: return 10; case 2: return 20; default: return 0; } }
    static int cuenta(int x) { int i = 0; while (i < x) { i = i + 1; } return i; }

    public static void main(String[] args) {
        System.out.println("suma " + suma(5));
        System.out.println(producto(3, 4));
        System.out.println("arreglo " + arreglo());
        System.out.println("ramas " + ramas(10));
        System.out.println("casos " + casos(2));
        System.out.println("cuenta " + cuenta(4));
        Log.d("Sonda", "listo");
    }
}
`
	d, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Sonda.java", contenido: sonda}))
	if err != nil {
		t.Fatalf("la sonda no se relee: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/myapp/Sonda;")
	if res.Error != "" {
		t.Fatalf("la sonda falló: %s", res.Error)
	}
	if res.Metodo != "main" || res.Actividad != "com.myapp.Sonda" {
		t.Errorf("se ejecutó %s.%s", res.Actividad, res.Metodo)
	}
	quieren := []string{"suma 10", "13", "arreglo 6", "ramas 2", "casos 20", "cuenta 4", "listo"}
	obtenidos := make([]string, 0, len(res.Capturas))
	for _, c := range res.Capturas {
		obtenidos = append(obtenidos, c.Texto)
	}
	if strings.Join(obtenidos, " | ") != strings.Join(quieren, " | ") {
		t.Errorf("las capturas son\n  %v\ny se esperaban\n  %v", obtenidos, quieren)
	}
	if res.Capturas[len(res.Capturas)-1].Origen != "Log.d" {
		t.Errorf("la última captura viene de %q y es un Log.d", res.Capturas[len(res.Capturas)-1].Origen)
	}
}

// TestElInterpreteEligeLaActividadYNoLaAnonima: el fallo medido con NekoDroid sobre un apk real era
// coger la clase anónima. Aunque se proponga `MainActivity$1`, la elegida tiene que ser la actividad.
func TestElInterpreteEligeLaActividadYNoLaAnonima(t *testing.T) {
	d, err := readDex(dexDeFuentes(t, fuentesDelContador(t)...))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := cargaPrograma(d)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	if _, ok := prog.clases["Lcom/hola/contador/MainActivity$1;"]; !ok {
		t.Fatal("la anónima no está en el dex: la prueba no estaría midiendo nada")
	}
	for _, preferida := range []string{"", "Lcom/hola/contador/MainActivity$1;",
		"Lcom/hola/contador/NoExiste;"} {
		clase, conMain := eligeClaseLanzadora(prog, preferida)
		if clase != "Lcom/hola/contador/MainActivity;" || conMain {
			t.Errorf("con preferida %q se eligió %q (main=%v)", preferida, clase, conMain)
		}
	}
	// y una clase con main gana a una con solo onCreate
	if clase, conMain := eligeClaseLanzadora(prog, ""); conMain {
		t.Errorf("%q se tomó por una clase con main", clase)
	}
}

// TestElDescriptorDeLaActividadCubreLasTresFormas: `.MainActivity`, `MainActivity` y el nombre
// completo son el mismo type_id.
func TestElDescriptorDeLaActividadCubreLasTresFormas(t *testing.T) {
	for _, nombre := range []string{".MainActivity", "MainActivity", "com.hola.contador.MainActivity"} {
		if got := descriptorDeActividad("com.hola.contador", nombre); got != "Lcom/hola/contador/MainActivity;" {
			t.Errorf("%q dio %q", nombre, got)
		}
	}
	if got := descriptorDeActividad("com.hola.contador", ""); got != "" {
		t.Errorf("sin nombre salió %q", got)
	}
}

// TestElStubSinCuerpoCapturaPrintInt: `printInt` es la salida de las apps que no usan TextView; sin
// cuerpo en el dex, el intérprete captura su argumento en vez de avisar.
func TestElStubSinCuerpoCapturaPrintInt(t *testing.T) {
	prog := &programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}
	m := nuevaMaquina(prog, nil)
	met := &metodoDalvik{nombre: "printInt", proto: "(I)V", retorno: "V"}
	if _, err := m.stubSinCuerpo("Lcom/myapp/Salida;", met, []any{int32(42)}); err != nil {
		t.Fatalf("printInt falló: %v", err)
	}
	if len(m.capturas) != 1 || m.capturas[0].Origen != "printInt" || m.capturas[0].Texto != "42" {
		t.Fatalf("printInt capturó %+v", m.capturas)
	}
	// y lo que no es un stub se avisa, no se finge ejecutado
	otro := &metodoDalvik{nombre: "dibuja", proto: "(I)V", retorno: "V"}
	if _, err := m.stubSinCuerpo("Lcom/myapp/Salida;", otro, []any{int32(1)}); err != nil {
		t.Fatalf("dibuja falló: %v", err)
	}
	if len(m.avisos) != 1 {
		t.Errorf("el método desconocido dejó %d avisos", len(m.avisos))
	}
}

// TestElInterpreteRechazaLasExcepcionesPorSuNombre mide la frontera: un `try`/`catch` no se ejecuta
// a medias, se dice que no se ejecuta.
func TestElInterpreteRechazaLasExcepcionesPorSuNombre(t *testing.T) {
	const conTry = `package com.myapp;
public class Riesgo {
    static int divide(int a, int b) {
        try { return a / b; } catch (ArithmeticException e) { return -1; }
    }
    public static void main(String[] args) {
        System.out.println("resultado " + divide(4, 2));
    }
}
`
	d, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Riesgo.java", contenido: conTry}))
	if err != nil {
		t.Fatalf("el fuente no se dexea: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/myapp/Riesgo;")
	if !strings.Contains(res.Error, "tabla de excepciones") {
		t.Fatalf("el try/catch no se rechazó por su nombre: %q", res.Error)
	}
	if len(res.Capturas) != 0 {
		t.Errorf("se capturó texto de una ejecución que se detuvo: %+v", res.Capturas)
	}
}

// TestElInterpreteResuelveSetTextConUnRecurso: `setText(R.string.saludo)` llega al stub como su id;
// la tabla del apk es la que lo convierte en el texto, igual que el layout resuelve `@string/x`.
func TestElInterpreteResuelveSetTextConUnRecurso(t *testing.T) {
	const conRecurso = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.widget.*;

public class Recurso extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        total.setText(R.string.saludo);
    }
}
`
	r, err := rDeTabla(tablaDelContador())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	d, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: r},
		fuenteJava{nombre: "Recurso.java", contenido: conRecurso}))
	if err != nil {
		t.Fatalf("el dex: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	tabla, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla no se relee: %v", err)
	}
	res, layoutID := ejecutaOnCreate(d, tabla, "Lcom/hola/contador/Recurso;")
	if res.Error != "" {
		t.Fatalf("la ejecución falló: %s", res.Error)
	}
	if layoutID != 0x7f020000 {
		t.Errorf("el layout es %#08x", layoutID)
	}
	if len(res.Capturas) != 1 || res.Capturas[0].Texto != "Hola desde un recurso" ||
		res.Capturas[0].ID != "id/total" {
		t.Fatalf("la captura es %+v y se esperaba el texto del recurso sobre id/total", res.Capturas)
	}
}
