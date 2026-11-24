//go:build !js

// La paridad con javac 17 de javaemisor_campo_generico_test.go, que se mueve entero: todas sus pruebas son oráculo de javac,
// que se lanza como proceso, y el hospedador de la capa wasm no crea procesos. Por eso el fichero
// lleva `//go:build !js` y no se salta en silencio bajo `GOOS=js`. No hay SKIP: sin javac 17
// fallan (ver `dependencias_test.go:5-8`).

package main

// §203: los campos y los parámetros con argumentos de tipo, medidos contra javac 17. `List<Apk>`
// deja el descriptor del borrado (`Ljava/util/List;`) y la `Signature` genérica en el `field_info`;
// el dex la lleva como `dalvik.annotation.Signature` y sin ella dejaría de coincidir byte a byte.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fuenteApkDelCampo = `package com.hola.apkcstudio;

public class Apk {
    private String ruta;

    public Apk(String ruta) {
        this.ruta = ruta;
    }

    public String getRuta() {
        return ruta;
    }
}
`

// fuenteContenedorGenerico mezcla las cuatro formas del tipo genérico en campos: simple con `final`,
// anidado, cualificado y arreglo; y lo mismo en un parámetro, una local y la vuelta de un método.
const fuenteContenedorGenerico = `package com.hola.apkcstudio;

import java.util.List;
import java.util.Map;

public class Contenedor {
    private final List<Apk> apks;
    private Map<String, List<Apk>> porNombre;
    private android.widget.ArrayAdapter<Apk> adaptador;
    private List<Apk>[] tandas;

    public Contenedor() {
        this.apks = null;
    }

    public List<Apk> leer(List<Apk> copia) {
        List<Apk> otra = copia;
        return otra;
    }
}
`

// TestCampoGenericoContraJavac17: el `.class` de cada campo tiene que decir lo mismo que el de javac
// 17 en `descriptor` y en `Signature`, los métodos lo mismo en `max_stack`/`locals` y filas de línea,
// y el dex entero tiene que ser el mismo byte a byte. `final` solo cambia los flags, que aquí se
// comparan también para que el campo del usuario quede medido.
func TestCampoGenericoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteApkDelCampo)
	pon(t, raiz, "Contenedor.java", fuenteContenedorGenerico)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Apk.java", contenido: fuenteApkDelCampo},
		{nombre: "Contenedor.java", contenido: fuenteContenedorGenerico},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los campos genéricos: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"), filepath.Join(raiz, "Contenedor.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}

	nuestra, suya := buscarClaseEmitida(t, nuestras, "Lcom/hola/apkcstudio/Contenedor;", deJavac)
	for _, esperado := range suya.Fields {
		var encontrado *classField
		for i := range nuestra.Fields {
			if nuestra.Fields[i].Name == esperado.Name {
				encontrado = &nuestra.Fields[i]
				break
			}
		}
		if encontrado == nil {
			t.Errorf("falta el campo %s", esperado.Name)
			continue
		}
		if encontrado.Desc != esperado.Desc {
			t.Errorf("%s: el descriptor es %q y el de javac %q", esperado.Name, encontrado.Desc, esperado.Desc)
		}
		if encontrado.Signature != esperado.Signature {
			t.Errorf("%s: la Signature es %q y la de javac %q", esperado.Name, encontrado.Signature, esperado.Signature)
		}
		if encontrado.Access != esperado.Access {
			t.Errorf("%s: los flags son %#04x y los de javac %#04x", esperado.Name, encontrado.Access, esperado.Access)
		}
	}
	// los descriptores del borrado, escritos a mano: el argumento no puede colarse en el campo
	for nombre, quiero := range map[string]string{
		"apks":      "Ljava/util/List;",
		"porNombre": "Ljava/util/Map;",
		"adaptador": "Landroid/widget/ArrayAdapter;",
		"tandas":    "[Ljava/util/List;",
	} {
		for _, f := range nuestra.Fields {
			if f.Name != nombre {
				continue
			}
			if f.Desc != quiero {
				t.Errorf("%s: el descriptor es %q y el crudo es %q", nombre, f.Desc, quiero)
			}
		}
	}
	compararMetodos(t, nuestra, suya)

	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// fuenteConThrows: el método genérico del caso del crítico, con dos excepciones concretas en el
// `throws`. JVMS 4.7.9.1 solo admite en la `Signature` los throws de tipo variable (`^TT;`), así que
// la firma es la del método sin ellos y los dos tipos van solo en el atributo `Exceptions`.
const fuenteConThrows = `package com.hola.apkcstudio;

import java.util.List;

public class Guarda {
    void guardar(List<Apk> apks) throws java.io.IOException, IllegalStateException {
    }
}
`

// TestThrowsFueraDeLaSignatureContraJavac17: la `Signature` del método no lleva los throws, el
// `Exceptions` sí los lleva a los dos, y el dex entero coincide byte a byte con javac 17.
func TestThrowsFueraDeLaSignatureContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Apk.java", fuenteApkDelCampo)
	pon(t, raiz, "Guarda.java", fuenteConThrows)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Apk.java", contenido: fuenteApkDelCampo},
		{nombre: "Guarda.java", contenido: fuenteConThrows},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el método con throws: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Apk.java"), filepath.Join(raiz, "Guarda.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}

	nuestra, suya := buscarClaseEmitida(t, nuestras, "Lcom/hola/apkcstudio/Guarda;", deJavac)
	busca := func(c classFull, nombre, desc string) *classMethod {
		for i := range c.Methods {
			if c.Methods[i].Name == nombre && c.Methods[i].Desc == desc {
				return &c.Methods[i]
			}
		}
		return nil
	}
	desc := "(Ljava/util/List;)V"
	nuestro := busca(nuestra, "guardar", desc)
	suyo := busca(suya, "guardar", desc)
	if nuestro == nil || suyo == nil {
		t.Fatalf("falta guardar%s: nuestro %v, javac %v", desc, nuestro, suyo)
	}
	if quiero := "(Ljava/util/List<Lcom/hola/apkcstudio/Apk;>;)V"; suyo.Signature != quiero {
		t.Fatalf("la Signature de javac es %q y se midió %q", suyo.Signature, quiero)
	}
	if nuestro.Signature != suyo.Signature {
		t.Errorf("la Signature es %q y la de javac %q; los throws concretos van solo en Exceptions",
			nuestro.Signature, suyo.Signature)
	}
	if len(nuestro.Thrown) != 2 || len(suyo.Thrown) != 2 {
		t.Fatalf("Exceptions lleva %v y javac %v; se esperaban las dos excepciones", nuestro.Thrown, suyo.Thrown)
	}
	for i := range suyo.Thrown {
		if nuestro.Thrown[i] != suyo.Thrown[i] {
			t.Errorf("Exceptions[%d] es %q y el de javac %q", i, nuestro.Thrown[i], suyo.Thrown[i])
		}
	}
	unido := strings.Join(nuestro.Thrown, " ")
	if !strings.Contains(unido, "IOException") || !strings.Contains(unido, "IllegalStateException") {
		t.Errorf("Exceptions no nombra las dos excepciones: %q", unido)
	}
	if nuestro.Access != suyo.Access {
		t.Errorf("los flags son %#04x y los de javac %#04x", nuestro.Access, suyo.Access)
	}
	compararMetodos(t, nuestra, suya)

	if err := compararDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}
