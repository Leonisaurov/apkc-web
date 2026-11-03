package main

import (
	"encoding/base64"
	"testing"
)

// La sonda del párrafo 83: `javac --release 8 -g:lines,source` de una clase con `private long largo` y
// `private double doble` tocados desde una anónima, que es lo que hace que javac escriba los `access$`
// con `dup2_x1` y deje `pop2` en el llamador.
//
//	public class D {
//	    private long largo = 5L;
//	    private double doble = 2.5;
//	    public long lee() { return largo; }
//	    public double leeDoble() { return doble; }
//	    public void escribe(long v) { largo = v; }
//	    public void escribeDoble(double v) { doble = v; }
//	    public Runnable cosa() { return new Runnable() { public void run() {
//	        largo = largo + 1; doble = doble + 1.5; } }; }
//	}
const dAnchaJavac = "yv66vgAAADQAMAkAAgADBwAEDAAFAAQBAAFEAQAFZG9ibGUJAAIABwwACAAJAQAFbGFyZ28BAAFKCgALAAwHAA0MAA4ADwEAEGphdmEvbGFuZy9PYmplY3QBAAY8aW5pdD4BAAMoKVYFAAAAAAAAAAUGQAQAAAAAAAAHABUBAANEJDEKABQAFwwADgAYAQAGKExEOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAA2xlZQEAAygpSgEACGxlZURvYmxlAQADKClEAQAHZXNjcmliZQEABChKKVYBAAxlc2NyaWJlRG9ibGUBAAQoRClWAQAEY29zYQEAFigpTGphdmEvbGFuZy9SdW5uYWJsZTsBAAphY2Nlc3MkMDAyAQAHKExEO0opSgEACmFjY2VzcyQwMDABAAYoTEQ7KUoBAAphY2Nlc3MkMTAyAQAHKExEO0QpRAEACmFjY2VzcyQxMDABAAYoTEQ7KUQBAApTb3VyY2VGaWxlAQAGRC5qYXZhAQAMSW5uZXJDbGFzc2VzACEAAgALAAAAAgACAAgACQAAAAIABQAEAAAACgABAA4ADwABABkAAAAzAAMAAQAAABMqtwAKKhQAELUABioUABK1AAGxAAAAAQAaAAAADgADAAAAAQAEAAIACwADAAEAGwAcAAEAGQAAAB0AAgABAAAABSq0AAatAAAAAQAaAAAABgABAAAABAABAB0AHgABABkAAAAdAAIAAQAAAAUqtAABrwAAAAEAGgAAAAYAAQAAAAUAAQAfACAAAQAZAAAAHgADAAMAAAAGKh+1AAaxAAAAAQAaAAAABgABAAAABgABACEAIgABABkAAAAeAAMAAwAAAAYqJ7UAAbEAAAABABoAAAAGAAEAAAAHAAEAIwAkAAEAGQAAACEAAwABAAAACbsAFFkqtwAWsAAAAAEAGgAAAAYAAQAAAAkQCAAlACYAAQAZAAAAHwAFAAMAAAAHKh9dtQAGrQAAAAEAGgAAAAYAAQAAAAEQCAAnACgAAQAZAAAAHQACAAEAAAAFKrQABq0AAAABABoAAAAGAAEAAAABEAgAKQAqAAEAGQAAAB8ABQADAAAAByonXbUAAa8AAAABABoAAAAGAAEAAAABEAgAKwAsAAEAGQAAAB0AAgABAAAABSq0AAGvAAAAAQAaAAAABgABAAAAAQACAC0AAAACAC4ALwAAAAoAAQAUAAAAAAAA"

// dAnonimaJavac: la `D$1` que javac genera para la anónima de `cosa()`.
const dAnonimaJavac = "yv66vgAAADQALwkAAgADBwAEDAAFAAYBAANEJDEBAAZ0aGlzJDABAANMRDsKAAgACQcACgwACwAMAQAQamF2YS9sYW5nL09iamVjdAEABjxpbml0PgEAAygpVgoADgAPBwAQDAARABIBAAFEAQAKYWNjZXNzJDAwMAEABihMRDspSgoADgAUDAAVABYBAAphY2Nlc3MkMDAyAQAHKExEO0opSgoADgAYDAAZABoBAAphY2Nlc3MkMTAwAQAGKExEOylEBj/4AAAAAAAACgAOAB4MAB8AIAEACmFjY2VzcyQxMDIBAAcoTEQ7RClEBwAiAQASamF2YS9sYW5nL1J1bm5hYmxlAQAGKExEOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAEE1ldGhvZFBhcmFtZXRlcnMBAANydW4BAApTb3VyY2VGaWxlAQAGRC5qYXZhAQAPRW5jbG9zaW5nTWV0aG9kDAAsAC0BAARjb3NhAQAWKClMamF2YS9sYW5nL1J1bm5hYmxlOwEADElubmVyQ2xhc3NlcwAgAAIACAABACEAARAQAAUABgAAAAIAAAALACMAAgAkAAAAIgACAAIAAAAKKiu1AAEqtwAHsQAAAAEAJQAAAAYAAQAAAAkAJgAAAAUBAACAEAABACcADAABACQAAABFAAUAAQAAACUqtAABKrQAAbgADQphuAATWCq0AAEqtAABuAAXFAAbY7gAHVixAAAAAQAlAAAADgADAAAACwARAAwAJAANAAMAKAAAAAIAKQAqAAAABAAOACsALgAAAAoAAQACAAAAAAAA"

