package main

// Las pruebas de la sesión viva: una sola máquina, una sola orden por entrega, sin replay. El
// proveedor del wasm se sustituye por llamadas directas a ejecutaOrden, que es puro y no sale.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const claseListenerContador = "Lcom/hola/contador/MainActivity$1;"

func dexDelContadorCon(t *testing.T, incremento string) []byte {
	t.Helper()
	r := rContador(t)
	fuente := strings.Replace(fuenteContadorStudio, "pulsos + 1", "pulsos + "+incremento, 1)
	if fuente == fuenteContadorStudio {
		t.Fatal("la fuente del contador no trae el sumador que se va a cambiar")
	}
	return dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: r},
		fuenteJava{nombre: "MainActivity.java", contenido: fuente})
}

func fuenteContadorConLog(incremento string) string {
	return strings.Replace(fuenteContadorStudio,
		"pulsos = pulsos + 1;",
		"android.util.Log.d(\"Contador\", \"clic\"); pulsos = pulsos + "+incremento+";", 1)
}

func dexDelContadorConLog(t *testing.T, incremento string) []byte {
	t.Helper()
	return dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteContadorConLog(incremento)})
}

func sesionDelContador(t *testing.T) *sesionPreview {
	t.Helper()
	apk := apkDelContador(t, fuentesDelContador(t)...)
	s, err := nuevaSesionPreview(apk, "", "")
	if err != nil {
		t.Fatalf("la sesión no arrancó: %v", err)
	}
	return s
}

// ordenSwap escribe el dex nuevo donde el parser de --intercambiar puede leerlo y devuelve el argv.
func ordenSwap(t *testing.T, dex []byte) []string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "nuevo.dex")
	if err := os.WriteFile(ruta, dex, 0o644); err != nil {
		t.Fatalf("el dex nuevo no se escribió: %v", err)
	}
	return []string{"--intercambiar", claseListenerContador + "/onClick(Landroid/view/View;)V=" + ruta}
}

func entrega(t *testing.T, s *sesionPreview, id string, argv []string) (string, int) {
	t.Helper()
	var linea string
	codigo := s.ejecutaOrden(id, argv, func(l string) { linea = l })
	if linea == "" {
		t.Fatalf("la orden %s no publicó una línea JSON", id)
	}
	return linea, codigo
}

