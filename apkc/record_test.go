package main

// El desugrado de un record, medido contra `javac --release 17` y `d8 --min-api 26`. La sonda es
// `record Punto(int x, int y)`: sus tres `invokedynamic` de `ObjectMethods.bootstrap` tienen que
// salir del dex como llamadas a los auxiliares de la clase, la superclase tiene que ser
// `RecordTag` y no puede quedar ni un `invoke-custom`.
//
// Comandos que dejaron las sondas:
//
//	javac --release 17 -d out Punto.java && javap -v -p out/Punto.class
//	d8 --min-api 26 --output d8out --lib "$ANDROID_HOME/platforms/android-34/android.jar" out/Punto.class
//	dexdump -d d8out/classes.dex

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
)

func recordFixture(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(puntoRecordJavac)
	if err != nil {
		t.Fatalf("el fixture del record no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el lector rechazó el record: %v", err)
	}
	return cf
}

// TestRecordSeDesugra: el sitio deja de ser `invokedynamic`, la clase dueña recibe sus cuatro
// auxiliares, el `super` pasa a `RecordTag` y el dex se arma sin un solo `invoke-custom`.
func TestRecordSeDesugra(t *testing.T) {
	cf := recordFixture(t)
	dex, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatalf("el record no se desugró: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex del record no se relee: %v", err)
	}
	if ruta := os.Getenv("APKC_RECORD_DEX"); ruta != "" {
		if err := os.WriteFile(ruta, dex, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv("APKC_RECORD_DUMP") != "" {
		vuelcaDex(t, d)
	}

	// la superclase: RecordTag, que es la que hace que el record cargue por debajo de la API 34
	var punto, tag *dexClass
	for i := range d.classes {
		switch d.classes[i].name {
		case "LPunto;":
			punto = &d.classes[i]
		case recordTagDesc:
			tag = &d.classes[i]
		}
	}
	if punto == nil {
		t.Fatal("el dex no trae LPunto;")
	}
	if tag == nil {
		t.Fatal("el dex no trae la superclase sintética RecordTag")
	}
	if punto.super != recordTagDesc {
		t.Errorf("LPunto; extiende %s y el desugrado pide %s", punto.super, recordTagDesc)
	}
	if tag.super != "Ljava/lang/Object;" {
		t.Errorf("RecordTag extiende %s y se espera java/lang/Object", tag.super)
	}

	// los cuatro auxiliares, privados y sintéticos, en la lista directa
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("leyendo el class_data: %v", err)
	}
	directos := map[string]int{}
	for _, m := range porClase["LPunto;"].directs {
		directos[d.methods[m.methodIdx].name] = int(m.accessFlags)
	}
	for _, nombre := range []string{recordCampos, recordToString, recordHashCode, recordEquals} {
		acceso, ok := directos[nombre]
		if !ok {
			t.Errorf("LPunto; no declara %s", nombre)
			continue
		}
		if acceso&accPrivate == 0 || acceso&accSynthetic == 0 {
			t.Errorf("%s sale con 0x%x y se espera PRIVATE SYNTHETIC", nombre, acceso)
		}
	}

	// ni una llamada dinámica en todo el fichero
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatalf("%s.%s: %v", c.name, d.methods[m.methodIdx].name, err)
			}
			for _, w := range ci.insns {
				if byte(w) == 0xfa || byte(w) == 0xfb || byte(w) == 0xfc {
					t.Errorf("%s.%s lleva la palabra %#04x: un invoke-custom no puede quedar en el dex",
						c.name, d.methods[m.methodIdx].name, w)
				}
			}
		}
	}

	// el texto del record: los literales que arma el `toString`
	for _, quiere := range []string{"Punto[", "x=", "y=", ", ", "]"} {
		if !contieneCadena(d.strings, quiere) {
			t.Errorf("el dex no lleva la cadena %q del toString", quiere)
		}
	}
	// y el arreglo boxeado del hashCode/equals, con su tipo
	if !contieneCadena(d.types, "[Ljava/lang/Object;") {
		t.Error("el dex no nombra [Ljava/lang/Object;")
	}
	for _, m := range []string{"java/util/Arrays", "java/lang/Integer"} {
		encontrado := false
		for _, t := range d.types {
			if t == "L"+m+";" {
				encontrado = true
			}
		}
		if !encontrado {
			t.Errorf("el dex no nombra %s", m)
		}
	}
}

func contieneCadena(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

// TestRecordYLambdaEnElMismoDex: el desugrado de un record no puede pisar el de una lambda. Van
// las dos clases juntas, y el dex tiene que traer la sintética de la lambda, los cuatro auxiliares
// del record y una sola superclase `RecordTag`.
func TestRecordYLambdaEnElMismoDex(t *testing.T) {
	dex, err := dexDeClases([]classFull{recordFixture(t), lambdaFixture(t, lamJavac)})
	if err != nil {
		t.Fatalf("el record y la lambda no se desugraron juntos: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	var record, sintetica, tags int
	for _, c := range d.classes {
		switch {
		case c.name == "LPunto;":
			record++
		case c.name == recordTagDesc:
			tags++
		case strings.HasPrefix(c.name, "LLam$$Lambda$"):
			sintetica++
		}
	}
	if record != 1 {
		t.Errorf("el dex trae %d veces LPunto;", record)
	}
	if sintetica != 1 {
		t.Errorf("el dex trae %d clases sintéticas de lambda, se espera 1", sintetica)
	}
	if tags != 1 {
		t.Errorf("el dex trae %d RecordTag, se espera 1", tags)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("leyendo el class_data: %v", err)
	}
	directos := map[string]bool{}
	for _, m := range porClase["LPunto;"].directs {
		directos[d.methods[m.methodIdx].name] = true
	}
	for _, nombre := range []string{recordCampos, recordToString, recordHashCode, recordEquals} {
		if !directos[nombre] {
			t.Errorf("LPunto; perdió %s cuando se dexó junto a la lambda", nombre)
		}
	}
}

// TestRecordTagNoSeDuplica: si la lista ya trae la superclase sintética, la expansión no añade
// otra. Es la clase que `d8` mete una sola vez para todos los records del fichero.
func TestRecordTagNoSeDuplica(t *testing.T) {
	ya := classFull{Name: recordTagDesc, Super: "Ljava/lang/Object;"}
	clases, err := expandirLambdas([]classFull{recordFixture(t), ya})
	if err != nil {
		t.Fatalf("la expansión falló: %v", err)
	}
	tags := 0
	for _, cf := range clases {
		if cf.Name == recordTagDesc {
			tags++
		}
	}
	if tags != 1 {
		t.Errorf("la expansión dejó %d RecordTag, se espera 1", tags)
	}
}

// vuelcaDex imprime las palabras de cada método, con el desmontaje cuando lo hay. Es un anteojo,
// no una comprobación: solo corre con APKC_RECORD_DUMP puesto.
func vuelcaDex(t *testing.T, d *dexFile) {
	t.Helper()
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.classes {
		cd := porClase[c.name]
		fmt.Printf("== %s super=%s access=0x%x\n", c.name, c.super, c.access)
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("  %s regs=%d ins=%d outs=%d access=0x%x\n",
				d.methods[m.methodIdx].name, ci.registersSize, ci.insSize, ci.outsSize, m.accessFlags)
			lista, err := desmontaPalabras(ci.insns)
			if err != nil {
				fmt.Println("   desmonta falla:", err)
				continue
			}
			fmt.Println("   ", strings.Join(lista, " | "))
		}
	}
}
