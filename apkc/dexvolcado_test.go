package main

// Volcado de las palabras de un `classes.dex`, por método y con sus ids: es la herramienta con la que
// se midieron los formatos nuevos del parrafo 50 (d8 sobre las clases de javac, y aquí se lean los
// números que escribió). Sin la variable de entorno se omite: no es una comprobación, es un anteojo.
//
//	apkc_dex=… sh -c 'APKC_DEX_FIXTURE=$apkc_dex go test -run TestVolcadoDeDex -v .'

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestVolcadoDeDex(t *testing.T) {
	ruta := os.Getenv("APKC_DEX_FIXTURE")
	if ruta == "" {
		t.Skip("sin APKC_DEX_FIXTURE")
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	d, err := readDex(b)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range d.fields {
		fmt.Printf("f%d %s.%s:%s\n", i, f.owner, f.name, f.typeOf)
	}
	for i, m := range d.methods {
		fmt.Printf("m%d %s.%s(%s)%s\n", i, m.owner, m.name, m.proto.shorty, m.proto.returnType)
	}
	porClase, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.classes {
		cd := porClase[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			ci, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("== %s %s regs=%d ins=%d outs=%d palabras=%d\n",
				c.name, d.methods[m.methodIdx].name, ci.registersSize, ci.insSize, ci.outsSize, len(ci.insns))
			for k, w := range ci.insns {
				fmt.Printf("  [%02d] %04x (op %02x alta %02x)\n", k, w, byte(w), byte(w>>8))
			}
			if lista, err := desmontaPalabras(ci.insns); err == nil {
				fmt.Println("  desmontado:", strings.Join(lista, " | "))
			} else {
				fmt.Println("  desmontado falla:", err)
			}
		}
	}
}