// totalDe lee el último texto del TextView del contador del modelo que publicó la orden.
func totalDe(t *testing.T, linea string) string {
	t.Helper()
	var modelo struct {
		Ejecucion struct {
			Capturas []struct {
				ID    string `json:"id"`
				Texto string `json:"texto"`
			} `json:"capturas"`
		} `json:"ejecucion"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo no se lee: %v", err)
	}
	for i := len(modelo.Ejecucion.Capturas) - 1; i >= 0; i-- {
		if modelo.Ejecucion.Capturas[i].ID == "id/total" {
			return modelo.Ejecucion.Capturas[i].Texto
		}
	}
	return ""
}

func TestLaSesionAplicaSoloLaOrdenNueva(t *testing.T) {
	s := sesionDelContador(t)
	publicadas := []string{}
	s.publica = func(id any, codigo int) {
		if codigo != 0 {
			t.Errorf("la orden %s devolvió código %d", id, codigo)
		}
		publicadas = append(publicadas, fmt.Sprint(id))
	}
	timeline := []string{"Pulsos: 0"}
	for n := 1; n <= 4; n++ {
		linea, _ := entrega(t, s, "clic-"+string(rune('0'+n)), []string{"--pulsar", "id/pulsar"})
		timeline = append(timeline, totalDe(t, linea))
	}
	linea, _ := entrega(t, s, "swap", ordenSwap(t, dexDelContadorCon(t, "5")))
	timeline = append(timeline, totalDe(t, linea))
	linea, _ = entrega(t, s, "clic-5", []string{"--pulsar", "id/pulsar"})
	timeline = append(timeline, totalDe(t, linea))
	quiero := []string{"Pulsos: 0", "Pulsos: 1", "Pulsos: 2", "Pulsos: 3", "Pulsos: 4",
		"Pulsos: 4", "Pulsos: 9"}
	if strings.Join(timeline, "|") != strings.Join(quiero, "|") {
		t.Fatalf("timeline = %v y se esperaba %v", timeline, quiero)
	}
	if got, want := strings.Join(publicadas, ","), "clic-1,clic-2,clic-3,clic-4,swap,clic-5"; got != want {
		t.Fatalf("fin-orden = %q y se esperaba %q", got, want)
	}
}

func TestLaSesionConservaElModeloAcumulado(t *testing.T) {
	// esta fuente sí escribe en la consola, para que el modelo acumule algo más que el texto
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteContadorConLog("1")})
	var err error
	s, err := nuevaSesionPreview(apk, "", "")
	if err != nil {
		t.Fatalf("la sesión no arrancó: %v", err)
	}
	entrega(t, s, "a", []string{"--pulsar", "id/pulsar"})
	entrega(t, s, "b", []string{"--pulsar", "id/pulsar"})
	if got := len(s.res.Pulsaciones); got != 2 {
		t.Fatalf("Pulsaciones = %v y se esperaban 2: la sesión replayó", s.res.Pulsaciones)
	}
	if len(s.res.Capturas) == 0 || len(s.res.Consola) == 0 || len(s.res.Oyentes) == 0 {
		t.Fatalf("el modelo acumulado está vacío: capturas=%d consola=%d oyentes=%d",
			len(s.res.Capturas), len(s.res.Consola), len(s.res.Oyentes))
	}
	entrega(t, s, "swap", ordenSwap(t, dexDelContadorConLog(t, "5")))
	if got := len(s.res.Intercambios); got != 1 {
		t.Fatalf("Intercambios = %d y se esperaba 1", got)
	}
	if got := len(s.res.Pulsaciones); got != 2 {
		t.Fatalf("el swap repitió los clics: Pulsaciones = %v", s.res.Pulsaciones)
	}
}

func TestLaSesionAplicaDeltaEnElCaminoReal(t *testing.T) {
	s := sesionDelContador(t)
	onCreate := s.m.buscaMetodo("Lcom/hola/contador/MainActivity;", "onCreate", "(Landroid/os/Bundle;)V")
	viejo := s.m.buscaMetodo(claseListenerContador, "onClick", "(Landroid/view/View;)V")
	if onCreate == nil || viejo == nil {
		t.Fatal("la sesión no trae onCreate ni el onClick del listener")
	}
	base := s.base
	entrega(t, s, "swap", ordenSwap(t, dexDelContadorCon(t, "5")))
	nuevo := s.m.buscaMetodo(claseListenerContador, "onClick", "(Landroid/view/View;)V")
	if nuevo == nil || nuevo == viejo {
		t.Fatal("el swap no enganchó el cuerpo nuevo del listener")
	}
	if s.m.buscaMetodo("Lcom/hola/contador/MainActivity;", "onCreate", "(Landroid/os/Bundle;)V") != onCreate {
		t.Fatal("el delta tocó onCreate, que no cambió")
	}
	if s.base == base {
		t.Fatal("la base del siguiente delta sigue siendo el dex viejo")
	}
	if len(s.res.Intercambios) != 1 || s.res.Intercambios[0].Metodo != "onClick" {
		t.Fatalf("el modelo publicó %+v y se esperaba un solo cambio de onClick", s.res.Intercambios)
	}
}

func TestLaSesionNoLlamaExitPorOrden(t *testing.T) {
	s := sesionDelContador(t)
	salidas := 0
	publicadas := 0
	s.salir = func(int) { salidas++ }
	s.publica = func(id any, codigo int) { publicadas++ }
	for n, argv := range [][]string{
		{"--pulsar", "id/pulsar"}, {"--pulsar", "id/pulsar"},
		{"--pulsar", "id/pulsar"}, {"--pulsar", "id/pulsar"},
	} {
		if codigo := s.ejecutaOrden(string(rune('1'+n)), argv, func(string) {}); codigo != 0 {
			t.Fatalf("la orden %d devolvió %d", n+1, codigo)
		}
	}
	if salidas != 0 {
		t.Fatalf("se llamó a salir %d veces desde una orden", salidas)
	}
	if publicadas != 4 {
		t.Fatalf("Publica se llamó %d veces y se esperaban 4", publicadas)
	}
	// y un posicional en la orden falla cerrado sin salir
	if codigo := s.ejecutaOrden("apk", []string{"--pulsar", "id/pulsar", "otro.apk"}, func(string) {}); codigo != 2 {
		t.Fatalf("una orden con apk devolvió %d y se esperaba 2", codigo)
	}
	if salidas != 0 || publicadas != 5 {
		t.Fatalf("tras la orden mala: salir=%d Publica=%d", salidas, publicadas)
	}
}

func TestElSegundoDeltaUsaLaBaseActualizada(t *testing.T) {
	// Un swap repetido contra la base ya actualizada no produce cambios: si la base se hubiera
	// quedado en el dex +1, el mismo dex +5 se volvería a instalar y habría dos intercambios.
	repetida := sesionDelContador(t)
	cinco := dexDelContadorCon(t, "5")
	entrega(t, repetida, "s5", ordenSwap(t, cinco))
	entrega(t, repetida, "s5-otra-vez", ordenSwap(t, cinco))
	if got := len(repetida.res.Intercambios); got != 1 {
		t.Fatalf("el swap repetido publicó %d intercambios y se esperaba 1", got)
	}

	s := sesionDelContador(t)
	base := s.base
	for n := 0; n < 4; n++ {
		entrega(t, s, "c", []string{"--pulsar", "id/pulsar"})
	}
	entrega(t, s, "s5", ordenSwap(t, cinco))
	if total := totalDe(t, mustLinea(t, s, "c5", []string{"--pulsar", "id/pulsar"})); total != "Pulsos: 9" {
		t.Fatalf("tras +5 el clic dio %q y se esperaba Pulsos: 9", total)
	}
	if s.base == base {
		t.Fatal("el primer delta no actualizó la base")
	}
	entrega(t, s, "s2", ordenSwap(t, dexDelContadorCon(t, "2")))
	linea := mustLinea(t, s, "c11", []string{"--pulsar", "id/pulsar"})
	if total := totalDe(t, linea); total != "Pulsos: 11" {
		t.Fatalf("el contador llegó a %q y se esperaba Pulsos: 11", total)
	}
	if len(s.res.Intercambios) != 2 {
		t.Fatalf("Intercambios = %d y se esperaban 2", len(s.res.Intercambios))
	}
	if met := s.m.buscaMetodo(claseListenerContador, "onClick", "(Landroid/view/View;)V"); met == nil || met.dex != s.base {
		t.Fatal("el método instalado en el segundo delta no viene del dex base actual")
	}
}

func mustLinea(t *testing.T, s *sesionPreview, id string, argv []string) string {
	t.Helper()
	linea, codigo := entrega(t, s, id, argv)
	if codigo != 0 {
		t.Fatalf("la orden %s falló con %d", id, codigo)
	}
	return linea
}

// Helpers del hot swap de recursos --------------------------------------------------------------

// apkDelContadorConLayout arma el apk del contador con el main.xml que se le dé, sin cambiar el
// manifiesto ni la tabla: es el segundo build que el usuario obtiene al editar solo el layout.
func apkDelContadorConLayout(t *testing.T, layout documentoAXML, fuentes ...fuenteJava) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layoutBytes, err := escribeAXML(layout)
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layoutBytes,
		"classes.dex":         dexDeFuentes(t, fuentes...),
	})
}

// ordenSwapRecursos escribe el dex y el apk nuevos donde el parser puede leerlos y devuelve una
// SOLA orden con las dos banderas: el código y la interfaz viajan juntos, como en la pestaña.
func ordenSwapRecursos(t *testing.T, dex, apk []byte) []string {
	t.Helper()
	dir := t.TempDir()
	rutaDex := filepath.Join(dir, "nuevo.dex")
	if err := os.WriteFile(rutaDex, dex, 0o644); err != nil {
		t.Fatalf("el dex nuevo no se escribió: %v", err)
	}
	rutaApk := filepath.Join(dir, "nuevo.apk")
	if err := os.WriteFile(rutaApk, apk, 0o644); err != nil {
		t.Fatalf("el apk nuevo no se escribió: %v", err)
	}
	return []string{
		"--intercambiar", claseListenerContador + "/onClick(Landroid/view/View;)V=" + rutaDex,
		"--intercambiar-recursos", rutaApk,
	}
}

// nodoPublicado es el trozo del JSON que el test lee del layout publicado.
type nodoPublicado struct {
	ID    string          `json:"id"`
	IDRef string          `json:"idRef"`
	Texto string          `json:"texto"`
	Hijos []nodoPublicado `json:"hijos"`
}

func raizPublicada(t *testing.T, linea string) nodoPublicado {
	t.Helper()
	var modelo struct {
		Raiz nodoPublicado `json:"raiz"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo no se lee: %v", err)
	}
	return modelo.Raiz
}

