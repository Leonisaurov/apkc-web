package main

import (
	"reflect"
	"testing"
)

// TestLaFirmaGenericaSeTroceaComoD8: la troceadura medida en el parrafo 101 con dexdump -a sobre
// `Gen` (clase, campo y metodo). Cada descriptor de clase `L…;`/`L…<` es un trozo y el resto
// (`<T::`, `>`, `()`, `>;`) va aparte.
func TestLaFirmaGenericaSeTroceaComoD8(t *testing.T) {
	casos := []struct {
		firma  string
		trozos []string
	}{
		{"<T::Ljava/lang/CharSequence;>Ljava/lang/Object;", []string{
			"<T::", "Ljava/lang/CharSequence;", ">", "Ljava/lang/Object;",
		}},
		{"Ljava/util/List<Ljava/lang/String;>;", []string{
			"Ljava/util/List<", "Ljava/lang/String;", ">;",
		}},
		{"<R:Ljava/lang/Object;>(TR;)TR;", []string{
			"<R:", "Ljava/lang/Object;", ">(TR;)TR;",
		}},
		{"()Ljava/util/Map<Ljava/lang/String;Ljava/lang/Integer;>;", []string{
			"()", "Ljava/util/Map<", "Ljava/lang/String;", "Ljava/lang/Integer;", ">;",
		}},
	}
	for _, c := range casos {
		if got := firmaEnTrozos(c.firma); !reflect.DeepEqual(got, c.trozos) {
			t.Errorf("firma %q:\nobtenido %q\nquerido  %q", c.firma, got, c.trozos)
		}
	}
}

func TestReglasDeCopianLaFirmaAlBloqueDeAnotaciones(t *testing.T) {
	cf := classFull{
		Name:      "LGen;",
		Signature: "<T::Ljava/lang/CharSequence;>Ljava/lang/Object;",
		Fields:    []classField{{Name: "xs", Desc: "Ljava/util/List;", Signature: "Ljava/util/List<Ljava/lang/String;>;"}},
		Methods:   []classMethod{{Name: "id", Desc: "(Ljava/lang/Object;)Ljava/lang/Object;", Signature: "<R:Ljava/lang/Object;>(TR;)TR;"}},
	}
	a := reglasDe(cf)
	if len(a.clase) != 1 || a.clase[0].tipo != anotacionSignature {
		t.Fatalf("la clase no lleva Signature: %+v", a.clase)
	}
	if len(a.campos) != 1 || a.campos[0].items[0].tipo != anotacionSignature {
		t.Fatalf("el campo no lleva Signature: %+v", a.campos)
	}
	if len(a.metodos) != 1 || len(a.metodos[0].items) != 1 || a.metodos[0].items[0].tipo != anotacionSignature {
		t.Fatalf("el metodo no lleva Signature: %+v", a.metodos)
	}
}
