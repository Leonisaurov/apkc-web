package main

// Cómo numera javac las clases anónimas: el contador es **por clase exterior** y el número se asigna
// en orden de creación. Una anónima que vive dentro de otra no gasta número del contador de `Sonda`
// —lleva el de su anónima exterior, `Sonda$1$1`—, así que la siguiente hermana sigue donde iba el
// contador: `Sonda$2`. Nuestro analizador sí se lo gastaba, porque el `++` se hacía para todas, y el
// frente de la captura anidada destapó la consecuencia en ApkShare: `MainActivity$4$1` donde javac
// escribe `MainActivity$3$1`. No rompía el build, pero el nombre y el `checksum` del dex divergían.
//
// Medido con javac 17 `--release 8 -g:lines,source` sobre `fuenteSondaNumeracion`:
//
//	$ /home/leonisauro/opt/jdk17/bin/javac --release 8 -g:lines,source -d out p/Sonda.java
//	$ ls out/p
//	Sonda$1$1.class  Sonda$1.class  Sonda$2.class  Sonda$3$1.class  Sonda$3$2$1.class
//	Sonda$3$2.class  Sonda$3.class  Sonda$4.class  Sonda$5.class  Sonda.class
//	$ /home/leonisauro/opt/jdk17/bin/javap -p out/p/Sonda\$3\$2\$1.class
//	class p.Sonda$3$2$1 implements java.lang.Runnable {
//	  final p.Sonda$3$2 this$2;
//	  p.Sonda$3$2$1(p.Sonda$3$2);
//	  public void run();
//	}
//
// Lo que se puede leer de ahí: el número sigue el orden de creación (la hermana `Sonda$2` se crea
// después de `Sonda$1` y antes de la anónima de su `run()`, que es `Sonda$1$1`), el contador lo
// comparten todas las anónimas de la clase exterior aunque vivan en métodos distintos —`Sonda$5` nace
// en el `static b()`—, y el anidamiento se conserva a cualquier profundidad, con el campo del exterior
// nombrado por su nivel: `this$1`, `this$2`.
//
// La falsificación devuelve el contador viejo con `anonimaAnidadaGastaNumeroDelExterior` y comprueba
// que reproduce los nombres divergentes: `Sonda$3`, `Sonda$4`, `Sonda$8` y `Sonda$9` donde javac deja
// `Sonda$2`, `Sonda$3`, `Sonda$4` y `Sonda$5`.

import (
	"sort"
	"strings"
	"testing"
)

// fuenteSondaNumeracion: cuatro anónimas hermanas en `a()` —la tercera anida tres niveles— y una
// quinta en el método `static b()`. Es el mínimo que separa el contador por clase exterior del global.
const fuenteSondaNumeracion = `package p;

class Sonda {
    void corre(Runnable r) { }

    static void dispara(Runnable r) { }

    void a() {
        corre(new Runnable() {
            public void run() {
                corre(new Runnable() {
                    public void run() { }
                });
            }
        });
        corre(new Runnable() {
            public void run() { }
        });
        corre(new Runnable() {
            public void run() {
                corre(new Runnable() {
                    public void run() { }
                });
                corre(new Runnable() {
                    public void run() {
                        corre(new Runnable() {
                            public void run() { }
                        });
                    }
                });
            }
        });
        corre(new Runnable() {
            public void run() { }
        });
    }

    static void b() {
        dispara(new Runnable() {
            public void run() { }
        });
    }
}
`

func fuentesSondaNumeracion(fuente string) []fuenteJava {
	return []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuente}}
}

// descriptoresDeLoEmitido: los nombres internos de nuestras clases como descriptores de campo, que es
// la forma en la que el lector de javac deja `classFull.Name`.
func descriptoresDeLoEmitido(cs []claseEmitida) []string {
	v := make([]string, 0, len(cs))
	for _, c := range cs {
		v = append(v, "L"+c.interno+";")
	}
	sort.Strings(v)
	return v
}

// descriptoresDeJavac: los nombres de lo que dejó javac, ordenados.
func descriptoresDeJavac(cs []classFull) []string {
	v := make([]string, 0, len(cs))
	for _, c := range cs {
		v = append(v, c.Name)
	}
	sort.Strings(v)
	return v
}