func nodoPorIDRef(raiz nodoPublicado, idRef string) *nodoPublicado {
	if raiz.IDRef == idRef {
		return &raiz
	}
	for i := range raiz.Hijos {
		if n := nodoPorIDRef(raiz.Hijos[i], idRef); n != nil {
			return n
		}
	}
	return nil
}

// TestElHotSwapDeRecursosRepintaElLayoutSinReiniciar es la falsación del defecto reportado: sin
// entregar el apk nuevo a la sesión viva, `modeloConResultado` re-parseaba SIEMPRE el main.xml de
// arranque. Aquí el segundo build cambia el id y el texto del TextView; el modelo tiene que publicar
// el nuevo y conservar el contador de los dos clics previos y el id de la orden.
func TestElHotSwapDeRecursosRepintaElLayoutSinReiniciar(t *testing.T) {
	s := sesionDelContador(t)

	// estado vivo antes del swap: dos clics con el sumador +1 del arranque
	for n := 0; n < 2; n++ {
		entrega(t, s, fmt.Sprintf("clic-%d", n+1), []string{"--pulsar", "id/pulsar"})
	}
	if got := len(s.res.Pulsaciones); got != 2 {
		t.Fatalf("Pulsaciones = %v y se esperaban 2 antes del swap", s.res.Pulsaciones)
	}
	antes := raizPublicada(t, mustLinea(t, s, "antes", []string{}))
	if n := nodoPorIDRef(antes, "0x7f010001"); n == nil || n.Texto != "Pulsos: 0" {
		t.Fatalf("la pantalla de arranque no trae el TextView original: %+v", antes.Hijos)
	}

	// segundo build: el dex pasa a sumar +5 y el main.xml trae otro id y otro texto literal
	dexNuevo := dexDelContadorCon(t, "5")
	layout := layoutDelContador()
	layout.raiz.hijos[0].attrs[0].datos = 0x7f010010 // era id/total (0x7f010001)
	layout.raiz.hijos[0].attrs[4].texto = "Interfaz cambiada"
	apkNuevo := apkDelContadorConLayout(t, layout, fuentesDelContador(t)...)

	base := s.base
	ids := []string{}
	s.publica = func(id any, codigo int) {
		if codigo != 0 {
			t.Errorf("la orden swap devolvió código %d", codigo)
		}
		ids = append(ids, fmt.Sprint(id))
	}
	linea := mustLinea(t, s, "swap", ordenSwapRecursos(t, dexNuevo, apkNuevo))

	// la pantalla sale del apk NUEVO: el TextView viejo desaparece y aparece el id y el texto nuevos
	raiz := raizPublicada(t, linea)
	if viejo := nodoPorIDRef(raiz, "0x7f010001"); viejo != nil {
		t.Fatalf("el modelo sigue trayendo el TextView del apk viejo: %+v", viejo)
	}
	nuevo := nodoPorIDRef(raiz, "0x7f010010")
	if nuevo == nil || nuevo.Texto != "Interfaz cambiada" {
		t.Fatalf("el modelo no publica el main.xml nuevo: %+v", raiz.Hijos)
	}

	// el estado de la máquina, el dex y el id de la orden sobreviven al cambio de interfaz
	if got := len(s.res.Pulsaciones); got != 2 {
		t.Fatalf("el swap repitió o perdió clics: Pulsaciones = %v", s.res.Pulsaciones)
	}
	if s.base == base {
		t.Fatal("el dex no se intercambió en la misma orden")
	}
	if !bytes.Equal(s.datos, apkNuevo) {
		t.Fatal("la sesión no guardó el apk nuevo como fuente del layout")
	}
	if total := totalDe(t, linea); total != "Pulsos: 2" {
		t.Fatalf("el contador acumulado se perdió: %q", total)
	}
	if got := strings.Join(ids, ","); got != "swap" {
		t.Fatalf("el id de la sesión cambió: publicó %q", got)
	}
	// y sigue viva: un clic con el dex nuevo suma +5 sobre los dos clics anteriores
	if total := totalDe(t, mustLinea(t, s, "clic-final", []string{"--pulsar", "id/pulsar"})); total != "Pulsos: 7" {
		t.Fatalf("tras el swap el clic dio %q y se esperaba Pulsos: 7", total)
	}
}

