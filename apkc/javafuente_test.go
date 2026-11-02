package main

import (
	"strings"
	"testing"
)

// La prueba del parser de la Fase 1G (parrafo 41). Los dos fuentes son los que de verdad hay que
// compilar en la pagina: la plantilla que genera `apkc create` y el MainActivity.java que produjo los
// seis .class fijados del studio. Lo que se comprueba es la forma del arbol, porque el emisor se mide
// contra javap en javaemisor_test.go.

const plantillaJava = `package com.miapp;

import android.app.*;
import android.os.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }
}
`

// muestraJava es el fuente del proyecto del studio, con la cadena que genero la pagina (seis .class,
// parrafo 22): una clase anonima que implementa una interface anidada, una llamada encadenada,
// MainActivity.this y un campo estatico del framework como argumento.
const muestraJava = `package com.hola.apkcstudio;

import android.app.*;
import android.widget.*;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add("Hello World!").setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {
            public boolean onMenuItemClick(MenuItem item) {
                Toast.makeText(MainActivity.this, "Clicked!", Toast.LENGTH_SHORT).show();
                return true;
            }
        });
        return true;
    }
}
`

const rJava = `package com.hola.apkcstudio;
public final class R {
  public static final class layout {
    public static final int main=0x7f020000;
  }
}
`

func TestParsearPlantilla(t *testing.T) {
	a, err := parsearJava(plantillaJava)
	if err != nil {
		t.Fatal(err)
	}
	if a.paquete != "com.miapp" {
		t.Errorf("el paquete sale %q", a.paquete)
	}
	if len(a.imports) != 2 || a.imports[0] != "android.app.*" {
		t.Errorf("los imports salen %v", a.imports)
	}
	if len(a.clases) != 1 {
		t.Fatalf("%d clases", len(a.clases))
	}
	c := a.clases[0]
	if c.nombre != "MainActivity" || c.super != "Activity" || !c.esPublica {
		t.Errorf("la clase sale %+v", c)
	}
	if len(c.metodos) != 1 {
		// el constructor <init> no esta en el fuente: lo sintetiza javac, y lo sintetiza tambien el
		// emisor de este fork (javaemisor_test.go lo comprueba contra el .class real)
		t.Fatalf("%d metodos declarados, medido 1", len(c.metodos))
	}
	if c.metodos[0].nombre != "onCreate" || c.metodos[0].retorno != "void" {
		t.Errorf("el primer metodo leido como %+v", c.metodos[0])
	}
	if len(c.metodos[0].params) != 1 || c.metodos[0].params[0].tipo != "Bundle" {
		t.Errorf("los parametros salen %+v", c.metodos[0].params)
	}
	cuerpo := c.metodos[0].cuerpo
	if len(cuerpo) != 2 {
		t.Fatalf("%d sentencias, medidas 2", len(cuerpo))
	}
	llamadaSuper, esLlamada := cuerpo[0].(jstmtExpr)
	if !esLlamada {
		t.Fatalf("la primera sentencia es %T", cuerpo[0])
	}
	sup, esSuper := llamadaSuper.e.(jllamada)
	if !esSuper || !sup.super || sup.nombre != "onCreate" || len(sup.args) != 1 {
		t.Errorf("la llamada al super sale %+v", llamadaSuper.e)
	}
	if arg, esAcceso := sup.args[0].(jacceso); !esAcceso || arg.nombre != "savedInstanceState" {
		t.Errorf("el argumento del super es %T (%+v)", sup.args[0], sup.args[0])
	}
	set, esSet := cuerpo[1].(jstmtExpr)
	if !esSet {
		t.Fatalf("la segunda sentencia es %T", cuerpo[1])
	}
	scv, esScv := set.e.(jllamada)
	if !esScv || scv.nombre != "setContentView" || scv.base != nil {
		t.Errorf("setContentView sale %+v", set.e)
	}
	campo, esCampo := scv.args[0].(jacceso)
	if !esCampo {
		t.Fatalf("el argumento es %T", scv.args[0])
	}
	// R.layout.main tiene que quedar como una cadena de accesos, para que el emisor la doblegue
	if medio, esMedio := campo.base.(jacceso); !esMedio || medio.nombre != "layout" {
		t.Errorf("la cadena de acceso sale %+v", campo.base)
	} else if raiz, esRaiz := medio.base.(jacceso); !esRaiz || raiz.nombre != "R" || raiz.base != nil {
		t.Errorf("la raiz del acceso sale %+v", medio.base)
	}
	if campo.donde.linea != 10 {
		t.Errorf("la linea del argumento es %d, en el fuente es la 10", campo.donde.linea)
	}
}

