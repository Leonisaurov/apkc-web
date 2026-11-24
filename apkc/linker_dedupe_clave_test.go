package main

// La deuda de `f1c0e45`: el dedupe de `values` era por bytes de fichero. Con dos AAR —o el proyecto
// y un AAR— que declaran la misma clave con distinto valor, el enlace se quedaba con la última leída
// y no avisaba. Aquí se mide la regla nueva: por clave (tipo, nombre y configuración) gana la
// prioridad proyecto > isla > AAR y, a igual prioridad, el nombre de la fuente y el del fichero, que
// no dependen del orden de lectura; la que se descarta sale por `LogW` con la clave, las dos fuentes
// y la ganadora. Los `values` idénticos byte a byte se siguen leyendo una sola vez.

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aarConValues escribe en `raiz/jar/` un `.aar` mínimo con un solo `res/values/strings.xml`. El
// enlace de recursos no necesita las clases para esta prueba.
func aarConValues(t *testing.T, raiz, nombre, values string) {
	t.Helper()
	jar := filepath.Join(raiz, "jar")
	if err := os.MkdirAll(jar, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(jar, nombre))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("res/values/strings.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(values)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// marcoDePrueba carga la tabla del framework, que el parser de values necesita para los ids de
// `android:`.
func marcoDelSdkDePrueba(t *testing.T) *marcoRecursos {
	t.Helper()
	m, err := marcoDelAndroidJar(jarDelSDK(t))
	if err != nil {
		t.Fatalf("la tabla del framework: %v", err)
	}
	return m
}

// avisosDurante corre `hacer` con la salida estándar desviada y devuelve lo que se escribió, que es
// donde `LogW` publica el aviso de una clave repetida.
func avisosDurante(t *testing.T, hacer func()) string {
	t.Helper()
	viejo := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	hacer()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = viejo
	datos, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(datos)
}

// entradasDeClave cuenta las entradas de `clave` dentro del tipo `tipo` en la tabla escrita: la
// misma clave no puede quedar dos veces en `resources.arsc`.
func entradasDeClave(tabla tablaEscribir, tipo, clave string) int {
	indice := -1
	for i, c := range tabla.claves {
		if c == clave {
			indice = i
			break
		}
	}
	if indice < 0 {
		return 0
	}
	id := byte(0)
	for i, n := range tabla.nombresTipo {
		if n == tipo {
			id = byte(i + 1)
		}
	}
	if id == 0 {
		return 0
	}
	n := 0
	for _, t := range tabla.tipos {
		if t.id != id {
			continue
		}
		for _, b := range t.bloques {
			for _, e := range b.entradas {
				if !e.falta && e.clave == indice {
					n++
				}
			}
		}
	}
	return n
}

// valorDeClave devuelve el texto al que apunta la entrada de `clave` en la tabla escrita: es el
// valor ganador ya enlazado, no lo que diga el pool global.
func valorDeClave(tabla tablaEscribir, tipo, clave string) (string, bool) {
	indice := -1
	for i, c := range tabla.claves {
		if c == clave {
			indice = i
			break
		}
	}
	if indice < 0 {
		return "", false
	}
	id := byte(0)
	for i, n := range tabla.nombresTipo {
		if n == tipo {
			id = byte(i + 1)
		}
	}
	if id == 0 {
		return "", false
	}
	for _, t := range tabla.tipos {
		if t.id != id {
			continue
		}
		for _, b := range t.bloques {
			for _, e := range b.entradas {
				if e.falta || e.clave != indice {
					continue
				}
				if int(e.valor.datos) < len(tabla.global) {
					return tabla.global[e.valor.datos], true
				}
				return "", false
			}
		}
	}
	return "", false
}

// voltear devuelve las fuentes en el orden inverso, que simula un lector que recorriera las carpetas
// al revés: el ganador tiene que ser el mismo.
func voltear(fuentes []fuenteXML) []fuenteXML {
	out := make([]fuenteXML, len(fuentes))
	for i, f := range fuentes {
		out[len(fuentes)-1-i] = f
	}
	return out
}

// enlazaConAvisos enlaza y devuelve también lo que se escribió por `LogW`, que es donde sale el
// aviso de una clave repetida.
func enlazaConAvisos(t *testing.T, m *marcoRecursos, valores, layouts, ficheros []fuenteXML, manifiesto fuenteXML) (*proyectoEnlazado, string) {
	t.Helper()
	var salida *proyectoEnlazado
	avisos := avisosDurante(t, func() {
		var err error
		salida, err = enlazarProyecto(valores, layouts, ficheros, manifiesto, m,
			opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
		if err != nil {
			t.Fatalf("el enlace: %v", err)
		}
	})
	return salida, avisos
}

// styleablesDeLaTabla devuelve las declaraciones de `nombre` que quedaron en la tabla: un
// `<declare-styleable>` no lleva configuración, así que su nombre es su clave.
func styleablesDeLaTabla(tabla tablaEscribir, nombre string) []styleableEscribir {
	out := []styleableEscribir{}
	for _, st := range tabla.styleables {
		if st.nombre == nombre {
			out = append(out, st)
		}
	}
	return out
}

func TestDedupeDeValuesPorClave(t *testing.T) {
	t.Run("dos AAR: gana el de nombre anterior y la clave queda una vez", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  valuesMinimo,
		})
		aarConValues(t, dir, "a.aar", `<resources>
    <string name="aar_saludo">hola desde a</string>
    <string name="solo_a">solo de a</string>
</resources>`)
		aarConValues(t, dir, "b.aar", `<resources>
    <string name="aar_saludo">hola desde b</string>
    <string name="solo_b">solo de b</string>
</resources>`)
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		if len(valores) != 3 {
			t.Fatalf("salieron %d values: se esperaban el del proyecto y uno por AAR", len(valores))
		}

		enlaza := func(entrada []fuenteXML) (*proyectoEnlazado, string) {
			var salida *proyectoEnlazado
			avisos := avisosDurante(t, func() {
				var err error
				salida, err = enlazarProyecto(entrada, layouts, ficheros, manifiesto, m,
					opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
				if err != nil {
					t.Fatalf("el enlace: %v", err)
				}
			})
			return salida, avisos
		}

		salida, avisos := enlaza(valores)
		t.Logf("aviso: %s", strings.TrimSpace(avisos))
		if v, ok := valorDeClave(salida.tabla, "string", "aar_saludo"); !ok || v != "hola desde a" {
			t.Errorf("la entrada de string/aar_saludo vale %q (ok=%v): se esperaba el valor del AAR a.aar", v, ok)
		}
		if !cadenaEnLaTabla(salida.tabla, "hola desde a") {
			t.Error("no llegó a la tabla el valor del AAR a.aar, que es el que gana")
		}
		if cadenaEnLaTabla(salida.tabla, "hola desde b") {
			t.Error("el valor del AAR b.aar pisó o convivió con el de a.aar")
		}
		if !cadenaEnLaTabla(salida.tabla, "solo de a") || !cadenaEnLaTabla(salida.tabla, "solo de b") {
			t.Error("se perdió un value que solo declaraba uno de los dos AAR")
		}
		if n := entradasDeClave(salida.tabla, "string", "aar_saludo"); n != 1 {
			t.Errorf("la clave string/aar_saludo tiene %d entradas en la tabla y solo cabe una", n)
		}
		for _, quiero := range []string{"string/aar_saludo", "el AAR a.aar", "el AAR b.aar", "gana el AAR a.aar"} {
			if !strings.Contains(avisos, quiero) {
				t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
			}
		}

		// el mismo par de fuentes leído al revés elige la misma ganadora
		alReves, avisosAlReves := enlaza(voltear(valores))
		if v, ok := valorDeClave(alReves.tabla, "string", "aar_saludo"); !ok || v != "hola desde a" {
			t.Errorf("con el orden invertido la entrada vale %q (ok=%v): el ganador depende de la lectura", v, ok)
		}
		if !cadenaEnLaTabla(alReves.tabla, "hola desde a") {
			t.Error("con el orden invertido no ganó el AAR a.aar")
		}
		if cadenaEnLaTabla(alReves.tabla, "hola desde b") {
			t.Error("con el orden invertido ganó el AAR b.aar: el resultado depende de la lectura")
		}
		if !strings.Contains(avisosAlReves, "gana el AAR a.aar") {
			t.Errorf("con el orden invertido el aviso no nombra la misma ganadora: %s", avisosAlReves)
		}
		g1, _ := valorDeClave(salida.tabla, "string", "aar_saludo")
		g2, _ := valorDeClave(alReves.tabla, "string", "aar_saludo")
		if g1 != g2 {
			t.Errorf("el ganador depende del orden de lectura: %q con el orden normal y %q al revés", g1, g2)
		}
	})

	t.Run("el proyecto gana al AAR aunque se lea después", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  `<resources><string name="aar_saludo">del proyecto</string></resources>`,
		})
		aarConValues(t, dir, "a.aar", `<resources><string name="aar_saludo">del aar</string></resources>`)
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		for _, entrada := range [][]fuenteXML{valores, voltear(valores)} {
			var salida *proyectoEnlazado
			avisos := avisosDurante(t, func() {
				var err error
				salida, err = enlazarProyecto(entrada, layouts, ficheros, manifiesto, m,
					opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
				if err != nil {
					t.Fatalf("el enlace: %v", err)
				}
			})
			if v, ok := valorDeClave(salida.tabla, "string", "aar_saludo"); !ok || v != "del proyecto" {
				t.Errorf("la entrada de string/aar_saludo vale %q (ok=%v): se esperaba el del proyecto", v, ok)
			}
			if !cadenaEnLaTabla(salida.tabla, "del proyecto") {
				t.Error("el valor del proyecto no llegó a la tabla")
			}
			if cadenaEnLaTabla(salida.tabla, "del aar") {
				t.Error("el valor del AAR pisó el del proyecto")
			}
			for _, quiero := range []string{"string/aar_saludo", "el proyecto", "el AAR a.aar", "gana el proyecto"} {
				if !strings.Contains(avisos, quiero) {
					t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
				}
			}
		}
	})

	t.Run("la isla gana al AAR", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		const isla = "0123456789ab"
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  valuesMinimo,
			"res/__deps/" + isla + "/values/strings-" + isla + ".xml": `<resources><string name="aar_saludo">de la isla</string></resources>`,
		})
		aarConValues(t, dir, "a.aar", `<resources><string name="aar_saludo">del aar</string></resources>`)
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		var salida *proyectoEnlazado
		avisos := avisosDurante(t, func() {
			var err error
			salida, err = enlazarProyecto(voltear(valores), layouts, ficheros, manifiesto, m,
				opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
			if err != nil {
				t.Fatalf("el enlace: %v", err)
			}
		})
		if v, ok := valorDeClave(salida.tabla, "string", "aar_saludo"); !ok || v != "de la isla" {
			t.Errorf("la entrada de string/aar_saludo vale %q (ok=%v): se esperaba el de la isla", v, ok)
		}
		if !cadenaEnLaTabla(salida.tabla, "de la isla") {
			t.Error("el valor de la isla no llegó a la tabla")
		}
		if cadenaEnLaTabla(salida.tabla, "del aar") {
			t.Error("el valor del AAR pisó el de la isla")
		}
		for _, quiero := range []string{"string/aar_saludo", "la isla " + isla, "el AAR a.aar", "gana la isla " + isla} {
			if !strings.Contains(avisos, quiero) {
				t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
			}
		}
	})

	t.Run("el values idéntico byte a byte se sigue leyendo una vez", func(t *testing.T) {
		const isla = "ba9876543210"
		const mismo = `<resources><string name="aar_saludo">el mismo fichero</string></resources>`
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  valuesMinimo,
			"res/__deps/" + isla + "/values/strings-" + isla + ".xml": mismo,
		})
		aarConValues(t, dir, "a.aar", mismo)
		valores, _, _, _, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		copias := 0
		for _, f := range valores {
			if string(f.contenido) == mismo {
				copias++
				if f.origen != "la isla "+isla {
					t.Errorf("la copia que se lee sale con origen %q", f.origen)
				}
			}
		}
		if copias != 1 {
			t.Errorf("el values idéntico salió %d veces y debe leerse una sola vez", copias)
		}
	})
}

