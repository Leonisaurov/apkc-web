package main

// §200: el acceso estatico encadenado como argumento (`setContentView(R.layout.main)`) byte a byte
// contra javac 17. El oraculo es el de `compilar_test.go`: las dos compilaciones se dexan con el mismo
// dexer y los dos dex tienen que ser identicos, byte a byte. Aqui javac corre en caliente porque el
// caso es nuevo y no habia fixture; si no hay un javac 17 en la maquina, el test se salta y lo dice.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteArgumentoEstatico es el caso del usuario, con las dos formas que produce un acceso de varios
// segmentos: una constante de `R.layout` como argumento de una llamada heredada y una de `R.id` como
// argumento de un constructor.
const fuenteArgumentoEstatico = `package com.hola.apkcstudio;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView t = new TextView(this);
        t.setId(R.id.pulsar);
    }
}
`

// buscarJavac17 devuelve el javac 17 de la máquina, o vacío si no hay. Es el compilador de referencia
// de este caso: el que se midio para el subconjunto.
func buscarJavac17() string {
	for _, c := range []string{os.Getenv("JAVAC17"), homeDir() + "/opt/jdk17/bin/javac", "javac"} {
		if c == "" {
			continue
		}
		ruta, err := exec.LookPath(c)
		if err != nil {
			continue
		}
		salida, err := exec.Command(ruta, "-version").CombinedOutput()
		if err != nil || !strings.Contains(string(salida), "javac 17") {
			continue
		}
		return ruta
	}
	return ""
}

func javac17(t *testing.T) string {
	t.Helper()
	if ruta := buscarJavac17(); ruta != "" {
		return ruta
	}
	t.Skip("sin javac 17: no hay con que comparar el acceso estatico encadenado")
	return ""
}

// TestAccesoEstaticoEncadenadoContraJavac17: nuestro dex de MainActivity y R, compilados por nosotros,
// y el de los mismos fuentes compilados por javac 17 con `--release 8`, tienen que coincidir byte a
// byte. Si el tipo de `R.layout.main` se estimara mal, la constante no se doblaria igual y la
// secuencia de instrucciones dejaria de coincidir.
func TestAccesoEstaticoEncadenadoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "MainActivity.java", fuenteArgumentoEstatico)
	pon(t, raiz, "R.java", fuenteRStudio)

	// lo nuestro: el emisor desde los mismos dos fuentes
	fuentes := []fuenteJava{
		{nombre: "MainActivity.java", contenido: fuenteArgumentoEstatico},
		{nombre: "R.java", contenido: fuenteRStudio},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo el caso: %v", err)
	}
	clases := make([]classFull, 0, len(nuestras))
	for _, c := range nuestras {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se relee: %v", c.interno, err)
		}
		clases = append(clases, cf)
	}
	dexNuestro, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	// lo de javac 17, con las mismas opciones con las que se midio el resto del subconjunto: version 52
	// y las lineas, que es lo unico que el emisor escribe en el ClassFile
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "MainActivity.java"), filepath.Join(raiz, "R.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	dexJavac, err := dexDeClases(deJavac)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}

	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s\n%s",
				i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras), nombresDe(nuestras))
		}
	}
}

// fuenteAsignacionCualificada es el caso de D4: el campo propio escrito con el receptor `this`. Es el
// `Apk.java` que el usuario vio rechazado con «una asignacion con mas de un nombre en el destino».
const fuenteAsignacionCualificada = `package com.hola.apkcstudio;

public class Apk {
    private String filePath;

    public Apk(String filePath) {
        this.filePath = filePath;
    }

    public String ruta() {
        return filePath;
    }
}
`

// TestAsignacionCualificadaContraJavac17: `this.filePath = filePath` tiene que salir del emisor como el
// `.class` de javac 17 y dar el mismo dex. El receptor `this` se emite con el `aload_0` que javac pone
// delante del valor, y la linea de la sentencia sigue marcandose al empezar el `jasign` —antes del
// receptor—, que es donde javac la pone: la `LineNumberTable` no se mueve.
//
// El `.class` no se compara byte a byte porque javac ordena el pool por insercion de simbolos y este
// emisor por declaracion; esa diferencia esta aceptada y escrita en `javaemisor_fuzz_test.go`. Lo que si
// se compara, ademas del dex entero (el oraculo estricto de siempre), es el `Code` y la `LineNumberTable`
// metodo a metodo: ahi se veria cualquier instruccion de mas o una fila movida.
func TestAsignacionCualificadaContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteAsignacionCualificada)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteAsignacionCualificada}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `this.filePath = filePath`: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "com/hola/apkcstudio/Apk" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
	}
	nuestra, err := readClassFull(nuestras[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	dexNuestro, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejo %d clases, una", len(deJavac))
	}
	suya := deJavac[0]

	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el metodo %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s%s: max_stack/locals %d/%d y javac %d/%d", esperado.Name, esperado.Desc,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
		// el Code no se compara byte a byte porque los dos bytes de cada indice del pool cambian con el
		// orden del pool; las instrucciones y sus referencias las normaliza el dex, que se compara entero
		// mas abajo. La LineNumberTable no depende del pool, asi que aqui si va fila a fila.
		if len(encontrada.Lines) != len(esperado.Lines) {
			t.Errorf("%s%s: %d filas de linea y javac %d", esperado.Name, esperado.Desc,
				len(encontrada.Lines), len(esperado.Lines))
			continue
		}
		for i := range esperado.Lines {
			if encontrada.Lines[i] != esperado.Lines[i] {
				t.Errorf("%s%s: la fila %d es %+v y la de javac %+v", esperado.Name, esperado.Desc,
					i, encontrada.Lines[i], esperado.Lines[i])
			}
		}
	}

	dexJavac, err := dexDeClases(deJavac)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s", i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras))
		}
	}
}

