//go:build !js

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// La paridad con javac 17 de javaemisor_comodin_test.go: javac se lanza como proceso y el hospedador de la capa wasm
// no crea procesos, así que el fichero lleva `//go:build !js` y sus pruebas no se saltan en
// silencio bajo `GOOS=js`. Lo que no necesita javac sigue en `javaemisor_comodin_test.go`, sin tag. No se saltan: sin
// javac 17 fallan (ver `dependencias_test.go:5-8`).

// TestComodinesDeTipoContraJavac17: nuestro `.class` de los tres comodines tiene que decir lo mismo
// que el de javac 17 en el descriptor —el del borrado— y en la `Signature` de cada método y del
// campo, y el dex entero tiene que ser el mismo byte a byte.
func TestComodinesDeTipoContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Pantalla.java", fuenteComodines)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Pantalla.java", contenido: fuenteComodines},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los comodines: %v", err)
	}
	emitida := porInterno(nuestras, "com/hola/apkcstudio/Pantalla")
	if emitida == nil {
		t.Fatalf("no salió Pantalla (%s)", nombresDe(nuestras))
	}
	nuestra, err := readClassFull(emitida.datos)
	if err != nil {
		t.Fatalf("nuestra Pantalla no se relee: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Pantalla.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	_, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/Pantalla;", deJavac)

	// las firmas que javac 17 `--release 8` escribe en este caso, para que el test nombre lo que
	// quiere y no solo se compare con lo que salga
	quiero := []struct{ metodo, desc, firma string }{
		{"onItemClick", "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V",
			"(Landroid/widget/AdapterView<*>;Landroid/view/View;IJ)V"},
		{"acota", "(Ljava/util/List;Ljava/util/List;)V",
			"(Ljava/util/List<+Ljava/lang/Number;>;Ljava/util/List<-Ljava/lang/String;>;)V"},
		{"todas", "()Ljava/util/List;", "()Ljava/util/List<*>;"},
		{"textos", "()Ljava/util/List;", "()Ljava/util/List<+Ljava/lang/CharSequence;>;"},
	}
	for _, c := range quiero {
		desc, firma := firmaDeMetodoDe(t, suya, c.metodo)
		if desc != c.desc || firma != c.firma {
			t.Fatalf("javac 17 dejó %s %s con la firma %s, y se midió %s / %s", c.metodo, c.desc, c.firma, desc, firma)
		}
		nuestraDesc, nuestraFirma := firmaDeMetodoDe(t, nuestra, c.metodo)
		if nuestraDesc != c.desc {
			t.Errorf("%s: el descriptor es %q y el borrado es %q", c.metodo, nuestraDesc, c.desc)
		}
		if nuestraFirma != c.firma {
			t.Errorf("%s: la Signature es %q y la de javac %q", c.metodo, nuestraFirma, c.firma)
		}
	}

	// el campo: `List<?>` es `Ljava/util/List;` en el descriptor y `Ljava/util/List<*>;` en la firma
	descCampo, firmaCampo := firmaDeCampoDe(t, suya, "cosas")
	if descCampo != "Ljava/util/List;" || firmaCampo != "Ljava/util/List<*>;" {
		t.Fatalf("javac 17 dejó el campo como %q / %q", descCampo, firmaCampo)
	}
	if d, f := firmaDeCampoDe(t, nuestra, "cosas"); d != descCampo || f != firmaCampo {
		t.Errorf("el campo es %q / %q y el de javac %q / %q", d, f, descCampo, firmaCampo)
	}

	comparaMetodos(t, nuestra, suya)
	if err := comparaDex(t, nuestras, deJavac); err != nil {
		t.Fatal(err)
	}
}

// TestListenerAnonimoComodinContraJavac17: la forma real de la línea 54, el método de la escucha
// anónima. La `Signature` del `onItemClick` de `Pantalla$1` es la que javac escribe, con el `*`. El
// dex de una anónima que implementa una interface del jar no se compara: difiere 76 B con y sin
// comodín —medido con esta misma fuente sin el `?`—, que es una frontera anterior a este frente.
func TestListenerAnonimoComodinContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17Exigido(t)
	raiz := t.TempDir()
	pon(t, raiz, "Pantalla.java", fuenteListenerComodin)

	nuestras, err := compilarProyectoJava([]fuenteJava{
		{nombre: "Pantalla.java", contenido: fuenteListenerComodin},
	}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó el comodín de la escucha anónima: %v", err)
	}

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Pantalla.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó el caso: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	nuestra, suya := buscaLaClase(t, nuestras, "Lcom/hola/apkcstudio/Pantalla$1;", deJavac)

	desc, firma := firmaDeMetodoDe(t, suya, "onItemClick")
	if quiero := "(Landroid/widget/AdapterView;Landroid/view/View;IJ)V"; desc != quiero {
		t.Fatalf("el descriptor de javac es %q y el borrado es %q", desc, quiero)
	}
	if quiero := "(Landroid/widget/AdapterView<*>;Landroid/view/View;IJ)V"; firma != quiero {
		t.Fatalf("la Signature de javac es %q y se midió %q", firma, quiero)
	}
	nuestraDesc, nuestraFirma := firmaDeMetodoDe(t, nuestra, "onItemClick")
	if nuestraDesc != desc {
		t.Errorf("el descriptor de la anónima es %q y el de javac %q", nuestraDesc, desc)
	}
	if nuestraFirma != firma {
		t.Errorf("la Signature de la anónima es %q y la de javac %q", nuestraFirma, firma)
	}
	comparaMetodos(t, nuestra, suya)
}
