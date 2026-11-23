package main

// El colector sigue el operando de `ldc_w` (§201). javac cambia a `ldc_w` (0x13) cuando el índice del
// pool pasa de 255, y el inventario del jar de soporte destapó dos clases en las que un `ldc_w` de una
// `CONSTANT_Class` dejaba el tipo sin internar: `ActionBarDrawerToggleHoneycomb$SetIndicatorInfo.<init>`
// hacía `ldc_w` de `Landroid/app/ActionBar;` y el dexer se paraba con «the type_id … was never
// interned». El oráculo `d8` escribe para esa instrucción `const-class` (0x1c, 21c) con el type_id de
// la clase, y el `dexdump` del aparato lo desarma como `1c02 0800 | const-class v2, Landroid/app/ActionBar;`.
//
// La sonda compila con javac 17 un `.class` cuyo pool pasa de 255: doscientas cadenas distintas antes
// de nombrar la clase dejan el `HashMap.class` en `ldc_w #407` (medido con `javap -v`). El test exige
// que el dex lleve el `const-class` con el type_id de `Ljava/util/HashMap;`, que es la misma forma y
// las mismas palabras que escribe `d8`; sin el caso `ldc_w` del colector el tipo no está en `type_ids`
// y el dexer falla.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuenteLdcW: el pool se llena con doscientas cadenas distintas (una por método) antes de que el
// último método nombre `java.util.HashMap`, así que javac 17 lo emite con el índice de dos bytes.
func fuenteLdcW() string {
	var b strings.Builder
	b.WriteString("package p;\npublic class Muchas {\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "    static String s%d() { return \"cadena-distinta-%d\"; }\n", i, i)
	}
	b.WriteString("    public Class<?> tipo() {\n        return java.util.HashMap.class;\n    }\n}\n")
	return b.String()
}

// palabraConstClass: el segundo byte de la primera unidad es el registro y la segunda unidad el
// type_id; devuelve las dos palabras y el registro.
func palabraConstClass(insns []uint16) (primera, segunda uint16, ok bool) {
	for i := 0; i+1 < len(insns); i++ {
		if byte(insns[i]) == opConstClase {
			return insns[i], insns[i+1], true
		}
	}
	return 0, 0, false
}

