package main

import (
	"fmt"
	"reflect"
	"testing"
)

// fuenteTryLocal reproduce el metodo `protegido` de banco/gen-proyecto-grande.py: `b` se declara
// dentro del `try` y se lee en el `return a + b` del mismo cuerpo, con `catch` y `finally`. Delante
// va un metodo con un local de bucle, que es el que dejaba una muerte de ambito suelta en el emisor.
const fuenteTryLocal = `public class Prog {
    private int fBase;
    private double fPeso;
    private int[] fTabla;
    public Prog(int base) {
        fBase = base;
        fTabla = new int[12];
        for (int k = 0; k < 12; k++) {
            fTabla[k] = k * base;
        }
    }
    int sumar(int n) { return n + fBase; }
    int elegir(int n) { switch (n) { case 0: return fBase; default: return n; } }
    public int protegido(int x) {
        try {
            if (x < 0) {
                throw new IllegalArgumentException("negativo");
            }
            int a = sumar(x);
            int resto = x % 3;
            int b = elegir(resto);
            return a + b;
        } catch (IllegalArgumentException e) {
            return -1;
        } finally {
            fPeso = fPeso + 0.5;
        }
    }
}
`

// TestLocalDentroDeTrySeConsideraAsignado comprueba las dos caras del arreglo. Primero, que el
// fuente del generador compila y sus marcos se leen: el `iload` de `resto` no puede caer sobre un
// hueco `top`. Segundo, que una muerte de ambito de OTRO metodo que caiga justo en ese `iload` no
// poda el hueco vivo; es la forma exacta en que fallaba el primer build del proyecto grande.
func TestLocalDentroDeTrySeConsideraAsignado(t *testing.T) {
	jar := jarDelSDK(t)
	salida, err := compilarJava(fuenteTryLocal, "Prog.java", jar)
	if err != nil {
		t.Fatalf("el emisor rechazo el fuente: %v", err)
	}
	if len(salida) != 1 {
		t.Fatalf("esperaba una clase y salieron %d", len(salida))
	}
	propios, err := marcosDeClase(salida[0].datos, "protegido")
	if err != nil {
		t.Fatalf("los marcos de protegido no se leen: %v", err)
	}
	if !propios.hayTabla || len(propios.marcos) == 0 {
		t.Fatal("un metodo con catch y finally lleva StackMapTable")
	}
	code, tries, p, maxLoc, desc, estatico, err := codigoDeMetodo(salida[0].datos, "protegido")
	if err != nil {
		t.Fatal(err)
	}
	instrs, err := walkJvmCode(code)
	if err != nil {
		t.Fatal(err)
	}
	pc := -1
	for _, in := range instrs {
		if in.op == 0x1d { // iload_3: el `resto` que el generador lee antes de `elegir`
			pc = in.pc
			break
		}
	}
	if pc < 0 {
		t.Fatal("el cuerpo de protegido no lee iload_3")
	}
	limpio, _, err := framesDeMetodo(code, desc, estatico, "Prog", poolBuilderDe(p), maxLoc, tries, nil, false)
	if err != nil {
		t.Fatalf("el analisis del cuerpo limpio fallo: %v", err)
	}
	sinMuertos, err := decodificaMarcos(limpio, p, desc, estatico, "Prog", maxLoc)
	if err != nil {
		t.Fatalf("la tabla del cuerpo limpio no se lee: %v", err)
	}
	// La muerte suelta del metodo anterior cae en ese mismo `iload_3`. Antes se podaba el hueco 3 y
	// framesDeMetodo devolvia «se lee el hueco 3 sin asignar en algun camino».
	muertos := []muerteAmbito{{pc: pc, indices: []int{3}}}
	cuerpo, _, err := framesDeMetodo(code, desc, estatico, "Prog", poolBuilderDe(p), maxLoc, tries, muertos, false)
	if err != nil {
		t.Fatalf("una muerte ajena tumbo el analisis: %v", err)
	}
	conMuertos, err := decodificaMarcos(cuerpo, p, desc, estatico, "Prog", maxLoc)
	if err != nil {
		t.Fatalf("la tabla resultante no se lee: %v", err)
	}
	if !reflect.DeepEqual(sinMuertos, conMuertos) {
		t.Fatalf("la muerte ajena cambio los marcos: limpio %v, con muertos %v", sinMuertos, conMuertos)
	}
}

