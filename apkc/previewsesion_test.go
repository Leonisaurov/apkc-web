package main

// Las pruebas de la sesión viva: una sola máquina, una sola orden por entrega, sin replay. El
// proveedor del wasm se sustituye por llamadas directas a ejecutaOrden, que es puro y no sale.

import (
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