func clasesAnchas(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(dAnchaJavac)
	if err != nil {
		t.Fatalf("el fixture de `D` no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de `D` no se lee: %v", err)
	}
	return cf
}

func claseAnonimaAncha(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(dAnonimaJavac)
	if err != nil {
		t.Fatalf("el fixture de `D$1` no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de `D$1` no se lee: %v", err)
	}
	return cf
}

// TestElAccesorDeUnCampoAncho: el `access$002` de un campo ancho es `iput-wide` (0x5a) y `return-wide`
// (0x10), sin ninguna instrucción por el `dup2_x1` —d8 tampoco escribe ninguna—, y el `pop2` que deja
// el llamador cuando el valor se tira **se absorbe** con la llamada: dos `move-result-wide` para cuatro
// invocaciones en `run`.
func TestElAccesorDeUnCampoAncho(t *testing.T) {
	cf := clasesAnchas(t)
	anonima := claseAnonimaAncha(t)
	td := newDexTables()
	if err := td.collectClasses([]classFull{cf, anonima}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()

	m, ok := buscaMetodo(cf, "access$002")
	if !ok {
		t.Fatal("`access$002` no está en la sonda")
	}
	ci, err := td.codeItemDe(cf, m)
	if err != nil {
		t.Fatalf("dexando el accesor: %v", err)
	}
	if len(ci.insns) < 3 {
		t.Fatalf("el accesor salió con %d unidades", len(ci.insns))
	}
	if byte(ci.insns[0]) != 0x5a {
		t.Errorf("la primera instrucción del accesor es %#02x y tenía que ser `iput-wide` (0x5a)", byte(ci.insns[0]))
	}
	if byte(ci.insns[2]) != 0x10 {
		t.Errorf("la tercera es %#02x y tenía que ser `return-wide` (0x10)", byte(ci.insns[2]))
	}

	run, ok := buscaMetodo(anonima, "run")
	if !ok {
		t.Fatal("`run` no está en la anónima")
	}
	ciRun, err := td.codeItemDe(anonima, run)
	if err != nil {
		t.Fatalf("dexando `run`: %v", err)
	}
	invocaciones, resultados := 0, 0
	for _, w := range ciRun.insns {
		switch byte(w) {
		case 0x71: // invoke-static
			invocaciones++
		case 0x0b: // move-result-wide
			resultados++
		}
	}
	if invocaciones != 4 {
		t.Errorf("`run` llama %d veces y se esperaban 4 (dos lecturas y dos escrituras)", invocaciones)
	}
	if resultados != 2 {
		t.Errorf("`run` guarda %d resultados y se esperaban 2: los dos `pop2` de las escrituras se absorben con la llamada", resultados)
	}
}

// TestElAccesorAnchoSaleDelFuente: el emisor escribe el accesor con `dup2_x1`, `max_locals` cuenta los
// HUECOS del descriptor —`(LD;J)J` pide tres— y el `.class` tiene la misma forma que el de javac. Sin
// el jar del SDK el fuente no se compila y el test se salta.
func TestElAccesorAnchoSaleDelFuente(t *testing.T) {
	fuente := `public class D {
    private long largo = 5L;
    private double doble = 2.5;
    public long lee() { return largo; }
    public double leeDoble() { return doble; }
    public void escribe(long v) { largo = v; }
    public void escribeDoble(double v) { doble = v; }
    public Runnable cosa() {
        return new Runnable() {
            public void run() {
                largo = largo + 1;
                doble = doble + 1.5;
            }
        };
    }
}
`
	salida, err := compilarProyectoJava([]fuenteJava{{nombre: "src/D.java", contenido: fuente}}, jarDelSDK(t))
	if err != nil {
		t.Fatalf("el emisor rechazó el fuente: %v", err)
	}
	var nuestra classFull
	encontrada := false
	nombres := []string{}
	for _, s := range salida {
		cf, err := readClassFull(s.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", s.interno, err)
		}
		nombres = append(nombres, cf.Name)
		if cf.Name == "LD;" {
			nuestra = cf
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatalf("el emisor no escribió la clase `D`; escribió %q", nombres)
	}
	suya := clasesAnchas(t)
	for _, nombre := range []string{"access$000", "access$002", "access$100", "access$102"} {
		a, ok := buscaMetodo(nuestra, nombre)
		if !ok {
			t.Fatalf("%s no está en nuestra clase", nombre)
		}
		b, ok := buscaMetodo(suya, nombre)
		if !ok {
			t.Fatalf("%s no está en la de javac", nombre)
		}
		if x, y := formaDeMetodo(t, nuestra, a), formaDeMetodo(t, suya, b); x != y {
			t.Errorf("%s:\nnuestro  %s\njavac    %s", nombre, x, y)
		}
		if a.LocalCount != b.LocalCount || a.MaxStack != b.MaxStack {
			t.Errorf("%s: huecos %d y pila %d; javac dice %d y %d", nombre, a.LocalCount, a.MaxStack, b.LocalCount, b.MaxStack)
		}
	}
}