func TestParsearMuestraDelStudio(t *testing.T) {
	a, err := parsearJava(muestraJava)
	if err != nil {
		t.Fatal(err)
	}
	c := a.clases[0]
	if len(c.metodos) != 2 {
		t.Fatalf("%d metodos, medidos 2", len(c.metodos))
	}
	menu := c.metodos[1]
	if menu.nombre != "onCreateOptionsMenu" || menu.retorno != "boolean" {
		t.Errorf("el segundo metodo sale %+v", menu)
	}
	if len(menu.cuerpo) != 2 {
		t.Fatalf("%d sentencias, medidas 2", len(menu.cuerpo))
	}
	exp, esExpr := menu.cuerpo[0].(jstmtExpr)
	if !esExpr {
		t.Fatalf("la primera es %T", menu.cuerpo[0])
	}
	// menu.add("Hello World!").setOnMenuItemClickListener(new ... ) : la cadena de llamadas
	encadenada, esLlamada := exp.e.(jllamada)
	if !esLlamada || encadenada.nombre != "setOnMenuItemClickListener" {
		t.Fatalf("la primera sentencia es %T", exp.e)
	}
	if len(encadenada.args) != 1 {
		t.Fatalf("%d argumentos en la llamada encadenada", len(encadenada.args))
	}
	anadido, esLlamadaInterna := encadenada.base.(jllamada)
	if !esLlamadaInterna || anadido.nombre != "add" || len(anadido.args) != 1 {
		t.Errorf("el receptor de la cadena sale %T (%+v)", encadenada.base, encadenada.base)
	} else if receptor, esAcc := anadido.base.(jacceso); !esAcc || receptor.nombre != "menu" || receptor.base != nil {
		t.Errorf("el receptor de add sale %T (%+v)", anadido.base, anadido.base)
	} else if lit, esLit := anadido.args[0].(jlit); !esLit || lit.texto != "Hello World!" {
		t.Errorf("la cadena del add sale %T", anadido.args[0])
	}
	nuevo, esNuevo := encadenada.args[0].(jnuevo)
	if !esNuevo {
		t.Fatalf("el argumento es %T, se esperaba un new", encadenada.args[0])
	}
	if nuevo.tipo != "MenuItem.OnMenuItemClickListener" {
		t.Errorf("la interface implementada sale %q", nuevo.tipo)
	}
	if nuevo.cuerpo == nil {
		t.Fatal("la clase anonima no se apunto")
	}
	if nuevo.cuerpo.nombre != "MainActivity$1" {
		t.Errorf("javac nombra MainActivity$1 y aqui sale %q", nuevo.cuerpo.nombre)
	}
	if len(nuevo.cuerpo.metodos) != 1 || nuevo.cuerpo.metodos[0].nombre != "onMenuItemClick" {
		t.Errorf("la anonima trae %d metodos: %+v", len(nuevo.cuerpo.metodos), nuevo.cuerpo.metodos)
	}
	cuerpoInterno := nuevo.cuerpo.metodos[0].cuerpo
	if len(cuerpoInterno) != 2 {
		t.Fatalf("%d sentencias dentro de la anonima, medidas 2", len(cuerpoInterno))
	}
	show, esShow := cuerpoInterno[0].(jstmtExpr)
	if !esShow {
		t.Fatalf("la primera de dentro es %T", cuerpoInterno[0])
	}
	tostada, esTostada := show.e.(jllamada)
	if !esTostada || tostada.nombre != "show" {
		t.Errorf("la cadena de Toast sale %T", show.e)
	}
	makeText, esMake := tostada.base.(jllamada)
	if !esMake || makeText.nombre != "makeText" || len(makeText.args) != 3 {
		t.Fatalf("makeText sale %T con %+v args", tostada.base, len(makeText.args))
	}
	// MainActivity.this: acceso con base `MainActivity` y nombre `this`... o this con nombre
	if captura, esCaptura := makeText.args[0].(jthis); !esCaptura || captura.nombre != "MainActivity" {
		if acc, esAcc := makeText.args[0].(jacceso); !esAcc || acc.nombre != "this" {
			t.Errorf("la captura del exterior sale %T (%+v)", makeText.args[0], makeText.args[0])
		}
	}
	if estatico, esEst := makeText.args[2].(jacceso); !esEst || estatico.nombre != "LENGTH_SHORT" {
		t.Errorf("el tercer argumento sale %T", makeText.args[2])
	}
	ret, esRet := cuerpoInterno[1].(jretorno)
	if !esRet {
		t.Fatalf("la segunda de dentro es %T", cuerpoInterno[1])
	}
	if lit, esLit := ret.e.(jlit); !esLit || !lit.booleano {
		t.Errorf("el return sale %T", ret.e)
	}
}

