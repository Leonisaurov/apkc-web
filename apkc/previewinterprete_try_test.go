package main

// Pruebas del try/catch del intérprete de la vista previa: el dex trae tabla de excepciones y el
// intérprete la ejecuta.
//
// El dex se genera en caliente con el emisor del fork (`compilarProyectoJava` + `dexDeClases`), el
// mismo camino que `apkc build --compiler go`: la tabla que se ejecuta es la que produce nuestro
// compilador, no una escrita a mano. El R.java sale de la tabla de recursos, para que los ids del
// fuente no discrepen de los del apk.

import (
	"strings"
	"testing"
)

// fuenteTryCatch lanza la NumberFormatException desde un método propio, dentro del try: la excepción
// cruza una llamada y el catch deja su mensaje en el TextView. El `setText` de dentro del try no
// llega a ejecutarse, porque su argumento es lo que revienta.
const fuenteTryCatch = `package com.hola.contador;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

public class TryCatch extends Activity {
    TextView total;

    int convierte(String s) {
        return Integer.parseInt(s);
    }

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        try {
            total.setText("valor: " + convierte("x"));
        } catch (NumberFormatException e) {
            total.setText("capturado: " + e.getMessage());
        }
    }
}
`

// fuenteThrowSuelto lanza una excepción que ningún catch del método recoge: el intérprete tiene que
// propagarla con su tipo y su mensaje, no seguir como si nada.
const fuenteThrowSuelto = `package com.hola.contador;

import android.app.Activity;
import android.os.Bundle;

public class Revienta extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        throw new IllegalStateException("boom");
    }
}
`

// TestElInterpreteEjecutaElTryCatchYDejaElTexto mide el camino completo: `Integer.parseInt("x")`
// revienta dentro de un método llamado desde el try, el handler del `try_item` cubre esa unidad, el
// `move-exception` deja el objeto en el registro del catch y `getMessage()` devuelve su mensaje, que
// acaba en la vista.
func TestElInterpreteEjecutaElTryCatchYDejaElTexto(t *testing.T) {
	d, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "TryCatch.java", contenido: fuenteTryCatch}))
	if err != nil {
		t.Fatalf("el dex del try/catch no se relee: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/hola/contador/TryCatch;")
	if res.Error != "" {
		t.Fatalf("el onCreate con try/catch falló: %s", res.Error)
	}
	if len(res.Capturas) != 1 {
		t.Fatalf("se esperaba una sola escritura (la del catch) y hay %+v", res.Capturas)
	}
	c := res.Capturas[0]
	if c.Texto != `capturado: For input string: "x"` {
		t.Errorf("el catch dejó %q y el mensaje de la NumberFormatException es "+
			`«capturado: For input string: "x"»`, c.Texto)
	}
	if c.IDRef != "0x7f010001" {
		t.Errorf("la escritura fue sobre %q y el TextView del layout es 0x7f010001", c.IDRef)
	}
	if len(res.Avisos) != 0 {
		t.Errorf("la ejecución dejó avisos que no debería: %v", res.Avisos)
	}
}

// TestElInterpretePropagaElThrowNoCapturado mide el otro lado: sin handler en la tabla, el error sale
// con el tipo y el mensaje de la excepción. Ni éxito fingido ni «dex malformado»: es una app que se
// cae, que es lo que hace en el aparato.
func TestElInterpretePropagaElThrowNoCapturado(t *testing.T) {
	d, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "Revienta.java", contenido: fuenteThrowSuelto}))
	if err != nil {
		t.Fatalf("el dex del throw no se relee: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/hola/contador/Revienta;")
	if res.Error == "" {
		t.Fatal("el onCreate lanza IllegalStateException y la ejecución no falló")
	}
	for _, parte := range []string{"IllegalStateException", "boom"} {
		if !strings.Contains(res.Error, parte) {
			t.Errorf("el error %q no nombra %q", res.Error, parte)
		}
	}
	if res.malformado {
		t.Errorf("una excepción de la app no invalida los bytes del dex: %q", res.Error)
	}
	if len(res.Capturas) != 0 {
		t.Errorf("nadie escribió en una vista y hay capturas: %+v", res.Capturas)
	}
}