// TestSeLeeDesdeNoAflojaLaVerificacion: la poda sigue aplicandose cuando el hueco se escribe antes
// de leerse (no esta vivo) y se descarta cuando la lectura viene primero. Asi no se relaja la
// deteccion de huecos verdaderamente sin asignar: esos siguen llegando a `cargaLocal`.
func TestSeLeeDesdeNoAflojaLaVerificacion(t *testing.T) {
	leePrimero := []jvmInstr{{pc: 10, op: 0x1d}, {pc: 11, op: 0xac}} // iload_3 ; ireturn
	if !seLeeDesde(leePrimero, 10, 3) {
		t.Fatal("un iload_3 justo despues del cierre dice que el hueco 3 esta muerto")
	}
	escribePrimero := []jvmInstr{{pc: 10, op: 0x3e}, {pc: 11, op: 0x1d}} // istore_3 ; iload_3
	if seLeeDesde(escribePrimero, 10, 3) {
		t.Fatal("un istore_3 antes del iload_3 no deberia considerar el hueco vivo")
	}
	if seLeeDesde(leePrimero, 10, 4) {
		t.Fatal("el hueco 4 no se lee en ese cuerpo")
	}
	// Las formas cortas se repiten de cuatro en cuatro por familia: el hueco va en el opcode y no
	// en los operandos. Confundir la familia con el hueco (`aload_3` -> 19, `lload_0` -> 4) hacia
	// que una muerte ajena no se reconociera como lectura y se podara un local vivo a `top`.
	casos := []struct {
		nombre       string
		in           jvmInstr
		lee, escribe int
	}{
		{"iload_3", jvmInstr{pc: 10, op: 0x1d}, 3, -1},
		{"aload_3", jvmInstr{pc: 10, op: 0x2d}, 3, -1},
		{"astore_3", jvmInstr{pc: 10, op: 0x4e}, -1, 3},
		{"lload_0", jvmInstr{pc: 10, op: 0x1e}, 0, -1},
		{"dload_3", jvmInstr{pc: 10, op: 0x29}, 3, -1},
		{"aload 3", jvmInstr{pc: 10, op: 0x19, ops: []byte{3}}, 3, -1},
		{"astore 3", jvmInstr{pc: 10, op: 0x3a, ops: []byte{3}}, -1, 3},
	}
	for _, c := range casos {
		lee, escribe := accesoLocal(c.in)
		if lee != c.lee || escribe != c.escribe {
			t.Errorf("%s: accesoLocal dio lee=%d escribe=%d; esperaba %d y %d", c.nombre, lee, escribe, c.lee, c.escribe)
			continue
		}
		cuerpo := []jvmInstr{c.in}
		if c.lee >= 0 && !seLeeDesde(cuerpo, 10, c.lee) {
			t.Errorf("%s: seLeeDesde no vio la lectura del hueco %d", c.nombre, c.lee)
		}
		if c.escribe >= 0 && seLeeDesde(cuerpo, 10, c.escribe) {
			t.Errorf("%s: seLeeDesde vio viva la escritura del hueco %d", c.nombre, c.escribe)
		}
	}
}

// poolBuilderDe vuelve a montar las entradas crudas del pool leido para que `framesDeMetodo` pueda
// resolver los mismos indices que el codigo emitido. Las cadenas de esta sonda son ASCII, asi que
// reconstruir el Utf8 desde `p.utf` da los bytes que habia en el fichero.
func poolBuilderDe(p *pool) *poolBuilder {
	max := 0
	for i := range p.tag {
		if i > max {
			max = i
		}
	}
	b := &poolBuilder{entradas: make([][]byte, max+1), indices: map[string]int{}}
	for i := 1; i <= max; i++ {
		switch p.tag[i] {
		case cpUtf8:
			s := p.utf[i]
			e := []byte{cpUtf8, byte(len(s) >> 8), byte(len(s))}
			b.entradas[i-1] = append(e, []byte(s)...)
		case cpClass:
			b.entradas[i-1] = append([]byte{cpClass}, u2be(uint16(p.class[i]))...)
		default:
			b.entradas[i-1] = append([]byte{p.tag[i]}, p.body[i]...)
		}
	}
	return b
}

// codigoDeMetodo saca el Code, el pool y la cabecera del metodo pedido con el mismo lector que
// usan los marcos, para alimentar framesDeMetodo sin un segundo lector que se desincronice.
func codigoDeMetodo(datos []byte, metodo string) ([]byte, []entradaTry, *pool, int, string, bool, error) {
	c := &cursor{b: datos}
	if c.u4() != classMagic {
		return nil, nil, nil, 0, "", false, fmt.Errorf("no es una clase")
	}
	c.u2()
	c.u2()
	p, err := c.constantPool()
	if err != nil {
		return nil, nil, nil, 0, "", false, err
	}
	c.u2() // access_flags de la clase
	c.u2() // this_class
	c.u2() // super_class
	c.u2s()
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		c.u2()
		c.u2()
		c.u2()
		leerAtributos(c, p)
	}
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		acc := c.u2()
		nombre := p.text(c.u2())
		desc := p.text(c.u2())
		attrs := leerAtributos(c, p)
		if nombre != metodo {
			continue
		}
		cc, ok := attrs["Code"]
		if !ok {
			return nil, nil, nil, 0, "", false, fmt.Errorf("%s no lleva Code", metodo)
		}
		cur := &cursor{b: cc}
		cur.u2() // max_stack
		maxLoc := cur.u2()
		code := cur.bytes(int(cur.u4()))
		tries := []entradaTry{}
		for j, m := 0, cur.u2(); j < m && !cur.failed(); j++ {
			ini, fin, manejador := int(cur.u2()), int(cur.u2()), int(cur.u2())
			idx := int(cur.u2())
			tipo := ""
			if idx != 0 {
				tipo = "L" + p.utf[p.class[idx]] + ";"
			}
			tries = append(tries, entradaTry{inicio: ini, fin: fin, handler: manejador, tipo: tipo})
		}
		if cur.failed() {
			return nil, nil, nil, 0, "", false, fmt.Errorf("%s: %s", metodo, cur.reason())
		}
		return code, tries, p, maxLoc, desc, acc&accStatic != 0, nil
	}
	return nil, nil, nil, 0, "", false, fmt.Errorf("el metodo %s no esta en la clase", metodo)
}