func TestParsearRDeNuestroEnlazador(t *testing.T) {
	// La R la escribe el enlazador de la Fase 3 con este formato exacto; si el parser no la lee, la
	// pagina no puede compilar su propio proyecto.
	a, err := parsearJava(rJava)
	if err != nil {
		t.Fatal(err)
	}
	c := a.clases[0]
	if !c.final || len(c.interior) != 1 {
		t.Fatalf("R sale %+v", c)
	}
	dentro := c.interior[0]
	if dentro.nombre != "R$layout" {
		t.Errorf("la anidada se nombra %q, javac la llama R$layout", dentro.nombre)
	}
	if len(dentro.campos) != 1 {
		t.Fatalf("%d campos", len(dentro.campos))
	}
	campo := dentro.campos[0]
	if !campo.esEstatico || !campo.esFinal || campo.tipo != "int" || campo.nombre != "main" {
		t.Errorf("el campo sale %+v", campo)
	}
}

func TestParsearRechazaLoQueNoSeEmite(t *testing.T) {
	casos := []struct {
		fuente string
		nombre string
		razon  string
	}{
		{"class A { void a() { int i = 1; int j = i > 0 ? 1 : 2; } }", "un ternario", "no es de este subconjunto"},
		{"class A { @Deprecated void a() {} }", "una anotacion que no es Override", "@Deprecated"},
		{"class A { private int x; }", "un campo private", ""},
		{"class A { protected int x; }", "un campo protected", ""},
		{"class A { private void a() {} }", "un metodo private", ""},
		{"class A { interface I {} }", "una interface escrita a mano", "interface"},
		{"class A { }", "un fuente sin metodos no es rechazo", ""},
	}
	for _, c := range casos {
		_, err := parsearJava(c.fuente)
		if c.razon == "" {
			if err != nil {
				t.Errorf("%s: se rechazo sin motivo: %v", c.nombre, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: fue aceptado", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.razon) {
			t.Errorf("%s: el rechazo no nombra el motivo (%q): %v", c.nombre, c.razon, err)
		}
		if !strings.Contains(err.Error(), "linea") {
			t.Errorf("%s: el rechazo no dice la linea: %v", c.nombre, err)
		}
	}
}

func TestLexicoRechazaCaracteresYComentarios(t *testing.T) {
	casos := []struct {
		fuente string
		razon  string
	}{
		{"class A { void a() { char c = 'x'; } }", "char"},
		{"class A { } /* sin cerrar", "sin cerrar"},
		{"class A { void a() { int x = 1; } }\n¥", "no es de este subconjunto"},
		{"class A { void a() { String s = \"sin cerrar; } }", "una cadena sin cerrar"},
		{"class A { double x = 1.0; }", "punto"},
	}
	for _, c := range casos {
		_, err := parsearJava(c.fuente)
		if err == nil {
			t.Errorf("%q: fue aceptado", c.fuente)
			continue
		}
		if !strings.Contains(err.Error(), c.razon) {
			t.Errorf("%q: el rechazo no nombra %q: %v", c.fuente, c.razon, err)
		}
	}
}
