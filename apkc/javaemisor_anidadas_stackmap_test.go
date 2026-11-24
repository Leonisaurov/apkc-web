package main

// El StackMapTable de una clase anidada, contra javac 17.
//
// Lo que se mide aquí es la *forma* del atributo y no solo su contenido: hasta este frente el emisor
// escribía un `full_frame` en cada comienzo de bloque, que el JVMS permite en cualquier posición y que
// la JVM verifica, pero que no es lo que escribe javac (`ClassWriter.getInstance` comprime a `same`,
// `same_locals_1_stack_item`, `append` y `chop` comparando con el marco anterior, y con
// `uninitializedThis` en el hueco 0 de un constructor hasta su `super`/`this`). El dex no lo ve —el
// dexer tira el atributo—, así que el oráculo de este frente es el `.class`: la secuencia de formas y
// sus posiciones, iguales a las de javac.
//
// Los índices del pool no se comparan (el pool está ordenado distinto); sí el tipo al que apuntan. El
// oráculo es javac 17.0.21 `--release 8 -g:lines,source`, el JDK que fija el gate (§152). La
// falsificación (`marcosSinComprimir`) devuelve el emisor a `full_frame`: sin ella la prueba no vería
// si la compresión sigue viva.

import (
	"fmt"
	"strings"
	"testing"
)

// fuenteAnidadaStackMap arma las formas que se comprimen sin tocar las dos fronteras conocidas (el
// tipo declarado de un local y los locales que salen de ámbito, que este emisor escribe con el tipo
// real y sin recortar): locales `int` y un `long`, ramas, un bucle, un `try` con su
// manejador, una anónima con captura y una anónima dentro de otra. Los constructores llevan un `if`
// **después** del `super`, que es donde javac escribe `full_frame` por el `uninitializedThis` del
// marco implícito y `same` en el siguiente.
const fuenteAnidadaStackMap = `package p;

public class Anidada {
    int n;

    Anidada(boolean b) {
        if (b) { n = 1; } else { n = 2; }
        if (n > 1) { n = n + 1; }
    }

    static int conRama(int k) {
        if (k > 3) { return 1; }
        return 2;
    }

    static int conBucle(int k) {
        int t = 0;
        while (t < k) { t = t + 1; }
        return t;
    }

    static int conLargo(int k) {
        long suma = k;
        if (suma > 3) { return 1; }
        return 2;
    }

    static int conTry(int k) {
        try {
            if (k > 0) { return k; }
            return -k;
        } catch (RuntimeException e) {
            return 0;
        }
    }

    static Runnable conCaptura(final int x) {
        return new Runnable() {
            public void run() {
                if (x > 0) { System.out.println(x); }
            }
        };
    }

    Runnable conAnidada() {
        return new Runnable() {
            public void run() {
                Runnable dentro = new Runnable() {
                    public void run() { System.out.println("x"); }
                };
                dentro.run();
            }
        };
    }
}
`

// formaDeTabla devuelve la secuencia de formas del StackMapTable expandida a texto: cada entrada con
// su forma, su delta y los tipos con el NOMBRE de la clase, no el índice del pool. Es lo comparable
// entre dos `.class` cuyo pool está ordenado distinto.
func formaDeTabla(tabla []byte, p *pool) (string, error) {
	c := &cursor{b: tabla}
	n := c.u2()
	var partes []string
	tipo := func(t tipoVerif) string {
		if t.etiqueta == 'u' {
			return "uninitializedThis"
		}
		return t.String()
	}
	for i := 0; i < n && !c.failed(); i++ {
		ft := c.u1()
		switch {
		case ft <= 63:
			partes = append(partes, fmt.Sprintf("same/%d", ft))
		case ft <= 127:
			t, err := tipoDeMarcoAnidada(c, p)
			if err != nil {
				return "", err
			}
			partes = append(partes, fmt.Sprintf("same1/%d/%s", ft-64, tipo(t)))
		case ft == 247:
			d := c.u2()
			t, err := tipoDeMarcoAnidada(c, p)
			if err != nil {
				return "", err
			}
			partes = append(partes, fmt.Sprintf("same1x/%d/%s", d, tipo(t)))
		case ft >= 248 && ft <= 250:
			d := c.u2()
			partes = append(partes, fmt.Sprintf("chop%d/%d", 251-int(ft), d))
		case ft == 251:
			partes = append(partes, fmt.Sprintf("samex/%d", c.u2()))
		case ft >= 252 && ft <= 254:
			d := c.u2()
			var ts []string
			for k := 0; k < int(ft)-251; k++ {
				t, err := tipoDeMarcoAnidada(c, p)
				if err != nil {
					return "", err
				}
				ts = append(ts, tipo(t))
			}
			partes = append(partes, fmt.Sprintf("append%d/%d/[%s]", int(ft)-251, d, strings.Join(ts, ",")))
		case ft == 255:
			d := c.u2()
			nl := c.u2()
			var ts []string
			for k := 0; k < nl; k++ {
				t, err := tipoDeMarcoAnidada(c, p)
				if err != nil {
					return "", err
				}
				ts = append(ts, tipo(t))
			}
			ns := c.u2()
			var ps []string
			for k := 0; k < ns; k++ {
				t, err := tipoDeMarcoAnidada(c, p)
				if err != nil {
					return "", err
				}
				ps = append(ps, tipo(t))
			}
			partes = append(partes, fmt.Sprintf("full/%d/[%s]/[%s]", d, strings.Join(ts, ","), strings.Join(ps, ",")))
		default:
			return "", fmt.Errorf("el frame_type %d no esta medido", ft)
		}
	}
	if c.failed() {
		return "", fmt.Errorf("la tabla se corta: %s", c.reason())
	}
	if len(partes) != n {
		return "", fmt.Errorf("%d entradas anunciadas y %d leidas", n, len(partes))
	}
	return strings.Join(partes, " "), nil
}

