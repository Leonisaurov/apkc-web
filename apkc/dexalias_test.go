package main

import "testing"

// La cuenta que pisa un operando vivo (§66). `v[i] = i + 1` pone el indice y el sumando en el mismo
// registro si el `iadd` se escribe como `/2addr` sobre la izquierda, y ART ejecuta entonces
// `v[i] = i` —el valor salio 3 en vez de 5—; `d8` lo resuelve dejando el resultado en el registro
// que ya no se usa (`add-int/2addr v2, v1` para `i + j`) o en uno nuevo con la forma de tres
// registros (`sub-int v2, v1, v2`, medido en `Vivo`/`Vivo2`). Estas palabras son las que escribe
// este traductor, leidas con `dexdump -d` del aparato.
func TestLaCuentaNoPisaUnOperandoVivo(t *testing.T) {
	fuente := `package p;
public class Vivo {
    static void guarda(int[] v, int i) { v[i] = i + 1; }
    static void cruza(int[] v, int i, int j) { v[i] = i - j; }
}
`
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Vivo.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	nuestra, err := readClassFull(salida[0].datos)
	if err != nil {
		t.Fatalf("la clase no se lee: %v", err)
	}
	dex, err := dexDeClases([]classFull{nuestra})
	if err != nil {
		t.Fatalf("dexando: %v", err)
	}
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	casos := []struct {
		metodo   string
		esperado []uint16
	}{
		// const/4 v0, #1 · add-int/2addr v0, v2 (el indice) · aput v0, v1, v2 · return-void
		{"guarda", []uint16{0x1012, 0x20b0, 0x004b, 0x0201, 0x000e}},
		// sub-int v0, v2, v3 (tres registros, la izquierda vive como indice) · aput v0, v1, v2 · return-void
		{"cruza", []uint16{0x0091, 0x0302, 0x004b, 0x0201, 0x000e}},
	}
	for _, c := range casos {
		ci, err := codeItemDeMetodo(d, c.metodo)
		if err != nil {
			t.Fatalf("%s: %v", c.metodo, err)
		}
		if len(ci.insns) != len(c.esperado) {
			t.Errorf("%s: %d palabras y la forma medida tiene %d: %#x", c.metodo, len(ci.insns), len(c.esperado), ci.insns)
			continue
		}
		for i, w := range c.esperado {
			if ci.insns[i] != w {
				t.Errorf("%s: la palabra %d es %#04x y la medida %#04x", c.metodo, i, ci.insns[i], w)
			}
		}
	}
}