// TestElHotSwapRechazaUnApkSinLayoutSinTocarLaInterfazViva es la falsación del defecto confirmado:
// un zip válido sin `res/layout/main.xml` pasaba la comprobación de zip, se asignaba a `s.datos` y
// la sesión quedaba apuntando a un apk que modeloConResultado no puede pintar. La orden tiene que
// fallar cerrada con código 2 y conservar el layout anterior.
func TestElHotSwapRechazaUnApkSinLayoutSinTocarLaInterfazViva(t *testing.T) {
	s := sesionDelContador(t)
	datosViejos := append([]byte(nil), s.datos...)
	entrega(t, s, "clic", []string{"--pulsar", "id/pulsar"})

	// zip válido: el manifiesto y la tabla siguen apuntando a main.xml, pero la entrada del
	// layout no está. Es el build que el crítico encontró irrepresentable.
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	sinLayout := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
	if _, err := zip.NewReader(bytes.NewReader(sinLayout), int64(len(sinLayout))); err != nil {
		t.Fatalf("el apk de la prueba no es un zip: %v", err)
	}
	ruta := filepath.Join(t.TempDir(), "sin-layout.apk")
	if err := os.WriteFile(ruta, sinLayout, 0o644); err != nil {
		t.Fatalf("el apk sin layout no se escribió: %v", err)
	}

	linea, codigo := entrega(t, s, "sin-layout", []string{"--intercambiar-recursos", ruta})
	if codigo != 2 {
		t.Fatalf("un apk sin layout devolvió %d y se esperaba 2", codigo)
	}
	if !bytes.Equal(s.datos, datosViejos) {
		t.Fatal("la sesión sustituyó el apk por uno sin layout")
	}
	if !strings.Contains(linea, banderaIntercambiarRecursos) ||
		!strings.Contains(linea, "res/layout/main.xml") {
		t.Fatalf("el error no dice qué falló: %s", linea)
	}

	// el modelo posterior sigue leyendo el layout viejo, y la orden siguiente sigue viva
	modelo, err := s.modeloConResultado()
	if err != nil {
		t.Fatalf("el modelo posterior al rechazo falló: %v", err)
	}
	if modelo.Layout != "res/layout/main.xml" {
		t.Fatalf("el layout posterior es %q y se esperaba el viejo", modelo.Layout)
	}
	lineaDespues := mustLinea(t, s, "despues", []string{})
	if total := totalDe(t, lineaDespues); total != "Pulsos: 1" {
		t.Fatalf("el estado posterior al rechazo dio %q y se esperaba Pulsos: 1", total)
	}
	raiz := raizPublicada(t, lineaDespues)
	if n := nodoPorIDRef(raiz, "0x7f010001"); n == nil || n.ID != "id/total" {
		t.Fatalf("la pantalla posterior al rechazo no trae el layout viejo: %+v", raiz.Hijos)
	}
}

