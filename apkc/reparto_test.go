package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// clasesSinteticas fabrica clases sin código: los method_id, field_id y type_id que declaran
// son los que un dex tendría que internar, y con 70 clases de 1000 miembros se pasa del techo
// de los índices de 16 bits sin compilar un solo .java ni depender de javac.
func clasesSinteticas(cuantas, metodos, campos, interfaces int) []classFull {
	clases := make([]classFull, 0, cuantas)
	for i := 0; i < cuantas; i++ {
		cf := classFull{
			Name:  fmt.Sprintf("Lgrande/C%d;", i),
			Super: "Ljava/lang/Object;",
		}
		for m := 0; m < metodos; m++ {
			cf.Methods = append(cf.Methods, classMethod{Name: fmt.Sprintf("m%d", m), Desc: "()I"})
		}
		for f := 0; f < campos; f++ {
			cf.Fields = append(cf.Fields, classField{Name: fmt.Sprintf("f%d", f), Desc: "I"})
		}
		for k := 0; k < interfaces; k++ {
			cf.Interfaces = append(cf.Interfaces, fmt.Sprintf("Lgrande/I%d_%d;", i, k))
		}
		clases = append(clases, cf)
	}
	return clases
}

func nombresDeGrupo(grupo grupoDex) []string {
	nombres := make([]string, 0, len(grupo.clases))
	for _, cf := range grupo.clases {
		nombres = append(nombres, cf.Name)
	}
	return nombres
}

// TestReparteEnDexesParteUnBucketQueNoCabe: 70 clases con 1000 métodos declarados son 70 000
// method_id, por encima de las 65 536 referencias que caben en los índices de 16 bits. El
// reparto tiene que cerrar el grupo antes del techo, no recortar índices ni dejar la clase
// fuera: se comprueba el orden, la cobertura y la cuenta de cada grupo.
func TestReparteEnDexesParteUnBucketQueNoCabe(t *testing.T) {
	clases := clasesSinteticas(70, 1000, 0, 0)
	grupos, err := reparteEnDexes(clases, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo: %v", err)
	}
	if len(grupos) < 2 {
		t.Fatalf("%d grupos para 70 000 métodos: el bucket no se partió", len(grupos))
	}
	orden := []string{}
	for i, g := range grupos {
		if c := cuentaDe(g.tablas); c.mayor() > refsPorDex {
			t.Errorf("el grupo %d se pasa del techo: %s", i+1, c)
		}
		if len(g.clases) == 0 {
			t.Errorf("el grupo %d está vacío", i+1)
		}
		orden = append(orden, nombresDeGrupo(g)...)
	}
	for i, cf := range clases {
		if orden[i] != cf.Name {
			t.Fatalf("el reparto cambió el orden: posición %d es %s y no %s", i, orden[i], cf.Name)
		}
	}
	if len(orden) != len(clases) {
		t.Fatalf("%d clases repartidas, %d de entrada", len(orden), len(clases))
	}
}

// TestLosDexDelRepartoSeReleen: cada grupo se ensambla sin pasar del techo y el lector de
// dexreader.go lo relee. Es la comprobación de que el reparto mide las tablas del fichero que
// de verdad se escribe, no una estimación que se quedó corta.
func TestLosDexDelRepartoSeReleen(t *testing.T) {
	clases := clasesSinteticas(70, 1000, 0, 0)
	dexes, err := dexDeClasesRepartidas(clases, nil)
	if err != nil {
		t.Fatalf("dexando repartido: %v", err)
	}
	if len(dexes) < 2 {
		t.Fatalf("%d dex para 70 000 métodos", len(dexes))
	}
	vistas := map[string]bool{}
	for i, b := range dexes {
		d, err := readDex(b)
		if err != nil {
			t.Fatalf("el dex %d no se relee: %v", i+1, err)
		}
		if len(d.methods) > refsPorDex {
			t.Errorf("el dex %d lleva %d method_id, por encima del techo", i+1, len(d.methods))
		}
		if len(d.fields) > refsPorDex || len(d.types) > refsPorDex || len(d.strings) > refsPorDex {
			t.Errorf("el dex %d se pasa del techo: %d campos, %d tipos, %d cadenas",
				i+1, len(d.fields), len(d.types), len(d.strings))
		}
		for _, c := range d.classes {
			vistas[c.name] = true
		}
	}
	for _, cf := range clases {
		if !vistas[cf.Name] {
			t.Errorf("%s no quedó en ningún dex", cf.Name)
		}
	}
}

