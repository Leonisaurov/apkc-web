package main

// Las pruebas del marco de recursos cacheado (marcocache.go): que la forma va y vuelve sin cambiar
// nada, que una caché de otra forma, de otro jar o cortada se descarta y cae al parseo, y cuánto
// ahorra la segunda pasada. La sonda es la misma que la del índice de I5: mediana de tres vueltas,
// con el registro de las tres para no enseñar solo la buena.

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// rutaDelMarcoCacheado es la caché que `check` dejó en el build del proyecto. Repite el nombre que
// arma ficheroMarcoCacheado sin depender de los globales, que dentro de `check` cambian por llamada.
func rutaDelMarcoCacheado(proyecto, rutaJar string) string {
	digest := sha1.Sum([]byte(rutaJar))
	return filepath.Join(proyecto, dirMarcoCacheado, hex.EncodeToString(digest[:])+".bin")
}

// marcoDePrueba es un marco pequeño con las cuatro piezas que la caché serializa: nombre, tipos,
// formatos, valores y ids. No sale del android.jar para que la prueba no dependa del SDK.
func marcoDePrueba() *marcoRecursos {
	return &marcoRecursos{
		nombre: "android",
		tipos:  []string{"attr", "id", "style"},
		ids: map[string]uint32{
			"attr/layout_width":              0x010100f4,
			"attr/text":                      0x0101014f,
			"id/b":                           0x01020000,
			"style/Theme.DeviceDefault":      0x0103012b,
			"string/Algo.Con.TildesY-\u00f1": 0x01040007,
		},
		valores: map[string]map[string]valorEscribir{
			"layout_width": {
				"match_parent": {tipo: tipoEnteroDec, datos: 0xffffffff},
				"wrap_content": {tipo: tipoEnteroDec, datos: 0xfffffffe},
			},
			"gravity": {
				"center": {tipo: tipoEnteroHex, datos: 0x11},
				"start":  {tipo: tipoEnteroHex, datos: 0x00800003, texto: "no se escribe"},
			},
		},
		formatos: map[string]uint32{"layout_width": 0x00020000, "gravity": 0x00020000},
	}
}