func TestLaSionFallaCerrada(t *testing.T) {
	s := sesionDelContador(t)
	base := s.base
	entrega(t, s, "c1", []string{"--pulsar", "id/pulsar"})
	cambios := len(s.res.Intercambios)
	roto := []byte("esto no es un dex") // el dex nuevo no se lee
	_, codigo := entrega(t, s, "roto", ordenSwap(t, roto))
	if codigo != 2 {
		t.Fatalf("un dex roto devolvió %d y se esperaba 2", codigo)
	}
	if s.base != base {
		t.Fatal("un swap fallido cambió la base del siguiente delta")
	}
	if len(s.res.Intercambios) != cambios {
		t.Fatalf("un swap fallido publicó %d intercambios", len(s.res.Intercambios))
	}
	linea := mustLinea(t, s, "c2", []string{"--pulsar", "id/pulsar"})
	if total := totalDe(t, linea); total != "Pulsos: 2" {
		t.Fatalf("la sesión después del fallo contestó %q", total)
	}
}

// TestLaSesionNoPegaElErrorDeUnaOrdenMala encadena una orden rechazada con otra buena. El error
// pertenece a la orden que lo produjo: la línea buena no debe llevar `error` aunque el resultado
// acumulado (capturas, consola, intercambios) siga vivo. Cubre también un fallo de aplicaDelta.
func TestLaSesionNoPegaElErrorDeUnaOrdenMala(t *testing.T) {
	s := sesionDelContador(t)
	mala, codigo := entrega(t, s, "mala", []string{"--pulsar", "id/pulsar", "otro.apk"})
	if codigo != 2 || !strings.Contains(mala, "\"error\"") {
		t.Fatalf("la orden mala dio código %d y línea %q", codigo, mala)
	}
	buena, codigo := entrega(t, s, "buena", []string{"--pulsar", "id/pulsar"})
	if codigo != 0 {
		t.Fatalf("la orden buena dio código %d", codigo)
	}
	if strings.Contains(buena, "\"error\"") {
		t.Fatalf("la orden buena publicó el error pegajoso: %s", buena)
	}
	t.Logf("segunda línea JSON (orden buena, sin error): %s", buena)
	if total := totalDe(t, buena); total != "Pulsos: 1" {
		t.Fatalf("la orden buena contestó %q", total)
	}

	base := s.base
	rota, codigo := entrega(t, s, "delta-roto", ordenSwap(t, []byte("esto no es un dex")))
	if codigo != 2 || !strings.Contains(rota, "\"error\"") {
		t.Fatalf("el delta roto dio código %d y línea %q", codigo, rota)
	}
	if s.base != base {
		t.Fatal("el delta roto cambió la base")
	}
	buena2, codigo := entrega(t, s, "buena-2", []string{"--pulsar", "id/pulsar"})
	if codigo != 0 {
		t.Fatalf("la orden posterior al delta roto dio código %d", codigo)
	}
	if strings.Contains(buena2, "\"error\"") {
		t.Fatalf("la orden posterior al delta roto publicó el error pegajoso: %s", buena2)
	}
	if total := totalDe(t, buena2); total != "Pulsos: 2" {
		t.Fatalf("la orden posterior al delta roto contestó %q", total)
	}
}