// TestElLdcWInternaElTipoYCoincideConD8: un `ldc_w` de una `CONSTANT_Class` tiene que internar su tipo
// como lo hace el `ldc` de un byte. Se compara la instrucción con la que escribe `d8` sobre el mismo
// `.class`: mismo registro y type_id de la misma clase.
func TestElLdcWInternaElTipoYCoincideConD8(t *testing.T) {
	javac := javac17(t)
	jar := jarDelSDK(t)
	raiz := t.TempDir()
	pon(t, raiz, "Muchas.java", fuenteLdcW())

	salida := filepath.Join(raiz, "javac-out")
	cmd := exec.Command(javac, "--release", "8", "-g:lines,source", "-classpath", jar, "-d", salida,
		filepath.Join(raiz, "Muchas.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac 17 rechazó la sonda: %v\n%s", err, out)
	}
	deJavac, err := leeClases([]string{salida})
	if err != nil {
		t.Fatalf("leyendo lo de javac: %v", err)
	}
	if len(deJavac) != 1 {
		t.Fatalf("javac dejó %d clases, una", len(deJavac))
	}
	suya := deJavac[0]

	// La premisa del caso: el `.class` de javac tiene que traer de verdad un `ldc_w` de una clase con
	// el índice por encima de 255. Si javac dejara de emitirlo, esta prueba no mediría nada.
	const desc = "Ljava/util/HashMap;"
	x := poolIndex{suya.p}
	visto := false
	for _, m := range suya.Methods {
		lista, err := walkJvmCode(m.Code)
		if err != nil {
			t.Fatalf("%s: el Code no se recorre: %v", m.Name, err)
		}
		for _, in := range lista {
			if in.name != "ldc_w" {
				continue
			}
			idx := dexU16(in.ops)
			if x.tagOf(idx) == cpClass && x.classDescriptor(idx) == desc {
				visto = true
				if idx <= 255 {
					t.Errorf("el `ldc_w` de %s lleva el índice %d: la sonda no pasó de 255 entradas", desc, idx)
				}
			}
		}
	}
	if !visto {
		t.Fatalf("el `.class` de javac no trae un `ldc_w` de %s: la premisa de la prueba no se cumple", desc)
	}

	// Lo que se mide: el dexer acepta el `.class` y el `const-class` nombra el tipo internado.
	nuestro, err := dexDeClases([]classFull{suya})
	if err != nil {
		t.Fatalf("dexando el `.class` de javac con `ldc_w`: %v", err)
	}
	d, err := readDex(nuestro)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	ci, err := codeItemDeMetodo(d, "tipo")
	if err != nil {
		t.Fatalf("`tipo`: %v", err)
	}
	primera, segunda, ok := palabraConstClass(ci.insns)
	if !ok {
		t.Fatalf("`tipo` no lleva `const-class` (0x%02x) en el cuerpo (%04x)", opConstClase, ci.insns)
	}
	if idx := int(segunda); idx >= len(d.types) || d.types[idx] != desc {
		t.Fatalf("el type_id %d del `const-class` es %q, y se espera %q (types: %v)",
			idx, idxDeTipos(d.types, idx), desc, d.types)
	}

	// Y la misma forma que `d8`: sobre el mismo `.class`, `d8` escribe el `const-class` con el mismo
	// registro y el type_id de la misma clase. Se busca el `d8` del SDK; sin él la comparación se salta
	// y se dice, porque el resto de la prueba ya mide el internado.
	d8 := buscarD8()
	if d8 == "" {
		t.Log("sin `d8` en el SDK: no se compara la palabra con el oráculo, el internado ya está medido")
		return
	}
	salidaD8 := filepath.Join(raiz, "d8-out")
	if err := ejecutarD8(d8, javac, filepath.Join(salida, "p", "Muchas.class"), salidaD8, jar); err != nil {
		t.Fatalf("d8 rechazó la sonda: %v", err)
	}
	deD8, err := leerDexDeDirectorio(salidaD8)
	if err != nil {
		t.Fatalf("leyendo lo de d8: %v", err)
	}
	ciD8, err := codeItemDeMetodo(deD8, "tipo")
	if err != nil {
		t.Fatalf("`tipo` en el dex de d8: %v", err)
	}
	pD8, sD8, ok := palabraConstClass(ciD8.insns)
	if !ok {
		t.Fatalf("el dex de d8 no lleva `const-class` en `tipo` (%04x)", ciD8.insns)
	}
	if pD8 != primera || sD8 != segunda {
		t.Errorf("el `const-class` es %04x %04x y el de d8 %04x %04x", primera, segunda, pD8, sD8)
	}
	if idx := int(sD8); idx >= len(deD8.types) || deD8.types[idx] != desc {
		t.Errorf("el type_id %d del `const-class` de d8 no es %q", idx, desc)
	}
}

// idxDeTipos: el descriptor del type_id, o un aviso si se sale de la tabla; solo da forma al mensaje.
func idxDeTipos(types []string, i int) string {
	if i < 0 || i >= len(types) {
		return "<fuera de la tabla>"
	}
	return types[i]
}

// buscarD8: el `d8` de la build-tools más alta del SDK, o vacío si no hay SDK en la máquina.
func buscarD8() string {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		return ""
	}
	dir := filepath.Join(sdk, "build-tools")
	mejor, err := latestBuildTools(dir)
	if err != nil {
		return ""
	}
	ruta := filepath.Join(dir, mejor, "d8")
	if _, err := os.Stat(ruta); err != nil {
		return ""
	}
	return ruta
}

// ejecutarD8: dexa el `.class` de `javac` con `d8 --min-api 21 --lib android.jar`. El `d8` de las
// build-tools necesita un `java`; se le pone delante el del JDK 17 que compiló la sonda.
func ejecutarD8(d8, javac, entrada, salida, jar string) error {
	if err := os.MkdirAll(salida, 0o755); err != nil {
		return err
	}
	cmd := exec.Command(d8, "--min-api", "21", "--lib", jar, "--output", salida, entrada)
	java := filepath.Join(filepath.Dir(javac), "java")
	cmd.Env = append(os.Environ(), "JAVA_HOME="+filepath.Dir(filepath.Dir(java)), "PATH="+filepath.Dir(java)+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%s", err, out)
	}
	return nil
}

// leerDexDeDirectorio: el `classes.dex` que dejó `d8` en su directorio de salida.
func leerDexDeDirectorio(dir string) (*dexFile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "classes.dex"))
	if err != nil {
		return nil, err
	}
	return readDex(b)
}