// TestMarcoCacheadoVaYVuelve fija las dos mitades: la forma reconstruye el mismo marco —ids, mapa
// inverso, tipos, valores y formatos— y una caché dudosa no se sirve. Las falsificaciones son las que
// importan: cada una tiene que devolver false, no un marco a medias.
func TestMarcoCacheadoVaYVuelve(t *testing.T) {
	raiz := t.TempDir()
	anterior := directorioDeMarcoCacheado
	directorioDeMarcoCacheado = filepath.Join(raiz, dirMarcoCacheado)
	t.Cleanup(func() {
		directorioDeMarcoCacheado = anterior
		reutilizarMarcoCacheado = true
	})
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(raiz, "android.jar")
	jarDePrueba(t, jar, map[string][]byte{
		"resources.arsc": {9, 9, 9},
		"p/A.class":      {1, 2, 3},
	})
	tamano, huella, err := huellaDelJar(jar)
	if err != nil {
		t.Fatalf("el jar de la prueba no deja huella: %v", err)
	}

	m := marcoDePrueba()
	escribirMarcoCacheado(jar, tamano, huella, m)
	destino := ficheroMarcoCacheado(jar)
	datos, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("la primera pasada no dejó la caché en %s: %v", destino, err)
	}

	antes := marcosDeCache
	got, ok := leerMarcoCacheado(jar, tamano, huella)
	if !ok {
		t.Fatal("la caché recién escrita no se pudo leer")
	}
	if marcosDeCache != antes+1 {
		t.Fatalf("el contador de marcos servidos es %d y debía ser %d", marcosDeCache, antes+1)
	}
	if got.nombre != m.nombre || !reflect.DeepEqual(got.tipos, m.tipos) {
		t.Errorf("el marco vuelve con nombre %q y tipos %v", got.nombre, got.tipos)
	}
	if !reflect.DeepEqual(got.ids, m.ids) {
		t.Errorf("los ids no vuelven iguales: %d frente a %d", len(got.ids), len(m.ids))
	}
	if !reflect.DeepEqual(got.valores, m.valores) {
		t.Errorf("los valores de atributo no vuelven iguales: %v", got.valores)
	}
	if !reflect.DeepEqual(got.formatos, m.formatos) {
		t.Errorf("los formatos no vuelven iguales: %v", got.formatos)
	}
	// el mapa inverso no se guarda: se reconstruye, y tiene que dar el mismo nombre
	if k, ok := got.nombreDeId(0x0103012b); !ok || k != "style/Theme.DeviceDefault" {
		t.Errorf("el mapa inverso reconstruido dio %q, ok=%v", k, ok)
	}

	// otra huella del jar: los mismos bytes guardados no valen para otro contenido ni otro tamaño
	if _, ok := leerMarcoCacheado(jar, tamano+1, huella); ok {
		t.Error("una caché con otro tamaño de jar fue servida")
	}
	if _, ok := leerMarcoCacheado(jar, tamano, huella^1); ok {
		t.Error("una caché con otro directorio central fue servida")
	}

	// el interruptor de la sonda: apagado, la caché no se lee aunque esté sana
	reutilizarMarcoCacheado = false
	if _, ok := leerMarcoCacheado(jar, tamano, huella); ok {
		t.Error("con la reutilización apagada se sirvió la caché")
	}
	reutilizarMarcoCacheado = true

	// una marca de otra versión de forma no se reinterpreta
	otraMarca := append([]byte(nil), datos...)
	copy(otraMarca, "apkc-marco-2\n")
	if err := os.WriteFile(destino, otraMarca, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := leerMarcoCacheado(jar, tamano, huella); ok {
		t.Error("una caché con otra marca de forma fue servida")
	}

	// la ruta del jar también va en la cabecera: cambiarla invalida el fichero
	otraRuta := append([]byte(nil), datos...)
	otraRuta[campoRutaMarco] ^= 0xff
	if err := os.WriteFile(destino, otraRuta, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := leerMarcoCacheado(jar, tamano, huella); ok {
		t.Error("una caché de otra ruta del jar fue servida")
	}

	// cortes en todos los sitios que importan: cabecera, mitad del cuerpo y un byte de menos
	for _, corte := range []int{0, largoMarcaMarco, cabeceraMarco, len(datos) / 2, len(datos) - 1} {
		if err := os.WriteFile(destino, datos[:corte], 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := leerMarcoCacheado(jar, tamano, huella); ok {
			t.Errorf("una caché cortada en %d de %d bytes fue servida", corte, len(datos))
		}
	}

	// basura al final del cuerpo: la forma se comprueba entera, no solo la cabecera
	basura := append(append([]byte(nil), datos...), 0x7f)
	if err := os.WriteFile(destino, basura, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := leerMarcoCacheado(jar, tamano, huella); ok {
		t.Error("una caché con un byte de más fue servida")
	}
	if marcosDeCache != antes+1 {
		t.Fatalf("las cachés inválidas movieron el contador: %d, se esperaba %d", marcosDeCache, antes+1)
	}
}

// mideMarco abre el marco del jar una vez y devuelve lo que tardó.
func mideMarco(t *testing.T, rutaJar string) time.Duration {
	t.Helper()
	comienzo := time.Now()
	m, err := marcoDelAndroidJar(rutaJar)
	d := time.Since(comienzo)
	if err != nil {
		t.Fatalf("marcoDelAndroidJar: %v", err)
	}
	if len(m.ids) == 0 {
		t.Fatal("el marco salió vacío")
	}
	return d
}

// TestMarcoCacheadoMideElAhorro es la sonda del frente: el mismo android.jar, tres vueltas, la
// caché apagada y encendida. La apagada es el camino que `check` pagaba en cada pasada —inflar los
// 16 MB y andar los bloques—; la encendida solo lee el fichero. Además, los dos marcos se comparan
// pieza a pieza: una caché rápida que cambie los ids no ahorra nada, rompe el enlazado.
func TestMarcoCacheadoMideElAhorro(t *testing.T) {
	ruta := rutaDelJar(t)
	raiz := t.TempDir()
	anterior := directorioDeMarcoCacheado
	directorioDeMarcoCacheado = filepath.Join(raiz, dirMarcoCacheado)
	t.Cleanup(func() {
		directorioDeMarcoCacheado = anterior
		reutilizarMarcoCacheado = true
	})
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	// calienta el índice de I5 y deja una caché buena: las tres vueltas miden lo que se quiere medir
	reutilizarMarcoCacheado = false
	mideMarco(t, ruta)
	reutilizarMarcoCacheado = true

	var frias, calientes []time.Duration
	for i := 0; i < 3; i++ {
		if err := os.Remove(ficheroMarcoCacheado(ruta)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		reutilizarMarcoCacheado = false
		frias = append(frias, mideMarco(t, ruta))
		reutilizarMarcoCacheado = true
		calientes = append(calientes, mideMarco(t, ruta))
		t.Logf("vuelta %d: sin caché %.1f ms; con la caché %.1f ms",
			i+1, milisegundos(frias[i]), milisegundos(calientes[i]))
	}
	t.Logf("medianas: sin caché %.1f ms; con la caché %.1f ms",
		milisegundos(mediana(frias)), milisegundos(mediana(calientes)))
	if mediana(calientes) >= mediana(frias) {
		t.Errorf("la caché no ahorra nada: %.1f ms frente a %.1f ms",
			milisegundos(mediana(calientes)), milisegundos(mediana(frias)))
	}

	// y la caché no puede cambiar el marco: es la misma tabla, leída de otro sitio
	if err := os.Remove(ficheroMarcoCacheado(ruta)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	reutilizarMarcoCacheado = false
	frio, err := marcoDelAndroidJar(ruta)
	reutilizarMarcoCacheado = true
	if err != nil {
		t.Fatal(err)
	}
	tamano, huella := tamanoYHuella(t, ruta)
	caliente, ok := leerMarcoCacheado(ruta, tamano, huella)
	if !ok {
		t.Fatal("la caché que dejó el camino frío no se pudo leer")
	}
	if !reflect.DeepEqual(frio.ids, caliente.ids) {
		t.Fatalf("los ids cambiaron: %d en frío y %d en caliente", len(frio.ids), len(caliente.ids))
	}
	if !reflect.DeepEqual(frio.valores, caliente.valores) || !reflect.DeepEqual(frio.formatos, caliente.formatos) {
		t.Fatal("los valores o los formatos de atributo cambiaron al pasar por la caché")
	}
}

// tamanoYHuella desempaqueta lo que la cabecera de la caché comprueba. Existe para no repetir el
// orden de los tres valores en cada prueba.
func tamanoYHuella(t *testing.T, rutaJar string) (int64, uint32) {
	t.Helper()
	tamano, huella, err := huellaDelJar(rutaJar)
	if err != nil {
		t.Fatalf("huella del jar: %v", err)
	}
	return tamano, huella
}

// TestCheckMarcoCacheadoCorruptoNoFalsifica es la falsificación acordada: con la caché rota, `check`
// no puede ni servirla ni ponerse en verde por su cuenta. Cae al parseo, da el mismo resultado y
// vuelve a dejar la caché sana para la pasada siguiente.
func TestCheckMarcoCacheadoCorruptoNoFalsifica(t *testing.T) {
	jar := jarDelSDK(t)
	raiz, fuente := proyectoDeCheck(t)
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz); len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("la primera pasada da errores: %+v", r)
	}
	cache := rutaDelMarcoCacheado(raiz, jar)
	datos, err := os.ReadFile(cache)
	if err != nil {
		t.Fatalf("la pasada no dejó la caché del marco en %s: %v", cache, err)
	}
	if err := os.WriteFile(cache, datos[:len(datos)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	antes := marcosDeCache
	r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz)
	if len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("con la caché rota el check no enlazó: %+v", r)
	}
	if marcosDeCache != antes {
		t.Fatal("se sirvió un marco de una caché cortada")
	}

	// y la pasada siguiente ya sale de la caché reescrita: el fallo cuesta un parseo, no todos
	r = revisarFuenteJava(fuente, "MainActivity.java", jar, raiz)
	if len(r.Errores) != 0 || !r.Emisor {
		t.Fatalf("la pasada que sigue a la reescritura no compiló: %+v", r)
	}
	if marcosDeCache != antes+1 {
		t.Fatalf("la caché no se reescribió sana: el contador pasó de %d a %d", antes, marcosDeCache)
	}
}

// TestCheckMarcoCacheadoMideElAhorro mide lo que el usuario nota: `revisarFuenteJava` sobre un
// proyecto con res/ y sin R escrita, que es la pasada que enlaza. Con el índice ya caliente, la
// diferencia entre apagar y encender esta caché es el coste de inflar y parsear resources.arsc.
func TestCheckMarcoCacheadoMideElAhorro(t *testing.T) {
	jar := jarDelSDK(t)
	raiz, fuente := proyectoDeCheck(t)
	if err := os.MkdirAll(filepath.Join(raiz, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reutilizarMarcoCacheado = true })
	// una pasada de calentamiento: escribe el índice del jar y la caché del marco
	if r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz); len(r.Errores) != 0 {
		t.Fatalf("el calentamiento da errores: %+v", r)
	}
	mide := func() time.Duration {
		t.Helper()
		comienzo := time.Now()
		r := revisarFuenteJava(fuente, "MainActivity.java", jar, raiz)
		d := time.Since(comienzo)
		if len(r.Errores) != 0 || !r.Emisor {
			t.Fatalf("el check no compiló: %+v", r)
		}
		// la pasada que se mide tiene que ser la que enlaza res/: si algún día el proyecto dejara
		// de necesitar el marco, la sonda mediría otra cosa y no el ahorro de esta caché
		if !strings.Contains(r.Recursos, "enlazada de res/") {
			t.Fatalf("la pasada no enlazó res/: %q", r.Recursos)
		}
		return d
	}
	var sinCache, conCache []time.Duration
	for i := 0; i < 3; i++ {
		if err := os.Remove(rutaDelMarcoCacheado(raiz, jar)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		reutilizarMarcoCacheado = false
		sinCache = append(sinCache, mide())
		reutilizarMarcoCacheado = true
		conCache = append(conCache, mide())
		t.Logf("vuelta %d: check sin la caché del marco %.1f ms; con la caché %.1f ms",
			i+1, milisegundos(sinCache[i]), milisegundos(conCache[i]))
	}
	t.Logf("medianas: check sin la caché del marco %.1f ms; con la caché %.1f ms",
		milisegundos(mediana(sinCache)), milisegundos(mediana(conCache)))
	if mediana(conCache) >= mediana(sinCache) {
		t.Errorf("la caché del marco no ahorra en el check: %.1f ms frente a %.1f ms",
			milisegundos(mediana(conCache)), milisegundos(mediana(sinCache)))
	}
}