// modeloNavegaSesion es el recorte del JSON que la navegación por sesión tiene que publicar: la
// actividad y el layout delante, el recorrido y los clics acumulados, y el rectángulo pulsable.
type modeloNavegaSesion struct {
	Actividad string `json:"actividad"`
	Layout    string `json:"layout"`
	Ejecucion struct {
		Actividad   string   `json:"actividad"`
		Pantallas   []string `json:"pantallas"`
		Pulsaciones []string `json:"pulsaciones"`
		Error       string   `json:"error"`
	} `json:"ejecucion"`
	Raiz *struct {
		Hijos []nodoNavegaSesion `json:"hijos"`
	} `json:"raiz"`
}

// nodoNavegaSesion es un view del modelo de navegación: basta con su id, su texto escrito, si
// registró oyente y su rectángulo para comprobar que un botón es pulsable.
type nodoNavegaSesion struct {
	ID            string `json:"id"`
	TextoDinamico string `json:"textoDinamico"`
	Oyente        bool   `json:"oyente"`
	Rect          struct {
		Ancho int `json:"ancho"`
		Alto  int `json:"alto"`
	} `json:"rect"`
}

func modeloNavegaDe(t *testing.T, linea string) modeloNavegaSesion {
	t.Helper()
	var m modeloNavegaSesion
	if err := json.Unmarshal([]byte(linea), &m); err != nil {
		t.Fatalf("el modelo de navegación no se lee: %v", err)
	}
	if m.Ejecucion.Error != "" {
		t.Fatalf("la ejecución de navegación falló: %s", m.Ejecucion.Error)
	}
	return m
}