// volcadoDeClases: la lista de clases que emitimos, para que un fallo diga por donde iba.
func volcadoDeClases(cs []claseEmitida) string {
	var sb strings.Builder
	for _, c := range cs {
		sb.WriteString("  " + c.interno + "\n")
	}
	return sb.String()
}

// fuenteAsignacionReceptor: el caso que el usuario vio rechazado, `appInfo.sourceDir = filePath`, con el
// receptor arbitrario: un parámetro de tipo `ApplicationInfo`. Añade el campo propio escrito por un
// receptor que es otro objeto (`otro.ruta`), el campo propio por `this` (que sigue siendo D4) y un campo
// ancho (`long`) para que la pila cuente las dos palabras.
const fuenteAsignacionReceptor = `package com.hola.apkcstudio;

import android.content.pm.ApplicationInfo;

public class Apk {
    private String ruta;
    private long marca;

    public Apk(ApplicationInfo appInfo, String filePath) {
        appInfo.sourceDir = filePath;
        this.ruta = appInfo.sourceDir;
    }

    public void copia(Apk otro, ApplicationInfo info) {
        otro.ruta = info.packageName;
        otro.marca = 7;
    }
}
`

// TestAsignacionPorReceptorContraJavac17: `appInfo.sourceDir = filePath` —un receptor que es un
// parámetro, no `this`— tiene que salir del emisor como el `.class` de javac 17 y dar el mismo dex byte
// a byte. Se comparan además el `Code` y la `LineNumberTable` método a método: si el receptor se
// marcara con su propia fila o si la pila contara una palabra de más, se vería ahí.
func TestAsignacionPorReceptorContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteAsignacionReceptor)

	nuestras, err := compilarProyectoJava([]fuenteJava{{nombre: "Apk.java", contenido: fuenteAsignacionReceptor}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazo `appInfo.sourceDir = filePath`: %v", err)
	}
	if len(nuestras) != 1 || nuestras[0].interno != "com/hola/apkcstudio/Apk" {
		t.Fatalf("salieron %d clases (%s), y javac deja una", len(nuestras), nombresDe(nuestras))
	}
	nuestra, err := readClassFull(nuestras[0].datos)
	if err != nil {
		t.Fatalf("nuestra clase no se lee: %v", err)
	}
	dexNuestro, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando lo nuestro: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazo el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejo %d clases, una", len(deJavac))
	}
	suya := deJavac[0]

	for _, esperado := range suya.Methods {
		var encontrada *classMethod
		for i := range nuestra.Methods {
			if nuestra.Methods[i].Name == esperado.Name && nuestra.Methods[i].Desc == esperado.Desc {
				encontrada = &nuestra.Methods[i]
				break
			}
		}
		if encontrada == nil {
			t.Errorf("falta el metodo %s%s", esperado.Name, esperado.Desc)
			continue
		}
		if encontrada.MaxStack != esperado.MaxStack || encontrada.LocalCount != esperado.LocalCount {
			t.Errorf("%s%s: max_stack/locals %d/%d y javac %d/%d", esperado.Name, esperado.Desc,
				encontrada.MaxStack, encontrada.LocalCount, esperado.MaxStack, esperado.LocalCount)
		}
		if len(encontrada.Lines) != len(esperado.Lines) {
			t.Errorf("%s%s: %d filas de linea y javac %d", esperado.Name, esperado.Desc,
				len(encontrada.Lines), len(esperado.Lines))
			continue
		}
		for i := range esperado.Lines {
			if encontrada.Lines[i] != esperado.Lines[i] {
				t.Errorf("%s%s: la fila %d es %+v y la de javac %+v", esperado.Name, esperado.Desc,
					i, encontrada.Lines[i], esperado.Lines[i])
			}
		}
	}

	dexJavac, err := dexDeClases(deJavac)
	if err != nil {
		t.Fatalf("dexando lo de javac: %v", err)
	}
	if len(dexNuestro) != len(dexJavac) {
		t.Fatalf("nuestro dex mide %d bytes y el de javac %d", len(dexNuestro), len(dexJavac))
	}
	for i := range dexNuestro {
		if dexNuestro[i] != dexJavac[i] {
			t.Fatalf("dex distinto en el byte %d: %#02x frente a %#02x\n%s", i, dexNuestro[i], dexJavac[i], volcadoDeClases(nuestras))
		}
	}
}
