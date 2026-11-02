package main

import "testing"

// TestUnaClaseLlamadaBNoEsElPrimitivo: el nombre interno de una clase del paquete por defecto puede
// ser una letra de las que nombran un primitivo (`B`, `I`, `S`…). La entrada `Class` del pool siempre
// es un nombre interno, no un descriptor, y confundirlos dejaba el dex con un type_id "B" que
// `dexdump` rechaza (`Invalid descriptor for class_idx`). Lo destapó una sonda llamada `B`.
func TestUnaClaseLlamadaBNoEsElPrimitivo(t *testing.T) {
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/B.java", contenido: "public class B { public static int x() { return 1; } }\n"}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases y se esperaba una", len(salida))
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	if cf.Name != "LB;" {
		t.Fatalf("el nombre de la clase es %q y deberia ser LB;", cf.Name)
	}
	datos, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	d, err := readDex(datos)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	for _, ty := range d.types {
		// `I` y `V` sí son type_ids legítimos (vuelta de un proto); el que no puede estar es el
		// nombre interno suelto de la clase
		if ty == "B" {
			t.Errorf("el type_id %q es el nombre interno suelto de la clase y no su descriptor", ty)
		}
	}
	if len(d.classes) != 1 || d.classes[0].name != "LB;" {
		t.Errorf("el class_def es %+v y deberia ser LB;", d.classes)
	}
}
