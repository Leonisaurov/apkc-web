package main

// §229: el import explícito gana al homónimo del proyecto. `resolverSimple` barría todas las clases del
// proyecto antes de mirar los imports, así que con dos `Helper` en paquetes distintos el que se hubiera
// registrado antes se comía al `import`. Medido con javac 17: el import de un solo tipo gana incluso al
// tipo del mismo paquete (JLS 6.5.5.1), y solo lo adelanta una clase del mismo compilation unit.

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// fuenteHelperImportUno y fuenteHelperImportDos son los dos homónimos: el cuerpo distinto (x * 2 frente
// a x + 100) hace que un dex idéntico solo salga si se llamó al que toca.
const fuenteHelperImportUno = `package com.x.uno;

public class Helper {
    public static int doble(int x) {
        return x * 2;
    }
}
`

const fuenteHelperImportDos = `package com.x.dos;

public class Helper {
    public static int doble(int x) {
        return x + 100;
    }
}
`

func fuenteMainImport(paquete string) string {
	return `package com.x.app;

import ` + paquete + `.Helper;

public class Main {
    public static int usa(int x) {
        return Helper.doble(x);
    }
}
`
}

// TestEmisorImportExplicitoGanaAlHomonimo cubre los dos órdenes de registro: en uno el homónimo del
// proyecto entra antes que el importado y en el otro después. El bug se ve en los dos porque el barrido
// no miraba el import.
func TestEmisorImportExplicitoGanaAlHomonimo(t *testing.T) {
	jar := jarDelSDKExigido(t)
	casos := []struct {
		nombre   string
		importa  string
		primero  string // el homónimo que se registra antes en el proyecto
		esperado string
		otro     string
	}{
		{
			nombre:  "import de uno con dos registrado antes",
			importa: "com.x.uno",
			// el orden lexicográfico de fuentesJava: app, dos, uno
			primero:  "dos",
			esperado: "com/x/uno/Helper",
			otro:     "com/x/dos/Helper",
		},
		{
			nombre:   "import de dos con uno registrado antes",
			importa:  "com.x.dos",
			primero:  "uno",
			esperado: "com/x/dos/Helper",
			otro:     "com/x/uno/Helper",
		},
	}
	contenido := map[string]string{"uno": fuenteHelperImportUno, "dos": fuenteHelperImportDos}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			segundo := "uno"
			if caso.primero == "uno" {
				segundo = "dos"
			}
			fuentes := []fuenteJava{
				{nombre: "src/com/x/app/Main.java", contenido: fuenteMainImport(caso.importa)},
				{nombre: "src/com/x/" + caso.primero + "/Helper.java", contenido: contenido[caso.primero]},
				{nombre: "src/com/x/" + segundo + "/Helper.java", contenido: contenido[segundo]},
			}
			nuestras, err := compilarProyectoJava(fuentes, jar)
			if err != nil {
				t.Fatalf("el import %s.Helper: %v", caso.importa, err)
			}
			main := porInterno(nuestras, "com/x/app/Main")
			if main == nil {
				t.Fatalf("no salió Main: %s", nombresDe(nuestras))
			}
			if !bytes.Contains(main.datos, []byte(caso.esperado)) {
				t.Errorf("el import %s.Helper no resolvió a %s: %s", caso.importa, caso.esperado, volcadoDeClases(nuestras))
			}
			if bytes.Contains(main.datos, []byte(caso.otro)) {
				t.Errorf("el import %s.Helper perdió contra el homónimo %s: %s", caso.importa, caso.otro, volcadoDeClases(nuestras))
			}
		})
	}
}

// TestEmisorImportExplicitoContraJavac17 fija el dex entero byte a byte para el orden en que el bug se
// veía de verdad (el homónimo `dos` se registra antes que el importado `uno`).
func TestEmisorImportExplicitoContraJavac17(t *testing.T) {
	jar := jarDelSDKExigido(t)
	javac := javac17Exigido(t)
	main := fuenteMainImport("com.x.uno")
	fuentes := []fuenteJava{
		{nombre: "src/com/x/app/Main.java", contenido: main},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperImportDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperImportUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "src/com/x/app/Main.java", main)
	pon(t, raiz, "src/com/x/dos/Helper.java", fuenteHelperImportDos)
	pon(t, raiz, "src/com/x/uno/Helper.java", fuenteHelperImportUno)
	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "src/com/x/app/Main.java"),
		filepath.Join(raiz, "src/com/x/dos/Helper.java"),
		filepath.Join(raiz, "src/com/x/uno/Helper.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	for _, nombre := range []string{"Lcom/x/app/Main;", "Lcom/x/dos/Helper;", "Lcom/x/uno/Helper;"} {
		nuestra, suya := buscarClaseEmitida(t, nuestras, nombre, deJavac)
		compararMetodos(t, nuestra, suya)
	}
	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