// TestDedupeDeStyleablesPorClave mide la otra mitad de la deuda: un `<declare-styleable>` no se
// deduplicaba, y dos fuentes que declaran «Vista» producían dos clases en R.java, que javac rechaza
// por el identificador repetido. Su nombre es la clave y se resuelve con la misma prioridad y el
// mismo aviso que los values.
func TestDedupeDeStyleablesPorClave(t *testing.T) {
	const vistaDelProyecto = `<resources>
    <attr name="propio" format="integer"/>
    <declare-styleable name="Vista"><attr name="propio"/></declare-styleable>
</resources>`
	const vistaDelAar = `<resources>
    <attr name="ajeno" format="color"/>
    <declare-styleable name="Vista"><attr name="ajeno"/></declare-styleable>
</resources>`

	t.Run("dos AAR: gana el de nombre anterior y Vista queda una vez", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  valuesMinimo,
		})
		aarConValues(t, dir, "a.aar", `<resources>
    <attr name="foo" format="integer"/>
    <declare-styleable name="Vista"><attr name="foo"/></declare-styleable>
</resources>`)
		aarConValues(t, dir, "b.aar", `<resources>
    <attr name="bar" format="color"/>
    <declare-styleable name="Vista"><attr name="bar"/></declare-styleable>
</resources>`)
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		for _, entrada := range [][]fuenteXML{valores, voltear(valores)} {
			salida, avisos := enlazaConAvisos(t, m, entrada, layouts, ficheros, manifiesto)
			t.Logf("aviso: %s", strings.TrimSpace(avisos))
			vueltas := styleablesDeLaTabla(salida.tabla, "Vista")
			if len(vueltas) != 1 {
				t.Fatalf("Vista salió %d veces y debe quedar una", len(vueltas))
			}
			if got := strings.Join(vueltas[0].attrs, ","); got != "foo" {
				t.Errorf("Vista quedó con los attrs %q y se esperaba foo, el del AAR a.aar", got)
			}
			if n := strings.Count(salida.r, "int[] Vista="); n != 1 {
				t.Errorf("R.java declara Vista %d veces y solo cabe una", n)
			}
			if !strings.Contains(salida.r, "Vista_foo") || strings.Contains(salida.r, "Vista_bar") {
				t.Errorf("R.java no se queda con los attrs de la ganadora:\n%s", salida.r)
			}
			for _, quiero := range []string{"styleable/Vista", "el AAR a.aar", "el AAR b.aar", "gana el AAR a.aar"} {
				if !strings.Contains(avisos, quiero) {
					t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
				}
			}
		}
	})

	t.Run("el proyecto gana al AAR", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml": manifestMinimo,
			"res/values/app.xml":  vistaDelProyecto,
		})
		aarConValues(t, dir, "a.aar", vistaDelAar)
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		for _, entrada := range [][]fuenteXML{valores, voltear(valores)} {
			salida, avisos := enlazaConAvisos(t, m, entrada, layouts, ficheros, manifiesto)
			vueltas := styleablesDeLaTabla(salida.tabla, "Vista")
			if len(vueltas) != 1 {
				t.Fatalf("Vista salió %d veces y debe quedar una", len(vueltas))
			}
			if got := strings.Join(vueltas[0].attrs, ","); got != "propio" {
				t.Errorf("Vista quedó con los attrs %q y se esperaba propio, el del proyecto", got)
			}
			if strings.Contains(salida.r, "Vista_ajeno") {
				t.Error("el attr del AAR llegó a la clase styleable")
			}
			for _, quiero := range []string{"styleable/Vista", "el proyecto", "el AAR a.aar", "gana el proyecto"} {
				if !strings.Contains(avisos, quiero) {
					t.Errorf("el aviso no nombra %q: %s", quiero, avisos)
				}
			}
		}
	})

	t.Run("el mismo styleable en dos configuraciones no duplica la clase", func(t *testing.T) {
		m := marcoDelSdkDePrueba(t)
		dir := proyecto(t, map[string]string{
			"AndroidManifest.xml":   manifestMinimo,
			"res/values/app.xml":    vistaDelProyecto,
			"res/values-es/app.xml": `<resources><declare-styleable name="Vista"><attr name="propio"/></declare-styleable></resources>`,
		})
		valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
		if err != nil {
			t.Fatalf("las fuentes: %v", err)
		}
		salida, _ := enlazaConAvisos(t, m, valores, layouts, ficheros, manifiesto)
		vueltas := styleablesDeLaTabla(salida.tabla, "Vista")
		if len(vueltas) != 1 {
			t.Fatalf("Vista salió %d veces con dos configuraciones y debe quedar una", len(vueltas))
		}
		if got := strings.Join(vueltas[0].attrs, ","); got != "propio" {
			t.Errorf("Vista quedó con los attrs %q", got)
		}
		if n := strings.Count(salida.r, "int[] Vista="); n != 1 {
			t.Errorf("R.java declara Vista %d veces y solo cabe una", n)
		}
	})
}
