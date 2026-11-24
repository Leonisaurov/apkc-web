package main

// §200: el acceso estatico encadenado como argumento (`setContentView(R.layout.main)`) byte a byte
// contra javac 17. El oraculo es el de `compilar_test.go`: las dos compilaciones se dexan con el mismo
// dexer y los dos dex tienen que ser identicos, byte a byte. Aqui javac corre en caliente porque el
// caso es nuevo y no habia fixture; si no hay un javac 17 en la maquina, el test se salta y lo dice.

import (
	"os"
	"os/exec"
	"strings"
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