// TestReparteEnDexesCuentaCamposYTipos: el techo no es solo de los métodos. 70 clases con 1000
// campos declarados son 70 000 field_id, y otras con 1000 interfaces son 70 000 type_id; las
// dos tablas llevan índices de 16 bits en las instrucciones y el reparto tiene que verlas.
func TestReparteEnDexesCuentaCamposYTipos(t *testing.T) {
	porCampos := clasesSinteticas(70, 0, 1000, 0)
	grupos, err := reparteEnDexes(porCampos, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo por campos: %v", err)
	}
	if len(grupos) < 2 {
		t.Fatalf("%d grupos para 70 000 campos", len(grupos))
	}
	for i, g := range grupos {
		if len(g.tablas.fields) > refsPorDex {
			t.Errorf("el grupo %d lleva %d field_id", i+1, len(g.tablas.fields))
		}
	}

	porTipos := clasesSinteticas(70, 0, 0, 1000)
	grupos, err = reparteEnDexes(porTipos, refsPorDex)
	if err != nil {
		t.Fatalf("repartiendo por tipos: %v", err)
	}
	if len(grupos) < 2 {
		t.Fatalf("%d grupos para 70 000 tipos", len(grupos))
	}
	for i, g := range grupos {
		if len(g.tablas.types) > refsPorDex {
			t.Errorf("el grupo %d lleva %d type_id", i+1, len(g.tablas.types))
		}
	}
}

// TestUnBucketQueCabeSigueSiendoUnDex: el reparto no toca los buckets pequeños. Un grupo, y el
// fichero idéntico byte a byte al de dexDeClases, que es el camino sin reparto.
func TestUnBucketQueCabeSigueSiendoUnDex(t *testing.T) {
	clases := clasesSinteticas(3, 40, 5, 1)
	uno, err := dexDeClases(clases)
	if err != nil {
		t.Fatalf("dexando sin reparto: %v", err)
	}
	dexes, err := dexDeClasesRepartidas(clases, nil)
	if err != nil {
		t.Fatalf("dexando con reparto: %v", err)
	}
	if len(dexes) != 1 {
		t.Fatalf("%d dex para tres clases que caben en uno", len(dexes))
	}
	if !bytes.Equal(uno, dexes[0]) {
		t.Fatal("el dex de un bucket que cabe cambió al pasar por el reparto")
	}
}

// TestElRepartoEsDeterminista: el mismo conjunto de clases da los mismos grupos, en el mismo
// orden, en dos vueltas. Es lo que hace que el nombre del segundo dex y su caché sean estables.
func TestElRepartoEsDeterminista(t *testing.T) {
	clases := clasesSinteticas(70, 1000, 0, 0)
	primera, err := reparteEnDexes(clases, refsPorDex)
	if err != nil {
		t.Fatal(err)
	}
	segunda, err := reparteEnDexes(clases, refsPorDex)
	if err != nil {
		t.Fatal(err)
	}
	if len(primera) != len(segunda) {
		t.Fatalf("dos vueltas dieron %d y %d grupos", len(primera), len(segunda))
	}
	for i := range primera {
		if a, b := strings.Join(nombresDeGrupo(primera[i]), ","), strings.Join(nombresDeGrupo(segunda[i]), ","); a != b {
			t.Fatalf("el grupo %d cambió entre vueltas:\n  %s\n  %s", i+1, a, b)
		}
	}
}

// TestUnaClaseQueNoCabeSolaEsUnError: ninguna tabla de un dex puede pasar de 65 536, y una
// clase sola que lo pase no se puede repartir. Se dice con su cuenta en vez de escribir un dex
// que apunte a otro sitio.
func TestUnaClaseQueNoCabeSolaEsUnError(t *testing.T) {
	clases := clasesSinteticas(1, 20, 0, 0)
	_, err := reparteEnDexes(clases, 10)
	if err == nil {
		t.Fatal("una clase de 20 métodos con techo 10 no puede caber en un dex")
	}
	if !strings.Contains(err.Error(), "no cabe sola en un dex") {
		t.Fatalf("el error no dice que la clase no cabe sola: %v", err)
	}
}