// tipoDeMarcoAnidada lee un verification_type_info de las seis formas del JVMS. Es local a esta prueba
// porque `leeTipoDeMarco` (jvmframes_test.go) no tiene los anchos: aquí hace falta el `long` del `if`
// de `conLargo`.
func tipoDeMarcoAnidada(c *cursor, p *pool) (tipoVerif, error) {
	switch tag := c.u1(); tag {
	case 0:
		return verifTop, nil
	case 1:
		return verifInt, nil
	case 2:
		return verifFlotante, nil
	case 3:
		return verifDoble, nil
	case 4:
		return verifLargo, nil
	case 5:
		return verifNulo, nil
	case 6:
		return verifNoIniciado, nil
	case 7:
		return verifRef(p.utf[p.class[c.u2()]]), nil
	case 8:
		return tipoVerif{etiqueta: 'u', clase: fmt.Sprintf("%d", c.u2())}, nil
	default:
		return tipoVerif{}, fmt.Errorf("el verification_type_info %d no esta medido", tag)
	}
}

// tablaYFormaDelMetodo saca del `.class` el atributo StackMapTable del método pedido —los bytes tal
// cual y su forma expandida a nombres—. Devuelve `hay=false` si el método no lo lleva.
func tablaYFormaDelMetodo(datos []byte, metodo string) (forma string, crudo []byte, hay bool, err error) {
	c := &cursor{b: datos}
	if c.u4() != classMagic {
		return "", nil, false, fmt.Errorf("no es una clase")
	}
	c.u2()
	c.u2()
	p, err := c.constantPool()
	if err != nil {
		return "", nil, false, err
	}
	c.u2()
	c.u2()
	c.u2()
	c.u2s()
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		c.u2()
		c.u2()
		c.u2()
		leerAtributos(c, p)
	}
	for i, n := 0, c.u2(); i < n && !c.failed(); i++ {
		c.u2()
		nombre := p.text(c.u2())
		c.u2()
		attrs := leerAtributos(c, p)
		if nombre != metodo {
			continue
		}
		codigo, ok := attrs["Code"]
		if !ok {
			return "", nil, false, fmt.Errorf("%s no lleva Code", metodo)
		}
		cc := &cursor{b: codigo}
		cc.u2()
		cc.u2()
		cc.bytes(int(cc.u4()))
		for j, m := 0, cc.u2(); j < m && !cc.failed(); j++ {
			cc.bytes(8)
		}
		internos := leerAtributos(cc, p)
		if cc.failed() {
			return "", nil, false, fmt.Errorf("%s: %s", metodo, cc.reason())
		}
		tabla, ok := internos["StackMapTable"]
		if !ok {
			return "", nil, false, nil
		}
		forma, err = formaDeTabla(tabla, p)
		return forma, tabla, true, err
	}
	return "", nil, false, fmt.Errorf("el metodo %s no esta en la clase", metodo)
}

// TestLosMarcosDelConstructorLlevanUninitializedThis fija la mitad que no se ve en el dex: el hueco 0
// del marco implícito de un constructor es `uninitializedThis` (ITEM 6) y por eso javac escribe
// `full_frame` en el primer `if` posterior al `super` y `same` en el siguiente. Se comprueba sobre los
// bytes del atributo de `Anidada.<init>`, sin expandir.
func TestLosMarcosDelConstructorLlevanUninitializedThis(t *testing.T) {
	jar := jarDelSDK(t)
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/p/Anidada.java", contenido: fuenteAnidadaStackMap}}, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	var crudo []byte
	for _, c := range salida {
		if c.interno != "p/Anidada" {
			continue
		}
		_, tabla, hay, err := tablaYFormaDelMetodo(c.datos, "<init>")
		if err != nil {
			t.Fatalf("Anidada.<init>: %v", err)
		}
		if !hay {
			t.Fatal("el constructor no lleva StackMapTable")
		}
		crudo = tabla
	}
	if len(crudo) == 0 {
		t.Fatal("no salió la clase p/Anidada")
	}
	// number_of_entries (2 bytes) y el frame_type de la primera entrada: 255 es `full_frame`, la forma
	// que javac elige para el primer marco del constructor porque el anterior todavía tiene
	// `uninitializedThis` y este ya la clase inicializada.
	if len(crudo) < 3 || crudo[2] != 255 {
		t.Fatalf("el primer marco del constructor no es `full_frame`: % x", crudo)
	}
}