// TestLaNumeracionDeAnonimasEsLaDeJavac: los nombres de las diez clases son los de javac 17 y el dex
// de los dos lados coincide byte a byte, con el atributo del constructor de la anónima fuera —la
// divergencia que ya fijó `javaemisor_captura_receptor_test.go`, no de esta ruta—.
func TestLaNumeracionDeAnonimasEsLaDeJavac(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesSondaNumeracion(fuenteSondaNumeracion)

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó las anónimas hermanas y anidadas: %v", err)
	}

	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaNumeracion)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	// el esquema de nombres, entero: por clase exterior, en orden de creación, con el sufijo del
	// anidamiento conservado
	queridos := []string{
		"Lp/Sonda$1$1;", "Lp/Sonda$1;", "Lp/Sonda$2;", "Lp/Sonda$3$1;", "Lp/Sonda$3$2$1;",
		"Lp/Sonda$3$2;", "Lp/Sonda$3;", "Lp/Sonda$4;", "Lp/Sonda$5;", "Lp/Sonda;",
	}
	nuestros := descriptoresDeLoEmitido(nuestras)
	losDeJavac := descriptoresDeJavac(deJavac)
	if strings.Join(nuestros, " ") != strings.Join(queridos, " ") {
		t.Errorf("nuestros nombres son\n  %s\ny javac deja\n  %s", strings.Join(nuestros, " "), strings.Join(queridos, " "))
	}
	if strings.Join(losDeJavac, " ") != strings.Join(queridos, " ") {
		t.Fatalf("el oráculo de javac cambió: dejó\n  %s\ny la medición fija\n  %s", strings.Join(losDeJavac, " "), strings.Join(queridos, " "))
	}

	// el nivel del exterior se conserva en el tercer anidamiento: this$2, no this$1 ni this$0
	nuestraProfunda, suyaProfunda := buscaLaClase(t, nuestras, "Lp/Sonda$3$2$1;", deJavac)
	if f := campoDe(nuestraProfunda, "this$2"); f.Desc != "Lp/Sonda$3$2;" {
		t.Errorf("la anónima de tercer nivel no guarda su exterior en this$2:Lp/Sonda$3$2;: %+v (campos %v)",
			f, nuestraProfunda.Fields)
	}
	comparaMetodos(t, nuestraProfunda, suyaProfunda)

	// y el dex entero, con el atributo del constructor de la anónima fuera
	if err := comparaDexSalvoConstructorDeAnonima(t, nuestras, deJavac); err != nil {
		t.Fatalf("%v\n%s", err, volcadoDeClases(nuestras))
	}
}

// TestLaNumeracionDeAnonimasFalsificacion: con el contador viejo encendido —el `++` para toda anónima,
// también la que vive dentro de otra— los nombres se corren y el dex deja de coincidir. Es el defecto
// que destapó ApkShare con `MainActivity$4$1`.
func TestLaNumeracionDeAnonimasFalsificacion(t *testing.T) {
	jar := jarDelSDK(t)
	fuentes := fuentesSondaNumeracion(fuenteSondaNumeracion)

	// con el contador de javac, los nombres son los medidos
	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("con el contador por clase exterior el emisor rechazó la sonda: %v", err)
	}
	if !strings.Contains(nombresDe(nuestras), "p/Sonda$2") {
		t.Fatalf("con el contador por clase exterior no salió p/Sonda$2: %s", nombresDe(nuestras))
	}

	anterior := anonimaAnidadaGastaNumeroDelExterior
	anonimaAnidadaGastaNumeroDelExterior = true
	t.Cleanup(func() { anonimaAnidadaGastaNumeroDelExterior = anterior })

	viejas, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("con el contador viejo el emisor rechazó la sonda: %v", err)
	}
	viejos := descriptoresDeLoEmitido(viejas)
	// las anidadas se comen un número cada una, así que las hermanas ya no son las de javac
	divergentes := []string{
		"Lp/Sonda$1$1;", "Lp/Sonda$1;", "Lp/Sonda$3;", "Lp/Sonda$4$1;", "Lp/Sonda$4$2$1;",
		"Lp/Sonda$4$2;", "Lp/Sonda$4;", "Lp/Sonda$8;", "Lp/Sonda$9;", "Lp/Sonda;",
	}
	if strings.Join(viejos, " ") != strings.Join(divergentes, " ") {
		t.Fatalf("el contador viejo no reproduce los nombres divergentes: %s", strings.Join(viejos, " "))
	}
	if strings.Contains(nombresDe(viejas), "p/Sonda$2") || strings.Contains(nombresDe(viejas), "p/Sonda$5") {
		t.Fatalf("el contador viejo sigue dejando los nombres de javac: %s", nombresDe(viejas))
	}

	// y el dex deja de ser el de javac, que es lo que delata el `checksum` distinto de ApkShare
	javac := javac17(t)
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuenteSondaNumeracion)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)
	if err := comparaDexSalvoConstructorDeAnonima(t, viejas, deJavac); err == nil {
		t.Fatal("con el contador viejo el dex sigue coincidiendo con javac: la falsificación no falsifica")
	} else {
		t.Logf("con el contador viejo el dex diverge: %v", err)
	}

	t.Logf("contador viejo: %s\ncontador de javac: %s", strings.Join(viejos, " "), strings.Join(descriptoresDeLoEmitido(nuestras), " "))
}
