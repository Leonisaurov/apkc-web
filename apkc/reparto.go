package main

// Reparto de un bucket grande en varios dex.
//
// Un dex referencia sus métodos, campos, tipos y cadenas con índices de 16 bits: la segunda
// unidad de un 21c, un 22c y un 35c es un u16 (la guarda de `indice`, dexinstr.go:1351), y lo
// mismo pasa con el `string_id` de un `const-string` (dexinstr.go:2458). Un fichero con 65 536
// referencias de una de esas tablas no se puede escribir, y el traductor lo rechaza por nombre
// en vez de recortar el índice. El techo es del fichero, no del bucket: lo que cabe hacer es
// repartir las clases en varios dex, que es lo que hace `d8` sobre el mismo conjunto y lo que
// el cargador de ART lee como classes.dex, classes2.dex, … (docs/evidencia.md §109).
//
// La medida es la del propio dexer y no una estimación: se internan las clases en las tablas de
// un grupo y, cuando la siguiente saca el grupo del techo, se cierra y se abre otro con ella.
// Esas tablas se guardan con el grupo y son las que ordena y escribe el ensamblador, así que
// medir no cuesta un recorrido de más sobre las clases.

import (
	"fmt"
	"strings"
)

// refsPorDex es el número de entradas que caben en cada tabla de un dex: el índice más alto que
// se puede escribir es 0xffff, o sea 65 536 entradas contando la cero.
const refsPorDex = 1 << 16

// grupoDex es un bloque de clases y las tablas que ya se midieron para él, sin ordenar todavía.
type grupoDex struct {
	clases []classFull
	tablas *dexTables
}

// cuentaDeReferencias es lo que un dex referencia con índices de 16 bits.
type cuentaDeReferencias struct {
	cadenas int
	tipos   int
	campos  int
	metodos int
}

func cuentaDe(t *dexTables) cuentaDeReferencias {
	return cuentaDeReferencias{
		cadenas: len(t.strings),
		tipos:   len(t.types),
		campos:  len(t.fields),
		metodos: len(t.methods),
	}
}

// mayor es la tabla que manda: las cuatro comparten el mismo techo.
func (c cuentaDeReferencias) mayor() int {
	m := c.cadenas
	for _, n := range []int{c.tipos, c.campos, c.metodos} {
		if n > m {
			m = n
		}
	}
	return m
}

func (c cuentaDeReferencias) String() string {
	return fmt.Sprintf("%d métodos, %d campos, %d tipos y %d cadenas",
		c.metodos, c.campos, c.tipos, c.cadenas)
}

// armaGrupo ordena las tablas ya medidas del grupo y escribe su dex. La caché de I4 se cierra
// aquí contra las tablas ordenadas, que es cuando se sabe si los índices guardados siguen
// nombrando lo mismo; con `c` a nil el grupo se traduce entero.
func armaGrupo(g grupoDex, c *cacheDex) ([]byte, error) {
	g.tablas.orderTables()
	if c != nil {
		c.activa(g.tablas)
		g.tablas.cache = c
	}
	return g.tablas.armaDex(g.clases)
}

// mideClases interna las clases en tablas nuevas, que es la medida exacta del dex que se
// escribiría con ellas: las referencias compartidas cuentan una sola vez.
func mideClases(clases []classFull) (*dexTables, error) {
	tablas := newDexTables()
	if err := tablas.collectClasses(clases); err != nil {
		return nil, err
	}
	return tablas, nil
}

// reparteEnDexes parte las clases, en su orden, en los grupos más grandes que caben en un dex.
// Cada grupo se mide con la tabla del grupo entero, no con la suma de las clases por separado:
// lo que no cabe son las referencias internadas, y dos clases que nombran el mismo método
// comparten su method_id. Una clase que no cabe sola es un error con su cuenta, no un dex
// recortado; y `limite` se puede bajar para probar el reparto sin fabricar 70 000 métodos.
func reparteEnDexes(clases []classFull, limite int) ([]grupoDex, error) {
	return reparteEnDexesConFijas(clases, nil, limite)
}