// TestLaSesionNavegaEntreActividades es la regresión de D7: el bucle de sesión entregaba el clic y
// drenaba las tareas vencidas, pero no estabilizaba la navegación, así que un `startActivity` dejaba
// la máquina vieja delante aunque el `onClick` hubiera corrido. Con la corrección, un clic en
// id/abrir abre la segunda actividad con su rect pulsable, y id/volver vuelve a la primera
// conservando lo que el primer onCreate escribió.
func TestLaSesionNavegaEntreActividades(t *testing.T) {
	s, err := nuevaSesionPreview(apkDelNavega(t), "", "")
	if err != nil {
		t.Fatalf("la sesión de dos pantallas no arrancó: %v", err)
	}

	abrir := modeloNavegaDe(t, mustLinea(t, s, "abrir", []string{"--pulsar", "id/abrir"}))
	if abrir.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("tras el clic la actividad es %q y se esperaba la segunda", abrir.Actividad)
	}
	if abrir.Ejecucion.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("la ejecución dice %q y se esperaba la segunda", abrir.Ejecucion.Actividad)
	}
	if abrir.Layout != "res/layout/segunda.xml" {
		t.Errorf("el layout es %q y se esperaba el de la segunda pantalla", abrir.Layout)
	}
	quiero := []string{"com.hola.navega.MainActivity", "com.hola.navega.SegundaActivity"}
	if strings.Join(abrir.Ejecucion.Pantallas, "|") != strings.Join(quiero, "|") {
		t.Errorf("el recorrido tras abrir es %v y se esperaba %v", abrir.Ejecucion.Pantallas, quiero)
	}
	if abrir.Raiz == nil {
		t.Fatal("la segunda pantalla no trae raíz")
	}
	var volver *nodoNavegaSesion
	for i := range abrir.Raiz.Hijos {
		if abrir.Raiz.Hijos[i].ID == "id/volver" {
			volver = &abrir.Raiz.Hijos[i]
		}
	}
	if volver == nil {
		t.Fatalf("la segunda pantalla no tiene el botón id/volver: %+v", abrir.Raiz.Hijos)
	}
	if !volver.Oyente || volver.Rect.Ancho <= 0 || volver.Rect.Alto <= 0 {
		t.Errorf("id/volver no es pulsable: oyente=%t rect=%+v", volver.Oyente, volver.Rect)
	}

	vuelta := modeloNavegaDe(t, mustLinea(t, s, "volver", []string{"--pulsar", "id/volver"}))
	if vuelta.Actividad != "com.hola.navega.MainActivity" {
		t.Errorf("tras volver la actividad es %q y se esperaba la primera", vuelta.Actividad)
	}
	if vuelta.Layout != "res/layout/main.xml" {
		t.Errorf("tras volver el layout es %q y se esperaba el principal", vuelta.Layout)
	}
	quiero = append(quiero, "com.hola.navega.MainActivity")
	if strings.Join(vuelta.Ejecucion.Pantallas, "|") != strings.Join(quiero, "|") {
		t.Errorf("el recorrido tras volver es %v y se esperaba %v", vuelta.Ejecucion.Pantallas, quiero)
	}
	if got := strings.Join(vuelta.Ejecucion.Pulsaciones, ","); got != "id/abrir,id/volver" {
		t.Errorf("los clics acumulados son %q y se esperaban dos", got)
	}
	if vuelta.Raiz == nil || len(vuelta.Raiz.Hijos) != 2 ||
		vuelta.Raiz.Hijos[0].TextoDinamico != "Pulsos: 1" {
		t.Errorf("al volver no se conservó el estado de la primera pantalla: %+v", vuelta.Raiz)
	}
}

// Helpers del cronómetro por sesión ------------------------------------------------------------

// tablaDelCronometroSesion es la tabla de una apk mínima con tres ids ordenados por nombre: el
// botón que inicia, el TextView del pulso y el TextView de la hora; el cuarto recurso es el layout.
func tablaDelCronometroSesion() tablaEscribir {
	return tablaEscribir{
		idPaquete: 0x7f, nombrePaquete: "com.hola.reloj",
		global: []string{"res/layout/main.xml"}, globalUTF8: true,
		nombresTipo: []string{"id", "layout"}, nombresUTF8: false,
		claves: []string{"iniciar", "main", "pulso", "tiempo"}, clavesUTF8: true,
		tipos: []tipoEscribir{
			{
				id: 1, specFlags: []uint32{0, 0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}}, // iniciar
					{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}}, // pulso
					{clave: 3, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}}, // tiempo
				}}},
			},
			{
				id: 2, specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 1, valor: valorEscribir{tipo: 0x03, datos: 0}}, // main
				}}},
			},
		},
	}
}

