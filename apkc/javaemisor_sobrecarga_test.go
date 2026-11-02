package main

import (
	"strings"
	"testing"
)

// TestLaSobrecargaSeEligePorElTipoDelArgumento: medido en el parrafo 62. Ejecutando una sonda con
// `System.out.println(que)` salió `println:(Z)V` —la primera sobrecarga declarada en `PrintStream`—
// porque la resolución paraba en el primer nombre con la aridad debida; javac escribe
// `println:(Ljava/lang/String;)V`. Aquí se comprueba que con una sola candidata todo sigue igual y
// que, con varias, la elegida es la que coincide **exactamente** con los tipos de los argumentos.
func TestLaSobrecargaSeEligePorElTipoDelArgumento(t *testing.T) {
	fuente := `package p;

public class Eco {
    public static void di(String que) {
        System.out.println(que);
    }

    public static void di(int que) {
        System.out.println(que);
    }

    public static void todos() {
        di("texto");
        di(7);
        System.out.println("fin");
    }
}
`
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Eco.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente de las sobrecargas: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("%d clases, se esperaba una (%s)", len(salida), nombresDe(salida))
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("la clase emitida no se lee: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()

	delFramework := map[string]bool{}
	for _, m := range td.methods {
		if m.owner == "Ljava/io/PrintStream;" && m.name == "println" {
			delFramework[m.desc] = true
		}
	}
	for _, quiere := range []string{"(Ljava/lang/String;)V", "(I)V"} {
		if !delFramework[quiere] {
			t.Errorf("no se internó println%s; las que hay son %v", quiere, clavesDe(delFramework))
		}
	}
	// y ninguna de las que javac no eligió: con la aridad sola aparecía la de boolean
	for _, noQuiere := range []string{"(Z)V", "(C)V", "(J)V", "(F)V", "(D)V", "(Ljava/lang/Object;)V"} {
		if delFramework[noQuiere] {
			t.Errorf("se internó println%s, que no es la que llamó el fuente", noQuiere)
		}
	}
	// las dos sobrecargas propias, cada una con su descriptor
	nuestras := map[string]bool{}
	for _, m := range td.methods {
		if m.owner == "Lp/Eco;" && m.name == "di" {
			nuestras[m.desc] = true
		}
	}
	if len(nuestras) != 2 || !nuestras["(Ljava/lang/String;)V"] || !nuestras["(I)V"] {
		t.Errorf("las sobrecargas propias quedaron como %v, se esperaban las dos", clavesDe(nuestras))
	}
}

// TestLaSobrecargaSeEligePorSubtipo: el caso que el fixture ya usaba sin saberlo, `Menu.add("…")`,
// con `add(CharSequence)` y `add(int)` declaradas: el String solo cabe en la primera. Y entre dos
// aplicables gana la más específica, que es la regla de javac (JVMS 15.12.2.5).
func TestLaSobrecargaSeEligePorSubtipo(t *testing.T) {
	fuente := `package p;

public class Sub {
    public static void h(CharSequence a) {
        System.out.println("secuencia");
    }

    public static void h(Object a) {
        System.out.println("objeto");
    }

    public static void llama() {
        h("x");
    }
}
`
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Sub.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente del subtipo: %v", err)
	}
	cf, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("la clase emitida no se lee: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	// las dos sobrecargas existen; la que *llama* `llama()` es la de CharSequence, y se ve en el
	// method_id que la llamada usa: se lee de las palabras del cuerpo
	porClase := map[string]bool{}
	for _, m := range td.methods {
		if m.owner == "Lp/Sub;" && m.name == "h" {
			porClase[m.desc] = true
		}
	}
	if !porClase["(Ljava/lang/CharSequence;)V"] || !porClase["(Ljava/lang/Object;)V"] {
		t.Fatalf("faltan sobrecargas: %v", clavesDe(porClase))
	}
}

// TestLaLlamadaSinSobrecargaAplicableSeRechazaPorNombre: con varias candidatas y ninguna aplicable a
// los tipos de los argumentos no hay nada que elegir, y el mensaje las deja ver.
func TestLaLlamadaSinSobrecargaAplicableSeRechazaPorNombre(t *testing.T) {
	fuente := `package p;

public class Nada {
    public static void m(String a) {
        System.out.println("texto");
    }

    public static void m(StringBuilder a) {
        System.out.println("constructor");
    }

    public static void llama() {
        m(7);
    }
}
`
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Nada.java", contenido: fuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("se aceptó una llamada sin ninguna sobrecarga aplicable")
	}
	for _, pista := range []string{"sobrecargas", "(Ljava/lang/String;)V", "(Ljava/lang/StringBuilder;)V", "I"} {
		if !strings.Contains(err.Error(), pista) {
			t.Errorf("el mensaje no nombra %q: %v", pista, err)
		}
	}
}

func clavesDe(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