// reparteEnDexesConFijas es reparteEnDexes con una lista de clases que no se pueden quedar fuera
// del primer dex. Por debajo de API 21 el runtime abre `classes.dex` y, para el resto, necesita el
// cargador del soporte de multidex: las clases del proyecto tienen que viajar en el primero o la
// app no llega a arrancar. Las fijas se adelantan en su orden relativo y el resto rellena ese
// primer grupo y los siguientes, así que el reparto de siempre (sin fijas) es el mismo de antes.
// Una fija que no cabe en el primer dex es un error con su cuenta y el motivo, no un
// `classes2.dex` que ese runtime no abriría sin cargador.
func reparteEnDexesConFijas(clases []classFull, fijas map[string]bool, limite int) ([]grupoDex, error) {
	if limite <= 0 {
		limite = refsPorDex
	}
	// el orden de entrada es el del internado y se conserva: las fijas van delante y las demás
	// detrás, cada grupo en el orden en que llegaron, para que el mismo proyecto dé el mismo dex
	orden := make([]classFull, 0, len(clases))
	fijasPresentes := 0
	for _, cf := range clases {
		if esClaseFija(cf.Name, fijas) {
			orden = append(orden, cf)
			fijasPresentes++
		}
	}
	for _, cf := range clases {
		if !esClaseFija(cf.Name, fijas) {
			orden = append(orden, cf)
		}
	}
	// las fijas que todavía no están en un grupo cerrado: en cuanto una no cabe, el reparto no
	// puede cumplir el contrato del dex primario y se rechaza con ese motivo
	fijasFuera := fijasPresentes
	grupos := []grupoDex{}
	enCurso := []classFull{}
	tablas := newDexTables()
	for _, cf := range orden {
		esFija := esClaseFija(cf.Name, fijas)
		// la clase se interna para saber si cabe, así que un grupo que se cierra después de
		// esta prueba lleva en su tabla una clase que no es suya: se vuelve a medir antes de
		// guardarlo. Es un recorrido de más por grupo, y solo cuando hay reparto.
		if err := tablas.collectClasses([]classFull{cf}); err != nil {
			return nil, err
		}
		c := cuentaDe(tablas)
		if len(enCurso) == 0 {
			if c.mayor() > limite {
				if esFija {
					return nil, errorMainDex(fijasPresentes, fijasFuera, c, limite)
				}
				return nil, fmt.Errorf("%s no cabe sola en un dex: %s, y el techo de los índices de 16 bits es %d",
					cf.Name, c, limite)
			}
			enCurso = append(enCurso, cf)
			if esFija {
				fijasFuera--
			}
			continue
		}
		if c.mayor() <= limite {
			enCurso = append(enCurso, cf)
			if esFija {
				fijasFuera--
			}
			continue
		}
		// el grupo se cierra aquí; si la que no cabe es una fija, el dex primario no puede
		// llevar todas las clases del proyecto y el reparto no se escribe
		if esFija {
			return nil, errorMainDex(fijasPresentes, fijasFuera, c, limite)
		}
		medido, err := mideClases(enCurso)
		if err != nil {
			return nil, err
		}
		grupos = append(grupos, grupoDex{clases: enCurso, tablas: medido})
		enCurso = []classFull{cf}
		if tablas, err = mideClases(enCurso); err != nil {
			return nil, err
		}
		if c := cuentaDe(tablas); c.mayor() > limite {
			return nil, fmt.Errorf("%s no cabe sola en un dex: %s, y el techo de los índices de 16 bits es %d",
				cf.Name, c, limite)
		}
	}
	if len(enCurso) > 0 {
		// las tablas de la última prueba son exactamente las de las clases que quedaron en el
		// grupo: cada clase que se añadió se internó en ellas y ningún cierre las tocó después
		grupos = append(grupos, grupoDex{clases: enCurso, tablas: tablas})
	}
	return grupos, nil
}

// esClaseFija dice si la clase, o la clase de la que nació una lambda sintética, está en la lista
// de fijas: una lambda del proyecto tiene que viajar con su dueña en el dex primario, y después de
// `expandirLambdas` la sintética se llama `Ldueña$$Lambda$N;` y vive al final de la lista.
func esClaseFija(nombre string, fijas map[string]bool) bool {
	if fijas[nombre] {
		return true
	}
	if i := strings.Index(nombre, "$$Lambda$"); i >= 0 {
		return fijas[nombre[:i]+";"]
	}
	return false
}

// errorMainDex es el rechazo del dex primario: las clases fijas no caben y el reparto no puede
// dejarlas en classes2.dex, porque por debajo de API 21 ese fichero no se lee sin el cargador del
// soporte de multidex. Nombra la cuenta, el techo y cuántas fijas se quedarían fuera.
func errorMainDex(fijas, fuera int, c cuentaDeReferencias, limite int) error {
	return fmt.Errorf("el dex primario (main-dex) no cabe: %d clases fijas piden %s, y el techo de los "+
		"índices de 16 bits son %d referencias por fichero; %d de ellas tendrían que ir en classes2.dex, "+
		"que el runtime de API < 21 no abre sin el soporte de multidex",
		fijas, c, limite, fuera)
}
