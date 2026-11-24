package main

// §229: el import explícito gana al homónimo del proyecto. `resolverSimple` barría todas las clases del
// proyecto antes de mirar los imports, así que con dos `Helper` en paquetes distintos el que se hubiera
// registrado antes se comía al `import`. Medido con javac 17: el import de un solo tipo gana incluso al
// tipo del mismo paquete (JLS 6.5.5.1), y solo lo adelanta una clase del mismo compilation unit. La
// paridad byte a byte con javac 17 vive en `javaemisor_import_paridad_test.go` con `//go:build !js`,
// porque bajo wasm no hay proceso que lanzar; aquí quedan los casos que solo miran lo emitido.

import (
	"bytes"
	"strings"
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

func fuenteMainImport(paqueteMain, paqueteImportado string) string {
	return `package ` + paqueteMain + `;

import ` + paqueteImportado + `.Helper;

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
		main     string // el paquete de Main; vacío es com.x.app
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
		{
			nombre:  "import de uno con el homónimo en el mismo paquete",
			main:    "com.x.dos",
			importa: "com.x.uno",
			// el mismo paquete se registra antes y javac aun así se queda con el import (JLS 6.5.5.1)
			primero:  "dos",
			esperado: "com/x/uno/Helper",
			otro:     "com/x/dos/Helper",
		},
	}
	contenido := map[string]string{"uno": fuenteHelperImportUno, "dos": fuenteHelperImportDos}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			paqueteMain := caso.main
			if paqueteMain == "" {
				paqueteMain = "com.x.app"
			}
			segundo := "uno"
			if caso.primero == "uno" {
				segundo = "dos"
			}
			rutaMain := "src/" + strings.ReplaceAll(paqueteMain, ".", "/") + "/Main.java"
			fuentes := []fuenteJava{
				{nombre: rutaMain, contenido: fuenteMainImport(paqueteMain, caso.importa)},
				{nombre: "src/com/x/" + caso.primero + "/Helper.java", contenido: contenido[caso.primero]},
				{nombre: "src/com/x/" + segundo + "/Helper.java", contenido: contenido[segundo]},
			}
			nuestras, err := compilarProyectoJava(fuentes, jar)
			if err != nil {
				t.Fatalf("el import %s.Helper: %v", caso.importa, err)
			}
			main := porInterno(nuestras, strings.ReplaceAll(paqueteMain, ".", "/")+"/Main")
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

// TestEmisorImportExplicitoMismoPaqueteGanaAlHomonimo: el caso que fija JLS 6.5.5.1 —`Main` está en el
// mismo paquete que un `Helper` e importa el de otro— sin javac de por medio: lo emitido tiene que
// nombrar `com/x/uno/Helper` y no el homónimo del paquete. La paridad con javac del mismo caso es
// `TestEmisorImportExplicitoMismoPaqueteContraJavac17`, en `javaemisor_import_paridad_test.go`.
func TestEmisorImportExplicitoMismoPaqueteGanaAlHomonimo(t *testing.T) {
	jar := jarDelSDKExigido(t)
	main := fuenteMainImport("com.x.dos", "com.x.uno")
	fuentes := []fuenteJava{
		{nombre: "src/com/x/dos/Main.java", contenido: main},
		{nombre: "src/com/x/dos/Helper.java", contenido: fuenteHelperImportDos},
		{nombre: "src/com/x/uno/Helper.java", contenido: fuenteHelperImportUno},
	}
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el caso: %v", err)
	}
	mainEmitida := porInterno(nuestras, "com/x/dos/Main")
	if mainEmitida == nil {
		t.Fatalf("no salió Main: %s", nombresDe(nuestras))
	}
	if !bytes.Contains(mainEmitida.datos, []byte("com/x/uno/Helper")) {
		t.Errorf("el import com.x.uno.Helper no ganó al homónimo del mismo paquete: %s", volcadoDeClases(nuestras))
	}
	if bytes.Contains(mainEmitida.datos, []byte("com/x/dos/Helper")) {
		t.Errorf("el import com.x.uno.Helper perdió contra el Helper del mismo paquete: %s", volcadoDeClases(nuestras))
	}
}