// layoutDelCronometroSesion pone los dos TextView y el botón con los ids de la tabla anterior:
// iniciar=0x7f010000, pulso=0x7f010001 y tiempo=0x7f010002 (el orden es el de las claves del tipo).
func layoutDelCronometroSesion() documentoAXML {
	texto := func(id uint32) *elementoEscribir {
		return &elementoEscribir{
			nombre: "TextView", linea: 3,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: id},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
			},
		}
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout", linea: 2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: 0x010100c4, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				texto(0x7f010002), // tiempo
				texto(0x7f010001), // pulso
				{
					nombre: "Button", linea: 20,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Iniciar"},
					},
				},
			},
		},
	}
}

func apkDelCronometroSesion(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelCronometroSesion())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelCronometroSesion())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDelCronometroSesion())
	if err != nil {
		t.Fatalf("el R.java no sale de la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "Casos.java", contenido: fuenteCronometroSesion}),
	})
}

// fuenteCronometroSesion arranca un tic a los 1000 ms cuando se pulsa id/iniciar. El modelo arranca
// sin hora escrita y solo la publica cuando vence la tarea: sin la orden --avanzar no hay tic.
const fuenteCronometroSesion = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity implements Runnable {
    TextView tiempo;
    TextView pulso;
    Handler reloj;
    int pulsos = 0;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        tiempo = (TextView) findViewById(R.id.tiempo);
        pulso = (TextView) findViewById(R.id.pulso);
        reloj = new Handler();
        findViewById(R.id.iniciar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                iniciar();
            }
        });
    }

    private void iniciar() {
        reloj.postDelayed(this, 1000L);
    }

    public void run() {
        pulsos = pulsos + 1;
        tiempo.setText("tiempo 00:01.00");
        pulso.setText("pulso " + pulsos);
    }
}
`

// capturaSesionDe lee el texto dinámico de un id del modelo que publicó la sesión.
func capturaSesionDe(t *testing.T, linea, id string) string {
	t.Helper()
	var modelo struct {
		Ejecucion struct {
			Capturas []struct {
				ID    string `json:"id"`
				Texto string `json:"texto"`
			} `json:"capturas"`
		} `json:"ejecucion"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo no se lee: %v", err)
	}
	for i := len(modelo.Ejecucion.Capturas) - 1; i >= 0; i-- {
		if modelo.Ejecucion.Capturas[i].ID == id {
			return modelo.Ejecucion.Capturas[i].Texto
		}
	}
	return ""
}

// TestLaSesionAvanzaElRelojConLaOrden es la falsación del defecto reportado: en modo sesión el
// `--avanzar` caía en `entregaEvento`, que no tiene caso para esa orden, y el fin-orden salía con
// código 0 mientras el reloj seguía en 0. Aquí se pulsa id/iniciar, se entrega --avanzar 1000 y el
// modelo tiene que publicar reloj=1000, la hora 00:01.00 y el pulso de la tarea vencida.
func TestLaSesionAvanzaElRelojConLaOrden(t *testing.T) {
	s, err := nuevaSesionPreview(apkDelCronometroSesion(t), "", "")
	if err != nil {
		t.Fatalf("la sesión del cronómetro no arrancó: %v", err)
	}
	mustLinea(t, s, "iniciar", []string{"--pulsar", "id/iniciar"})
	linea := mustLinea(t, s, "avance", []string{"--avanzar", "1000"})

	var modelo struct {
		Ejecucion struct {
			Reloj int64 `json:"reloj"`
		} `json:"ejecucion"`
	}
	if err := json.Unmarshal([]byte(linea), &modelo); err != nil {
		t.Fatalf("el modelo no se lee: %v", err)
	}
	if modelo.Ejecucion.Reloj != 1000 {
		t.Fatalf("reloj = %d y se esperaba 1000: el --avanzar se descartó en la sesión", modelo.Ejecucion.Reloj)
	}
	if got := capturaSesionDe(t, linea, "id/tiempo"); got != "tiempo 00:01.00" {
		t.Errorf("tiempo = %q y se esperaba «tiempo 00:01.00»", got)
	}
	if got := capturaSesionDe(t, linea, "id/pulso"); got != "pulso 1" {
		t.Errorf("pulso = %q y se esperaba «pulso 1»", got)
	}
}
