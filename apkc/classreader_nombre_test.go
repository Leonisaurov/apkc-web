package main

import "testing"

// TestUnaClaseLlamadaBNoEsElPrimitivo: el nombre interno de una clase del paquete por defecto puede
// ser una letra de las que nombran un primitivo (`B`, `I`, `S`…) o empezar por la `L` de un descriptor
// (`L`). La entrada `Class` del pool siempre es un nombre interno, no un descriptor, y confundirlos
// dejaba el dex con un type_id "B" o "L" que `dexdump` rechaza (`Invalid descriptor for class_idx` y
// `Invalid type descriptor`). Lo destaparon dos sondas, `B` y `L`.
func TestUnaClaseLlamadaBNoEsElPrimitivo(t *testing.T) {
	for _, nombre := range []string{"B", "L"} {
		fuente := "public class " + nombre + " { public static int x() { return 1; } }\n"
		salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/" + nombre + ".java", contenido: fuente}}, jarDelSDK(t))
		if err != nil {
			t.Fatalf("%s: el emisor rechazo el fuente: %v", nombre, err)
		}
		if len(salida) != 1 {
			t.Fatalf("%s: %d clases y se esperaba una", nombre, len(salida))
		}
		cf, err := readClassFull(salida[0].datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", nombre, err)
		}
		querido := "L" + nombre + ";"
		if cf.Name != querido {
			t.Fatalf("%s: el nombre de la clase es %q y deberia ser %s", nombre, cf.Name, querido)
		}
		datos, err := dexDeClases([]classFull{cf})
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		d, err := readDex(datos)
		if err != nil {
			t.Fatalf("%s: el dex no se relee: %v", nombre, err)
		}
		for _, ty := range d.types {
			// `I` y `V` sí son type_ids legítimos (vuelta de un proto); el que no puede estar es el
			// nombre interno suelto de la clase
			if ty == nombre {
				t.Errorf("%s: el type_id %q es el nombre interno suelto de la clase y no su descriptor", nombre, ty)
			}
		}
		if len(d.classes) != 1 || d.classes[0].name != querido {
			t.Errorf("%s: el class_def es %+v y deberia ser %s", nombre, d.classes, querido)
		}
	}
}
